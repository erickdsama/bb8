"""Prueba un modelo de wake word con archivos de audio o con el micrófono.

    python probar.py oye_bb8.onnx grabaciones/*.wav       # puntuación máxima por archivo
    python probar.py oye_bb8.onnx --ruido carpeta/        # falsas activaciones por hora
    python probar.py oye_bb8.onnx --mic                   # en vivo (Ctrl+C para salir)

Sirve para elegir BB8_WAKEWORD_UMBRAL: las grabaciones de "oye BB-8" deben quedar arriba
del umbral y las horas de plática o tele, casi nunca.
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parent))
from muestras import a_16k  # noqa: E402

BLOQUE = 1280   # 80 ms, igual que voz/escucha.py


def cargar_modelo(ruta: str):
    from openwakeword.model import Model
    from openwakeword.utils import download_models

    download_models(["hey_jarvis"])   # melspectrogram y embedding, si faltan
    return Model(wakeword_models=[ruta], inference_framework="onnx" if ruta.endswith(".onnx") else "tflite")


def leer(ruta: Path) -> np.ndarray:
    import soundfile as sf

    datos, tasa = sf.read(ruta, dtype="float32")
    return (np.clip(a_16k(datos, tasa), -1, 1) * 32767).astype(np.int16)


def puntuaciones(modelo, audio: np.ndarray) -> np.ndarray:
    modelo.reset()
    audio = np.concatenate([np.zeros(16000, np.int16), audio, np.zeros(16000, np.int16)])
    out = []
    for i in range(0, len(audio) - BLOQUE + 1, BLOQUE):
        pred = modelo.predict(audio[i:i + BLOQUE])
        out.append(max(pred.values()) if pred else 0.0)
    return np.array(out)


def archivos(rutas: list[str]) -> list[Path]:
    salida = []
    for r in map(Path, rutas):
        salida += sorted(p for p in r.rglob("*") if p.suffix.lower() in (".wav", ".flac")) if r.is_dir() else [r]
    return salida


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("modelo")
    ap.add_argument("audios", nargs="*", help="wav/flac o carpetas con frases que deberían activarlo")
    ap.add_argument("--ruido", nargs="+", help="wav/flac o carpetas con audio que NO debe activarlo")
    ap.add_argument("--mic", action="store_true")
    ap.add_argument("--umbral", type=float, default=0.5)
    args = ap.parse_args()
    modelo = cargar_modelo(args.modelo)

    if args.audios:
        ps = archivos(args.audios)
        maximos = []
        for p in ps:
            m = float(puntuaciones(modelo, leer(p)).max())
            maximos.append(m)
            print(f"{'✅' if m >= args.umbral else '❌'} {m:.3f}  {p.name}")
        print(f"\nDetectadas {sum(m >= args.umbral for m in maximos)}/{len(ps)} con umbral {args.umbral}")

    if args.ruido:
        horas = activaciones = 0.0
        for p in archivos(args.ruido):
            audio = leer(p)
            s = puntuaciones(modelo, audio)
            # cuenta flancos de subida, no cada bloque de 80 ms arriba del umbral
            arriba = s >= args.umbral
            activaciones += int(np.sum(arriba[1:] & ~arriba[:-1]) + arriba[0])
            horas += len(audio) / 16000 / 3600
        print(f"{activaciones:.0f} activaciones en {horas * 60:.1f} min → {activaciones / max(horas, 1e-9):.2f} por hora")

    if args.mic:
        import sounddevice as sd

        print("Escuchando… di 'oye BB-8' (Ctrl+C para salir)")
        with sd.InputStream(samplerate=16000, channels=1, dtype="int16", blocksize=BLOQUE) as mic:
            while True:
                bloque, _ = mic.read(BLOQUE)
                pred = modelo.predict(bloque[:, 0])
                s = max(pred.values()) if pred else 0.0
                barra = "█" * int(s * 40)
                print(f"\r{s:.2f} {barra:<40}{'  ¡DETECTADO!' if s >= args.umbral else '             '}",
                      end="", flush=True)


if __name__ == "__main__":
    main()
