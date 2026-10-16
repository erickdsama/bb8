"""Plano de la casa y dónde está el robot en él.

El plano es un JSON (formato en wiki/Protocolo.md § 2, ruta en BB8_MAPA) con
cuartos, puertas, muros, muebles y "inicio": dónde arranca el robot. Ejes del
plano: x al este (derecha del dibujo), y al norte (arriba), rumbo en grados
horario desde el este (0 = este, 90 = sur), con el mismo signo que turn.

La odometría del servicio (x adelante y y a la derecha del arranque, rumbo
horario) se lleva al plano con un ancla: la pose de plano del punto de
arranque. set_position mueve el ancla para corregir la deriva.

La última pose se guarda en disco (BB8_MAPA_ESTADO) para no perderla si el
servicio se reinicia o la Pi duerme; al arrancar con ella, la posición queda
"incierta" hasta que alguien la confirme con set_position. También queda
incierta si el robot se inclina (¿lo cargaron?) o la IMU deja de responder.

Claude puede cambiar el plano (update_map): agregar, mover o quitar muebles y
abrir o cerrar puertas. Se escribe al mismo JSON, dejando una copia .bak.
"""
from __future__ import annotations

import json
import logging
import math
import os
import time
from collections import deque

log = logging.getLogger("bb8.mapa")

CELDA_M = 0.25      # resolución del croquis de texto
RADIO_ROBOT = 0.15  # para no aceptar una posición dentro de un muro
GUARDAR_CADA_S = 2.0


def _norm180(deg: float) -> float:
    deg = (deg + 180) % 360 - 180
    return 180.0 if deg == -180 else deg


def _dentro(o: dict, x: float, y: float, margen: float = 0.0) -> bool:
    return o["x"][0] - margen <= x <= o["x"][1] + margen and o["y"][0] - margen <= y <= o["y"][1] + margen


def _centro(o: dict) -> tuple[float, float]:
    return (o["x"][0] + o["x"][1]) / 2, (o["y"][0] + o["y"][1]) / 2


def _corta(o: dict, x0: float, y0: float, x1: float, y1: float) -> bool:
    """¿El segmento (x0, y0)→(x1, y1) cruza el rectángulo o? (slab test)."""
    dx, dy = x1 - x0, y1 - y0
    t0, t1 = 0.0, 1.0
    for p, d, lo, hi in ((x0, dx, o["x"][0], o["x"][1]), (y0, dy, o["y"][0], o["y"][1])):
        if abs(d) < 1e-9:
            if not lo <= p <= hi:
                return False
            continue
        a, b = sorted(((lo - p) / d, (hi - p) / d))
        t0, t1 = max(t0, a), min(t1, b)
        if t0 > t1:
            return False
    return True


def nombre_puerta(d: dict) -> str:
    return "↔".join(d["entre"])


