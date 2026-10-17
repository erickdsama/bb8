"""BB-8's voice: beeps + Piper on the Pi (part 4), or the head speaker through the body (part 5).

The head is only reachable through bb8-body (secure link), so eye and head speech
go to the local API: POST /tools/say and /tools/set_eye_color.
"""
from __future__ import annotations

import logging
from pathlib import Path

import httpx
import numpy as np

from head.sounds import beep_wav

from . import config as V
from .audio import Microfono, reproducir, wav_a_numpy

log = logging.getLogger("bb8.voz.salida")

# English emotion → Spanish name of the local beep table (head/sounds.py).
SONIDO_ES = {
    "happy": "feliz", "excited": "emocionado", "greeting": "saludo", "curious": "curioso",
    "question": "pregunta", "thinking": "pensando", "yes": "si", "no": "no", "alert": "alerta",
    "scared": "asustado", "sad": "triste", "error": "error", "yawn": "bostezo", "wake": "despertar",
    "laugh": "risa",
}


class Voz:
    def __init__(self, mic: Microfono | None = None):
        self.mic = mic
        self.piper = None
        if V.SALIDA == "local":
            if Path(V.PIPER_MODELO).exists():
                from piper import PiperVoice

                self.piper = PiperVoice.load(V.PIPER_MODELO)
            else:
                log.warning("No está %s: solo pitidos. Ver README, Parte 4.", V.PIPER_MODELO)
        self._http = httpx.Client(base_url=V.MOTION_URL, timeout=30)

    def pitido(self, sonido: str) -> None:
        """sonido: an English emotion name (happy, curious…)."""
        if V.SALIDA == "cabeza":
            self._tool("say", {"sound": sonido})
            return
        m, tasa = wav_a_numpy(beep_wav(SONIDO_ES.get(sonido, sonido))[0])
        reproducir(m, tasa, self.mic)

    def decir(self, texto: str) -> None:
        if not texto:
            return
        if V.SALIDA == "cabeza":
            self._tool("say", {"text": texto})   # the head beeps before speaking
            return
        partes = [wav_a_numpy(beep_wav(texto)[0])]
        if self.piper:
            trozos = list(self.piper.synthesize(texto))
            if trozos:
                pcm = np.frombuffer(b"".join(t.audio_int16_bytes for t in trozos), dtype=np.int16)
                partes.append((pcm, trozos[0].sample_rate))
        for m, tasa in partes:
            reproducir(m, tasa, self.mic)

    def ojo(self, color: str, patron: str = "steady") -> None:
        """Best effort: in part 4 there is no head yet. color/patron in English."""
        self._tool("set_eye_color", {"color": color, "pattern": patron}, quiet=True)

    def _tool(self, nombre: str, args: dict, quiet: bool = False) -> None:
        habla = nombre == "say"
        if self.mic and habla:
            self.mic.silenciado.set()
        try:
            r = self._http.post(f"/tools/{nombre}", json=args).json()
            if r.get("result") != "done" and not quiet:
                log.warning("Head %s: %s", nombre, r.get("reason"))
        except httpx.HTTPError as e:
            if not quiet:
                log.warning("Head %s: %s", nombre, e)
        finally:
            if self.mic and habla:
                self.mic.vaciar()
                self.mic.silenciado.clear()
