"""Visor 2D del dummy en el navegador: http://127.0.0.1:8080/sim

La página (visor.html) pregunta /sim/mundo unas 10 veces por segundo y dibuja la
habitación, el robot, hacia dónde mira la cabeza, el rayo del ToF, el color del ojo
y lo último que dijo. Es solo para mirar: no manda órdenes al robot (salvo el botón
de empujar, que usa /sim/empujar igual que dummy.probar).
"""
from __future__ import annotations

import math
import os
import time

from starlette.requests import Request
from starlette.responses import FileResponse, JSONResponse
from starlette.routing import Route

from bb8 import config as C

from .world import ROBOT_RADIUS, TOF_MAX, TOF_OFFSET, World

HTML = os.path.join(os.path.dirname(os.path.abspath(__file__)), "visor.html")


def estado(world: World, arduino, eye, speaker) -> dict:
    """Todo lo que el visor dibuja, en una foto instantánea."""
    with arduino.lock:
        cabeza = arduino.head
        pwm = [round(v) for v in arduino.cur]
        objetivo = [round(v) for v in arduino.target]
        banderas = sorted(arduino.flags | ({"sleep"} if arduino.asleep else set()))
        inclinacion = arduino.tilt
    with world.lock:
        ang = world.theta - math.radians(cabeza)
        ox = world.x + TOF_OFFSET * math.cos(ang)
        oy = world.y + TOF_OFFSET * math.sin(ang)
    d, golpe = world.raycast(ang, ox, oy)
    alcance = min(d, TOF_MAX)
    ultimo = speaker.ultimo
    voz = None
    if ultimo:
        voz = {"texto": ultimo["texto"], "sonido": ultimo["sonido"],
               "hace_s": round(time.time() - ultimo["t"], 2), "segundos": ultimo["segundos"]}
    return world.snapshot() | {
        "cabeza_deg": round(cabeza, 1),
        "pwm": pwm, "pwm_objetivo": objetivo,
        "banderas": banderas, "inclinacion_deg": round(inclinacion, 1),
        "tof": {"mm": int(d * 1000) if d <= TOF_MAX else 0, "freno_mm": C.OBSTACLE_MM,
                "desde": [round(ox, 3), round(oy, 3)],
                "hasta": [round(ox + alcance * math.cos(ang), 3), round(oy + alcance * math.sin(ang), 3)],
                "objeto": golpe.name if golpe and d <= TOF_MAX else None},
        "ojo": {"rgb": list(eye.rgb), "patron": eye.pattern, "brillo": eye.brightness},
        "voz": voz,
        "radio_m": ROBOT_RADIUS,
    }


def rutas(world: World, arduino, eye, speaker) -> list[Route]:
    async def pagina(req: Request):
        return FileResponse(HTML, media_type="text/html; charset=utf-8")

    async def mundo(req: Request):
        return JSONResponse(estado(world, arduino, eye, speaker))

    async def empujar(req: Request):
        d = await req.json()
        world.push(float(d.get("grados", 40)), float(d.get("segundos", 1.0)))
        return JSONResponse({"ok": True})

    return [Route("/sim", pagina), Route("/sim/mundo", mundo),
            Route("/sim/empujar", empujar, methods=["POST"])]
