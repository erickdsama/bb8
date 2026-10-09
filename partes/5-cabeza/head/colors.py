NAMED = {
    "rojo": (255, 0, 0), "verde": (0, 255, 0), "azul": (40, 110, 255),
    "blanco": (255, 255, 255), "naranja": (255, 110, 0), "amarillo": (255, 210, 0),
    "morado": (150, 0, 255), "cian": (0, 220, 255), "apagado": (0, 0, 0),
}
PATTERNS = ("fijo", "respirar", "parpadeo", "apagado")
MAX_BRIGHTNESS = 0.3


def parse_color(c: str) -> tuple[int, int, int]:
    c = c.strip().lower()
    if c in NAMED:
        return NAMED[c]
    h = c.lstrip("#")
    if len(h) == 6:
        try:
            return int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)
        except ValueError:
            pass
    raise ValueError(f"color desconocido: {c!r}. Usa #RRGGBB o {', '.join(NAMED)}")
