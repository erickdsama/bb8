"""Pitidos de droide generados con matemática pura (sin archivos WAV ni numpy).

Cada sonido es una lista de sílabas: un barrido de frecuencia con vibrato y un timbre.
Cada vez que suena cambia un poco el tono y el tempo (hay 4 variantes en caché), para
que no parezca grabado. Un texto cualquiera da un balbuceo propio: mismo texto, mismos
pitidos, con la entonación de su puntuación (¿…? sube al final, ¡…! va más agudo).

Escuchar en el PC:
    python -m head.sounds                    # todos, con su descripción
    python -m head.sounds feliz curioso      # algunos
    python -m head.sounds --texto "¿Quién anda ahí?"
    python -m head.sounds --guardar sonidos/ # un WAV por sonido
    python -m head.sounds --html sonidos.html  # página para oírlos en el navegador
"""
from __future__ import annotations

import argparse
import base64
import functools
import io
import math
import os
import platform
import random
import re
import shutil
import subprocess
import sys
import tempfile
import time
import wave
from array import array
from typing import NamedTuple

RATE = 22050
TAU = 2 * math.pi


class Silaba(NamedTuple):
    f0: float               # Hz al empezar; 0 = silencio
    f1: float               # Hz al terminar
    dur: float              # segundos
    vib: float = 0.0        # vibrato: fracción de la frecuencia (0.05 = ±5 %)
    vib_hz: float = 0.0     # vibrato: oscilaciones por segundo
    timbre: str = "suave"   # suave, brillante o zumbido
    vol: float = 1.0


def S(f0, f1, dur, vib=0.0, vib_hz=0.0, timbre="suave", vol=1.0) -> Silaba:
    return Silaba(f0, f1, dur, vib, vib_hz, timbre, vol)


def P(dur: float) -> Silaba:
    return Silaba(0, 0, dur)


def _trino(fa: float, fb: float, n: int, dur: float = 0.035, **kw) -> list[Silaba]:
    return [S(fa if i % 2 == 0 else fb, fb if i % 2 == 0 else fa, dur, **kw) for i in range(n)]


# Armónicos (multiplicador, peso) por timbre.
TIMBRES = {
    "suave": ((1, 0.8), (2, 0.2)),
    "brillante": ((1, 0.6), (2, 0.25), (3, 0.15)),
    "zumbido": ((1, 0.55), (3, 0.25), (5, 0.12), (7, 0.08)),   # casi cuadrada: gruñón
}

