"""API HTTP local del servicio de movimiento (PROTOCOLO.md, sección 2).

    python -m bb8.motion_api                       # en la Pi, Arduino por USB
    python -m bb8.motion_api --serial socket://127.0.0.1:5555 --head http://127.0.0.1:8080
"""
from __future__ import annotations

import argparse
import contextlib
import logging

import uvicorn
from starlette.applications import Starlette
from starlette.requests import Request
from starlette.responses import JSONResponse
from starlette.routing import Route

from . import config as C
from .motion import MotionService
from .serial_link import SerialLink

log = logging.getLogger("bb8.api")


def build_app(service: MotionService, gamepad: bool = False) -> Starlette:
    async def body(req: Request) -> dict:
        try:
            return await req.json()
        except Exception:
            return {}

    def num(d: dict, key: str) -> float:
        if key not in d:
            raise ValueError(f"falta '{key}'")
        return float(d[key])

    def handler(fn):
        async def ep(req: Request):
            try:
                return JSONResponse(await fn(await body(req)))
            except ValueError as e:
                return JSONResponse({"resultado": "error", "motivo": str(e)}, status_code=400)
            except Exception as e:
                log.exception("Fallo en %s", req.url.path)
                return JSONResponse({"resultado": "error", "motivo": str(e)}, status_code=500)
        return ep

    routes = [
        Route("/move", handler(lambda d: service.move(num(d, "metros"))), methods=["POST"]),
        Route("/turn", handler(lambda d: service.turn(num(d, "grados"))), methods=["POST"]),
        Route("/look_at", handler(lambda d: service.look_at(num(d, "grados"))), methods=["POST"]),
        Route("/stop", handler(lambda d: service.stop()), methods=["POST"]),
        Route("/manual", handler(lambda d: service.manual(num(d, "izq"), num(d, "der"))), methods=["POST"]),
        Route("/sleep", handler(lambda d: service.sleep()), methods=["POST"]),
        Route("/wake", handler(lambda d: service.wake()), methods=["POST"]),
        Route("/apagar", handler(lambda d: service.poweroff(float(d.get("segundos", 15)))), methods=["POST"]),
        Route("/pose", lambda req: JSONResponse(service.pose()), methods=["GET"]),
    ]

    @contextlib.asynccontextmanager
    async def lifespan(app):
        await service.start()
        tasks = []
        if gamepad:
            from .gamepad import run_gamepad
            import asyncio
            tasks.append(asyncio.create_task(run_gamepad(service)))
        log.info("Servicio de movimiento listo")
        yield
        for t in tasks:
            t.cancel()
        await service.close()

    return Starlette(routes=routes, lifespan=lifespan)


def main() -> None:
    ap = argparse.ArgumentParser(description="Servicio de movimiento del BB-8")
    ap.add_argument("--serial", default=C.SERIAL_URL, help="ej. /dev/ttyACM0 o socket://127.0.0.1:5555")
    ap.add_argument("--head", default=C.HEAD_URL, help="URL de la cabeza")
    ap.add_argument("--port", type=int, default=C.MOTION_PORT)
    ap.add_argument("--mando", action="store_true", help="leer el mando Bluetooth con evdev (solo Linux)")
    ap.add_argument("-v", "--verbose", action="store_true")
    a = ap.parse_args()
    logging.basicConfig(level=logging.DEBUG if a.verbose else logging.INFO,
                        format="%(asctime)s %(name)s %(levelname)s %(message)s")
    for noisy in ("httpx", "mcp"):
        logging.getLogger(noisy).setLevel(logging.WARNING)
    service = MotionService(SerialLink(a.serial, C.SERIAL_BAUD), a.head)
    # Solo localhost: el MCP y el agente de voz corren en la misma Pi.
    uvicorn.run(build_app(service, a.mando), host=C.MOTION_HOST, port=a.port, log_level="warning")


if __name__ == "__main__":
    main()
