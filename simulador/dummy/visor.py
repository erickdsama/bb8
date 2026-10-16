"""Visor 2D del dummy en el navegador: http://127.0.0.1:8080/sim

La página (visor.html) pregunta /sim/mundo unas 10 veces por segundo y dibuja la
habitación, el robot, hacia dónde mira la cabeza, el rayo del ToF, el color del ojo
y lo último que dijo. Es solo para mirar: no manda órdenes al robot (salvo el botón
de empujar, que usa /sim/empujar igual que dummy.probar, arrastrar a la persona del
mundo "casa", que usa /sim/mover, y hacerla hablar, que usa /sim/hablar y le llega al
robot por el ReSpeaker simulado).
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


def estado(world: World, arduino, eye, speaker, oido=None) -> dict:
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
        "oido": oido_visor(oido),
        "radio_m": ROBOT_RADIUS,
    }


def oido_visor(oido) -> dict | None:
    u = oido and oido.ultimo
    if not u:
        return None
    return {k: u[k] for k in ("texto", "fuente", "oido", "angulo_deg", "nivel_db", "muros", "real_deg", "theta_deg")} | {
        "hace_s": round(time.time() - u["t"], 2)}


def rutas(world: World, arduino, eye, speaker, oido=None) -> list[Route]:
    async def pagina(req: Request):
        return FileResponse(HTML, media_type="text/html; charset=utf-8")

    async def mundo(req: Request):
        return JSONResponse(estado(world, arduino, eye, speaker, oido))

    async def empujar(req: Request):
        d = await req.json()
        world.push(float(d.get("grados", 40)), float(d.get("segundos", 1.0)))
        return JSONResponse({"ok": True})

    async def mover(req: Request):
        d = await req.json()
        ok = world.mover(str(d.get("nombre", "persona")), float(d["x"]), float(d["y"]))
        return JSONResponse({"ok": ok})

    async def hablar(req: Request):
        """Alguien dice algo: en (x, y) si vienen, si no donde está la persona."""
        d = await req.json()
        if "x" in d and "y" in d:
            x, y = float(d["x"]), float(d["y"])
        else:
            p = next((b for b in world.boxes if b.tipo == "persona"), None)
            if p is None:
                return JSONResponse({"ok": False, "error": "este mundo no tiene persona: manda x, y"}, status_code=400)
            x, y = p.centro()
        evento = oido.escuchar(str(d.get("texto") or "¡BB-8!"), x, y)
        entregado = await oido.entregar(evento)
        return JSONResponse({"ok": True, "entregado": entregado} | oido_visor(oido))

    return [Route("/sim", pagina), Route("/sim/mundo", mundo),
            Route("/sim/hablar", hablar, methods=["POST"]),
            Route("/sim/empujar", empujar, methods=["POST"]),
            Route("/sim/mover", mover, methods=["POST"])]
