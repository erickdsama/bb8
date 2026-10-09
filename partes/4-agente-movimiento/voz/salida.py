"""Voz de BB-8: pitidos + Piper. En la Pi (Parte 4) o en la cabeza (Parte 5)."""
from __future__ import annotations

import logging
from pathlib import Path

import httpx
import numpy as np

from head.sounds import beep_wav

from . import config as V
from .audio import Microfono, reproducir, wav_a_numpy

log = logging.getLogger("bb8.voz.salida")


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
        self._http = httpx.Client(base_url=V.HEAD_URL, timeout=20)

    def pitido(self, sonido: str) -> None:
        if V.SALIDA == "cabeza":
            self._post("/hablar", {"sonido": sonido})
            return
        m, tasa = wav_a_numpy(beep_wav(sonido)[0])
        reproducir(m, tasa, self.mic)

    def decir(self, texto: str) -> None:
        if not texto:
            return
        if V.SALIDA == "cabeza":
            self._post("/hablar", {"texto": texto})   # la cabeza ya pita antes de hablar
            return
        partes = [wav_a_numpy(beep_wav(texto)[0])]
        if self.piper:
            trozos = list(self.piper.synthesize(texto))
            if trozos:
                pcm = np.frombuffer(b"".join(t.audio_int16_bytes for t in trozos), dtype=np.int16)
                partes.append((pcm, trozos[0].sample_rate))
        for m, tasa in partes:
            reproducir(m, tasa, self.mic)

    def ojo(self, color: str, patron: str = "fijo") -> None:
        """Mejor esfuerzo: en la Parte 4 todavía no hay cabeza."""
        self._post("/ojo", {"color": color, "patron": patron}, quiet=True)

    def _post(self, ruta: str, cuerpo: dict, quiet: bool = False) -> None:
        if self.mic and ruta == "/hablar":
            self.mic.silenciado.set()
        try:
            self._http.post(ruta, json=cuerpo)
        except httpx.HTTPError as e:
            if not quiet:
                log.warning("Cabeza %s: %s", ruta, e)
        finally:
            if self.mic and ruta == "/hablar":
                self.mic.vaciar()
                self.mic.silenciado.clear()