# nombre → (cómo suena, sílabas). El nombre es también la emoción (head.personalidad).
SONIDOS: dict[str, tuple[str, list[Silaba]]] = {
    "feliz": ("chirridos que suben, saltarines", [
        S(900, 1500, 0.08), S(1500, 2200, 0.07, 0.03, 18), P(0.03),
        S(1200, 1900, 0.09), S(1900, 2500, 0.06, timbre="brillante")]),
    "emocionado": ("trino rápido y agudo que se dispara hacia arriba", [
        *_trino(1600, 2300, 6, 0.03, timbre="brillante"), P(0.02),
        S(1200, 2900, 0.22, 0.05, 22, "brillante")]),
    "saludo": ("bi-du-íiip: dos notas y una larga que sube ondulando", [
        S(1100, 1100, 0.06), P(0.03), S(1500, 1300, 0.06), P(0.04),
        S(900, 1900, 0.30, 0.04, 9)]),
    "curioso": ("sube y baja como preguntándose algo", [
        S(700, 1100, 0.09), S(1100, 850, 0.08), P(0.04), S(850, 1700, 0.20, 0.03, 7)]),
    "pregunta": ("nota plana y luego una subida: ¿eh?", [
        S(700, 700, 0.10), P(0.02), S(800, 1600, 0.18)]),
    "pensando": ("blips suaves sin prisa, tecleando por dentro", [
        S(950, 950, 0.04, vol=0.6), P(0.07), S(1150, 1150, 0.04, vol=0.6), P(0.05),
        S(1050, 1050, 0.04, vol=0.6), P(0.09), S(1250, 1200, 0.05, vol=0.6), P(0.06),
        S(1000, 1080, 0.06, vol=0.6)]),
    "si": ("dos blips que suben: ¡ajá!", [
        S(1000, 1050, 0.06), P(0.03), S(1400, 1700, 0.09)]),
    "no": ("dos notas que bajan, un poco gruñonas", [
        S(900, 750, 0.09, timbre="zumbido", vol=0.7), P(0.04),
        S(700, 450, 0.16, timbre="zumbido", vol=0.7)]),
    "alerta": ("tres pitidos secos y agudos", [
        S(1800, 1800, 0.06, timbre="brillante"), P(0.04)] * 3),
    "asustado": ("grito agudo que tiembla y un trino nervioso", [
        S(2400, 1400, 0.12, 0.08, 30, "brillante"), S(1500, 2800, 0.18, 0.10, 26, "brillante"),
        P(0.02), *_trino(2600, 2000, 6, 0.025)]),
    "triste": ("dos notas largas que caen, con temblor lento", [
        S(900, 500, 0.30, 0.03, 5), P(0.05), S(600, 320, 0.45, 0.04, 4)]),
    "error": ("zumbido grave: algo salió mal", [
        S(420, 420, 0.12, timbre="zumbido"), P(0.05), S(320, 240, 0.28, 0.02, 12, "zumbido")]),
    "bostezo": ("sube despacio y cae largo: tengo sueño", [
        S(450, 850, 0.35, vol=0.7), S(850, 330, 0.65, 0.02, 3, vol=0.6)]),
    "despertar": ("barrido de grave a agudo y dos chirridos: sistemas en línea", [
        S(300, 1800, 0.35, 0.01, 6), P(0.04), S(1500, 2000, 0.05), P(0.02), S(1800, 2300, 0.06)]),
    "risa": ("ji-ji-ji electrónico que baja", [
        *(s for i in range(5) for s in (S(1900 - 120 * i, 1500 - 120 * i, 0.05), P(0.025)))]),
}


# ── Síntesis ─────────────────────────────────────────────────────────────────

def _render(silabas: list[Silaba], volume: float, tono: float = 1.0, tempo: float = 1.0) -> array:
    out = array("h")
    sin = math.sin
    phase = 0.0
    for s in silabas:
        n = int(RATE * s.dur / tempo)
        if s.f0 <= 0:
            out.extend([0] * n)
            continue
        f0, f1 = s.f0 * tono, s.f1 * tono
        ratio = f1 / f0
        harm = TIMBRES[s.timbre]
        amp = volume * s.vol * 32767
        ataque, cola = max(1, int(0.004 * RATE)), max(1, int(0.012 * RATE))
        wv = TAU * s.vib_hz / RATE
        for i in range(n):
            x = i / n
            f = f0 * ratio ** x     # barrido exponencial: suena parejo al oído
            if s.vib:
                f *= 1 + s.vib * sin(wv * i)
            phase += TAU * f / RATE
            env = min(1.0, i / ataque, (n - i) / cola)
            v = 0.0
            for k, w in harm:
                v += w * sin(k * phase)
            out.append(int(amp * env * v))
    return out


def _wav(muestras: array) -> bytes:
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        if sys.byteorder == "big":
            muestras = array("h", muestras)
            muestras.byteswap()
        w.writeframes(muestras.tobytes())
    return buf.getvalue()