class Mapa:
    def __init__(self, ruta: str, ruta_estado: str = ""):
        self.ruta = ruta
        self.ruta_estado = ruta_estado
        self.plano: dict | None = None
        self.error: str | None = None   # el JSON en disco no se pudo leer
        self._mtime = 0.0
        self.ancla: tuple[float, float, float] | None = None  # x, y, rumbo del arranque en el plano
        self.incierta: str | None = None                       # por qué no confiar en la pose
        self._recorrido_ancla = 0.0
        self._guardado_t = 0.0

    # --- archivo -----------------------------------------------------------------

    def cargar(self) -> bool:
        """Lee el plano si cambió en disco. Si el JSON está roto se queda con el anterior.

        El ancla solo se fija la primera vez: editar el plano no borra las correcciones.
        """
        if not self.ruta or not os.path.exists(self.ruta):
            return self.plano is not None
        mtime = os.path.getmtime(self.ruta)
        if mtime == self._mtime:
            return self.plano is not None
        self._mtime = mtime
        try:
            with open(self.ruta, encoding="utf-8") as f:
                plano = json.load(f)
            for k in ("tam_m", "cuartos"):
                if k not in plano:
                    raise ValueError(f"falta '{k}'")
        except (OSError, ValueError) as e:  # json.JSONDecodeError es ValueError y trae línea y columna
            self.error = f"{self.ruta}: {e}"
            log.error("Plano inválido, sigo con el anterior: %s", self.error)
            return self.plano is not None
        self.plano, self.error = plano, None
        log.info("Plano «%s» cargado de %s", plano.get("nombre"), self.ruta)
        if self.ancla is None:
            self._anclar_al_arrancar()
        return True

    def _anclar_al_arrancar(self) -> None:
        ini = self.plano.get("inicio", {"x": 0, "y": 0, "rumbo_deg": 0})
        self.ancla = (float(ini["x"]), float(ini["y"]), float(ini["rumbo_deg"]))
        try:
            with open(self.ruta_estado, encoding="utf-8") as f:
                e = json.load(f)
            if e.get("plano") == self.plano.get("nombre"):
                self.ancla = (float(e["x"]), float(e["y"]), float(e["rumbo_deg"]))
                self.incierta = "arranqué con la última posición guardada; pudieron moverme apagado"
                log.info("Pose guardada: %.2f, %.2f, %.0f°", *self.ancla)
        except (OSError, ValueError, KeyError):
            pass

    def _escribir_plano(self) -> None:
        """Guarda el plano (con copia .bak del anterior) sin dejarlo a medias."""
        if os.path.exists(self.ruta):
            os.replace(self.ruta, self.ruta + ".bak")
        tmp = self.ruta + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(self.plano, f, ensure_ascii=False, indent=1)
        os.replace(tmp, self.ruta)
        self._mtime = os.path.getmtime(self.ruta)

    def guardar_pose(self, odo: tuple[float, float, float], forzar: bool = False) -> None:
        """Deja la pose en disco cada pocos segundos para sobrevivir a un reinicio."""
        if not (self.ruta_estado and self.plano and self.ancla):
            return
        if not forzar and time.monotonic() - self._guardado_t < GUARDAR_CADA_S:
            return
        self._guardado_t = time.monotonic()
        x, y, r = self.a_plano(*odo)
        try:
            os.makedirs(os.path.dirname(self.ruta_estado) or ".", exist_ok=True)
            tmp = self.ruta_estado + ".tmp"
            with open(tmp, "w", encoding="utf-8") as f:
                json.dump({"plano": self.plano.get("nombre"), "x": round(x, 3), "y": round(y, 3),
                           "rumbo_deg": round(r, 1)}, f)
            os.replace(tmp, self.ruta_estado)
        except OSError as e:
            log.warning("No pude guardar la pose: %s", e)

    # --- odometría ↔ plano -------------------------------------------------------

    def a_plano(self, x_m: float, y_m: float, rumbo: float) -> tuple[float, float, float]:
        ax, ay, ar = self.ancla
        r = math.radians(ar)
        # adelante del arranque = (cos r, −sin r); derecha = (−sin r, −cos r)
        return (ax + x_m * math.cos(r) - y_m * math.sin(r),
                ay - x_m * math.sin(r) - y_m * math.cos(r),
                _norm180(ar + rumbo))

    def fijar(self, x: float, y: float, rumbo_deg: float | None, odo: tuple[float, float, float],
              recorrido: float) -> None:
        """Mueve el ancla para que la odometría actual caiga en (x, y, rumbo_deg)."""
        if not self.cargar():
            raise ValueError(self.error or "no hay plano: pon la ruta del JSON en BB8_MAPA")
        w, h = self.plano["tam_m"]
        if not (0 <= x <= w and 0 <= y <= h):
            raise ValueError(f"({x}, {y}) queda fuera de la casa (0…{w}, 0…{h})")
        muro = next((m for m in self.plano.get("muros", []) if _dentro(m, x, y, RADIO_ROBOT - 0.05)), None)
        if muro:
            raise ValueError(f"({x}, {y}) cae dentro de un muro ({muro['x']}, {muro['y']})")
        x_m, y_m, rumbo = odo
        if rumbo_deg is None:
            rumbo_deg = self.a_plano(*odo)[2]
        ar = rumbo_deg - rumbo
        r = math.radians(ar)
        self.ancla = (x - x_m * math.cos(r) + y_m * math.sin(r),
                      y + x_m * math.sin(r) + y_m * math.cos(r), _norm180(ar))
        self.incierta, self._recorrido_ancla = None, recorrido
        self.guardar_pose(odo, forzar=True)

    # --- cambios al plano (update_map) ---------------------------------------------

    def cambiar(self, d: dict) -> dict:
        if not self.cargar():
            raise ValueError(self.error or "no hay plano: pon la ruta del JSON en BB8_MAPA")
        accion, nombre = d.get("accion"), str(d.get("nombre") or "").strip()
        muebles, puertas = self.plano.setdefault("muebles", []), self.plano.get("puertas", [])
        w, h = self.plano["tam_m"]
        num = lambda k: float(d[k]) if d.get(k) is not None else None  # noqa: E731
        x, y = num("x"), num("y")
        if x is not None and y is not None and not (0 <= x <= w and 0 <= y <= h):
            raise ValueError(f"({x}, {y}) queda fuera de la casa (0…{w}, 0…{h})")
        if not nombre:
            raise ValueError("falta 'nombre'")
        mueble = next((m for m in muebles if m["nombre"] == nombre), None)

        if accion == "agregar_mueble":
            if mueble:
                raise ValueError(f"ya hay un mueble «{nombre}»; usa mover_mueble")
            if x is None or y is None:
                raise ValueError("agregar_mueble necesita x, y (centro)")
            an, la = num("ancho_m") or 0.4, num("largo_m") or 0.4
            muebles.append({"nombre": nombre, "x": [round(x - an / 2, 2), round(x + an / 2, 2)],
                            "y": [round(y - la / 2, 2), round(y + la / 2, 2)], "alto_m": num("alto_m") or 0.5})
        elif accion == "mover_mueble":
            if not mueble:
                raise ValueError(f"no hay mueble «{nombre}»; hay: {', '.join(m['nombre'] for m in muebles)}")
            if x is None or y is None:
                raise ValueError("mover_mueble necesita x, y (centro nuevo)")
            cx, cy = _centro(mueble)
            mueble["x"] = [round(v + x - cx, 2) for v in mueble["x"]]
            mueble["y"] = [round(v + y - cy, 2) for v in mueble["y"]]
        elif accion == "quitar_mueble":
            if not mueble:
                raise ValueError(f"no hay mueble «{nombre}»; hay: {', '.join(m['nombre'] for m in muebles)}")
            muebles.remove(mueble)
        elif accion in ("cerrar_puerta", "abrir_puerta"):
            lados = {s.strip() for s in nombre.replace("↔", "-").split("-")}
            puerta = next((p for p in puertas if set(p["entre"]) == lados), None)
            if not puerta:
                raise ValueError(f"no hay puerta «{nombre}»; hay: {', '.join(map(nombre_puerta, puertas)) or 'ninguna'}")
            puerta["cerrada"] = accion == "cerrar_puerta"
        else:
            raise ValueError("accion es agregar_mueble, mover_mueble, quitar_mueble, cerrar_puerta o abrir_puerta")
        self._escribir_plano()
        log.info("Plano cambiado: %s «%s»", accion, nombre)
        return {"resultado": "done"}

    # --- lo que ve Claude --------------------------------------------------------

    def _bloqueos(self) -> list[dict]:
        """Muros y puertas cerradas (una puerta cerrada se trata como un cuadro de su ancho)."""
        out = list(self.plano.get("muros", []))
        for p in self.plano.get("puertas", []):
            if p.get("cerrada"):
                r = p.get("ancho_m", 0.9) / 2
                out.append({"x": [p["x"] - r, p["x"] + r], "y": [p["y"] - r, p["y"] + r]})
        return out

    def _cuarto(self, x: float, y: float) -> tuple[str | None, str | None]:
        """(cuarto, puerta): el cuarto donde está, o la puerta si está en el hueco."""
        c = next((c["nombre"] for c in self.plano["cuartos"] if _dentro(c, x, y)), None)
        if c:
            return c, None
        for p in self.plano.get("puertas", []):
            if math.hypot(x - p["x"], y - p["y"]) <= p.get("ancho_m", 0.9) / 2 + 0.1:
                return None, nombre_puerta(p)
        return None, None

    def _ruta(self, desde: str | None, hasta: str) -> list[str] | None:
        """Puertas abiertas que hay que cruzar de un cuarto a otro (BFS); None si no se llega."""
        if desde is None:
            return None
        previo = {desde: None}
        cola = deque([desde])
        while cola:
            c = cola.popleft()
            if c == hasta:
                ruta = []
                while previo[c] is not None:
                    c, p = previo[c]
                    ruta.append(nombre_puerta(p))
                return ruta[::-1]
            for p in self.plano.get("puertas", []):
                if p.get("cerrada") or c not in p["entre"]:
                    continue
                otro = p["entre"][1] if p["entre"][0] == c else p["entre"][0]
                if otro not in previo:
                    previo[otro] = (c, p)
                    cola.append(otro)
        return None

    def describir(self, odo: tuple[float, float, float], recorrido: float) -> dict:
        if not self.cargar():
            return {"resultado": "error", "motivo": self.error or "no hay plano: pon la ruta del JSON en BB8_MAPA"}
        p = self.plano
        x, y, rumbo = self.a_plano(*odo)
        bloqueos = self._bloqueos()
        cuarto, en_puerta = self._cuarto(x, y)

        def hacia(tx: float, ty: float) -> dict:
            rumbo_obj = -math.degrees(math.atan2(ty - y, tx - x))  # horario desde el este
            directo = not any(_corta(b, x, y, tx, ty) for b in bloqueos)  # sin muros ni puertas cerradas
            return {"distancia_m": round(math.hypot(tx - x, ty - y), 2),
                    "girar_deg": round(_norm180(rumbo_obj - rumbo), 0), "directo": directo}

        def cuarto_info(c: dict) -> dict:
            out = {"nombre": c["nombre"], "x": c["x"], "y": c["y"], **hacia(*_centro(c))}
            ruta = self._ruta(cuarto, c["nombre"])
            if c["nombre"] != cuarto:
                out["ruta"] = ruta if ruta is not None else "sin camino por puertas abiertas"
            return out

        robot = {"x": round(x, 2), "y": round(y, 2), "rumbo_deg": round(rumbo, 1), "cuarto": cuarto}
        if en_puerta:
            robot["en_puerta"] = en_puerta
        posicion = {"incierta": self.incierta is not None,
                    "recorrido_desde_correccion_m": round(recorrido - self._recorrido_ancla, 1)}
        if self.incierta:
            posicion["motivo"] = self.incierta
        out = {
            "plano": p.get("nombre"), "tam_m": p.get("tam_m"), "robot": robot, "posicion": posicion,
            "cuartos": [cuarto_info(c) for c in p["cuartos"]],
            "puertas": [{"entre": d["entre"], "x": d["x"], "y": d["y"], "ancho_m": d.get("ancho_m"),
                         **({"cerrada": True} if d.get("cerrada") else {}), **hacia(d["x"], d["y"])}
                        for d in p.get("puertas", [])],
            "muebles": [{"nombre": m["nombre"], "x": m["x"], "y": m["y"], **hacia(*_centro(m))}
                        for m in p.get("muebles", [])],
            "croquis": self._croquis(x, y, rumbo),
        }
        if self.error:
            out["aviso"] = f"el plano en disco tiene un error, uso la versión anterior: {self.error}"
        return out

    def _croquis(self, x: float, y: float, rumbo: float) -> list[str]:
        """Norte arriba, una letra cada 25 cm: # muro, m mueble, . piso, = puerta cerrada, flecha el robot."""
        w, h = self.plano.get("tam_m", [0, 0])
        cols, filas = int(math.ceil(w / CELDA_M)), int(math.ceil(h / CELDA_M))
        rej = [["."] * cols for _ in range(filas)]
        celda = lambda cx, cy: (int(cx / CELDA_M), int((h - cy) / CELDA_M))  # noqa: E731

        def pinta(o: dict, letra: str, margen: float = 0.0) -> None:
            for f in range(filas):
                for c in range(cols):
                    if _dentro(o, (c + 0.5) * CELDA_M, h - (f + 0.5) * CELDA_M, margen):
                        rej[f][c] = letra

        for m in self.plano.get("muebles", []):
            pinta(m, "m")
        for m in self.plano.get("muros", []):
            pinta(m, "#", 0.07)
        for p in self.plano.get("puertas", []):
            if not p.get("cerrada"):
                continue
            c, f = celda(p["x"], p["y"])
            horizontal = any(0 <= c + k < cols and 0 <= f < filas and rej[f][c + k] == "#" for k in (-3, -2, 2, 3))
            r = int(p.get("ancho_m", 0.9) / 2 / CELDA_M + 0.5)
            for k in range(-r, r + 1):
                cc, ff = (c + k, f) if horizontal else (c, f + k)
                if 0 <= cc < cols and 0 <= ff < filas and rej[ff][cc] != "#":
                    rej[ff][cc] = "="
        c, f = celda(x, y)
        if 0 <= f < filas and 0 <= c < cols:
            rej[f][c] = "→↘↓↙←↖↑↗"[int(((rumbo % 360) + 22.5) // 45) % 8]
        borde = "#" * (cols + 2)
        return [borde] + ["#" + "".join(r) + "#" for r in rej] + [borde]
