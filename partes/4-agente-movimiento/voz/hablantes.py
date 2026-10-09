"""Quién habla: embedding de voz con Resemblyzer y similitud coseno contra voces.json.

Registrar una voz (cada persona, una vez; 4 frases de ~3 s):
    python -m voz.hablantes registrar Erick
    python -m voz.hablantes lista
    python -m voz.hablantes borrar Erick

No es biometría segura: basta para saludar por nombre y no obedecer a desconocidos.
Mientras voces.json esté vacío o falte resemblyzer, todos cuentan como conocidos.
"""
from __future__ import annotations

import argparse
import json
import logging
import time

import numpy as np

from . import config as V

log = logging.getLogger("bb8.voz.hablantes")


class Hablantes:
    def __init__(self):
        self.voces: dict[str, list[float]] = {}
        if V.VOCES_JSON.exists():
            self.voces = json.loads(V.VOCES_JSON.read_text())
        self.encoder = None
        try:
            from resemblyzer import VoiceEncoder

            self.encoder = VoiceEncoder("cpu", verbose=False)
        except Exception as e:
            log.warning("Sin identificación de voz (%s): pip install resemblyzer", e)
        if not self.voces:
            log.warning("No hay voces registradas: BB-8 obedecerá a cualquiera")

    @property
    def activo(self) -> bool:
        return bool(self.encoder and self.voces)

    def embedding(self, audio: np.ndarray) -> np.ndarray:
        from resemblyzer import preprocess_wav

        wav = preprocess_wav(audio.astype(np.float32) / 32768.0, source_sr=V.TASA)
        return self.encoder.embed_utterance(wav)

    def identificar(self, audio: np.ndarray) -> tuple[str | None, float]:
        """(nombre, similitud). nombre None = desconocido. Sin registro → ("", 1.0)."""
        if not self.activo:
            return "", 1.0
        if len(audio) < V.TASA * 1.0:   # menos de 1 s no alcanza para decidir
            return None, 0.0
        e = self.embedding(audio)
        mejor, sim = None, 0.0
        for nombre, ref in self.voces.items():
            s = float(np.dot(e, np.asarray(ref)))   # los embeddings ya vienen normalizados
            if s > sim:
                mejor, sim = nombre, s
        log.info("Voz: %s (%.2f)", mejor, sim)
        return (mejor if sim >= V.VOZ_UMBRAL else None), sim

    def guardar(self) -> None:
        V.VOCES_JSON.write_text(json.dumps(self.voces))


FRASES = [
    "Hola BB-8, soy yo, ven acá por favor.",
    "Da una vuelta y mira qué hay en la cocina.",
    "Hoy hace buen día para rodar por la casa.",
    "BB-8, detente y espera a que regrese.",
]


def registrar(nombre: str) -> None:
    from .audio import Microfono

    h = Hablantes()
    if not h.encoder:
        raise SystemExit("Instala resemblyzer primero: pip install resemblyzer")
    mic = Microfono()
    embs = []
    for frase in FRASES:
        input(f"\nEnter y lee en voz alta: «{frase}»")
        mic.vaciar()
        bloques, t0 = [], time.monotonic()
        while time.monotonic() - t0 < 3.5:
            bloques.append(mic.bloque())
        audio = np.concatenate(bloques)
        if np.abs(audio).mean() < 200:
            print("  Casi no se oyó nada; acércate al micrófono y repite.")
            continue
        embs.append(h.embedding(audio))
        print("  ✓")
    if len(embs) < 3:
        raise SystemExit("Hacen falta al menos 3 frases buenas")
    m = np.mean(embs, axis=0)
    h.voces[nombre] = (m / np.linalg.norm(m)).tolist()
    h.guardar()
    print(f"\nVoz de {nombre} guardada en {V.VOCES_JSON}")


def main() -> None:
    ap = argparse.ArgumentParser(description="Registro de voces del BB-8")
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("registrar").add_argument("nombre")
    sub.add_parser("lista")
    sub.add_parser("borrar").add_argument("nombre")
    a = ap.parse_args()
    logging.basicConfig(level=logging.INFO)
    if a.cmd == "registrar":
        registrar(a.nombre)
    elif a.cmd == "lista":
        voces = json.loads(V.VOCES_JSON.read_text()) if V.VOCES_JSON.exists() else {}
        print("\n".join(voces) or "(ninguna)")
    else:
        h = Hablantes()
        h.voces.pop(a.nombre, None)
        h.guardar()
        print(f"{a.nombre} borrado")


if __name__ == "__main__":
    main()
