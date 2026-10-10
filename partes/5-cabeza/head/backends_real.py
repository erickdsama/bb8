"""Hardware real de la cabeza en la Pi Zero 2 W. Sin probar todavía (Parte 5).

- Cámara OV5647 con picamera2 (sudo apt install python3-picamera2); sin ella arranca igual
- ToF VL53L0X (el del pedido; pip install adafruit-circuitpython-vl53l0x) o VL53L1X, se detecta solo
- Anillo NeoPixel de 16 LED en GPIO18 (pip install adafruit-circuitpython-neopixel; requiere root)
- Altavoz: Piper para voz (si está instalado) y aplay para pitidos
"""
from __future__ import annotations

import io
import logging
import math
import shutil
import subprocess
import threading
import time

from .colors import MAX_BRIGHTNESS, parse_color
from .sounds import beep_wav

log = logging.getLogger("bb8.head")


class PiCamera:
    name = "picamera2"

    def __init__(self):
        from picamera2 import Picamera2

        self.cam = Picamera2()
        self.cam.configure(self.cam.create_still_configuration(main={"size": (1296, 972)}))
        self.cam.start()
        self._lock = threading.Lock()

    def pausar(self) -> None:
        """Reposo ligero: la cámara en idle consume ~150 mA."""
        with self._lock:
            self.cam.stop()

    def reanudar(self) -> None:
        with self._lock:
            self.cam.start()

    def capture_jpeg(self, width: int) -> bytes:
        from PIL import Image

        with self._lock:
            arr = self.cam.capture_array()
        img = Image.fromarray(arr).convert("RGB")
        img.thumbnail((width, width))
        buf = io.BytesIO()
        img.save(buf, "JPEG", quality=80)
        return buf.getvalue()


class SinCamara:
    """Sin OV5647 (o sin picamera2): la cabeza arranca igual; /foto y /cara responden 503."""
    name = "ninguna"

    def pausar(self) -> None:
        pass

    def reanudar(self) -> None:
        pass

    def capture_jpeg(self, width: int) -> bytes:
        raise RuntimeError("la cabeza no tiene cámara conectada")


def camara_auto():
    try:
        return PiCamera()
    except Exception as e:
        log.warning("Sin cámara (%s): /foto y /cara no estarán disponibles", e)
        return SinCamara()


class VL53L1X:
    name = "vl53l1x"

    def __init__(self):
        import adafruit_vl53l1x
        import board

        self.sensor = adafruit_vl53l1x.VL53L1X(board.I2C())
        self.sensor.distance_mode = 2  # largo, hasta 4 m
        self.sensor.timing_budget = 50
        self.sensor.start_ranging()
        self._mm = 0
        threading.Thread(target=self._loop, daemon=True).start()

    def _loop(self) -> None:
        while True:
            try:
                if self.sensor.data_ready:
                    cm = self.sensor.distance
                    self._mm = int(cm * 10) if cm else 0
                    self.sensor.clear_interrupt()
            except OSError as e:
                log.warning("VL53L1X: %s", e)
                self._mm = 0
            time.sleep(0.02)

    def read_mm(self) -> int:
        return self._mm


class VL53L0X:
    """El sensor del pedido 377466. Alcance ~1.2 m en interiores."""
    name = "vl53l0x"

    def __init__(self):
        import adafruit_vl53l0x
        import board

        self.sensor = adafruit_vl53l0x.VL53L0X(board.I2C())
        self.sensor.measurement_timing_budget = 33000
        self.sensor.start_continuous()
        self._mm = 0
        threading.Thread(target=self._loop, daemon=True).start()

    def _loop(self) -> None:
        while True:
            try:
                mm = self.sensor.range
                self._mm = mm if 0 < mm < 2000 else 0   # 8190 = nada a la vista
            except OSError as e:
                log.warning("VL53L0X: %s", e)
                self._mm = 0
            time.sleep(0.03)

    def read_mm(self) -> int:
        return self._mm


class SinToF:
    name = "ninguno"

    def read_mm(self) -> int:
        return 0


def tof_auto():
    """VL53L0X o VL53L1X, el que conteste en 0x29."""
    for cls in (VL53L0X, VL53L1X):
        try:
            return cls()
        except Exception as e:
            log.info("%s no disponible: %s", cls.__name__, e)
    log.warning("Sin ToF en la cabeza: el freno por obstáculo dependerá del Arduino")
    return SinToF()


class NeoPixelEye:
    def __init__(self, n: int = 16):
        import board
        import neopixel

        self.px = neopixel.NeoPixel(board.D18, n, brightness=1.0, auto_write=False)
        self.rgb, self.pattern, self.brightness = (40, 110, 255), "fijo", MAX_BRIGHTNESS
        threading.Thread(target=self._loop, daemon=True).start()

    def set(self, color: str, pattern: str, brightness: float) -> None:
        self.rgb = parse_color(color)
        self.pattern, self.brightness = pattern, min(brightness, MAX_BRIGHTNESS)

    def _loop(self) -> None:
        t0 = time.monotonic()
        while True:
            t = time.monotonic() - t0
            k = self.brightness
            if self.pattern == "apagado":
                k = 0
            elif self.pattern == "respirar":
                k *= 0.15 + 0.85 * (0.5 + 0.5 * math.sin(t * 2))
            elif self.pattern == "parpadeo":
                k *= 1 if int(t * 4) % 2 == 0 else 0
            self.px.fill(tuple(int(c * k) for c in self.rgb))
            self.px.show()
            time.sleep(0.03)


class Speaker:
    """Piper para el texto (si existe) y aplay para los pitidos."""

    def __init__(self, piper_model: str = "/opt/piper/es_ES-davefx-medium.onnx"):
        self.piper = shutil.which("piper")
        self.model = piper_model
        self._lock = threading.Lock()

    def say(self, text: str | None, sound: str | None) -> float:
        t0 = time.monotonic()
        with self._lock:
            wav, _ = beep_wav(sound or text or "")
            subprocess.run(["aplay", "-q", "-"], input=wav, check=False)
            if text and self.piper:
                p = subprocess.run([self.piper, "--model", self.model, "--output_file", "-"],
                                   input=text.encode(), capture_output=True, check=False)
                subprocess.run(["aplay", "-q", "-"], input=p.stdout, check=False)
            elif text:
                log.warning("Piper no está instalado; solo pitidos")
        return round(time.monotonic() - t0, 2)
