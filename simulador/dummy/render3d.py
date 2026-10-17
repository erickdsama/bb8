"""Cámara sintética en 3D: un rayo por píxel contra el mundo, con numpy.

Lo que ve la cámara de la cabeza (a 35 cm del piso) con una lente rectilínea:
piso de madera o azulejo, paredes con zoclo hasta el techo, las piezas de cada
mueble (cajas, cilindros y esferas de world.Prim) y una persona armada al vuelo
que siempre mira al robot. Luz difusa desde arriba, sombra suave donde los
muebles tocan el piso y algo de caída con la distancia.

Para que sea rápido, cada pieza solo se prueba contra los píxeles del rectángulo
de pantalla que ocupa.
"""
from __future__ import annotations

import math

import numpy as np

from .world import PARED, TECHO, Prim, World

LUZ = np.array([0.35, 0.45, 0.82]) / np.linalg.norm([0.35, 0.45, 0.82])
TECHO_COLOR = np.array([238, 240, 242], np.float32)
ZOCLO = np.array([235, 238, 240], np.float32)
JEANS, PIEL, PELO, OJOS = (110, 70, 50), (130, 170, 215), (30, 40, 55), (40, 30, 30)

# piezas: 0 pared exterior, 1 piso, 2 techo, 3+ piezas de muebles
PARED_EXT, PISO, TECHO_ID = 0, 1, 2


def persona(cx: float, cy: float, camara: tuple[float, float], playera) -> list[Prim]:
    """Piernas, torso, brazos, cuello y cabeza con pelo, ojos y boca, mirando a la cámara."""
    gx, gy = camara[0] - cx, camara[1] - cy
    n = math.hypot(gx, gy) or 1.0
    gx, gy = gx / n, gy / n          # hacia la cámara
    lx, ly = -gy, gx                 # a un lado
    at = lambda d, l: (cx + gx * d + lx * l, cy + gy * d + ly * l)  # noqa: E731
    p = []
    for lado in (-1, 1):
        x, y = at(0, 0.09 * lado)
        p += [Prim("cilindro", (35, 35, 35), (x, y, 0.08, 0, 0.07)),
              Prim("cilindro", JEANS, (x, y, 0.07, 0.07, 0.88))]
        x, y = at(0, 0.235 * lado)
        p += [Prim("cilindro", playera, (x, y, 0.055, 1.12, 1.42)),
              Prim("cilindro", PIEL, (x, y, 0.045, 0.82, 1.12)),
              Prim("esfera", PIEL, (x, y, 0.79, 0.05))]
        x, y = at(0.1, 0.04 * lado)
        p.append(Prim("esfera", OJOS, (x, y, 1.635, 0.016)))
    x, y = at(0, 0)
    p += [Prim("cilindro", playera, (x, y, 0.17, 0.86, 1.43)),
          Prim("esfera", playera, (x, y, 1.40, 0.17)),
          Prim("cilindro", PIEL, (x, y, 0.05, 1.42, 1.52)),
          Prim("esfera", PIEL, (x, y, 1.61, 0.112))]
    x, y = at(-0.025, 0)
    p.append(Prim("esfera", PELO, (x, y, 1.64, 0.115)))
    x, y = at(0.103, 0)
    p.append(Prim("esfera", (60, 60, 160), (x, y, 1.57, 0.014)))
    return p


def _piezas(world: World, cam: tuple[float, float]) -> list[Prim]:
    out = []
    for b in world.boxes:
        out += persona(*b.centro(), cam, b.color) if b.tipo == "persona" else b.piezas()
    return out


def _esquinas(p: Prim) -> np.ndarray:
    if p.tipo == "caja":
        x0, y0, z0, x1, y1, z1 = p.a
    elif p.tipo == "cilindro":
        cx, cy, r, z0, z1 = p.a
        x0, y0, x1, y1 = cx - r, cy - r, cx + r, cy + r
    else:
        cx, cy, cz, r = p.a
        x0, y0, z0, x1, y1, z1 = cx - r, cy - r, cz - r, cx + r, cy + r, cz + r
    return np.array([[x, y, z] for x in (x0, x1) for y in (y0, y1) for z in (z0, z1)])


