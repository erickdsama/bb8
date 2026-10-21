"""Transcripción: faster-whisper local (int8 en la CPU de la Pi) o una API en la nube.

En una Pi 3 (1 GB, 4× Cortex-A53) Whisper `base` local tarda varios segundos por frase.
Con BB8_STT=nube el audio va a una API compatible con OpenAI (Groq, OpenAI o un
servidor whisper.cpp en tu PC) y la Pi se ahorra ~200 MB de RAM y la espera.
"""
from __future__ import annotations

import io
import json
import logging
import socket
import time
import wave

import numpy as np

from . import config as V

log = logging.getLogger("bb8.voz.stt")

PISTA = "BB-8, ven acá. Da una vuelta. Mira a la izquierda. Erick."


class TranscriptorLocal:
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
            initial_prompt=PISTA)
        texto = " ".join(s.text.strip() for s in segmentos).strip()
        log.info("«%s» (%.1f s)", texto, time.monotonic() - t0)
        return texto


class TranscriptorNube:
    """POST /audio/transcriptions al estilo OpenAI: multipart con el WAV, model y language."""

    def __init__(self):
        import httpx

        if not V.STT_URL:
            raise SystemExit("BB8_STT=nube necesita BB8_STT_URL")
        cabeceras = {"Authorization": f"Bearer {V.STT_API_KEY}"} if V.STT_API_KEY else {}
        self._http = httpx.Client(timeout=15, headers=cabeceras)
        log.info("Transcripción en la nube: %s (%s)", V.STT_URL, V.STT_MODELO)

    def __call__(self, audio: np.ndarray) -> str:
        t0 = time.monotonic()
        buf = io.BytesIO()
        with wave.open(buf, "wb") as w:
            w.setnchannels(1)
            w.setsampwidth(2)
            w.setframerate(V.TASA)
            w.writeframes(audio.astype(np.int16).tobytes())
        r = self._http.post(
            V.STT_URL,
            files={"file": ("frase.wav", buf.getvalue(), "audio/wav")},
            data={"model": V.STT_MODELO, "language": "es", "prompt": PISTA, "response_format": "json"})
        r.raise_for_status()
        texto = r.json().get("text", "").strip()
        log.info("«%s» (%.1f s, nube)", texto, time.monotonic() - t0)
        return texto


class TranscriptorWyoming:
    """Wyoming protocol over TCP (rhasspy wyoming-vosk / wyoming-whisper).

    Each event is a JSON line; audio-chunk is followed by its PCM payload. Replies may
    carry their data inline or as a separate block (data_length).
    """

    def __init__(self, destino: str = V.STT_WYOMING):
        if not destino:
            raise SystemExit("BB8_STT=wyoming needs BB8_STT_WYOMING=host:port")
        host, _, puerto = destino.rpartition(":")
        self.host, self.puerto = host, int(puerto)
        log.info("Transcription on the Wyoming server %s:%d", self.host, self.puerto)

    @staticmethod
    def _evento(tipo: str, datos: dict, carga: bytes = b"") -> bytes:
        cab = {"type": tipo, "data": datos}
        if carga:
            cab["payload_length"] = len(carga)
        return (json.dumps(cab) + "\n").encode() + carga

    def __call__(self, audio: np.ndarray) -> str:
        t0 = time.monotonic()
        fmt = {"rate": V.TASA, "width": 2, "channels": 1}
        x = audio.astype(np.float32)
        pico = float(np.abs(x).max()) if len(x) else 0.0
        if 0 < pico < 16000:   # quiet head mic: bring the peak to about -6 dBFS
            x *= min(16000 / pico, 8.0)
        pcm = x.astype(np.int16).tobytes()
        with socket.create_connection((self.host, self.puerto), timeout=15) as s:
            f = s.makefile("rwb")
            f.write(self._evento("transcribe", {"language": "es"}))
            f.write(self._evento("audio-start", fmt))
            paso = V.BLOQUE * 2
            for i in range(0, len(pcm), paso):
                f.write(self._evento("audio-chunk", fmt, pcm[i:i + paso]))
            f.write(self._evento("audio-stop", {}))
            f.flush()
            while linea := f.readline():
                cab = json.loads(linea)
                datos = cab.get("data") or {}
                if n := cab.get("data_length"):
                    datos.update(json.loads(f.read(n)))
                if n := cab.get("payload_length"):
                    f.read(n)
                if cab.get("type") == "transcript":
                    texto = (datos.get("text") or "").strip()
                    log.info("«%s» (%.2f s, wyoming)", texto, time.monotonic() - t0)
                    return texto
        raise ConnectionError("Wyoming server closed without a transcript")


def crear_transcriptor():
    if V.STT == "wyoming":
        return TranscriptorWyoming()
    if V.STT == "nube":
        return TranscriptorNube()
    if V.STT != "local":
        raise SystemExit(f"BB8_STT={V.STT!r}: use local, nube or wyoming")
    return TranscriptorLocal()

