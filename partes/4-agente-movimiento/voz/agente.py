"""BB-8's brain lives in bb8-body (Go): an OpenRouter agent over the shared tools.

The voice front end only sends what it heard: POST /agent/ask on the local API
(127.0.0.1:8770) with the text, who spoke and whether that voice is known. The
movement lock for unknown voices is enforced in Go, in code.

Try it without a microphone, with the dummy running and agent.api_key set in the
config page:
    python -m voz.agente "da una vuelta y dime qué ves"
"""
from __future__ import annotations

import argparse
import asyncio
import logging

import httpx

from head.personalidad import frase

from . import config as V

log = logging.getLogger("bb8.voz.agente")


class Agente:
    def __init__(self, url: str = V.MOTION_URL):
        self._http = httpx.AsyncClient(base_url=url, timeout=120)

    async def conectar(self) -> None:
        r = await self._http.get("/agent")
        estado = r.json()
        if not estado.get("ready"):
            log.warning("Agent not ready: %s", estado.get("detail"))
        log.info("Agent: %s", estado.get("model"))

    async def cerrar(self) -> None:
        await self._http.aclose()

    async def turno(self, texto: str, hablante: str | None) -> str:
        """One spoken order → final text reply (to speak).

        hablante None = unregistered voice; "" = no voice registry (everyone counts as known).
        """
        # "" = no voices registered (voice ID off): the body lets it move, and the
        # model must not read it as a stranger ("someone" made it refuse).
        quien = "unknown" if hablante is None else (hablante or "voice ID off")
        # speaker: the registered name, so the body applies that person's profile.
        r = await self._http.post("/agent/ask", json={"text": f"[Speaker: {quien}] {texto}",
                                                      "speaker": hablante or "",
                                                      "speaker_known": hablante is not None})
        out = r.json()
        if r.status_code != 200:
            raise RuntimeError(out.get("error", f"HTTP {r.status_code}"))
        if out.get("tools_used"):
            log.info("→ %s", ", ".join(out["tools_used"]))
        return out.get("reply") or frase("enredado")


async def _cli(texto: str, hablante: str | None) -> None:
    ag = Agente()
    await ag.conectar()
    try:
        print(await ag.turno(texto, hablante))
    finally:
        await ag.cerrar()


def main() -> None:
    ap = argparse.ArgumentParser(description="One text order to the agent (no microphone)")
    ap.add_argument("texto")
    ap.add_argument("--hablante", default="Erick", help="'' = unknown voice")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(message)s")
    logging.getLogger("httpx").setLevel(logging.WARNING)
    asyncio.run(_cli(a.texto, a.hablante or None))


if __name__ == "__main__":
    main()