def renderizar(world: World, x: float, y: float, theta: float, cam_h: float, fov_deg: float,
               W: int = 640, H: int = 480) -> np.ndarray:
    """Imagen BGR (H, W, 3) de lo que ve una cámara en (x, y, cam_h) mirando a theta (rad, mundo)."""
    f = (W / 2) / math.tan(math.radians(fov_deg / 2))
    fwd = np.array([math.cos(theta), math.sin(theta), 0.0])
    der = np.array([math.sin(theta), -math.cos(theta), 0.0])
    arriba = np.array([0.0, 0.0, 1.0])
    O = np.array([x, y, cam_h])
    u = ((np.arange(W) + 0.5 - W / 2) / f)[None, :, None]
    v = ((H / 2 - np.arange(H) - 0.5) / f)[:, None, None]
    D = fwd + u * der + v * arriba                       # (H, W, 3), sin normalizar

    with np.errstate(divide="ignore", invalid="ignore"):
        inv = 1.0 / D
        # paredes de afuera: el rayo sale del cuarto
        tx = np.where(D[..., 0] > 0, (world.w - x) * inv[..., 0], np.where(D[..., 0] < 0, -x * inv[..., 0], np.inf))
        ty = np.where(D[..., 1] > 0, (world.h - y) * inv[..., 1], np.where(D[..., 1] < 0, -y * inv[..., 1], np.inf))
        T = np.minimum(tx, ty)
        N = np.zeros((H, W, 3))
        ex = tx < ty
        N[..., 0] = np.where(ex, -np.sign(D[..., 0]), 0)
        N[..., 1] = np.where(ex, 0, -np.sign(D[..., 1]))
        quien = np.full((H, W), PARED_EXT, np.int16)
        tz = np.where(D[..., 2] < 0, -cam_h * inv[..., 2], np.where(D[..., 2] > 0, (TECHO - cam_h) * inv[..., 2], np.inf))
        m = tz < T
        T = np.where(m, tz, T)
        N[m] = 0
        N[..., 2] = np.where(m, -np.sign(D[..., 2]), N[..., 2])
        quien[m] = np.where(D[..., 2][m] < 0, PISO, TECHO_ID)

        piezas = _piezas(world, (x, y))
        for k, p in enumerate(piezas):
            r = _rect(p, O, fwd, der, arriba, f, W, H)
            if r is None:
                continue
            r0, r1, c0, c1 = r
            Ds, Ts = D[r0:r1, c0:c1], T[r0:r1, c0:c1]
            t, n = _cruce(p, O, Ds, inv[r0:r1, c0:c1])
            m = t < Ts
            if not m.any():
                continue
            Ts[m] = t[m]
            N[r0:r1, c0:c1][m] = n[m]
            quien[r0:r1, c0:c1][m] = 3 + k

    P = O + T[..., None] * D
    dist = T * np.linalg.norm(D, axis=-1)
    return _sombrear(world, piezas, P, N, quien, dist, D / np.linalg.norm(D, axis=-1, keepdims=True))


def _rect(p: Prim, O, fwd, der, arriba, f, W, H):
    """Rectángulo de pantalla (filas, columnas) que puede ocupar la pieza, o None si está detrás."""
    rel = _esquinas(p) - O
    d = rel @ fwd
    if d.max() < 0.01:
        return None
    if d.min() < 0.01:   # cruza el plano de la cámara: toda la pantalla
        return 0, H, 0, W
    xs = W / 2 + f * (rel @ der) / d
    ys = H / 2 - f * (rel @ arriba) / d
    c0, c1 = max(0, int(xs.min()) - 1), min(W, int(xs.max()) + 2)
    r0, r1 = max(0, int(ys.min()) - 1), min(H, int(ys.max()) + 2)
    return None if c0 >= c1 or r0 >= r1 else (r0, r1, c0, c1)


def _cruce(p: Prim, O, D, inv):
    """Distancia (inf si no toca) y normal del rayo contra una pieza."""
    sh = D.shape[:2]
    if p.tipo == "caja":
        lo, hi = np.array(p.a[:3]), np.array(p.a[3:])
        t1, t2 = (lo - O) * inv, (hi - O) * inv
        tmin, tmax = np.minimum(t1, t2), np.maximum(t1, t2)
        tn, tf = tmin.max(-1), tmax.min(-1)
        ok = (tf >= tn) & (tn > 1e-4)
        eje = tmin.argmax(-1)
        n = np.zeros(sh + (3,))
        np.put_along_axis(n, eje[..., None], -np.sign(np.take_along_axis(D, eje[..., None], -1)), -1)
        return np.where(ok, tn, np.inf), n
    if p.tipo == "esfera":
        c, r = np.array(p.a[:3]), p.a[3]
        oc = O - c
        a = (D * D).sum(-1)
        b = 2 * (D @ oc)
        disc = b * b - 4 * a * (oc @ oc - r * r)
        t = (-b - np.sqrt(np.maximum(disc, 0))) / (2 * a)
        ok = (disc >= 0) & (t > 1e-4)
        n = (O + t[..., None] * D - c) / r
        return np.where(ok, t, np.inf), n
    # cilindro vertical: costado y tapas
    cx, cy, r, z0, z1 = p.a
    ox, oy = O[0] - cx, O[1] - cy
    dx, dy, dz = D[..., 0], D[..., 1], D[..., 2]
    a = dx * dx + dy * dy
    b = 2 * (ox * dx + oy * dy)
    disc = b * b - 4 * a * (ox * ox + oy * oy - r * r)
    ts = (-b - np.sqrt(np.maximum(disc, 0))) / (2 * a)
    zs = O[2] + ts * dz
    ts = np.where((disc >= 0) & (ts > 1e-4) & (zs >= z0) & (zs <= z1), ts, np.inf)
    zc = np.where(O[2] > z1, z1, z0)   # la tapa que se ve desde la cámara
    tc = (zc - O[2]) * inv[..., 2]
    hx, hy = ox + tc * dx, oy + tc * dy
    tc = np.where((tc > 1e-4) & (hx * hx + hy * hy <= r * r), tc, np.inf)
    lado = ts <= tc
    t = np.minimum(ts, tc)
    n = np.zeros(sh + (3,))
    n[..., 0] = np.where(lado, (ox + ts * dx) / r, 0)
    n[..., 1] = np.where(lado, (oy + ts * dy) / r, 0)
    n[..., 2] = np.where(lado, 0, 1.0 if O[2] > z1 else -1.0)
    return t, n


