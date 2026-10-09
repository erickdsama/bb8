"""Arduino simulado: habla el protocolo serial por TCP (pyserial socket://).

Imita el firmware: rampa de 30 por ciclo de 10 ms, watchdog de 500 ms, freno por
inclinación > 35° y por D < 250 mm, servo del poste a ~300°/s, encoders y
giroscopio sacados del mundo simulado.
"""
from __future__ import annotations

import logging
import random
import socketserver
import threading
import time

from bb8 import config as C
from bb8 import protocol as P

from .world import World

log = logging.getLogger("dummy.arduino")

CYCLE_S = 0.01
RAMP = 30
WATCHDOG_S = 0.5
TILT_BRAKE, TILT_RELEASE = 35.0, 25.0
D_VALID_S = 0.3
SERVO_DEG_S = 300.0
FW = "sim-0.1"


class ArduinoSim:
    def __init__(self, world: World):
        self.world = world
        self.lock = threading.Lock()
        self.target = [0.0, 0.0]
        self.cur = [0.0, 0.0]
        self.enc = [0.0, 0.0]
        self.head = 0.0
        self.head_target = 0.0
        self.last_m = 0.0
        self.d_mm, self.d_t = 0, 0.0
        self.tilt = 0.0
        self.flags: set[str] = set()
        self.asleep = False
        self._stop = threading.Event()
        threading.Thread(target=self._loop, name="arduino-sim", daemon=True).start()

    # --- reflejos cada 10 ms -------------------------------------------------

    def _d_valid(self, now: float) -> int:
        return self.d_mm if self.d_mm > 0 and now - self.d_t < D_VALID_S else 0

    def _brake(self) -> None:
        self.target = [0.0, 0.0]
        self.cur = [0.0, 0.0]

    def _loop(self) -> None:
        prev = time.monotonic()
        while not self._stop.is_set():
            time.sleep(CYCLE_S)
            now = time.monotonic()
            dt, prev = now - prev, now
            with self.lock:
                accel = sum(abs(t - c) for t, c in zip(self.target, self.cur))
                self.tilt = self.world.tilt_deg(accel)
                if self.tilt > TILT_BRAKE:
                    if "tilt" not in self.flags:
                        log.warning("Inclinación %.0f° → freno", self.tilt)
                    self.flags.add("tilt")
                    self._brake()
                elif self.tilt < TILT_RELEASE:
                    self.flags.discard("tilt")

                d = self._d_valid(now)
                if d and d < C.OBSTACLE_MM:
                    if self.cur[0] + self.cur[1] > 0 or self.target[0] + self.target[1] > 0:
                        log.info("Obstáculo a %d mm → freno", d)
                        self._brake()
                    self.flags.add("obstacle")
                else:
                    self.flags.discard("obstacle")

                if self.target != [0.0, 0.0] and now - self.last_m > WATCHDOG_S:
                    log.warning("Watchdog: 500 ms sin M → freno")
                    self.flags.add("watchdog")
                    self._brake()

                for i in (0, 1):  # rampa (el PID real se abstrae aquí)
                    diff = self.target[i] - self.cur[i]
                    self.cur[i] += max(-RAMP, min(RAMP, diff))

                dl, dr = self.world.step(dt, *self.cur)
                slip = 1 + random.gauss(0, 0.01)
                self.enc[0] += dl * C.TICKS_PER_M * slip
                self.enc[1] += dr * C.TICKS_PER_M * slip

                step = SERVO_DEG_S * dt
                self.head += max(-step, min(step, self.head_target - self.head))

    # --- órdenes -------------------------------------------------------------

    def handle(self, line: str) -> str:
        try:
            cmd = P.parse_command(line)
        except P.ProtocolError as e:
            return f"ERR {e.reason}"
        now = time.monotonic()
        with self.lock:
            op, a = cmd.op, cmd.args
            if op == "M":
                if self.asleep:
                    return "ERR sleep"
                if "tilt" in self.flags:
                    return "ERR tilt"
                d = self._d_valid(now)
                if a[0] + a[1] > 0 and d and d < C.OBSTACLE_MM:
                    self._brake()
                    return "ERR obstacle"
                self.target = [float(a[0]), float(a[1])]
                self.last_m = now
                self.flags.discard("watchdog")
                return "OK"
            if op == "S":
                self._brake()
                return "OK"
            if op == "H":
                if self.asleep:
                    return "ERR sleep"
                self.head_target = float(a[0])
                return "OK"
            if op == "D":
                self.d_mm, self.d_t = a[0], now
                return "OK"
            if op == "?":
                st = P.Status(
                    battery_v=12.0 - random.uniform(0, 0.05), tilt_deg=self.tilt,
                    yaw_deg=self.world.yaw_deg() + random.gauss(0, 0.1),
                    enc_left=int(self.enc[0]), enc_right=int(self.enc[1]),
                    head_deg=int(round(self.head)), tof_mm=self._d_valid(now),
                    flags=self.flags | ({"sleep"} if self.asleep else set()))
                return P.format_status(st)
            if op == "Z":
                self._brake()
                self.asleep = True
                return "OK"
            if op == "W":
                self.asleep = False
                return "OK"
            if op == "I":
                return f"OK BB8 fw={FW}"
            if op == "P":  # reposo profundo: aquí solo se duerme, no hay Pi que cortar
                if a[0] and a[0] < 5:
                    return "ERR range"
                if a[0]:
                    log.warning("P %d: en el robot real la Pi se apagaría en %d s", a[0], a[0])
                    self._brake()
                    self.asleep = True
                return "OK"
        return "ERR syntax"

    def disconnected(self) -> None:
        with self.lock:
            self._brake()


def serve(sim: ArduinoSim, host: str = "127.0.0.1", port: int = 5555) -> socketserver.TCPServer:
    class Handler(socketserver.StreamRequestHandler):
        def handle(self):
            log.info("Pi conectada desde %s", self.client_address)
            for raw in self.rfile:
                reply = sim.handle(raw.decode("ascii", "replace").strip())
                self.wfile.write((reply + "\n").encode("ascii"))
            log.info("Pi desconectada")
            sim.disconnected()

    socketserver.ThreadingTCPServer.allow_reuse_address = True
    srv = socketserver.ThreadingTCPServer((host, port), Handler)
    srv.daemon_threads = True
    threading.Thread(target=srv.serve_forever, name="arduino-tcp", daemon=True).start()
    log.info("Arduino simulado en socket://%s:%d", host, port)
    return srv
