"""Mundo simulado: cinemática diferencial, paredes, muebles y raycast.

La física y el ToF usan la huella 2D de cada Box; la cámara sintética (render3d.py)
dibuja sus piezas 3D (Prim): patas de mesa, respaldo del sofá, la persona…

Hay varios mundos (MUNDOS): "habitacion" es el de siempre (4×3 m, silla y caja) y el
que usa dummy.probar; "casa" es una casa de 8×6 m con cuatro cuartos, puertas,
muebles de distintas alturas y una persona que se arrastra en el visor.

Coordenadas del mundo en metros, x a la derecha, y hacia arriba, ángulo θ
antihorario. El robot reporta rumbo + derecha, así que yaw = −(θ − θ0).
"""
from __future__ import annotations

import math
import random
import threading
import time
from dataclasses import dataclass, field

from bb8 import config as C

ROBOT_RADIUS = 0.11   # base de 22 cm
TOF_OFFSET = 0.10     # el sensor está a 10 cm del centro, en la cabeza
TOF_MAX = 4.0


TECHO = 2.5           # altura del techo (y de los muros interiores)


@dataclass
class Prim:
    """Pieza 3D para la cámara sintética. Solo se ve: la física usa la huella de Box.

    caja:     a = (x0, y0, z0, x1, y1, z1)
    cilindro: a = (cx, cy, r, z0, z1), vertical
    esfera:   a = (cx, cy, cz, r)
    """
    tipo: str
    color: tuple[int, int, int]  # BGR
    a: tuple[float, ...]
    estilo: str = ""             # "pared" (zoclo), "libros", "pantalla"

    def mover(self, dx: float, dy: float) -> None:
        a = list(self.a)
        a[0] += dx
        a[1] += dy
        if self.tipo == "caja":
            a[3] += dx
            a[4] += dy
        self.a = tuple(a)


@dataclass
class Box:
    name: str
    x0: float
    y0: float
    x1: float
    y1: float
    color: tuple[int, int, int]  # BGR para OpenCV
    height: float = 0.4
    tipo: str = "mueble"          # mueble | pared | persona
    # Cómo se ve en 3D; vacío = una caja de la altura de la huella. La persona se arma al vuelo.
    vis: list[Prim] = field(default_factory=list)

    def centro(self) -> tuple[float, float]:
        return (self.x0 + self.x1) / 2, (self.y0 + self.y1) / 2

    def piezas(self) -> list[Prim]:
        return self.vis or [Prim("caja", self.color, (self.x0, self.y0, 0.0, self.x1, self.y1, self.height))]


PARED = (200, 214, 226)  # BGR: crema, igual que las paredes de afuera


def _oscuro(c: tuple[int, int, int], k: float = 0.6) -> tuple[int, int, int]:
    return tuple(int(v * k) for v in c)


def _caja(x0, y0, z0, x1, y1, z1, color, estilo="") -> Prim:
    return Prim("caja", color, (x0, y0, z0, x1, y1, z1), estilo)


def _pared(x0, y0, x1, y1) -> Box:
    return Box("pared", x0, y0, x1, y1, PARED, TECHO, "pared", [_caja(x0, y0, 0, x1, y1, TECHO, PARED, "pared")])


def _mesa(nombre, x0, y0, x1, y1, h, color) -> Box:
    """Cubierta de 4 cm y cuatro patas: se ve por debajo."""
    p, pata = 0.05, _oscuro(color)
    vis = [_caja(x0, y0, h - 0.04, x1, y1, h, color)]
    for px in (x0 + 0.03, x1 - 0.03 - p):
        for py in (y0 + 0.03, y1 - 0.03 - p):
            vis.append(_caja(px, py, 0, px + p, py + p, h - 0.04, pata))
    return Box(nombre, x0, y0, x1, y1, color, h, vis=vis)


def _silla(nombre, x0, y0, x1, y1, color) -> Box:
    """Asiento a 45 cm, patas y respaldo del lado x1."""
    m = _mesa(nombre, x0, y0, x1, y1, 0.47, color)
    m.vis.append(_caja(x1 - 0.05, y0, 0.47, x1, y1, 0.95, color))
    m.height = 0.45
    return m


def _sofa(nombre, x0, y0, x1, y1, color) -> Box:
    """Respaldo contra la pared de y0, brazos en los extremos."""
    vis = [_caja(x0, y0, 0, x1, y1, 0.42, color),
           _caja(x0 + 0.1, y0 + 0.25, 0.42, x1 - 0.1, y1, 0.48, _oscuro(color, 1.15)),  # cojines
           _caja(x0, y0, 0.42, x1, y0 + 0.25, 0.85, color),
           _caja(x0, y0, 0.42, x0 + 0.12, y1, 0.62, color), _caja(x1 - 0.12, y0, 0.42, x1, y1, 0.62, color)]
    return Box(nombre, x0, y0, x1, y1, color, 0.85, vis=vis)


