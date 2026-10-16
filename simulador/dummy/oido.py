"""ReSpeaker 2-Mic HAT simulado: de dónde viene una voz en el mundo del dummy.

En el robot real el HAT va sobre la Pi 3, dentro del cuerpo, con sus dos cápsulas
a la izquierda y a la derecha. Con dos micrófonos solo se mide el retraso entre
ellos, que depende del seno del ángulo: una voz a 30° a la derecha por delante y
otra a 150° (30° por detrás) suenan igual. Por eso el ángulo va de −90 a 90.

Aquí se imita eso: ángulo respecto al frente del cuerpo (+ derecha), plegado
adelante/atrás, con ruido, y más débil y más ruidoso si hay muros de por medio.
El resultado se manda al servicio de movimiento (POST /oido), igual que lo hará
el agente de voz con el ReSpeaker de verdad.
"""
from __future__ import annotations

import logging
import math
import random
import time

import httpx

from bb8 import config as C

from .world import World, _cruce

log = logging.getLogger("dummy.oido")

NIVEL_1M_DB = 62.0     # voz normal a 1 m
UMBRAL_DB = 30.0       # por debajo, el micrófono no la distingue del ruido
MURO_DB = 12.0         # lo que se pierde por cada muro de por medio
RUIDO_SENO = 0.05      # ruido del seno del ángulo sin muros (~3° al frente)
RUIDO_MURO = 0.2       # ruido extra por muro: el sonido llega rebotado


class OidoSim:
    def __init__(self, world: World):
        self.world = world
        self.ultimo: dict | None = None  # para el visor

    def _muros(self, x0: float, y0: float, x1: float, y1: float) -> int:
        d = math.hypot(x1 - x0, y1 - y0)
        if d < 1e-6:
            return 0
        dx, dy = (x1 - x0) / d, (y1 - y0) / d
        n = 0
        for b in self.world.boxes:
            if b.tipo == "pared":
                t = _cruce(b, x0, y0, dx, dy)
                n += t is not None and t < d
        return n

    def escuchar(self, texto: str, x: float, y: float) -> dict:
        """Una voz en (x, y): lo que reportaría el ReSpeaker."""
        w = self.world
        with w.lock:
            rx, ry, th = w.x, w.y, w.theta
        d = math.hypot(x - rx, y - ry)
        muros = self._muros(x, y, rx, ry)
        nivel = NIVEL_1M_DB - 20 * math.log10(max(d, 0.3)) - MURO_DB * muros
        # ángulo verdadero respecto al frente del cuerpo, + derecha (el mundo es antihorario)
        real = -math.degrees(math.atan2(y - ry, x - rx) - th)
        real = (real + 180) % 360 - 180
        evento = {"texto": texto, "fuente": [round(x, 2), round(y, 2)], "t": time.time(),
                  "distancia_m": round(d, 2), "muros": muros, "nivel_db": round(nivel, 1),
                  "real_deg": round(real, 1), "theta_deg": math.degrees(th)}
        if nivel < UMBRAL_DB:
            evento |= {"oido": False, "angulo_deg": None}
        else:
            s = math.sin(math.radians(real)) + random.gauss(0, RUIDO_SENO + RUIDO_MURO * muros)
            evento |= {"oido": True, "angulo_deg": round(math.degrees(math.asin(max(-1.0, min(1.0, s)))), 1)}
        self.ultimo = evento
        log.info("👂 «%s» desde %.1f m, %d muro(s): %.0f dB, ángulo %s (real %.0f°)",
                 texto, d, muros, nivel, evento["angulo_deg"], real)
        return evento

    async def entregar(self, evento: dict) -> bool:
        """Manda lo oído al servicio de movimiento, como el agente de voz con el HAT real."""
        if not evento["oido"]:
            return False
        try:
            async with httpx.AsyncClient(base_url=C.MOTION_URL, timeout=2) as cli:
                r = await cli.post("/hearing", json={"text": evento["texto"], "angle_deg": evento["angulo_deg"],
                                                    "level_db": evento["nivel_db"]})
                return r.status_code == 200
        except httpx.HTTPError as e:
            log.warning("No pude avisar al servicio de movimiento: %s", e)
            return False
