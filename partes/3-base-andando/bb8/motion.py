"""Servicio de movimiento: único dueño del serial y árbitro mando/LLM.

Traduce move(1.0) y turn(90) en órdenes M con lazo cerrado sobre encoders y
giroscopio, sondea el estado del Arduino cada 50 ms y reenvía el ToF de la
cabeza como D <mm>. Ver PROTOCOLO.md, secciones 1 y 2.
"""
from __future__ import annotations

import asyncio
import logging
import math
import time

import httpx

from . import config as C
from . import protocol as P
from .serial_link import SerialLink

log = logging.getLogger("bb8.motion")


def _clamp(v: float, lo: float, hi: float) -> float:
    return max(lo, min(hi, v))


class MotionService:
    def __init__(self, link: SerialLink, head_url: str = C.HEAD_URL):
        self.link = link
        self.head_url = head_url.rstrip("/")
        self.status = P.Status()
        self.status_t = 0.0
        self.tof_mm = 0
        self.asleep = False
        self.x_m = 0.0
        self.y_m = 0.0
        self._prev_enc: tuple[int, int] | None = None
        self._tick = asyncio.Event()
        self._motion_lock = asyncio.Lock()
        self._cancel_gen = 0
        self._last_manual = -math.inf
        self._tasks: list[asyncio.Task] = []
        self._http: httpx.AsyncClient | None = None
        self._head_ok = True

    # --- arranque y lazos de fondo -------------------------------------------

    async def start(self) -> None:
        await asyncio.to_thread(self.link.open)
        await self._send(P.cmd_head(0))
        self._http = httpx.AsyncClient(timeout=0.3)
        self._tasks = [asyncio.create_task(self._poll_loop()),
                       asyncio.create_task(self._tof_loop())]
        await self._wait_status()

    async def close(self) -> None:
        for t in self._tasks:
            t.cancel()
        try:
            await self._send(P.CMD_STOP)
        except Exception:
            pass
        if self._http:
            await self._http.aclose()
        self.link.close()

    async def _send(self, line: str) -> str:
        return await asyncio.to_thread(self.link.send, line)

    async def _poll_loop(self) -> None:
        while True:
            try:
                st = P.parse_status(await self._send(P.CMD_STATUS))
                self._update_odometry(st)
                self.status, self.status_t = st, time.monotonic()
                tick, self._tick = self._tick, asyncio.Event()
                tick.set()
            except Exception as e:
                log.warning("Sondeo de estado falló: %s", e)
            await asyncio.sleep(C.POLL_S)

    async def _wait_status(self) -> P.Status:
        try:
            await asyncio.wait_for(self._tick.wait(), timeout=0.5)
        except asyncio.TimeoutError:
            log.warning("Sin estado nuevo del Arduino en 500 ms")
        return self.status

    def _update_odometry(self, st: P.Status) -> None:
        enc = (st.enc_left, st.enc_right)
        if self._prev_enc is not None:
            d = ((enc[0] - self._prev_enc[0]) + (enc[1] - self._prev_enc[1])) / 2 / C.TICKS_PER_M
            yaw = math.radians(st.yaw_deg)
            self.x_m += d * math.cos(yaw)
            self.y_m += d * math.sin(yaw)  # y positivo a la derecha, como el rumbo
        self._prev_enc = enc

    async def _tof_loop(self) -> None:
        assert self._http
        while True:
            try:
                r = await self._http.get(f"{self.head_url}/tof")
                self.tof_mm = int(r.json().get("mm", 0))
                if not self._head_ok:
                    log.info("Cabeza de vuelta en %s", self.head_url)
                self._head_ok = True
            except Exception as e:
                self.tof_mm = 0
                if self._head_ok:
                    log.warning("No hay ToF de la cabeza (%s): sin freno por obstáculo", e)
                self._head_ok = False
            # Solo vale como distancia del camino si la cabeza mira al frente.
            fwd = abs(self.status.head_deg) <= C.TOF_FORWARD_MAX_HEAD_DEG
            try:
                await self._send(P.cmd_distance(self.tof_mm if fwd else 0))
            except Exception as e:
                log.warning("No se pudo reenviar D: %s", e)
            await asyncio.sleep(C.TOF_POLL_S)

    # --- árbitro ---------------------------------------------------------------

    def manual_active(self) -> bool:
        return time.monotonic() - self._last_manual < C.MANUAL_HOLD_S

    def control(self) -> str:
        if self.manual_active():
            return "manual"
        return "llm" if self._motion_lock.locked() else "idle"

    async def manual(self, left: float, right: float) -> dict:
        """Orden directa del mando: gana siempre y cancela lo que haga el LLM."""
        self._last_manual = time.monotonic()
        self._cancel_gen += 1
        lim = C.PWM_MANUAL_MAX
        try:
            await self._send(P.cmd_motors(_clamp(left, -lim, lim), _clamp(right, -lim, lim)))
        except P.ProtocolError as e:
            return {"resultado": "error", "motivo": e.reason}
        return {"resultado": "done"}

    # --- herramientas ----------------------------------------------------------

    async def stop(self) -> dict:
        self._cancel_gen += 1
        await self._send(P.CMD_STOP)
        return {"resultado": "done"}

    async def move(self, meters: float) -> dict:
        meters = _clamp(meters, -C.MOVE_MAX_M, C.MOVE_MAX_M)
        return await self._run(self._drive, meters)

    async def turn(self, degrees: float) -> dict:
        degrees = _clamp(degrees, -C.TURN_MAX_DEG, C.TURN_MAX_DEG)
        return await self._run(self._spin, degrees)

    async def look_at(self, degrees: float) -> dict:
        degrees = _clamp(degrees, -C.HEAD_MAX_DEG, C.HEAD_MAX_DEG)
        return await self._run(self._head, degrees)

    async def sleep(self) -> dict:
        await self.stop()
        await self._send(P.CMD_SLEEP)
        self.asleep = True
        return {"resultado": "done"}

    async def wake(self) -> dict:
        await self._send(P.CMD_WAKE)
        self.asleep = False
        return {"resultado": "done"}

    async def poweroff(self, seconds: float) -> dict:
        """Reposo profundo: el Arduino cortará la Pi en `seconds`. Quien llama hace el shutdown."""
        await self.stop()
        await self._send(P.cmd_poweroff(int(_clamp(seconds, 5, 120))))
        self.asleep = True
        return {"resultado": "done", "corte_en_s": int(_clamp(seconds, 5, 120))}

    def pose(self) -> dict:
        st = self.status
        return {
            "x_m": round(self.x_m, 3), "y_m": round(self.y_m, 3),
            "rumbo_deg": round(st.yaw_deg, 1), "inclinacion_deg": round(st.tilt_deg, 1),
            "bateria_v": round(st.battery_v, 2), "cabeza_deg": st.head_deg,
            "tof_m": round(self.tof_mm / 1000, 3) if self.tof_mm else None,
            "banderas": sorted(st.flags), "control": self.control(),
            "dormido": self.asleep, "arduino": self.link.ident,
            "estado_hace_s": round(time.monotonic() - self.status_t, 2),
        }

    # --- cola de órdenes -------------------------------------------------------

    async def _run(self, fn, arg) -> dict:
        if self.manual_active():
            return {"resultado": "manual_override"}
        if self.asleep:  # reposo ligero: una orden lo despierta (W) en lugar de fallar
            await self.wake()
        gen = self._cancel_gen
        async with self._motion_lock:  # una orden a la vez; las demás esperan
            if gen != self._cancel_gen:
                return {"resultado": "cancelled"}
            try:
                return await fn(arg, gen)
            except P.ProtocolError as e:
                await self._send(P.CMD_STOP)
                return {"resultado": "error", "motivo": e.reason}
            except Exception:
                await self._send(P.CMD_STOP)
                raise

    async def _interrupted(self, gen: int, out: dict) -> dict | None:
        """Comprueba cancelación, mando, inclinación y tiempo. Devuelve el resultado si hay que parar."""
        if gen != self._cancel_gen:
            if self.manual_active():
                return {"resultado": "manual_override", **out}  # el mando ya manda sus M
            await self._send(P.CMD_STOP)
            return {"resultado": "cancelled", **out}
        if "tilt" in self.status.flags:
            await self._send(P.CMD_STOP)
            return {"resultado": "error", "motivo": "tilt", **out}
        return None

    async def _drive(self, meters: float, gen: int) -> dict:
        st = await self._wait_status()
        e0 = (st.enc_left + st.enc_right) / 2
        yaw0 = st.yaw_deg
        target = meters * C.TICKS_PER_M
        sign = 1 if meters >= 0 else -1
        t0 = time.monotonic()

        def traveled() -> float:
            s = self.status
            return ((s.enc_left + s.enc_right) / 2 - e0) / C.TICKS_PER_M

        while True:
            out = {"metros": round(traveled(), 3)}
            if (r := await self._interrupted(gen, out)) is not None:
                return r
            if sign > 0 and "obstacle" in self.status.flags:
                await self._send(P.CMD_STOP)
                return self._blocked(out)
            remaining = (target - (traveled() * C.TICKS_PER_M)) / C.TICKS_PER_M
            if remaining * sign < 0.01:
                break
            if time.monotonic() - t0 > C.ORDER_MAX_S:
                await self._send(P.CMD_STOP)
                return {"resultado": "timeout", **out}
            speed = sign * _clamp(abs(remaining) * 400, C.PWM_MIN, C.PWM_CRUISE)
            corr = 3.0 * (self.status.yaw_deg - yaw0)  # mantener el rumbo
            try:
                await self._send(P.cmd_motors(speed - corr, speed + corr))
            except P.ProtocolError as e:
                if e.reason == "obstacle":
                    await self._send(P.CMD_STOP)
                    return self._blocked(out)
                raise
            await self._wait_status()

        await self._send(P.CMD_STOP)
        await asyncio.sleep(0.15)
        await self._wait_status()
        return {"resultado": "done", "metros": round(traveled(), 3),
                "segundos": round(time.monotonic() - t0, 2)}

    def _blocked(self, out: dict) -> dict:
        d = self.tof_mm or self.status.tof_mm
        return {"resultado": "blocked", **out, "distancia_m": round(d / 1000, 3) if d else None,
                "motivo": "obstacle"}

    async def _spin(self, degrees: float, gen: int) -> dict:
        st = await self._wait_status()
        yaw0 = st.yaw_deg
        target = yaw0 + degrees
        t0 = time.monotonic()
        while True:
            out = {"grados": round(self.status.yaw_deg - yaw0, 1)}
            if (r := await self._interrupted(gen, out)) is not None:
                return r
            err = target - self.status.yaw_deg
            if abs(err) < 2.0 or err * degrees < 0:  # llegó o se pasó
                break
            if time.monotonic() - t0 > C.ORDER_MAX_S:
                await self._send(P.CMD_STOP)
                return {"resultado": "timeout", **out}
            s = math.copysign(_clamp(abs(err) * 2.0, C.PWM_TURN_MIN, C.PWM_TURN), err)
            await self._send(P.cmd_motors(s, -s))  # + derecha: izquierda adelante
            await self._wait_status()
        await self._send(P.CMD_STOP)
        await asyncio.sleep(0.15)
        await self._wait_status()
        return {"resultado": "done", "grados": round(self.status.yaw_deg - yaw0, 1),
                "segundos": round(time.monotonic() - t0, 2)}

    async def _head(self, degrees: float, gen: int) -> dict:
        await self._send(P.cmd_head(degrees))
        t0 = time.monotonic()
        while abs(self.status.head_deg - round(degrees)) > 1 and time.monotonic() - t0 < 1.5:
            if gen != self._cancel_gen:
                return {"resultado": "cancelled", "grados": self.status.head_deg}
            await self._wait_status()
        return {"resultado": "done", "grados": self.status.head_deg}
