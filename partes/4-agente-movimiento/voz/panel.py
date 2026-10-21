"""Live events for bb8-body's /voice page (POST /voice/event). Fire and forget:
a background thread posts them so the wake word loop never waits on HTTP."""
from __future__ import annotations

import logging
import queue
import threading

import httpx

from . import config as V

log = logging.getLogger("bb8.voz.panel")


class Panel:
    def __init__(self, url: str = V.MOTION_URL):
        self._url = url.rstrip("/") + "/voice/event"
        self._q: queue.Queue[dict] = queue.Queue(maxsize=200)
        threading.Thread(target=self._enviar, name="panel", daemon=True).start()

    def evento(self, tipo: str, **datos) -> None:
        try:
            self._q.put_nowait({"type": tipo, **datos})
        except queue.Full:
            pass

    def _enviar(self) -> None:
        with httpx.Client(timeout=2) as cli:
            while True:
                ev = self._q.get()
                try:
                    cli.post(self._url, json=ev)
                except (httpx.HTTPError, TypeError, ValueError) as e:   # never let the thread die
                    log.debug("panel: %s", e)
