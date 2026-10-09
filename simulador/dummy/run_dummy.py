"""Robot dummy: Arduino simulado (TCP 5555) + cabeza simulada (HTTP 8080).

    python -m dummy.run_dummy                    # webcam 0, o vista sintética si no hay
    python -m dummy.run_dummy --camara sintetica # ve la habitación simulada
    python -m dummy.run_dummy --camara 1         # otra webcam
"""
from __future__ import annotations

import argparse
import logging

import uvicorn
from starlette.requests import Request
from starlette.responses import JSONResponse
from starlette.routing import Route

from head import backends_sim as HS
from head.server import build_app

from .arduino_sim import ArduinoSim, serve
from .world import World

log = logging.getLogger("dummy")


def main() -> None:
    ap = argparse.ArgumentParser(description="BB-8 dummy: Arduino y Pi Zero simulados")
    ap.add_argument("--camara", default="webcam", help="webcam | sintetica | índice de cámara (0, 1…)")
    ap.add_argument("--puerto-serial", type=int, default=5555)
    ap.add_argument("--puerto-cabeza", type=int, default=8080)
    ap.add_argument("--sin-sonido", action="store_true", help="no reproducir pitidos en el PC")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")

    world = World()
    arduino = ArduinoSim(world)
    serve(arduino, port=a.puerto_serial)

    head_deg = lambda: arduino.head  # noqa: E731
    eye = HS.SimEye()
    camera = None
    if a.camara != "sintetica":
        idx = 0 if a.camara == "webcam" else int(a.camara)
        try:
            camera = HS.WebcamCamera(idx)
            log.info("Visión: webcam %d del PC", idx)
        except Exception as e:
            log.warning("%s; uso la vista sintética", e)
    if camera is None:
        camera = HS.SyntheticCamera(world, head_deg, eye)
        log.info("Visión: vista sintética de la habitación simulada")

    async def empujar(req: Request):
        d = await req.json()
        world.push(float(d.get("grados", 40)), float(d.get("segundos", 1.0)))
        return JSONResponse({"ok": True})

    async def mundo(req: Request):
        return JSONResponse(world.snapshot() | {"cabeza_deg": round(arduino.head, 1)})

    app = build_app(camera, HS.SimToF(world, head_deg), eye, HS.SimSpeaker(play=not a.sin_sonido),
                    extra_routes=[Route("/sim/empujar", empujar, methods=["POST"]), Route("/sim/mundo", mundo)])
    log.info("Cabeza simulada en http://127.0.0.1:%d", a.puerto_cabeza)
    uvicorn.run(app, host="127.0.0.1", port=a.puerto_cabeza, log_level="warning")


if __name__ == "__main__":
    main()
