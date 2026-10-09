"""Enlace serial con el Arduino: una orden, una respuesta, una a la vez.

Usa serial_for_url de pyserial, así el mismo código abre /dev/ttyACM0 en la Pi
o socket://127.0.0.1:5555 contra el Arduino simulado.
"""
from __future__ import annotations

import logging
import threading
import time

import serial

from . import protocol

log = logging.getLogger("bb8.serial")

REPLY_TIMEOUT_S = 0.2


class SerialLink:
    def __init__(self, url: str, baud: int = 115200):
        self.url = url
        self.baud = baud
        self._ser: serial.SerialBase | None = None
        self._lock = threading.Lock()
        self.ident = ""

    def open(self) -> str:
        self._ser = serial.serial_for_url(self.url, baudrate=self.baud, timeout=REPLY_TIMEOUT_S)
        if not self.url.startswith("socket://"):
            time.sleep(2.0)  # abrir el puerto reinicia el Uno/Nano
        self._ser.reset_input_buffer()
        self.ident = "OK " + self.send(protocol.CMD_IDENT)
        if not self.ident.startswith("OK BB8"):
            raise RuntimeError(f"El dispositivo en {self.url} no es un BB-8: {self.ident!r}")
        log.info("Arduino en %s: %s", self.url, self.ident)
        return self.ident

    def close(self) -> None:
        if self._ser:
            self._ser.close()
            self._ser = None

    def send(self, line: str) -> str:
        """Manda una línea y devuelve lo que va después de OK. ERR → ProtocolError."""
        if self._ser is None:
            raise RuntimeError("Puerto serial cerrado")
        with self._lock:
            self._ser.write((line + "\n").encode("ascii"))
            raw = self._ser.readline()
        if not raw:
            raise TimeoutError(f"El Arduino no respondió a {line!r}")
        reply = raw.decode("ascii", "replace").strip()
        log.debug("%s -> %s", line, reply)
        return protocol.parse_reply(reply)
