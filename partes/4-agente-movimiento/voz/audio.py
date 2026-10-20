"""Micrófono y altavoz.

El micrófono por defecto es el ReSpeaker de la cabeza: la Zero detecta la voz (VAD) y
manda solo ese audio por el enlace; bb8-body lo reparte en GET /audio/stream
(BB8_MIC=cabeza). Con BB8_MIC=local se usa un micrófono en la Pi con sounddevice
(PortAudio; en la Pi: sudo apt install libportaudio2).
"""
from __future__ import annotations

import io
import json
import logging
import queue
import struct
import threading
import time
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


class MicrofonoRed:
    """El micrófono de la cabeza a través de bb8-body. Misma interfaz que Microfono.

    Formato de GET /audio/stream: [u32 BE largo][JSON], y tras cada {"type":"audio"} otro
    [u32 BE largo][PCM s16le mono 16 kHz]. Entre frases la cabeza no manda nada; aquí se
    entregan bloques de silencio a ritmo real para que la wake word y el fin de frase
    funcionen igual que con un micrófono local.
    """

    def __init__(self, url: str = V.MOTION_URL):
        self.url = url.rstrip("/") + "/audio/stream"
        self._q: queue.Queue[np.ndarray] = queue.Queue(maxsize=400)
        self.silenciado = threading.Event()
        self._activa = threading.Event()     # hay una frase llegando
        self._resto = np.zeros(0, dtype=np.int16)
        # True when the last block was filler: the head's own VAD sent nothing,
        # i.e. nobody is speaking. Escucha uses it to end a phrase.
        self.relleno = True
        threading.Thread(target=self._leer, name="mic-red", daemon=True).start()
        log.info("Micrófono: la cabeza, vía %s", self.url)

    def _leer(self) -> None:
        import httpx

        while True:
            try:
                with httpx.stream("GET", self.url, timeout=httpx.Timeout(10, read=30)) as r:
                    r.raise_for_status()
                    buf = bytearray()
                    pcm_pendiente = False
                    for trozo in r.iter_bytes():
                        buf += trozo
                        while len(buf) >= 4:
                            n = struct.unpack(">I", buf[:4])[0]
                            if len(buf) < 4 + n:
                                break
                            datos, buf = bytes(buf[4:4 + n]), buf[4 + n:]
                            if pcm_pendiente:
                                pcm_pendiente = False
                                self._audio(np.frombuffer(datos, dtype=np.int16))
                                continue
                            f = json.loads(datos)
                            if f["type"] == "audio":
                                pcm_pendiente = True
                            elif f["type"] == "audio_start":
                                self._activa.set()
                            elif f["type"] == "audio_end":
                                self._audio(np.zeros(0, dtype=np.int16), fin=True)
                                self._activa.clear()
            except Exception as e:
                self._activa.clear()
                log.warning("Sin audio de la cabeza (%s); reintento en 3 s", e)
                time.sleep(3)

    def _audio(self, m: np.ndarray, fin: bool = False) -> None:
        if self.silenciado.is_set():
            self._resto = np.zeros(0, dtype=np.int16)
            return
        m = np.concatenate([self._resto, m])
        if fin and len(m) % V.BLOQUE:
            m = np.concatenate([m, np.zeros(V.BLOQUE - len(m) % V.BLOQUE, dtype=np.int16)])
        n = len(m) // V.BLOQUE * V.BLOQUE
        for i in range(0, n, V.BLOQUE):
            try:
                self._q.put_nowait(m[i:i + V.BLOQUE])
            except queue.Full:
                pass
        self._resto = m[n:]

    def bloque(self, timeout: float | None = None) -> np.ndarray:
        # Poll in 80 ms steps: a phrase can start or the link can drop while we wait,
        # and the old single get() raised Empty when audio_start raced with it.
        limite = None if timeout is None else time.monotonic() + timeout
        while True:
            activa = self._activa.is_set()
            try:
                b = self._q.get(timeout=V.BLOQUE / V.TASA)
                self.relleno = False
                return b
            except queue.Empty:
                if not (activa or self._activa.is_set()):
                    self.relleno = True
                    return np.zeros(V.BLOQUE, dtype=np.int16)   # nadie habla: silencio a ritmo real
                if limite is not None and time.monotonic() >= limite:
                    raise

    def vaciar(self) -> None:
        while not self._q.empty():
            self._q.get_nowait()


def abrir_microfono():
    """El de la cabeza (BB8_MIC=cabeza, por defecto) o uno local (BB8_MIC=local)."""
    return MicrofonoRed() if V.MIC == "cabeza" else Microfono()


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
