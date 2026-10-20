"""Agente de voz del BB-8 en la Pi principal.

    python -m voz                       # needs bb8-body running (agent.api_key set in the config page)
    python -m voz --sin-voz-registrada  # obedece a cualquiera aunque haya voces registradas

Ciclo: wake word → graba hasta el silencio → Whisper → ¿quién habla? → Claude con
el agente de bb8-body (OpenRouter, POST /agent/ask) → Piper. "BB-8, a dormir" pasa a reposo profundo (Parte 6).
"""
from __future__ import annotations

import argparse
import asyncio
import logging
import re
import time

import httpx

from head.personalidad import frase

from . import config as V
from .agente import Agente
from .audio import abrir_microfono
from .escucha import Escucha
from .hablantes import Hablantes
from .panel import Panel
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


async def _registrar(audio, texto: str, respuesta: str) -> None:
    """Keep the phrase, its transcript and the reply in the audio log (voice.audio_log)."""
    try:
        async with httpx.AsyncClient(timeout=3) as cli:
            await cli.post(f"{V.MOTION_URL}/audio/phrase",
                           params={"text": texto or "(nada)", "reply": respuesta or ""},
                           content=audio.astype("int16").tobytes())
    except httpx.HTTPError:
        pass


async def bucle(sin_registro: bool) -> None:
    mic = abrir_microfono()
    panel = Panel()
    escucha = Escucha(mic, panel)
    stt = crear_transcriptor()
    hablantes = Hablantes()
    voz = Voz(mic)
    agente = Agente()
    await agente.conectar()
    voz.ojo("white")
    voz.pitido("wake")
    voz.ojo("blue", "breathe")
    log.info("✅ BB-8 escuchando (wake word: %s)", V.WAKEWORD)
    seguimiento = 0   # >0: BB-8 just asked something, listen without the wake word
    try:
        while True:
            if seguimiento:
                log.info("Follow-up %d: listening without the wake word", seguimiento)
            else:
                panel.evento("state", state="waiting")
                await asyncio.to_thread(escucha.esperar_wakeword)
            await _energia("/actividad")
            # Cyan + the shortest beep (0.18 s) say "listening". The head mutes its
            # mic while it plays, so a long beep cut commands said in one go.
            voz.ojo("cyan")
            voz.pitido("yes")
            panel.evento("state", state="listening")
            espera = V.ESPERA_SEGUIMIENTO_S if seguimiento else V.ESPERA_VOZ_S
            audio = await asyncio.to_thread(escucha.grabar_frase, espera)
            if audio is None:
                seguimiento = 0
                voz.ojo("blue", "breathe")
                continue
            voz.ojo("orange", "breathe")   # pensando
            panel.evento("state", state="thinking")
            t_stt = time.monotonic()
            tareas = [asyncio.to_thread(stt, audio), asyncio.to_thread(hablantes.identificar, audio)]
            if V.BUSCAR_CARA:
                from .cara import mirar_a_quien_habla
                tareas.append(mirar_a_quien_habla())
            res = await asyncio.gather(*tareas, return_exceptions=True)
            texto, ident = res[0], res[1]
            panel.evento("heard", text=texto if isinstance(texto, str) else "",
                         seconds=round(len(audio) / V.TASA, 1), stt_s=round(time.monotonic() - t_stt, 2))
            if isinstance(texto, Exception) or not texto:
                await _registrar(audio, "", "")
                log.warning("Transcripción vacía o fallida: %s", texto)
                voz.pitido("sad")
                voz.ojo("blue", "breathe")
                seguimiento = 0
                continue
            quien = "" if sin_registro or isinstance(ident, Exception) else ident[0]
            if A_DORMIR.search(texto) and quien is not None:
                voz.pitido("yawn")
                voz.decir(frase("buenas_noches"))
                await _energia("/dormir")
                seguimiento = 0
                continue
            try:
                respuesta = await agente.turno(texto, quien)
            except Exception as e:
                log.exception("Agent")
                voz.ojo("red")
                voz.pitido("error")
                respuesta = frase("sin_senal")
                if "connect" in str(e).lower():
                    await agente.cerrar()
                    agente = Agente()
                    await agente.conectar()
            log.info("BB-8: %s", respuesta)
            panel.evento("reply", text=respuesta)
            await _registrar(audio, texto, respuesta)
            voz.ojo("green")
            panel.evento("state", state="speaking")
            voz.decir(respuesta)
            # A reply that asks something keeps the conversation open.
            pregunta = respuesta.rstrip().endswith("?")
            seguimiento = seguimiento + 1 if pregunta and seguimiento < V.SEGUIMIENTOS_MAX else 0
            if not seguimiento:
                voz.ojo("blue", "breathe")
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
