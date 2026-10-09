"""Micrófono y altavoz con sounddevice (PortAudio). En la Pi: sudo apt install libportaudio2."""
from __future__ import annotations

import io
import logging
import queue
import threading
import wave

import numpy as np

from . import config as V

log = logging.getLogger("bb8.voz.audio")


class Microfono:
    """Entrega bloques de 80 ms (int16, 16 kHz). Mientras BB-8 habla se descarta lo que oye:
    sin cancelación de eco se oiría a sí mismo y dispararía su propia wake word."""

    def __init__(self, dispositivo: str | int | None = V.MICROFONO):
        import sounddevice as sd

        self._q: queue.Queue[np.ndarray] = queue.Queue(maxsize=200)
        self.silenciado = threading.Event()
        if isinstance(dispositivo, str) and dispositivo.isdigit():
            dispositivo = int(dispositivo)
        self._stream = sd.InputStream(samplerate=V.TASA, blocksize=V.BLOQUE, channels=1,
                                      dtype="int16", device=dispositivo, callback=self._cb)
        self._stream.start()
        log.info("Micrófono abierto: %s", self._stream.device)

    def _cb(self, data, frames, t, status) -> None:
        if status:
            log.debug("audio: %s", status)
        if self.silenciado.is_set():
            return
        try:
            self._q.put_nowait(data[:, 0].copy())
        except queue.Full:
            pass

    def bloque(self, timeout: float | None = None) -> np.ndarray:
        return self._q.get(timeout=timeout)

    def vaciar(self) -> None:
        while not self._q.empty():
            self._q.get_nowait()


def wav_a_numpy(wav: bytes) -> tuple[np.ndarray, int]:
    with wave.open(io.BytesIO(wav)) as w:
        return np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16), w.getframerate()


def reproducir(muestras: np.ndarray, tasa: int, mic: Microfono | None = None) -> None:
    import sounddevice as sd

    if mic:
        mic.silenciado.set()
    try:
        sd.play(muestras, tasa, blocking=True)
    finally:
        if mic:
            sd.sleep(150)          # cola del eco en la habitación
            mic.vaciar()
            mic.silenciado.clear()
