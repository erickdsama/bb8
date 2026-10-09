"""Volumen, peso aproximado y tamaño de cada STL de cad/stl (para la tabla de la wiki).

    python3 cad/herramientas/masas.py
El peso supone pieza maciza en PLA (1.24 g/cm³): es un tope; con 15 % de relleno las
piezas gruesas pesan bastante menos. Los gajos del casco sí son casi macizos (4 mm de pared).
"""
import pathlib
import struct


def leer(ruta):
    datos = ruta.read_bytes()
    n = struct.unpack_from("<I", datos, 80)[0]
    tri = []
    for i in range(n):
        v = struct.unpack_from("<12f", datos, 84 + 50 * i)
        tri.append((v[3:6], v[6:9], v[9:12]))
    return tri


def volumen(tri):
    s = 0.0
    for a, b, c in tri:
        s += (a[0] * (b[1] * c[2] - b[2] * c[1]) - a[1] * (b[0] * c[2] - b[2] * c[0])
              + a[2] * (b[0] * c[1] - b[1] * c[0]))
    return abs(s) / 6


def caja(tri):
    p = [v for t in tri for v in t]
    return [max(q[i] for q in p) - min(q[i] for q in p) for i in range(3)]


if __name__ == "__main__":
    carpeta = pathlib.Path(__file__).resolve().parent.parent / "stl"
    for ruta in sorted(carpeta.glob("*.stl")):
        t = leer(ruta)
        v = volumen(t) / 1000
        x, y, z = caja(t)
        print(f"| `{ruta.name}` | {x:.0f} × {y:.0f} × {z:.0f} | {v:.0f} | {v * 1.24:.0f} |")
