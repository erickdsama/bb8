"""Convierte STL ASCII (lo que exporta OpenSCAD 2021) a STL binario, en su lugar.

    python3 cad/herramientas/stl_binario.py cad/stl/*.stl
"""
import struct
import sys


def convertir(ruta):
    with open(ruta, "rb") as f:
        inicio = f.read(5)
    if inicio != b"solid":
        return False
    facetas = []
    normal, vertices = None, []
    with open(ruta, encoding="ascii", errors="replace") as f:
        for linea in f:
            p = linea.split()
            if not p:
                continue
            if p[0] == "facet":
                normal = tuple(float(x) for x in p[2:5])
                vertices = []
            elif p[0] == "vertex":
                vertices.append(tuple(float(x) for x in p[1:4]))
            elif p[0] == "endfacet":
                facetas.append((normal, vertices))
    with open(ruta, "wb") as f:
        f.write(b"BB-8 cad/ OpenSCAD".ljust(80, b" "))
        f.write(struct.pack("<I", len(facetas)))
        for n, v in facetas:
            f.write(struct.pack("<12fH", *n, *v[0], *v[1], *v[2], 0))
    return True


if __name__ == "__main__":
    for r in sys.argv[1:]:
        if convertir(r):
            print("binario:", r)
