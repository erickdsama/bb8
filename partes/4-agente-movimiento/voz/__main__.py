"""Agente de voz del BB-8 en la Pi principal.

    python -m voz                       # necesita ANTHROPIC_API_KEY, el MCP y el servicio de movimiento
    python -m voz --sin-voz-registrada  # obedece a cualquiera aunque haya voces registradas

Ciclo: wake word → graba hasta el silencio → Whisper → ¿quién habla? → Claude con
las herramientas MCP → Piper. "BB-8, a dormir" pasa a reposo profundo (Parte 6).
"""
from __future__ import annotations

import argparse
import asyncio
import logging
import re

import httpx

from . import config as V
from .agente import Agente
from .audio import Microfono
from .escucha import Escucha
from .hablantes import Hablantes
from .salida import Voz
from .stt import crear_transcriptor

log = logging.getLogger("bb8.voz")

A_DORMIR = re.compile(r"\b(a dormir|a mimir|duérmete|duermete|apágate|apagate)\b", re.I)


async def _energia(ruta: str) -> None:
    """Avisa al gestor de energía (Parte 6). Si no corre, no pasa nada."""
    try:
        async with httpx.AsyncClient(timeout=2) as cli:
            await cli.post(f"{V.ENERGIA_URL}{ruta}")
    except httpx.HTTPError:
        pass


async def bucle(sin_registro: bool) -> None:
    mic = Microfono()
    escucha = Escucha(mic)
    stt = crear_transcriptor()
    hablantes = Hablantes()
    voz = Voz(mic)
    agente = Agente()
    await agente.conectar()
    voz.ojo("azul", "respirar")
    voz.pitido("feliz")
    log.info("✅ BB-8 escuchando (wake word: %s)", V.WAKEWORD)
    try:
        while True:
            await asyncio.to_thread(escucha.esperar_wakeword)
            await _energia("/actividad")
            voz.ojo("cian")
            voz.pitido("pregunta")
            audio = await asyncio.to_thread(escucha.grabar_frase)
            if audio is None:
                voz.ojo("azul", "respirar")
                continue
            voz.ojo("naranja", "respirar")   # pensando
            tareas = [asyncio.to_thread(stt, audio), asyncio.to_thread(hablantes.identificar, audio)]
            if V.BUSCAR_CARA:
                from .cara import mirar_a_quien_habla
                tareas.append(mirar_a_quien_habla())
            res = await asyncio.gather(*tareas, return_exceptions=True)
            texto, ident = res[0], res[1]
            if isinstance(texto, Exception) or not texto:
                log.warning("Transcripción vacía o fallida: %s", texto)
                voz.pitido("triste")
                voz.ojo("azul", "respirar")
                continue
            quien = "" if sin_registro or isinstance(ident, Exception) else ident[0]
            if A_DORMIR.search(texto) and quien is not None:
                voz.decir("Bip. Buenas noches.")
                await _energia("/dormir")
                continue
            try:
                respuesta = await agente.turno(texto, quien)
            except Exception as e:
                log.exception("Claude")
                voz.ojo("rojo")
                respuesta = "Bip bip. No me llega la señal."
                if "connect" in str(e).lower():
                    await agente.cerrar()
                    agente = Agente()
                    await agente.conectar()
            log.info("BB-8: %s", respuesta)
            voz.ojo("verde")
            voz.decir(respuesta)
            voz.ojo("azul", "respirar")
    finally:
        await agente.cerrar()


def main() -> None:
    ap = argparse.ArgumentParser(description="Agente de voz del BB-8")
    ap.add_argument("--sin-voz-registrada", action="store_true", help="obedece a cualquier voz")
    ap.add_argument("-v", "--verbose", action="store_true")
    a = ap.parse_args()
    logging.basicConfig(level=logging.DEBUG if a.verbose else logging.INFO,
                        format="%(asctime)s %(name)s %(levelname)s %(message)s")
    for ruido in ("httpx", "mcp", "faster_whisper"):
        logging.getLogger(ruido).setLevel(logging.WARNING)
    asyncio.run(bucle(a.sin_voz_registrada))


if __name__ == "__main__":
    main()
