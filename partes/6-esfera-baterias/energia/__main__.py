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
        self.motion = httpx.AsyncClient(base_url=C.MOTION_URL, timeout=5)
        self.cabeza = httpx.AsyncClient(base_url=C.HEAD_URL, timeout=10)

    # --- avisos externos ------------------------------------------------------

    async def actividad(self, motivo: str) -> None:
        self.ultima_actividad = time.monotonic()
        if self.estado != "activo":
            log.info("Actividad (%s) → activo", motivo)
            await self._a("activo")

    async def dormir(self) -> None:
        await self._a("profundo" if PROFUNDO_HABILITADO else "ligero")

    # --- transiciones -----------------------------------------------------------

    async def _post(self, cli: httpx.AsyncClient, ruta: str, cuerpo: dict | None = None) -> None:
        try:
            await cli.post(ruta, json=cuerpo or {})
        except httpx.HTTPError as e:
            log.warning("%s%s: %s", cli.base_url, ruta, e)

    async def _a(self, nuevo: str) -> None:
        if nuevo == self.estado:
            return
        viejo, self.estado, self.desde = self.estado, nuevo, time.monotonic()
        log.info("%s → %s", viejo, nuevo)
        if nuevo == "activo":
            if viejo in ("ligero", "profundo"):
                await self._post(self.motion, "/wake")
                await self._post(self.cabeza, "/reposo", {"activo": True})
            await self._post(self.cabeza, "/ojo", {"color": "azul", "patron": "fijo"})
        elif nuevo == "escuchando":
            await self._post(self.cabeza, "/ojo", {"color": "azul", "patron": "respirar"})
        elif nuevo == "ligero":
            await self._post(self.motion, "/sleep")
            await self._post(self.cabeza, "/reposo", {"activo": False})
        elif nuevo == "profundo":
            await self._post(self.cabeza, "/hablar", {"texto": "Me voy a dormir. Muéveme para despertarme."})
            await self._post(self.cabeza, "/reposo", {"activo": False})
            await self._post(self.motion, "/apagar", {"segundos": CORTE_S})
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
        if self.estado == "ligero" and not p.get("dormido"):
            return True   # una orden MCP despertó al servicio de movimiento
        if prev is None:
            return False
        if abs(p["rumbo_deg"] - prev["rumbo_deg"]) > 3 or p.get("cabeza_deg") != prev.get("cabeza_deg"):
            return True
        d = math.hypot(p["x_m"] - prev["x_m"], p["y_m"] - prev["y_m"])
        # En reposo la IMU sigue midiendo: si alguien lo inclina o lo empuja, despierta.
        return d > 0.02 or abs(p["inclinacion_deg"] - prev["inclinacion_deg"]) > 5

    async def _bateria(self, v: float) -> None:
        if v <= 0:          # 0 = sin divisor en A0 (fuente de pared)
            return
        ahora = time.monotonic()
        if v < BAT_APAGAR_V:
            self._bat_baja_desde = self._bat_baja_desde or ahora
            if ahora - self._bat_baja_desde > 30 and self.estado != "profundo":
                log.warning("Batería %.2f V: a dormir", v)
                await self._post(self.cabeza, "/hablar", {"texto": "Batería agotada. Necesito el cargador."})
                await self.dormir()
        else:
            self._bat_baja_desde = None
        if v < BAT_AVISO_V and ahora - self._ultimo_aviso > 300 and self.estado != "profundo":
            self._ultimo_aviso = ahora
            log.warning("Batería baja: %.2f V", v)
            await self._post(self.cabeza, "/ojo", {"color": "rojo", "patron": "parpadeo"})
            await self._post(self.cabeza, "/hablar", {"texto": "Bip. Batería baja."})

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
            await self._bateria(float(pose.get("bateria_v") or 0))
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
