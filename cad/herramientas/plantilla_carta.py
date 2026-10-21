"""Parte la plantilla de la plataforma (210 mm) en 2 hojas carta horizontales, a escala 1:1.

    python3 cad/herramientas/plantilla_carta.py      # escribe cad/plantillas/plataforma_carta.pdf

Lee cad/plantillas/plataforma.svg (la exporta exportar.sh) y escribe una hoja por mitad,
con 10 mm de traslape, cruces para alinear y una regla de 100 mm para revisar que la
impresora no escaló. Necesita Inkscape (SVG → PDF) y pdfunite (poppler-utils).
"""
import pathlib
import re
import subprocess
import tempfile

AQUI = pathlib.Path(__file__).resolve().parent.parent / "plantillas"
ANCHO, ALTO = 279.4, 215.9            # carta horizontal, mm
TRASLAPE = 5                          # cada hoja pasa 5 mm del centro: 10 mm en total

svg = (AQUI / "plataforma.svg").read_text()
cuerpo = re.search(r"<path.*?/>", svg, re.S).group(0)
cuerpo = cuerpo.replace('fill="lightgray"', 'fill="none"').replace('stroke-width="0.5"', 'stroke-width="0.3"')
vb = list(map(float, re.search(r'viewBox="([^"]+)"', svg).group(1).split()))   # x0 y0 ancho alto
x0, y0, w, h = vb


def hoja(n, y_desde, y_hasta, titulo, pega):
    alto = y_hasta - y_desde
    ox = (ANCHO - w) / 2 - x0               # traslada la plantilla al centro de la hoja
    oy = 30 - y_desde
    cruz = lambda x, y: (f'<path d="M {x - 6},{y} H {x + 6} M {x},{y - 6} V {y + 6}" stroke="black" stroke-width="0.25"/>'
                         f'<circle cx="{x}" cy="{y}" r="2.5" fill="none" stroke="black" stroke-width="0.25"/>')
    linea_y = 0 + oy                        # el centro de la plataforma (y = 0)
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="{ANCHO}mm" height="{ALTO}mm" viewBox="0 0 {ANCHO} {ALTO}">
<rect width="{ANCHO}" height="{ALTO}" fill="white"/>
<defs><clipPath id="c"><rect x="{x0 + ox - 1}" y="{y_desde + oy}" width="{w + 2}" height="{alto}"/></clipPath></defs>
<g clip-path="url(#c)"><g transform="translate({ox},{oy})">{cuerpo}</g></g>
{f'<rect x="{x0 + ox - 4}" y="{linea_y}" width="{w + 8}" height="{TRASLAPE}" fill="#000" fill-opacity="0.08"/>' if n == 2 else ""}
<path d="M {x0 + ox - 8},{linea_y} H {x0 + ox + w + 8}" stroke="black" stroke-width="0.35" stroke-dasharray="3 2"/>
{cruz(ox - 75, linea_y)}{cruz(ox + 75, linea_y)}
<g font-family="sans-serif" fill="black">
<text x="12" y="14" font-size="6" font-weight="bold">BB-8 · plataforma 1:1 · hoja {n} de 2 · {titulo}</text>
<text x="12" y="21" font-size="3.6">{pega}</text>
<text x="12" y="{ALTO - 22}" font-size="3.6">Imprime al 100 % ("tamaño real", sin "ajustar a la página"). La regla debe medir 100 mm exactos.</text>
<path d="M 12,{ALTO - 14} h 100 M 12,{ALTO - 17} v 6 M 62,{ALTO - 16} v 4 M 112,{ALTO - 17} v 6" stroke="black" stroke-width="0.4"/>
<text x="12" y="{ALTO - 6}" font-size="3.2">0</text><text x="58" y="{ALTO - 6}" font-size="3.2">50</text><text x="104" y="{ALTO - 6}" font-size="3.2">100 mm</text>
<text x="{ANCHO - 12}" y="{ALTO - 6}" font-size="3.2" text-anchor="end">Las muescas de los lados son para las ruedas · centro = agujero de 2 mm</text>
</g>
</svg>"""


with tempfile.TemporaryDirectory() as tmp:
    pdfs = []
    for n, (a, b, t, p) in enumerate([
        (y0, TRASLAPE, "ADELANTE (Raspberry Pi)", "Recorta esta hoja por la línea punteada y pégala ENCIMA de la hoja 2: su borde sobre la línea punteada de la hoja 2 y las medias cruces formando cruces completas."),
        (-TRASLAPE, y0 + h, "ATRÁS (Arduino)", "Esta hoja va abajo. La franja gris es para el pegamento; la hoja 1 la tapa hasta la línea punteada."),
    ], 1):
        s = pathlib.Path(tmp) / f"h{n}.svg"
        s.write_text(hoja(n, a, b, t, p))
        pdf = s.with_suffix(".pdf")
        subprocess.run(["inkscape", str(s), "--export-type=pdf", f"--export-filename={pdf}"], check=True, capture_output=True)
        pdfs.append(str(pdf))
    subprocess.run(["pdfunite", *pdfs, str(AQUI / "plataforma_carta.pdf")], check=True)
print(AQUI / "plataforma_carta.pdf")
