"""Transcripción local con faster-whisper (int8 en la CPU de la Pi)."""
from __future__ import annotations

import logging
import time

import numpy as np

from . import config as V

log = logging.getLogger("bb8.voz.stt")


class Transcriptor:
    def __init__(self, modelo: str = V.WHISPER):
        from faster_whisper import WhisperModel

        t0 = time.monotonic()
        self.modelo = WhisperModel(modelo, device="cpu", compute_type="int8", cpu_threads=V.WHISPER_HILOS)
        log.info("Whisper %s cargado en %.1f s", modelo, time.monotonic() - t0)

    def __call__(self, audio: np.ndarray) -> str:
        t0 = time.monotonic()
        x = audio.astype(np.float32) / 32768.0
        segmentos, _ = self.modelo.transcribe(
            x, language="es", beam_size=1, vad_filter=False, condition_on_previous_text=False,
            initial_prompt="BB-8, ven acá. Da una vuelta. Mira a la izquierda. Erick.")
        texto = " ".join(s.text.strip() for s in segmentos).strip()
        log.info("«%s» (%.1f s)", texto, time.monotonic() - t0)
        return texto