def _cama(nombre, x0, y0, x1, y1, color) -> Box:
    """Base, colchón, almohada y cabecera del lado y1."""
    vis = [_caja(x0, y0, 0, x1, y1, 0.3, (60, 90, 120)),
           _caja(x0 + 0.03, y0 + 0.03, 0.3, x1 - 0.03, y1 - 0.05, 0.5, color),
           _caja(x0 + 0.2, y1 - 0.45, 0.5, x1 - 0.2, y1 - 0.12, 0.6, (245, 245, 245)),
           _caja(x0, y1 - 0.05, 0, x1, y1, 1.0, (60, 90, 120))]
    return Box(nombre, x0, y0, x1, y1, color, 0.55, vis=vis)


def _tele(nombre, x0, y0, x1, y1) -> Box:
    """Mueble bajo y pantalla delgada encima, mirando a +x."""
    mueble = (50, 80, 110)
    vis = [_caja(x0, y0, 0, x1, y1, 0.5, mueble),
           _caja(x0 + 0.15, y0 + 0.1, 0.5, x0 + 0.22, y1 - 0.1, 0.56, (30, 30, 30)),
           _caja(x0 + 0.12, y0 + 0.05, 0.56, x0 + 0.18, y1 - 0.05, 1.3, (25, 25, 25), "pantalla")]
    return Box(nombre, x0, y0, x1, y1, (40, 40, 40), 1.3, vis=vis)


def _planta(nombre, x0, y0, x1, y1, h) -> Box:
    cx, cy, r = (x0 + x1) / 2, (y0 + y1) / 2, min(x1 - x0, y1 - y0) / 2
    hoja = (60, 140, 50)
    vis = [Prim("cilindro", (60, 90, 170), (cx, cy, r * 0.6, 0, 0.35)),
           Prim("esfera", hoja, (cx, cy, h - r * 0.9, r * 0.95)),
           Prim("esfera", _oscuro(hoja, 0.8), (cx + r * 0.3, cy - r * 0.2, h - r * 1.6, r * 0.7))]
    return Box(nombre, x0, y0, x1, y1, hoja, h, vis=vis)


def _ropero(nombre, x0, y0, x1, y1, h, color) -> Box:
    """Dos puertas con una rendija y jaladeras, del lado y0."""
    mx = (x0 + x1) / 2
    vis = [_caja(x0, y0 + 0.01, 0, x1, y1, h, color),
           _caja(x0 + 0.01, y0, 0.05, mx - 0.005, y0 + 0.01, h - 0.05, _oscuro(color, 1.1)),
           _caja(mx + 0.005, y0, 0.05, x1 - 0.01, y0 + 0.01, h - 0.05, _oscuro(color, 1.1)),
           _caja(mx - 0.06, y0 - 0.02, 0.9, mx - 0.04, y0, 1.2, (180, 180, 180)),
           _caja(mx + 0.04, y0 - 0.02, 0.9, mx + 0.06, y0, 1.2, (180, 180, 180))]
    return Box(nombre, x0, y0, x1, y1, color, h, vis=vis)


def _persona(x: float, y: float) -> Box:
    """Huella de 40×30 cm. La cámara la dibuja con piernas, torso, brazos y cara (render3d)."""
    return Box("persona", x - 0.2, y - 0.15, x + 0.2, y + 0.15, (150, 80, 40), 1.7, "persona")


def _habitacion() -> dict:
    """La habitación original de 4×3 m. dummy.probar cuenta con la silla a 1.6 m al frente."""
    return {"tam": (4.0, 3.0), "pose": (1.0, 1.5, 0.0), "zonas": [], "pisos": [],
            "cuartos": [("habitacion", 0.0, 0.0, 4.0, 3.0)], "puertas": [],
            "cajas": [_silla("silla", 2.6, 1.25, 3.0, 1.75, (40, 90, 170)),
                      Box("caja", 0.4, 2.3, 0.9, 2.7, (90, 150, 190), 0.3)]}


