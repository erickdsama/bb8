"""Protocolo serial Pi ↔ Arduino (ver PROTOCOLO.md, sección 1).

Este módulo no abre puertos: solo arma y lee líneas. Lo usan el servicio de
movimiento (lado Pi) y el Arduino simulado (lado firmware), así ambos lados
hablan exactamente lo mismo.
"""
from __future__ import annotations

from dataclasses import dataclass, field

PWM_LIMIT = 255
HEAD_LIMIT = 90
TOF_LIMIT_MM = 4000
MAX_LINE = 63

FLAGS = ("obstacle", "tilt", "watchdog", "sleep", "bateria", "imu")


class ProtocolError(Exception):
    """El Arduino respondió ERR <motivo>."""

    def __init__(self, reason: str, line: str = ""):
        super().__init__(reason)
        self.reason = reason
        self.line = line


# --- Lado Pi: armar órdenes -------------------------------------------------

def _clamp(v: float, lim: int) -> int:
    return max(-lim, min(lim, int(round(v))))


def cmd_motors(left: float, right: float) -> str:
    return f"M {_clamp(left, PWM_LIMIT)} {_clamp(right, PWM_LIMIT)}"


def cmd_head(deg: float) -> str:
    return f"H {_clamp(deg, HEAD_LIMIT)}"


def cmd_distance(mm: float) -> str:
    return f"D {max(0, min(TOF_LIMIT_MM, int(mm)))}"


def cmd_poweroff(seconds: int) -> str:
    """P <s>: el Arduino corta la Pi en s segundos y duerme (reposo profundo). P 0 cancela."""
    return f"P {int(seconds)}"


CMD_STOP = "S"
CMD_STATUS = "?"
CMD_SLEEP = "Z"
CMD_WAKE = "W"
CMD_IDENT = "I"


# --- Lado Pi: leer respuestas -----------------------------------------------

def parse_reply(line: str) -> str:
    """Devuelve el texto después de OK, o lanza ProtocolError si es ERR."""
    line = line.strip()
    if line == "OK" or line.startswith("OK "):
        return line[3:]
    if line.startswith("ERR"):
        raise ProtocolError(line[4:].strip() or "unknown", line)
    raise ProtocolError("garbage", line)


@dataclass
class Status:
    battery_v: float = 0.0
    tilt_deg: float = 0.0
    yaw_deg: float = 0.0
    enc_left: int = 0
    enc_right: int = 0
    head_deg: int = 0
    tof_mm: int = 0
    flags: set[str] = field(default_factory=set)
    raw: str = ""


def parse_status(payload: str) -> Status:
    """Lee 'V=11.62 T=3.1 Y=-12.4 E=1420,1398 H=0 D=1830 F=-'. Ignora claves nuevas."""
    st = Status(raw=payload)
    for tok in payload.split():
        if "=" not in tok:
            continue
        k, v = tok.split("=", 1)
        try:
            if k == "V":
                st.battery_v = float(v)
            elif k == "T":
                st.tilt_deg = float(v)
            elif k == "Y":
                st.yaw_deg = float(v)
            elif k == "E":
                l, r = v.split(",")
                st.enc_left, st.enc_right = int(l), int(r)
            elif k == "H":
                st.head_deg = int(float(v))
            elif k == "D":
                st.tof_mm = int(float(v))
            elif k == "F":
                st.flags = set() if v == "-" else set(v.split(","))
        except ValueError:
            continue
    return st


# --- Lado firmware: leer órdenes y armar respuestas -------------------------

@dataclass
class Command:
    op: str
    args: tuple[int, ...] = ()


_ARITY = {"M": 2, "H": 1, "D": 1, "S": 0, "?": 0, "Z": 0, "W": 0, "I": 0, "P": 1}
_RANGES = {"M": (-PWM_LIMIT, PWM_LIMIT), "H": (-HEAD_LIMIT, HEAD_LIMIT), "D": (0, TOF_LIMIT_MM),
           "P": (0, 120)}


def parse_command(line: str) -> Command:
    """Valida una línea como lo haría el firmware. Lanza ProtocolError(syntax|range)."""
    toks = line.strip().split()
    if not toks or len(line) > MAX_LINE or toks[0] not in _ARITY:
        raise ProtocolError("syntax", line)
    op = toks[0]
    if len(toks) - 1 != _ARITY[op]:
        raise ProtocolError("syntax", line)
    try:
        args = tuple(int(t) for t in toks[1:])
    except ValueError:
        raise ProtocolError("syntax", line) from None
    lo_hi = _RANGES.get(op)
    if lo_hi and any(not lo_hi[0] <= a <= lo_hi[1] for a in args):
        raise ProtocolError("range", line)
    return Command(op, args)


def format_status(st: Status) -> str:
    flags = ",".join(sorted(st.flags)) or "-"
    return (f"OK V={st.battery_v:.2f} T={st.tilt_deg:.1f} Y={st.yaw_deg:.1f} "
            f"E={st.enc_left},{st.enc_right} H={st.head_deg} D={st.tof_mm} F={flags}")
