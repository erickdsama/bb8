"""Habitación simulada: cinemática diferencial, paredes, una silla y raycast.

Coordenadas del mundo en metros, x a la derecha, y hacia arriba, ángulo θ
antihorario. El robot reporta rumbo + derecha, así que yaw = −(θ − θ0).
"""
from __future__ import annotations

import math
import random
import threading
import time
from dataclasses import dataclass

from bb8 import config as C

ROBOT_RADIUS = 0.11   # base de 22 cm
TOF_OFFSET = 0.10     # el sensor está a 10 cm del centro, en la cabeza
TOF_MAX = 4.0


@dataclass
class Box:
    name: str
    x0: float
    y0: float
    x1: float
    y1: float
    color: tuple[int, int, int]  # BGR para OpenCV
    height: float = 0.4


class World:
    def __init__(self, width: float = 4.0, height: float = 3.0):
        self.w, self.h = width, height
        self.boxes = [
            Box("silla", 2.6, 1.25, 3.0, 1.75, (40, 90, 170), 0.45),
            Box("caja", 0.4, 2.3, 0.9, 2.7, (60, 140, 60), 0.3),
        ]
        self.x, self.y, self.theta = 1.0, 1.5, 0.0
        self.theta0 = self.theta
        self.bumped = False
        self._push_until = 0.0
        self._push_deg = 0.0
        self.lock = threading.RLock()

    # --- dinámica ------------------------------------------------------------

    def step(self, dt: float, pwm_l: float, pwm_r: float) -> tuple[float, float]:
        """Avanza dt segundos. Devuelve lo que giró cada rueda en metros."""
        vl = pwm_l / 255 * C.V_MAX_MPS
        vr = pwm_r / 255 * C.V_MAX_MPS
        dl, dr = vl * dt, vr * dt
        with self.lock:
            d = (dl + dr) / 2
            dth = (dr - dl) / C.TRACK_WIDTH_M
            th = self.theta + dth / 2
            nx, ny = self.x + d * math.cos(th), self.y + d * math.sin(th)
            self.theta += dth
            if self._collides(nx, ny):
                self.bumped = True  # choca: las ruedas patinan, el robot no avanza
            else:
                self.bumped = False
                self.x, self.y = nx, ny
        return dl, dr

    def _collides(self, x: float, y: float) -> bool:
        r = ROBOT_RADIUS
        if x < r or y < r or x > self.w - r or y > self.h - r:
            return True
        for b in self.boxes:
            cx, cy = min(max(x, b.x0), b.x1), min(max(y, b.y0), b.y1)
            if (x - cx) ** 2 + (y - cy) ** 2 < r * r:
                return True
        return False

    def yaw_deg(self) -> float:
        with self.lock:
            return -math.degrees(self.theta - self.theta0)

    def push(self, deg: float, seconds: float = 1.0) -> None:
        self._push_deg, self._push_until = deg, time.monotonic() + seconds

    def tilt_deg(self, accel: float = 0.0) -> float:
        base = abs(accel) * 0.02 + random.uniform(0, 0.4)
        if time.monotonic() < self._push_until:
            base += self._push_deg
        return base

    # --- sensores ------------------------------------------------------------

    def raycast(self, angle: float, ox: float | None = None, oy: float | None = None) -> tuple[float, Box | None]:
        """Distancia desde (ox, oy) en dirección angle (rad, mundo) a lo primero que toque."""
        with self.lock:
            ox = self.x if ox is None else ox
            oy = self.y if oy is None else oy
        dx, dy = math.cos(angle), math.sin(angle)
        best, hit = TOF_MAX * 2, None
        # paredes
        for t in ((-ox / dx) if dx < 0 else (self.w - ox) / dx if dx > 0 else math.inf,
                  (-oy / dy) if dy < 0 else (self.h - oy) / dy if dy > 0 else math.inf):
            if 0 <= t < best:
                best = t
        # cajas (slab test)
        for b in self.boxes:
            tmin, tmax = -math.inf, math.inf
            ok = True
            for o, d, lo, hi in ((ox, dx, b.x0, b.x1), (oy, dy, b.y0, b.y1)):
                if abs(d) < 1e-9:
                    if not lo <= o <= hi:
                        ok = False
                    continue
                t1, t2 = (lo - o) / d, (hi - o) / d
                tmin, tmax = max(tmin, min(t1, t2)), min(tmax, max(t1, t2))
            if ok and tmax >= max(tmin, 0) and 0 <= tmin < best:
                best, hit = tmin, b
        return best, hit

    def tof_mm(self, head_deg: float) -> int:
        """Lectura del VL53L1X: mira hacia donde apunta la cabeza (+ derecha)."""
        with self.lock:
            ang = self.theta - math.radians(head_deg)
            ox = self.x + TOF_OFFSET * math.cos(ang)
            oy = self.y + TOF_OFFSET * math.sin(ang)
        d, _ = self.raycast(ang, ox, oy)
        if d > TOF_MAX:
            return 0
        return max(40, int(d * 1000 + random.gauss(0, 5)))

    def snapshot(self) -> dict:
        with self.lock:
            return {"x_m": round(self.x, 3), "y_m": round(self.y, 3),
                    "theta_deg": round(math.degrees(self.theta), 1),
                    "rumbo_deg": round(self.yaw_deg(), 1), "chocando": self.bumped,
                    "habitacion_m": [self.w, self.h],
                    "obstaculos": [{"nombre": b.name, "x": [b.x0, b.x1], "y": [b.y0, b.y1]} for b in self.boxes]}