def _casa() -> dict:
    """Casa de 8×6 m: sala y cocina abajo, recámara y estudio arriba.

    Muros interiores de 10 cm con puertas de 90 cm: sala↔cocina en x≈4.5 (y 1.0–1.9),
    sala↔recámara en y≈3.5 (x 1.6–2.5) y cocina↔estudio en y≈3.5 (x 5.5–6.4).
    La recámara y el estudio no se comunican. La persona empieza en el estudio.
    """
    paredes = [
        _pared(4.5, 0.0, 4.6, 1.0), _pared(4.5, 1.9, 4.6, 3.5),          # sala | cocina
        _pared(0.0, 3.5, 1.6, 3.6), _pared(2.5, 3.5, 5.5, 3.6),          # planta baja | alta
        _pared(6.4, 3.5, 8.0, 3.6),
        _pared(4.5, 3.6, 4.6, 6.0),                                       # recámara | estudio
    ]
    librero = (40, 70, 110)
    muebles = [
        # sala
        _sofa("sofa", 0.4, 0.05, 2.6, 0.9, (70, 70, 140)),
        _mesa("mesita", 1.1, 1.5, 1.9, 2.0, 0.45, (50, 100, 150)),
        _tele("tele", 0.05, 1.3, 0.45, 2.7),
        _planta("planta", 3.9, 2.9, 4.3, 3.3, 1.2),
        # cocina
        Box("refri", 4.7, 2.75, 5.4, 3.45, (225, 225, 225), 1.8,
            vis=[_caja(4.7, 2.75, 0, 5.4, 3.45, 1.8, (225, 225, 225)),
                 _caja(4.69, 2.8, 1.18, 4.7, 3.4, 1.2, (150, 150, 150)),
                 _caja(4.67, 3.3, 0.6, 4.69, 3.33, 1.1, (120, 120, 120)),
                 _caja(4.67, 3.3, 1.3, 4.69, 3.33, 1.6, (120, 120, 120))]),
        Box("barra", 7.35, 0.05, 7.95, 3.0, (110, 130, 150), 0.92,
            vis=[_caja(7.4, 0.05, 0, 7.95, 3.0, 0.88, (110, 130, 150)),
                 _caja(7.33, 0.05, 0.88, 7.95, 3.0, 0.92, (190, 195, 200))]),
        _mesa("comedor", 5.3, 1.1, 6.5, 2.1, 0.75, (40, 80, 130)),
        Box("bote", 4.7, 0.1, 5.0, 0.4, (80, 80, 80), 0.6,
            vis=[Prim("cilindro", (80, 80, 80), (4.85, 0.25, 0.15, 0, 0.6))]),
        # recámara
        _cama("cama", 0.1, 4.3, 2.1, 5.95, (170, 120, 200)),
        _mesa("buro", 2.2, 5.5, 2.6, 5.95, 0.5, (60, 110, 160)),
        _ropero("ropero", 3.4, 5.35, 4.45, 5.95, 2.0, (50, 90, 130)),
        Box("pelota", 3.0, 4.2, 3.25, 4.45, (30, 30, 220), 0.25,
            vis=[Prim("esfera", (30, 30, 220), (3.125, 4.325, 0.125, 0.125))]),
        # estudio
        Box("escritorio", 6.2, 5.3, 7.9, 5.95, (60, 100, 140), 0.75,
            vis=_mesa("", 6.2, 5.3, 7.9, 5.95, 0.75, (60, 100, 140)).vis + [
                _caja(6.9, 5.7, 0.75, 7.4, 5.74, 1.08, (25, 25, 25), "pantalla"),
                _caja(7.12, 5.74, 0.75, 7.18, 5.8, 0.85, (40, 40, 40))]),
        Box("librero", 4.65, 4.1, 5.0, 5.8, librero, 1.8,
            vis=[_caja(4.6, 4.1, 0, 4.98, 5.8, 1.8, librero),
                 _caja(4.98, 4.12, 0.02, 5.0, 5.78, 1.78, librero, "libros")]),
        _planta("planta2", 7.45, 3.7, 7.9, 4.15, 1.0),
        _persona(6.9, 4.7),
    ]
    return {"tam": (8.0, 6.0), "pose": (2.6, 1.6, 0.0), "cajas": paredes + muebles,
            "zonas": [("sala", 2.2, 2.9), ("cocina", 6.2, 0.4), ("recámara", 1.0, 3.9), ("estudio", 6.0, 3.9)],
            # pisos: madera por defecto; la cocina con azulejo
            "pisos": [(4.6, 0.0, 8.0, 3.5, "azulejo")],
            "cuartos": [("sala", 0.0, 0.0, 4.5, 3.5), ("cocina", 4.6, 0.0, 8.0, 3.5),
                        ("recámara", 0.0, 3.6, 4.5, 6.0), ("estudio", 4.6, 3.6, 8.0, 6.0)],
            # (cuarto, cuarto, x, y del centro del hueco, ancho)
            "puertas": [("sala", "cocina", 4.55, 1.45, 0.9), ("sala", "recámara", 2.05, 3.55, 0.9),
                        ("cocina", "estudio", 5.95, 3.55, 0.9)]}


MUNDOS = {"habitacion": _habitacion, "casa": _casa}


