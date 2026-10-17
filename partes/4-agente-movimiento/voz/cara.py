"""Turn the head toward whoever spoke, using the mic direction (ReSpeaker 2-Mic).

The old face sweep (/cara with OpenCV on the head) is gone: the Go head does not
run OpenCV. Now the head looks at the first sound candidate (front side) from
GET /hearing; the model can still take_photo to check.
"""
from __future__ import annotations

import logging

import httpx

from . import config as V

log = logging.getLogger("bb8.voz.cara")


async def mirar_a_quien_habla() -> float | None:
    async with httpx.AsyncClient(base_url=V.MOTION_URL, timeout=5) as cli:
        oido = (await cli.get("/hearing")).json()
        cands = oido.get("candidates_deg") or []
        if not oido.get("heard") or not cands or oido.get("ago_s", 99) > 5:
            return None
        final = round(max(-90, min(90, cands[0])))
        r = (await cli.post("/tools/look_at", json={"degrees": final})).json()
        if r.get("result") != "done":
            log.info("Cannot turn the head: %s", r)
            return None
        log.info("Voice at %s° → head to %d°", cands[0], final)
        return final
