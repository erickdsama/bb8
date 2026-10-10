"""Cabeza simulada para el PC: webcam (o vista sintética), ToF del mundo, ojo y altavoz falsos."""
from __future__ import annotations

import logging
import math
import os
import platform
import subprocess
import sys
import tempfile
import threading
import time
from typing import Callable

import cv2
import numpy as np

from .colors import parse_color
from .sounds import beep_wav

log = logging.getLogger("dummy.head")

FOV_DEG = 54.0  # OV5647 horizontal


def encode_jpeg(frame: np.ndarray, width: int) -> bytes:
    h, w = frame.shape[:2]
    scale = width / max(h, w)
    if scale < 1:
        frame = cv2.resize(frame, (int(w * scale), int(h * scale)), interpolation=cv2.INTER_AREA)
    ok, buf = cv2.imencode(".jpg", frame, [cv2.IMWRITE_JPEG_QUALITY, 80])
    if not ok:
        raise RuntimeError("No se pudo codificar JPEG")
    return buf.tobytes()


class WebcamCamera:
    """Lee la webcam del PC en un hilo; /foto devuelve el último cuadro."""

    name = "webcam"

    def __init__(self, index: int = 0):
        backend = cv2.CAP_DSHOW if sys.platform == "win32" else cv2.CAP_ANY
        self.cap = cv2.VideoCapture(index, backend)
        if not self.cap.isOpened():
            raise RuntimeError(f"No se pudo abrir la cámara {index}")
        self.cap.set(cv2.CAP_PROP_FRAME_WIDTH, 1280)
        self.cap.set(cv2.CAP_PROP_FRAME_HEIGHT, 720)
        ok, frame = self.cap.read()
        if not ok:
            raise RuntimeError(f"La cámara {index} no entrega imagen")
        self._frame = frame
        self._lock = threading.Lock()
        threading.Thread(target=self._grab, name="webcam", daemon=True).start()

    def _grab(self) -> None:
        while True:
            ok, frame = self.cap.read()
            if ok:
                with self._lock:
                    self._frame = frame
            else:
                time.sleep(0.1)

    def capture_jpeg(self, width: int) -> bytes:
        with self._lock:
            frame = self._frame.copy()
        return encode_jpeg(frame, width)


class SyntheticCamera:
    """Vista en primera persona de la habitación simulada, por raycast de columnas."""

    name = "sintetica"

    def __init__(self, world, head_deg: Callable[[], float], eye: "SimEye"):
        self.world, self.head_deg, self.eye = world, head_deg, eye

    def capture_jpeg(self, width: int) -> bytes:
        W, H = 640, 480
        img = np.zeros((H, W, 3), np.uint8)
        img[: H // 2] = (95, 92, 90)      # pared detrás de los muebles
        img[H // 2:] = (120, 135, 150)    # piso
        with self.world.lock:
            theta = self.world.theta - math.radians(self.head_deg())
            x, y = self.world.x, self.world.y
        cols = 160
        cw = W // cols
        f = (W / 2) / math.tan(math.radians(FOV_DEG / 2))  # distancia focal en px
        cam_h = 0.35  # altura de la cámara sobre el piso
        for c in range(cols):
            off = math.radians((c + 0.5) / cols * FOV_DEG - FOV_DEG / 2)
            d, hit = self.world.raycast(theta - off, x, y)
            d = max(0.05, d * math.cos(off))  # sin ojo de pez
            obj_h, base = (2.4, (200, 190, 180)) if hit is None else (hit.height, hit.color)
            shade = max(0.35, 1 - d / 5)
            color = tuple(int(v * shade) for v in base)
            top = int(H / 2 - f * (obj_h - cam_h) / d)
            bottom = int(H / 2 + f * cam_h / d)
            cv2.rectangle(img, (c * cw, max(0, top)), ((c + 1) * cw - 1, min(H - 1, bottom)), color, -1)
        r, g, b = self.eye.rgb
        cv2.circle(img, (W - 24, 24), 12, (b, g, r) if (r, g, b) != (0, 0, 0) else (40, 40, 40), -1)
        cv2.putText(img, f"camara simulada  x={x:.2f} y={y:.2f} cabeza={self.head_deg():+.0f}",
                    (10, H - 12), cv2.FONT_HERSHEY_SIMPLEX, 0.5, (255, 255, 255), 1, cv2.LINE_AA)
        return encode_jpeg(img, width)


class SimToF:
    name = "sim"

    def __init__(self, world, head_deg: Callable[[], float]):
        self.world, self.head_deg = world, head_deg

    def read_mm(self) -> int:
        return self.world.tof_mm(self.head_deg())


class SimEye:
    def __init__(self):
        self.rgb = (40, 110, 255)
        self.pattern = "fijo"
        self.brightness = 0.3

    def set(self, color: str, pattern: str, brightness: float) -> None:
        self.rgb = (0, 0, 0) if pattern == "apagado" else parse_color(color)
        self.pattern, self.brightness = pattern, brightness
        log.info("👁  Ojo %s %s al %d %%", color, pattern, brightness * 100)


class SimSpeaker:
    """Imprime lo que dice y, si se puede, reproduce los pitidos en el PC."""

    def __init__(self, play: bool = True):
        self.play = play and os.environ.get("BB8_SIN_SONIDO") is None
        self.ultimo: dict | None = None  # lo último que dijo, para el visor del simulador

    def say(self, text: str | None, sound: str | None) -> float:
        wav, secs = beep_wav(sound or text or "")
        if text:
            log.info("🔊 BB-8 dice: %s", text)
            print(f"\n   🤖 BB-8: «{text}»\n", flush=True)
            secs += 0.06 * len(text)  # tiempo aproximado de Piper
        else:
            log.info("🔊 Pitido: %s", sound)
            print(f"\n   🤖 BB-8: *pitido {sound}*\n", flush=True)
        self.ultimo = {"texto": text, "sonido": sound, "t": time.time(), "segundos": round(secs, 2)}
        if self.play:
            threading.Thread(target=self._play, args=(wav,), daemon=True).start()
        return round(secs, 2)

    @staticmethod
    def _play(wav: bytes) -> None:
        try:
            if sys.platform == "win32":
                import winsound
                winsound.PlaySound(wav, winsound.SND_MEMORY)
                return
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
                f.write(wav)
            player = ["afplay", f.name] if platform.system() == "Darwin" else ["aplay", "-q", f.name]
            subprocess.run(player, check=False, timeout=5,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            os.unlink(f.name)
        except Exception:
            pass
