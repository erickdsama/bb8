"""Gestor de energía del BB-8 (Parte 6). Corre en la Pi principal junto al servicio de movimiento.

    python -m energia                    # reposo ligero; el profundo solo con BB8_REPOSO_PROFUNDO=1
    python -m energia --prueba           # tiempos cortos (10 s / 20 s / 40 s) y sin apagar la Pi

Estados (documento técnico, "Modo reposo"):
    activo ──30 s sin actividad──▶ escuchando ──5 min──▶ ligero ──30 min o "a dormir"──▶ profundo
    cualquier actividad (wake word, orden MCP, mando, alguien lo mueve) ──▶ activo

Ligero: Z al Arduino (servo sin PWM, motores libres), cámara pausada y ojo tenue.
Profundo: P <s> al Arduino y shutdown de la Pi; el Arduino corta la Pi y la vuelve a
encender cuando la IMU nota movimiento (~25 s de arranque). Necesita el P-MOSFET en A3.

API en 127.0.0.1:8771: POST /actividad, POST /dormir, GET /estado.
"""
from __future__ import annotations

import argparse
import asyncio
import contextlib
import logging
import math
import os
import time

import httpx
import uvicorn
from starlette.applications import Starlette
from starlette.responses import JSONResponse
from starlette.routing import Route

from bb8 import config as C

log = logging.getLogger("bb8.energia")

PUERTO = 8771
PROFUNDO_HABILITADO = os.environ.get("BB8_REPOSO_PROFUNDO", "0") == "1"
BAT_AVISO_V = 10.5          # 3.5 V por celda: avisar
BAT_APAGAR_V = 10.0         # 3.33 V por celda: dormir profundo (o ligero si no hay MOSFET)
CORTE_S = 20                # el Arduino corta la Pi 20 s después de P: le da tiempo al shutdown


