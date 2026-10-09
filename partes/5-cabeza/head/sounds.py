"""Pitidos de droide generados con matemática pura (sin archivos WAV)."""
from __future__ import annotations

import io
import math
import random
import struct
import wave

RATE = 22050

# (frecuencia inicial, final, duración s) por sílaba
PATTERNS = {
    "feliz": [(900, 1500, 0.08), (1500, 2200, 0.07), (1200, 1900, 0.10)],
    "triste": [(900, 500, 0.25), (600, 350, 0.35)],
    "alerta": [(1800, 1800, 0.06), (0, 0, 0.04)] * 3,
    "pregunta": [(700, 700, 0.10), (800, 1600, 0.18)],
}


def _syllables(name: str) -> list[tuple[float, float, float]]:
    if name in PATTERNS:
        return PATTERNS[name]
    rnd = random.Random(name)  # mismo texto → mismos pitidos
    return [(rnd.uniform(600, 1800), rnd.uniform(600, 2200), rnd.uniform(0.04, 0.12))
            for _ in range(rnd.randint(2, 5))]


def beep_wav(name: str, volume: float = 0.4) -> tuple[bytes, float]:
    """WAV mono 16 bit. name es un sonido de PATTERNS o cualquier texto (pitido aleatorio)."""
    frames = bytearray()
    phase = 0.0
    total = 0.0
    for f0, f1, dur in _syllables(name):
        n = int(RATE * dur)
        for i in range(n):
            f = f0 + (f1 - f0) * i / max(1, n)
            phase += 2 * math.pi * f / RATE
            env = min(1.0, i / 200, (n - i) / 200)
            s = volume * env * (1 if f0 else 0) * (0.7 * math.sin(phase) + 0.3 * math.sin(2 * phase))
            frames += struct.pack("<h", int(s * 32767))
        total += dur
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(bytes(frames))
    return buf.getvalue(), total
