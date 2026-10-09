"""Mando Bluetooth con evdev (solo Linux). Prioridad sobre el LLM.

Stick izquierdo: Y adelante/atrás, X giro. Mezcla arcade izq = y + x, der = y − x.
Botón sur (X/A) = alto. Reenvía M cada 100 ms mientras el stick está fuera de la
zona muerta, para alimentar el watchdog del Arduino.
"""
from __future__ import annotations

import asyncio
import logging
import time

from . import config as C

log = logging.getLogger("bb8.gamepad")


def _find_device():
    import evdev  # pip install evdev

    for path in evdev.list_devices():
        dev = evdev.InputDevice(path)
        caps = dev.capabilities().get(evdev.ecodes.EV_ABS, [])
        codes = {c[0] if isinstance(c, tuple) else c for c in caps}
        if evdev.ecodes.ABS_X in codes and evdev.ecodes.ABS_Y in codes:
            return dev
    return None


def mix(x: float, y: float) -> tuple[float, float]:
    """x, y en −1..1 (y + adelante) → PWM izq, der con zona muerta y límite."""
    lim = C.PWM_MANUAL_MAX
    l, r = (y + x) * lim, (y - x) * lim
    dz = C.GAMEPAD_DEADZONE
    l = 0.0 if abs(l) < dz else max(-lim, min(lim, l))
    r = 0.0 if abs(r) < dz else max(-lim, min(lim, r))
    return l, r


async def run_gamepad(service) -> None:
    import evdev

    while True:
        dev = _find_device()
        if dev is None:
            log.info("Sin mando; reintento en 5 s")
            await asyncio.sleep(5)
            continue
        log.info("Mando: %s", dev.name)
        info = {c: dev.absinfo(c) for c in (evdev.ecodes.ABS_X, evdev.ecodes.ABS_Y)}
        axes = {evdev.ecodes.ABS_X: 0.0, evdev.ecodes.ABS_Y: 0.0}
        last_sent = (0.0, 0.0)
        last_t = 0.0

        def norm(code, value):
            ai = info[code]
            mid = (ai.max + ai.min) / 2
            return (value - mid) / ((ai.max - ai.min) / 2)

        try:
            while True:
                try:
                    ev = await asyncio.wait_for(dev.async_read_one(), timeout=C.KEEPALIVE_S)
                except asyncio.TimeoutError:
                    ev = None
                if ev is not None and ev.type == evdev.ecodes.EV_ABS and ev.code in axes:
                    axes[ev.code] = norm(ev.code, ev.value)
                elif ev is not None and ev.type == evdev.ecodes.EV_KEY and ev.code == evdev.ecodes.BTN_SOUTH and ev.value:
                    await service.stop()
                    continue
                cmd = mix(axes[evdev.ecodes.ABS_X], -axes[evdev.ecodes.ABS_Y])
                now = time.monotonic()
                moving = cmd != (0.0, 0.0)
                if (cmd != last_sent or (moving and now - last_t >= C.KEEPALIVE_S)):
                    await service.manual(*cmd)
                    last_sent, last_t = cmd, now
        except OSError:
            log.warning("Se desconectó el mando")
            await service.stop()