def _hash(*a) -> np.ndarray:
    """Ruido determinista en [0, 1) a partir de enteros (vetas de madera, libros…)."""
    h = np.zeros_like(a[0], dtype=np.int64)
    for i, v in enumerate(a):
        h = h ^ (v.astype(np.int64) * (73856093, 19349663, 83492791)[i % 3])
    return ((h * 2654435761) % 4096) / 4096.0


def _sombrear(world: World, piezas: list[Prim], P, N, quien, dist, D) -> np.ndarray:
    H, W = quien.shape
    C = np.zeros((H, W, 3), np.float32)
    px, py, pz = P[..., 0], P[..., 1], P[..., 2]

    # piso: madera (tablas de 18 cm) o azulejo (40 cm)
    piso = quien == PISO
    if piso.any():
        fx, fy = px[piso], py[piso]
        fila = np.floor(fy / 0.18)
        tabla = np.floor(fx / 1.2 + fila * 0.37)
        tono = 0.85 + 0.25 * _hash(fila, tabla)
        junta = ((fy / 0.18) % 1 < 0.04) | (((fx / 1.2 + fila * 0.37) % 1) < 0.008)
        madera = np.array([95, 140, 185], np.float32) * tono[:, None]
        madera[junta] *= 0.6
        col = madera
        for x0, y0, x1, y1, patron in world.pisos:
            if patron == "azulejo":
                dentro = (fx >= x0) & (fx <= x1) & (fy >= y0) & (fy <= y1)
                ix, iy = np.floor(fx / 0.4), np.floor(fy / 0.4)
                az = np.where((((ix + iy) % 2) == 0)[:, None], [225, 228, 230], [200, 205, 210]).astype(np.float32)
                az[((fx / 0.4) % 1 < 0.03) | ((fy / 0.4) % 1 < 0.03)] = (150, 150, 150)
                col = np.where(dentro[:, None], az, col)
        # sombra suave junto a muebles y paredes
        ao = np.minimum(np.minimum(fx, world.w - fx), np.minimum(fy, world.h - fy))
        for p in piezas:
            if p.tipo == "caja" and p.a[2] < 0.05:
                ddx = np.maximum(np.maximum(p.a[0] - fx, fx - p.a[3]), 0)
                ddy = np.maximum(np.maximum(p.a[1] - fy, fy - p.a[4]), 0)
                ao = np.minimum(ao, np.hypot(ddx, ddy))
            elif p.tipo == "cilindro" and p.a[3] < 0.05:
                ao = np.minimum(ao, np.maximum(np.hypot(fx - p.a[0], fy - p.a[1]) - p.a[2], 0))
        C[piso] = col * (1 - 0.45 * np.exp(-ao / 0.12))[:, None]

    C[quien == TECHO_ID] = TECHO_COLOR

    # paredes de afuera
    ext = quien == PARED_EXT
    C[ext] = np.array(PARED, np.float32)

    # muebles
    for k, p in enumerate(piezas):
        m = quien == 3 + k
        if not m.any():
            continue
        col = np.broadcast_to(np.array(p.color, np.float32), (int(m.sum()), 3)).copy()
        if p.estilo == "libros":
            z, a = pz[m], px[m] + py[m]
            estante = np.floor(z / 0.36)
            tono = _hash(estante, np.floor(a / 0.035))
            paleta = np.array([[40, 40, 160], [140, 90, 40], [50, 120, 60], [60, 160, 200], [120, 60, 120], [200, 200, 200]], np.float32)
            col = paleta[(tono * len(paleta)).astype(int)]
            col[(z % 0.36) < 0.03] = (40, 70, 110)
        elif p.estilo == "pantalla":
            col[:] = (45, 35, 30)
        C[m] = col

    # zoclo en todas las paredes
    pared = ext.copy()
    for k, p in enumerate(piezas):
        if p.estilo == "pared":
            pared |= quien == 3 + k
    C[pared & (pz < 0.09)] = ZOCLO * 0.92

    # luz: difusa desde arriba + otra desde la cámara (D unitario), y caída con la distancia
    luz = 0.6 + 0.25 * np.clip(N @ LUZ, 0, 1) + 0.2 * np.clip(-(N * D).sum(-1), 0, 1)
    luz = np.where(piso | (quien == TECHO_ID), 0.95, luz)
    luz *= np.clip(1.12 - dist / 16, 0.6, 1.0)
    img = C * luz[..., None]
    return np.clip(img, 0, 255).astype(np.uint8)
