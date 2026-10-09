"""Servidor HTTP de la cabeza (PROTOCOLO.md, sección 3).

En la Pi Zero 2 W:
    python -m head.server            # hardware real, puerto 8080
En el PC lo arranca dummy.run_dummy con los backends simulados.
"""
from __future__ import annotations

import argparse
import asyncio
import logging
import time

from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse, Response
from starlette.routing import Route

from .colors import MAX_BRIGHTNESS, PATTERNS, parse_color

log = logging.getLogger("bb8.head")
SOUNDS = ("feliz", "triste", "alerta", "pregunta")


def build_app(camera, tof, eye, speaker, extra_routes: list[Route] | None = None) -> Starlette:
    async def estado(req: Request):
        return JSONResponse({"ok": True, "camara": camera.name, "tof": tof.name})

    async def foto(req: Request):
        width = max(64, min(1920, int(req.query_params.get("ancho", 640))))
        try:
            jpg = await asyncio.to_thread(camera.capture_jpeg, width)
        except Exception as e:
            log.exception("Foto")
            return JSONResponse({"ok": False, "error": str(e)}, status_code=500)
        return Response(jpg, media_type="image/jpeg")

    async def tof_ep(req: Request):
        return JSONResponse({"mm": int(tof.read_mm()), "t": time.time()})

    async def ojo(req: Request):
        d = await req.json()
        pattern = d.get("patron", "fijo")
        color = d.get("color", "azul")
        try:
            parse_color(color)
            if pattern not in PATTERNS:
                raise ValueError(f"patrón desconocido: {pattern}. Usa {', '.join(PATTERNS)}")
            brightness = max(0.0, min(MAX_BRIGHTNESS, float(d.get("brillo", MAX_BRIGHTNESS))))
        except ValueError as e:
            return JSONResponse({"ok": False, "error": str(e)}, status_code=400)
        eye.set(color, pattern, brightness)
        return JSONResponse({"ok": True})

    async def hablar(req: Request):
        d = await req.json()
        text, sound = d.get("texto"), d.get("sonido")
        if sound and sound not in SOUNDS:
            return JSONResponse({"ok": False, "error": f"sonido desconocido. Usa {', '.join(SOUNDS)}"}, 400)
        if not text and not sound:
            return JSONResponse({"ok": False, "error": "manda 'texto' o 'sonido'"}, 400)
        secs = await asyncio.to_thread(speaker.say, text, sound)
        return JSONResponse({"ok": True, "segundos": secs})

    async def cara(req: Request):
        """Caras en la foto actual: x de −1 (izquierda) a 1 (derecha), area relativa. Parte 5."""
        try:
            caras = await asyncio.to_thread(_detectar_caras, camera)
        except ImportError:
            return JSONResponse({"ok": False, "error": "falta OpenCV 4 (sudo apt install python3-opencv, o pip install 'opencv-python-headless<5')"}, 501)
        return JSONResponse({"ok": True, "caras": caras})

    async def reposo(req: Request):
        """{"activo": false} apaga ojo y cámara (reposo ligero); {"activo": true} los vuelve a encender."""
        d = await req.json()
        activo = bool(d.get("activo", True))
        if activo:
            if hasattr(camera, "reanudar"):
                await asyncio.to_thread(camera.reanudar)
            eye.set("azul", "fijo", MAX_BRIGHTNESS)
        else:
            if hasattr(camera, "pausar"):
                await asyncio.to_thread(camera.pausar)
            eye.set("azul", "respirar", 0.03)   # respiración tenue: se nota que duerme (<30 mA)
        return JSONResponse({"ok": True, "activo": activo})

    routes = [
        Route("/estado", estado), Route("/cara", cara), Route("/reposo", reposo, methods=["POST"]), Route("/foto", foto), Route("/tof", tof_ep),
        Route("/ojo", ojo, methods=["POST"]), Route("/hablar", hablar, methods=["POST"]),
        *(extra_routes or []),
    ]
    return Starlette(routes=routes)


_cascada = None


def _detectar_caras(camera) -> list[dict]:
    import cv2
    import numpy as np

    global _cascada
    if _cascada is None:
        if not hasattr(cv2, "CascadeClassifier"):   # OpenCV 5 sacó los Haar del paquete base
            raise ImportError("OpenCV 5 sin Haar")
        _cascada = cv2.CascadeClassifier(cv2.data.haarcascades + "haarcascade_frontalface_default.xml")
    jpg = camera.capture_jpeg(320)
    img = cv2.imdecode(np.frombuffer(jpg, np.uint8), cv2.IMREAD_GRAYSCALE)
    h, w = img.shape
    caras = _cascada.detectMultiScale(img, scaleFactor=1.15, minNeighbors=5, minSize=(24, 24))
    return sorted(({"x": round((x + cw / 2) / w * 2 - 1, 3), "y": round((y + ch / 2) / h * 2 - 1, 3),
                    "area": round(cw * ch / (w * h), 4)} for x, y, cw, ch in caras),
                  key=lambda c: -c["area"])


def main() -> None:
    import uvicorn

    from . import backends_real as R

    ap = argparse.ArgumentParser(description="Cabeza del BB-8 (Pi Zero 2 W)")
    ap.add_argument("--port", type=int, default=8080)
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
    app = build_app(R.PiCamera(), R.tof_auto(), R.NeoPixelEye(), R.Speaker())
    uvicorn.run(app, host="0.0.0.0", port=a.port, log_level="warning")


if __name__ == "__main__":
    main()