def balbuceo(texto: str) -> list[Silaba]:
    """Pitidos para acompañar una frase: ~1 por cada dos sílabas, entre 2 y 7."""
    rnd = random.Random(texto)                    # mismo texto → mismos pitidos
    t = texto.strip()
    silabas = len(re.findall(r"[aeiouáéíóúü]+", t.lower())) or len(t) // 3
    n = max(2, min(7, silabas // 2))
    pregunta, exclama = "?" in t[-2:], "!" in t
    base = rnd.uniform(850, 1250) * (1.25 if exclama else 1.0)
    out: list[Silaba] = []
    for i in range(n):
        f = base * rnd.uniform(0.75, 1.4)
        if i == n - 1:                            # la entonación se nota en la última
            g = f * (1.7 if pregunta else 1.3 if exclama else 0.65)
            out.append(S(f, g, rnd.uniform(0.10, 0.16), 0.03 if pregunta else 0, 8))
        else:
            out.append(S(f, f * rnd.uniform(0.85, 1.25), rnd.uniform(0.04, 0.09) / (1.3 if exclama else 1)))
            out.append(P(rnd.uniform(0.012, 0.04)))
    return out


@functools.lru_cache(maxsize=128)
def _cached(name: str, variante: int, volume: float) -> tuple[bytes, float]:
    if name in SONIDOS:
        silabas = SONIDOS[name][1]
    else:
        silabas, variante = balbuceo(name), 0     # el balbuceo de un texto no varía
    rnd = random.Random(f"{name}/{variante}")
    tono, tempo = (1.0, 1.0) if variante == 0 else (rnd.uniform(0.94, 1.07), rnd.uniform(0.92, 1.08))
    m = _render(silabas, volume, tono, tempo)
    return _wav(m), round(len(m) / RATE, 3)


def beep_wav(name: str, volume: float = 0.4, variar: bool = True) -> tuple[bytes, float]:
    """WAV mono 16 bit y su duración. name es un sonido de SONIDOS o cualquier texto (balbuceo).

    Con variar=True cada llamada toma una de 4 variantes (tono y tempo un poco distintos).
    """
    return _cached(name, random.randrange(4) if variar else 0, volume)


# ── Reproducir y escuchar en el PC ───────────────────────────────────────────

def reproducir(wav: bytes, esperar: bool = True) -> None:
    """Reproduce un WAV en el PC (Windows, macOS o Linux con aplay/paplay/ffplay). Mejor esfuerzo."""
    try:
        if sys.platform == "win32":
            import winsound
            winsound.PlaySound(wav, winsound.SND_MEMORY)
            return
        if platform.system() == "Darwin":
            player = ["afplay"]
        else:
            player = next(([p, *extra] for p, extra in (("aplay", ["-q"]), ("paplay", []),
                           ("ffplay", ["-nodisp", "-autoexit", "-loglevel", "quiet"])) if shutil.which(p)), None)
            if player is None:
                return
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            f.write(wav)
        p = subprocess.Popen([*player, f.name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        if esperar:
            p.wait(timeout=10)
            os.unlink(f.name)
    except Exception:
        pass


def _contorno_svg(silabas: list[Silaba], ancho: int = 180, alto: int = 40) -> str:
    """Curva de tono (escala logarítmica, 250 a 3000 Hz) de un sonido, como SVG."""
    total = sum(s.dur for s in silabas)
    lo, hi = math.log(250), math.log(3000)
    trazos, t = [], 0.0
    for s in silabas:
        if s.f0 > 0:
            pts = []
            for k in range(9):
                x, f = k / 8, s.f0 * (s.f1 / s.f0) ** (k / 8)
                pts.append(f"{(t + x * s.dur) / total * ancho:.1f},{alto - (math.log(f) - lo) / (hi - lo) * alto:.1f}")
            trazos.append(f'<polyline points="{" ".join(pts)}"/>')
        t += s.dur
    return (f'<svg viewBox="0 -3 {ancho} {alto + 6}" width="{ancho}" height="{alto + 6}" aria-hidden="true">'
            f'{"".join(trazos)}</svg>')


def pagina_html(frases: list[str] | None = None, completa: bool = True) -> str:
    """Página con todos los sonidos (3 variantes cada uno) y frases de ejemplo, en WAV embebido.

    completa=False omite doctype/html/head/body (para publicarla como Artifact)."""
    from .colors import NAMED
    from .personalidad import EMOCIONES

    def audio(wav: bytes) -> str:
        return f'<audio preload="none" src="data:audio/wav;base64,{base64.b64encode(wav).decode()}"></audio>'

    def boton(etiqueta: str, wav: bytes, titulo: str) -> str:
        return f'<button type="button" title="{titulo}">{etiqueta}</button>{audio(wav)}'

    filas = []
    for name, (desc, silabas) in SONIDOS.items():
        emo = EMOCIONES[name]
        rgb = "#%02x%02x%02x" % NAMED[emo.color]
        wav0, secs = _cached(name, 0, 0.5)
        botones = boton("▶", wav0, "versión base") + "".join(
            boton(str(v + 1), _cached(name, v, 0.5)[0], f"variante {v + 1}") for v in (1, 2))
        filas.append(f"""<li class="son">
  <div class="cab"><span class="ojo {emo.patron}" style="--ojo:{rgb}"></span><h3>{name}</h3>
    <span class="dur">{secs:.2f} s</span></div>
  <p>{desc}</p>
  <p class="cuando">Cuándo: {emo.cuando}. Ojo {emo.color}, {emo.patron}.</p>
  <div class="pie">{_contorno_svg(silabas)}<div class="botones">{botones}</div></div>
</li>""")
    ejemplos = "".join(f'<li class="frase"><span>«{t}»</span>{boton("▶", _cached(t, 0, 0.5)[0], "balbuceo")}</li>'
                       for t in (frases or []))
    cuerpo = f"""<title>Sonidos de BB-8</title>
<style>
/* Una columna: catálogo de 15 emociones en tarjetas de dos columnas, luego frases */
:root {{ --fondo:#f4f5f7; --tarjeta:#ffffff; --tinta:#1d2430; --suave:#5b6575; --linea:#d9dde4;
  --naranja:#d8641e; --curva:#d8641e; color-scheme:light; }}
@media (prefers-color-scheme: dark) {{ :root:not([data-theme="light"]) {{ --fondo:#14171c; --tarjeta:#1d2129;
  --tinta:#e8ebf0; --suave:#9aa3b2; --linea:#2f3540; --naranja:#f08a43; --curva:#f08a43; color-scheme:dark; }} }}
:root[data-theme="dark"] {{ --fondo:#14171c; --tarjeta:#1d2129; --tinta:#e8ebf0; --suave:#9aa3b2;
  --linea:#2f3540; --naranja:#f08a43; --curva:#f08a43; color-scheme:dark; }}
body {{ background:var(--fondo); color:var(--tinta); font:15px/1.5 system-ui,-apple-system,"Segoe UI",sans-serif;
  margin:0; padding-inline:16px; padding-block:24px 48px; }}
main {{ max-width:860px; margin:0 auto; }}
h1 {{ font-size:1.9rem; margin:0 0 .3rem; text-wrap:balance; }}
h1 b {{ color:var(--naranja); }}
h2 {{ font-size:1.15rem; margin:2rem 0 .6rem; }}
.intro {{ color:var(--suave); max-width:65ch; margin:0 0 1.4rem; }}
code {{ font-family:ui-monospace,Menlo,Consolas,monospace; font-size:.9em; }}
ul {{ list-style:none; padding:0; margin:0; }}
.sonidos {{ display:grid; grid-template-columns:repeat(auto-fill,minmax(min(100%,380px),1fr)); gap:12px; }}
.son {{ background:var(--tarjeta); border:1px solid var(--linea); border-radius:10px; padding:14px 16px;
  display:flex; flex-direction:column; gap:4px; min-width:0; }}
.cab {{ display:flex; align-items:center; gap:8px; }}
.cab h3 {{ margin:0; font-size:1.05rem; }}
.dur {{ margin-left:auto; color:var(--suave); font-variant-numeric:tabular-nums; font-size:.85rem; }}
.son p {{ margin:0; }}
.cuando {{ color:var(--suave); font-size:.88rem; }}
.pie {{ display:flex; align-items:center; justify-content:space-between; gap:10px; flex-wrap:wrap; margin-top:6px; }}
svg {{ max-width:100%; }}
polyline {{ fill:none; stroke:var(--curva); stroke-width:2.2; stroke-linecap:round; stroke-linejoin:round; }}
.ojo {{ width:14px; height:14px; border-radius:50%; background:var(--ojo); box-shadow:0 0 8px var(--ojo); flex:none; }}
@media (prefers-reduced-motion: no-preference) {{
  .ojo.respirar {{ animation:respirar 3s ease-in-out infinite; }}
  .ojo.parpadeo {{ animation:parpadeo .5s steps(2) infinite; }} }}
@keyframes respirar {{ 50% {{ opacity:.25; }} }}
@keyframes parpadeo {{ 50% {{ opacity:0; }} }}
.botones {{ display:flex; gap:6px; }}
button {{ font:inherit; min-width:2.6em; padding:4px 10px; border-radius:6px; cursor:pointer;
  border:1px solid var(--linea); background:var(--fondo); color:var(--tinta); }}
button:first-of-type {{ background:var(--naranja); border-color:var(--naranja); color:var(--tarjeta); }}
button:focus-visible {{ outline:2px solid var(--naranja); outline-offset:2px; }}
.frases {{ display:flex; flex-direction:column; gap:6px; }}
.frase {{ display:flex; align-items:center; justify-content:space-between; gap:12px; padding:8px 12px;
  background:var(--tarjeta); border:1px solid var(--linea); border-radius:8px; }}
.frase span {{ min-width:0; }}
</style>
<main>
<h1>Sonidos de <b>BB-8</b></h1>
<p class="intro">Los 15 pitidos de emoción que genera <code>head/sounds.py</code>, con el color de ojo que les pone
<code>express</code>. ▶ es la versión base; 2 y 3 son variantes de tono y tempo, como suenan en el robot.
La curva es el tono de cada sílaba, de 250 a 3000 Hz.</p>
<ul class="sonidos">{"".join(filas)}</ul>
<h2>Balbuceo antes de hablar</h2>
<p class="intro">Antes de cada frase de Piper suena su propio balbuceo; la puntuación cambia la entonación.</p>
<ul class="frases">{ejemplos}</ul>
</main>
<script>
document.querySelectorAll("button").forEach(b => b.addEventListener("click", () => {{
  document.querySelectorAll("audio").forEach(a => a.pause());
  const a = b.nextElementSibling; a.currentTime = 0; a.play();
}}));
</script>"""
    if not completa:
        return cuerpo
    return f'<!doctype html><html lang="es"><head><meta charset="utf-8">' \
           f'<meta name="viewport" content="width=device-width, initial-scale=1"></head><body>{cuerpo}</body></html>'


def main() -> None:
    ap = argparse.ArgumentParser(description="Escuchar los pitidos de BB-8 en el PC")
    ap.add_argument("sonidos", nargs="*", help=f"de: {', '.join(SONIDOS)} (por defecto, todos)")
    ap.add_argument("--texto", help="balbuceo de una frase")
    ap.add_argument("--guardar", metavar="CARPETA", help="escribir un WAV por sonido en vez de sonar")
    ap.add_argument("--html", metavar="ARCHIVO", help="escribir una página para oírlos en el navegador")
    a = ap.parse_args()

    if a.html:
        from .personalidad import ejemplos_de_frases
        with open(a.html, "w", encoding="utf-8") as f:
            f.write(pagina_html(ejemplos_de_frases()))
        print(f"Abre {a.html} en el navegador")
        return
    nombres = [a.texto] if a.texto else (a.sonidos or list(SONIDOS))
    for n in nombres:
        if n not in SONIDOS and not a.texto:
            sys.exit(f"sonido desconocido: {n}. Usa {', '.join(SONIDOS)} o --texto")
    if a.guardar:
        os.makedirs(a.guardar, exist_ok=True)
        for n in nombres:
            ruta = os.path.join(a.guardar, f"{'texto' if a.texto else n}.wav")
            with open(ruta, "wb") as f:
                f.write(beep_wav(n, 0.5, variar=False)[0])
            print(ruta)
        return
    for n in nombres:
        wav, secs = beep_wav(n, 0.5)
        print(f"🔊 {n:<11} {secs:4.2f} s  {SONIDOS[n][0] if n in SONIDOS else ''}")
        reproducir(wav)
        time.sleep(0.35)


if __name__ == "__main__":
    main()
