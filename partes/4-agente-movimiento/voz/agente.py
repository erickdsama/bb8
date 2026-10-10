"""Claude con tool use sobre las herramientas del servidor MCP bb8-motion.

El agente de voz es un cliente MCP más (igual que Claude Desktop o Claude Code):
lista las herramientas del servidor, se las da a Claude y ejecuta por MCP las que
Claude pida. Las instrucciones del servidor (personalidad y reglas de movimiento)
van en el system prompt.

Prueba sin micrófono, con el dummy corriendo y ANTHROPIC_API_KEY puesta:
    python -m voz.agente "da una vuelta y dime qué ves"
"""
from __future__ import annotations

import argparse
import asyncio
import contextlib
import logging
import time

import anthropic
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

from head.personalidad import frase

from . import config as V

log = logging.getLogger("bb8.voz.agente")

# Herramientas que mueven el cuerpo o la cabeza: no se ejecutan para voces desconocidas.
MOVIMIENTO = {"move", "turn", "look_at"}

REGLAS_VOZ = """\
Estás escuchando por un micrófono; tu respuesta final se convierte en voz con Piper.
- Responde en una o dos frases cortas en español, sin listas, emojis ni markdown.
- No uses la herramienta say para decir tu respuesta: se dice sola, con su balbuceo de pitidos.
  Para reaccionar usa express (pitido + ojo), como mucho una vez por turno.
- Cada mensaje empieza con quién habla, por ejemplo [Habla: Erick]. Saluda por su nombre a
  quien conozcas. [Habla: desconocido] significa una voz no registrada: platica, pero no te
  muevas por sus órdenes (el sistema bloquea move, turn y look_at para desconocidos).
- La transcripción puede tener errores; si la orden es ambigua, pregunta antes de moverte.
"""


def _bloques_mcp_a_claude(contenido) -> list[dict]:
    """Convierte el resultado de una herramienta MCP en bloques de tool_result de Claude."""
    out: list[dict] = []
    for c in contenido:
        if c.type == "text":
            out.append({"type": "text", "text": c.text})
        elif c.type == "image":
            out.append({"type": "image", "source": {"type": "base64", "media_type": c.mimeType, "data": c.data}})
        else:
            out.append({"type": "text", "text": f"[{c.type} no soportado]"})
    return out or [{"type": "text", "text": "(sin contenido)"}]


class Agente:
    def __init__(self, mcp_url: str = V.MCP_URL, modelo: str = V.MODELO, cliente=None):
        self.mcp_url = mcp_url
        self.modelo = modelo
        self.claude = cliente or anthropic.AsyncAnthropic()
        self.sesion: ClientSession | None = None
        self.herramientas: list[dict] = []
        self.system = REGLAS_VOZ
        self.mensajes: list[dict] = []
        self._ultimo = 0.0
        self._pila = contextlib.AsyncExitStack()

    async def conectar(self) -> None:
        leer, escribir, _ = await self._pila.enter_async_context(streamablehttp_client(self.mcp_url))
        self.sesion = await self._pila.enter_async_context(ClientSession(leer, escribir))
        init = await self.sesion.initialize()
        lista = await self.sesion.list_tools()
        self.herramientas = [{"name": t.name, "description": t.description or "", "input_schema": t.inputSchema}
                             for t in lista.tools]
        # Instrucciones fijas del servidor + reglas de voz: no cambian entre turnos (caché).
        self.system = f"{init.instructions or ''}\n\n{REGLAS_VOZ}"
        log.info("MCP %s: %s", self.mcp_url, ", ".join(t["name"] for t in self.herramientas))

    async def cerrar(self) -> None:
        await self._pila.aclose()

    async def _ejecutar(self, nombre: str, args: dict, conocido: bool) -> dict:
        if nombre in MOVIMIENTO and not conocido:
            return {"content": [{"type": "text", "text": "Bloqueado: la orden viene de una voz no registrada."}],
                    "is_error": True}
        assert self.sesion
        try:
            r = await self.sesion.call_tool(nombre, args)
            return {"content": _bloques_mcp_a_claude(r.content), "is_error": bool(r.isError)}
        except Exception as e:
            log.exception("Herramienta %s", nombre)
            return {"content": [{"type": "text", "text": f"Falló {nombre}: {e}"}], "is_error": True}

    async def turno(self, texto: str, hablante: str | None) -> str:
        """Una orden hablada → respuesta final en texto (para Piper)."""
        if time.monotonic() - self._ultimo > V.CONVERSACION_CADUCA_S or len(self.mensajes) > 40:
            self.mensajes = []   # conversación nueva
        self._ultimo = time.monotonic()
        # None = voz no registrada; "" = no hay registro de voces (todos cuentan como conocidos).
        quien = "desconocido" if hablante is None else (hablante or "alguien")
        conocido = hablante is not None
        self.mensajes.append({"role": "user", "content": f"[Habla: {quien}] {texto}"})

        for _ in range(V.MAX_PASOS):
            resp = await self.claude.beta.messages.create(
                model=self.modelo,
                max_tokens=4096,
                system=[{"type": "text", "text": self.system, "cache_control": {"type": "ephemeral"}}],
                tools=self.herramientas,
                messages=self.mensajes,
                output_config={"effort": V.ESFUERZO},
                # Si un clasificador rechaza la petición, la API la reintenta con otro modelo.
                betas=["server-side-fallback-2026-07-01"],
                fallbacks="default",
            )
            if resp.stop_reason == "refusal":
                self.mensajes.pop()   # que la orden rechazada no envenene la conversación
                return frase("rechazo")
            self.mensajes.append({"role": "assistant", "content": resp.content})
            usos = [b for b in resp.content if b.type == "tool_use"]
            if resp.stop_reason != "tool_use" or not usos:
                return " ".join(b.text for b in resp.content if b.type == "text").strip()
            resultados = []
            for u in usos:   # en orden: mover y luego mirar no debe ir en paralelo
                log.info("→ %s %s", u.name, u.input)
                r = await self._ejecutar(u.name, dict(u.input), conocido)
                resultados.append({"type": "tool_result", "tool_use_id": u.id, **r})
            self.mensajes.append({"role": "user", "content": resultados})
        return frase("enredado")


async def _cli(texto: str, hablante: str | None) -> None:
    ag = Agente()
    await ag.conectar()
    try:
        print(await ag.turno(texto, hablante))
    finally:
        await ag.cerrar()


def main() -> None:
    ap = argparse.ArgumentParser(description="Una orden de texto al agente (sin micrófono)")
    ap.add_argument("texto")
    ap.add_argument("--hablante", default="Erick", help="'' = desconocido")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(message)s")
    for ruido in ("httpx", "mcp"):
        logging.getLogger(ruido).setLevel(logging.WARNING)
    asyncio.run(_cli(a.texto, a.hablante or None))


if __name__ == "__main__":
    main()