class World:
    def __init__(self, mundo: str = "habitacion"):
        self.mundo = mundo
        m = MUNDOS[mundo]()
        (self.w, self.h), self.boxes, self.zonas, self.pisos = m["tam"], m["cajas"], m["zonas"], m["pisos"]
        self.cuartos, self.puertas, self.inicio = m["cuartos"], m["puertas"], m["pose"]
        self.x, self.y, self.theta = m["pose"]
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
        best, hit = self._pared_exterior(angle, ox, oy), None
        for b in self.boxes:
            t = _cruce(b, ox, oy, math.cos(angle), math.sin(angle))
            if t is not None and t < best:
                best, hit = t, b
        return best, hit

    def _pared_exterior(self, angle: float, ox: float, oy: float) -> float:
        dx, dy = math.cos(angle), math.sin(angle)
        best = TOF_MAX * 2
        for t in ((-ox / dx) if dx < 0 else (self.w - ox) / dx if dx > 0 else math.inf,
                  (-oy / dy) if dy < 0 else (self.h - oy) / dy if dy > 0 else math.inf):
            if 0 <= t < best:
                best = t
        return best

    def plano(self) -> dict:
        """El plano de la casa como lo tendría el robot real (formato de wiki/Protocolo.md § 2).

        Solo lo fijo: cuartos, puertas, muros y muebles; la persona no está (se mueve).
        "inicio" es dónde y hacia dónde arranca el robot: x al este, y al norte,
        rumbo en grados horario desde el este (0 = este, 90 = sur), como turn.
        """
        caja = lambda b: {"x": [round(b.x0, 2), round(b.x1, 2)], "y": [round(b.y0, 2), round(b.y1, 2)]}  # noqa: E731
        x, y, th = self.inicio
        return {
            "name": self.mundo, "size_m": [self.w, self.h],
            "start": {"x": x, "y": y, "heading_deg": round(-math.degrees(th), 1)},
            "rooms": [{"name": n, "x": [x0, x1], "y": [y0, y1]} for n, x0, y0, x1, y1 in self.cuartos],
            "doors": [{"between": [a, b], "x": px, "y": py, "width_m": an} for a, b, px, py, an in self.puertas],
            "walls": [caja(b) for b in self.boxes if b.tipo == "pared"],
            "furniture": [{"name": b.name, **caja(b), "height_m": b.height} for b in self.boxes if b.tipo == "mueble"],
        }

    def vista(self, theta: float, cam_h: float, fov_deg: float, W: int = 640, H: int = 480):
        """Imagen BGR de la cámara en el robot mirando a theta (rad, mundo). La usa SyntheticCamera."""
        from .render3d import renderizar
        with self.lock:
            x, y = self.x, self.y
        return renderizar(self, x, y, theta, cam_h, fov_deg, W, H)

    def mover(self, nombre: str, x: float, y: float) -> bool:
        """Mueve un objeto (la persona) a (x, y). No lo deja encima del robot ni fuera del mundo."""
        with self.lock:
            b = next((b for b in self.boxes if b.name == nombre and b.tipo != "pared"), None)
            if b is None:
                return False
            hw, hh = (b.x1 - b.x0) / 2, (b.y1 - b.y0) / 2
            x, y = min(max(x, hw), self.w - hw), min(max(y, hh), self.h - hh)
            cx, cy = min(max(self.x, x - hw), x + hw), min(max(self.y, y - hh), y + hh)
            if (self.x - cx) ** 2 + (self.y - cy) ** 2 < (ROBOT_RADIUS + 0.03) ** 2:
                return False
            dx, dy = x - hw - b.x0, y - hh - b.y0
            b.x0, b.x1, b.y0, b.y1 = x - hw, x + hw, y - hh, y + hh
            for p in b.vis:
                p.mover(dx, dy)
            return True

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
                    "mundo": self.mundo, "habitacion_m": [self.w, self.h],
                    "zonas": [{"nombre": n, "x": x, "y": y} for n, x, y in self.zonas],
                    "obstaculos": [{"nombre": b.name, "tipo": b.tipo, "x": [b.x0, b.x1], "y": [b.y0, b.y1],
                                    "color": "#%02x%02x%02x" % b.color[::-1]} for b in self.boxes]}


def _cruce(b: Box, ox: float, oy: float, dx: float, dy: float) -> float | None:
    """Distancia a la que el rayo entra en la caja (slab test), o None si no la toca."""
    tmin, tmax = -math.inf, math.inf
    for o, d, lo, hi in ((ox, dx, b.x0, b.x1), (oy, dy, b.y0, b.y1)):
        if abs(d) < 1e-9:
            if not lo <= o <= hi:
                return None
            continue
        t1, t2 = (lo - o) / d, (hi - o) / d
        tmin, tmax = max(tmin, min(t1, t2)), min(tmax, max(t1, t2))
    return tmin if tmax >= max(tmin, 0) and tmin >= 0 else None
