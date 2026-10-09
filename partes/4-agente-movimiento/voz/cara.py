"""Parte 5: girar la cabeza hacia quien habla (plan B sin ReSpeaker).

Barre la cabeza a −45°, 0° y +45°, pide /cara a la Zero en cada posición y
centra la cara más grande. Si no ve a nadie, vuelve al frente.
"""
from __future__ import annotations

import logging

import httpx

from . import config as V

log = logging.getLogger("bb8.voz.cara")


async def mirar_a_quien_habla() -> float | None:
    async with httpx.AsyncClient(timeout=5) as cli:
        mejor: tuple[float, float] | None = None   # (área, ángulo)
        for ang in (0, -45, 45):
            r = (await cli.post(f"{V.MOTION_URL}/look_at", json={"grados": ang})).json()
            if r.get("resultado") != "done":
                log.info("No puedo girar la cabeza: %s", r)
                return None
            caras = (await cli.get(f"{V.HEAD_URL}/cara")).json().get("caras", [])
            for c in caras:
                # x va de −1 (izquierda de la foto) a 1 (derecha): grados a sumar a la cabeza.
                destino = ang + c["x"] * V.CAMARA_FOV_H / 2
                if mejor is None or c["area"] > mejor[0]:
                    mejor = (c["area"], destino)
            if mejor and ang == 0:
                break   # ya está al frente: no barrer
        final = round(max(-90, min(90, mejor[1]))) if mejor else 0
        await cli.post(f"{V.MOTION_URL}/look_at", json={"grados": final})
        log.info("Cara %s → cabeza a %d°", "encontrada" if mejor else "no encontrada", final)
        return final if mejor else None