class Energia:
    def __init__(self, prueba: bool = False):
        self.t_escuchando, self.t_ligero, self.t_profundo = (10, 20, 40) if prueba else (30, 300, 1800)
        self.prueba = prueba
        self.estado = "activo"
        self.ultima_actividad = time.monotonic()
        self.desde = time.monotonic()
        self._pose_prev: dict | None = None
        self._bat_baja_desde: float | None = None
        self._ultimo_aviso = -math.inf
        # bb8-body local API; the head is only reachable through it (secure link).
        self.motion = httpx.AsyncClient(base_url=C.MOTION_URL, timeout=10)

    # --- avisos externos ------------------------------------------------------

    async def actividad(self, motivo: str) -> None:
        self.ultima_actividad = time.monotonic()
        if self.estado != "activo":
            log.info("Actividad (%s) → activo", motivo)
            await self._a("activo")

    async def dormir(self) -> None:
        await self._a("profundo" if PROFUNDO_HABILITADO else "ligero")

    # --- transiciones -----------------------------------------------------------

    async def _post(self, ruta: str, cuerpo: dict | None = None) -> None:
        try:
            await self.motion.post(ruta, json=cuerpo or {})
        except httpx.HTTPError as e:
            log.warning("%s: %s", ruta, e)

    async def _ojo(self, color: str, patron: str) -> None:
        await self._post("/tools/set_eye_color", {"color": color, "pattern": patron})

    async def _decir(self, texto: str) -> None:
        await self._post("/tools/say", {"text": texto})

    async def _a(self, nuevo: str) -> None:
        if nuevo == self.estado:
            return
        viejo, self.estado, self.desde = self.estado, nuevo, time.monotonic()
        log.info("%s → %s", viejo, nuevo)
        if nuevo == "activo":
            if viejo in ("ligero", "profundo"):
                await self._post("/wake")
                await self._post("/head/rest", {"active": True})
            await self._ojo("blue", "steady")
        elif nuevo == "escuchando":
            await self._ojo("blue", "breathe")
        elif nuevo == "ligero":
            await self._post("/sleep")
            await self._post("/head/rest", {"active": False})
        elif nuevo == "profundo":
            await self._decir("Me voy a dormir. Muéveme para despertarme.")
            await self._post("/head/rest", {"active": False})
            await self._post("/poweroff", {"seconds": CORTE_S})
            if self.prueba:
                log.warning("--prueba: aquí la Pi haría 'systemctl poweroff'")
                return
            # La cabeza sigue en reposo ligero con su power bank (~12 h); apágala de su interruptor
            # si el BB-8 se va a guardar.
            proc = await asyncio.create_subprocess_exec("sudo", "-n", "systemctl", "poweroff")
            await proc.wait()

    # --- lazo -------------------------------------------------------------------

    def _hubo_movimiento(self, p: dict) -> bool:
        prev, self._pose_prev = self._pose_prev, p
        if p.get("control") in ("llm", "manual"):
            return True
        if self.estado == "ligero" and not p.get("asleep"):
            return True   # una orden MCP despertó al servicio de movimiento
        if prev is None:
            return False
        if abs(p["heading_deg"] - prev["heading_deg"]) > 3 or p.get("head_deg") != prev.get("head_deg"):
            return True
        d = math.hypot(p["x_m"] - prev["x_m"], p["y_m"] - prev["y_m"])
        # En reposo la IMU sigue midiendo: si alguien lo inclina o lo empuja, despierta.
        return d > 0.02 or abs(p["tilt_deg"] - prev["tilt_deg"]) > 5

    async def _bateria(self, v: float) -> None:
        if v <= 0:          # 0 = sin divisor en A0 (fuente de pared)
            return
        ahora = time.monotonic()
        if v < BAT_APAGAR_V:
            self._bat_baja_desde = self._bat_baja_desde or ahora
            if ahora - self._bat_baja_desde > 30 and self.estado != "profundo":
                log.warning("Batería %.2f V: a dormir", v)
                await self._decir("Batería agotada. Necesito el cargador.")
                await self.dormir()
        else:
            self._bat_baja_desde = None
        if v < BAT_AVISO_V and ahora - self._ultimo_aviso > 300 and self.estado != "profundo":
            self._ultimo_aviso = ahora
            log.warning("Batería baja: %.2f V", v)
            await self._ojo("red", "blink")
            await self._decir("Bip. Batería baja.")

    async def lazo(self) -> None:
        while True:
            await asyncio.sleep(1.0)
            if self.estado == "profundo":
                continue
            try:
                pose = (await self.motion.get("/pose")).json()
            except Exception as e:
                log.debug("Sin pose: %s", e)
                continue
            if self._hubo_movimiento(pose):
                await self.actividad("movimiento")
            await self._bateria(float(pose.get("battery_v") or 0))
            quieto = time.monotonic() - self.ultima_actividad
            if self.estado == "activo" and quieto > self.t_escuchando:
                await self._a("escuchando")
            elif self.estado == "escuchando" and quieto > self.t_ligero:
                await self._a("ligero")
            elif self.estado == "ligero" and quieto > self.t_profundo and PROFUNDO_HABILITADO:
                await self._a("profundo")


def build_app(e: Energia) -> Starlette:
    async def actividad(req):
        await e.actividad("voz")
        return JSONResponse({"estado": e.estado})

    async def dormir(req):
        asyncio.create_task(e.dormir())
        return JSONResponse({"estado": "durmiendo"})

    async def estado(req):
        return JSONResponse({"estado": e.estado, "quieto_s": round(time.monotonic() - e.ultima_actividad),
                             "reposo_profundo": PROFUNDO_HABILITADO})

    @contextlib.asynccontextmanager
    async def vida(app):
        t = asyncio.create_task(e.lazo())
        yield
        t.cancel()

    return Starlette(routes=[Route("/actividad", actividad, methods=["POST"]),
                             Route("/dormir", dormir, methods=["POST"]),
                             Route("/estado", estado)], lifespan=vida)


def main() -> None:
    ap = argparse.ArgumentParser(description="Gestor de energía del BB-8")
    ap.add_argument("--prueba", action="store_true", help="tiempos cortos y sin apagar la Pi")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
    logging.getLogger("httpx").setLevel(logging.WARNING)
    if not PROFUNDO_HABILITADO:
        log.info("Reposo profundo desactivado (BB8_REPOSO_PROFUNDO=1 cuando el MOSFET esté en A3)")
    uvicorn.run(build_app(Energia(a.prueba)), host="127.0.0.1", port=PUERTO, log_level="warning")


if __name__ == "__main__":
    main()
