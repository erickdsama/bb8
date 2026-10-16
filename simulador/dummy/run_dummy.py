"""Robot dummy: Arduino simulado (TCP 5555) + cabeza simulada (HTTP 8080).

    python -m dummy.run_dummy                    # webcam 0, o vista sintética si no hay
    python -m dummy.run_dummy --camara sintetica # ve la habitación simulada
    python -m dummy.run_dummy --camara 1         # otra webcam
    python -m dummy.run_dummy --camara ninguna   # cabeza sin cámara (/foto responde 503)
    python -m dummy.run_dummy --mundo casa       # casa de 8×6 m con una persona
    python -m dummy.run_dummy --fov 120          # cámara sintética gran angular
"""
from __future__ import annotations

import argparse
import json
import logging
import os

import uvicorn

from head import backends_sim as HS
from head.server import build_app

from . import visor
from .arduino_sim import ArduinoSim, serve
from .oido import OidoSim
from .world import MUNDOS, World

log = logging.getLogger("dummy")


def main() -> None:
    ap = argparse.ArgumentParser(description="BB-8 dummy: Arduino y Pi Zero simulados")
    ap.add_argument("--camara", default="webcam", help="webcam | sintetica | ninguna | índice de cámara (0, 1…)")
    ap.add_argument("--puerto-serial", type=int, default=5555)
    ap.add_argument("--puerto-cabeza", type=int, default=8080)
    ap.add_argument("--sin-sonido", action="store_true", help="no reproducir pitidos en el PC")
    ap.add_argument("--fov", type=float, default=HS.FOV_DEG,
                    help="apertura horizontal de la cámara sintética en grados (54 = OV5647 normal, 120 = gran angular)")
    ap.add_argument("--mundo", default="habitacion", choices=sorted(MUNDOS),
                    help="habitacion (4×3 m, la de dummy.probar) | casa (8×6 m, cuatro cuartos y una persona)")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")

    world = World(a.mundo)
    log.info("Mundo: %s (%g×%g m)", a.mundo, world.w, world.h)
    if os.environ.get("BB8_MAPA"):  # el plano para el servicio de movimiento (lo pone lanzar_dummy)
        with open(os.environ["BB8_MAPA"], "w", encoding="utf-8") as f:
            json.dump(world.plano(), f, ensure_ascii=False, indent=1)
        log.info("Plano en %s", os.environ["BB8_MAPA"])
    arduino = ArduinoSim(world)
    serve(arduino, port=a.puerto_serial)

    head_deg = lambda: arduino.head  # noqa: E731
    eye = HS.SimEye()
    camera = None
    if a.camara == "ninguna":
        from head.backends_real import SinCamara

        camera = SinCamara()
        log.info("Visión: sin cámara, como la cabeza antes de que llegue la OV5647")
    elif a.camara != "sintetica":
        idx = 0 if a.camara == "webcam" else int(a.camara)
        try:
            camera = HS.WebcamCamera(idx)
            log.info("Visión: webcam %d del PC", idx)
        except Exception as e:
            log.warning("%s; uso la vista sintética", e)
    if camera is None:
        camera = HS.SyntheticCamera(world, head_deg, eye, max(20.0, min(170.0, a.fov)))
        log.info("Visión: vista sintética, apertura %.0f°", camera.fov)

    speaker = HS.SimSpeaker(play=not a.sin_sonido)
    app = build_app(camera, HS.SimToF(world, head_deg), eye, speaker,
                    extra_routes=visor.rutas(world, arduino, eye, speaker, OidoSim(world, head_deg)))
    log.info("Cabeza simulada en http://127.0.0.1:%d", a.puerto_cabeza)
    log.info("Visor 2D en http://127.0.0.1:%d/sim", a.puerto_cabeza)
    uvicorn.run(app, host="127.0.0.1", port=a.puerto_cabeza, log_level="warning")


if __name__ == "__main__":
    main()
