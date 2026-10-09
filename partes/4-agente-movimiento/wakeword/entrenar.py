"""Entrena la wake word "oye BB-8" (openWakeWord) con voces sintéticas en español.

    python entrenar.py --trabajo /content/ww todo           # los cuatro pasos
    python entrenar.py --trabajo /content/ww datos          # 1. eco, ruido y negativos (~20 GB)
    python entrenar.py --trabajo /content/ww muestras       # 2. "oye BB-8" y parecidas con Piper
    python entrenar.py --trabajo /content/ww aumentar       # 3. ruido + eco → features
    python entrenar.py --trabajo /content/ww modelo         # 4. entrena y exporta oye_bb8.onnx

Opciones útiles:
    --mis-grabaciones DIR   agrega tus grabaciones reales de "oye BB-8" a las positivas
    --rapido                pocas muestras y pasos: para comprobar que todo corre, no para usarlo

Se corre en Colab con GPU (cuaderno entrenar_oye_bb8.ipynb) o en una PC con Linux. No en
la Pi. Todos los pasos son reanudables. Detalle: wiki/Wake-word-oye-BB-8.md
"""
from __future__ import annotations

import argparse
import logging
import shutil
import subprocess
import sys
from pathlib import Path

import yaml

AQUI = Path(__file__).resolve().parent
sys.path.insert(0, str(AQUI))

import datos  # noqa: E402
import muestras  # noqa: E402

log = logging.getLogger("bb8.wakeword")

RUTAS = ("output_dir", "false_positive_validation_data_path")
RUTAS_LISTA = ("rir_paths", "background_paths")


def cargar_config(ruta: Path, trabajo: Path, rapido: bool) -> dict:
    """Lee el YAML, vuelve absolutas las rutas (relativas a `trabajo`) y lo guarda ahí."""
    cfg = yaml.safe_load(ruta.read_text())
    absoluta = lambda p: str(p if Path(p).is_absolute() else trabajo / p)  # noqa: E731
    for k in RUTAS:
        cfg[k] = absoluta(cfg[k])
    for k in RUTAS_LISTA:
        cfg[k] = [absoluta(p) for p in cfg[k]]
    cfg["feature_data_files"] = {k: absoluta(v) for k, v in cfg["feature_data_files"].items()}
    cfg["piper_sample_generator_path"] = str(AQUI / "sin_generador")
    if rapido:
        cfg.update(n_samples=400, n_samples_val=100, steps=500)
    (trabajo / "oye_bb8.yml").write_text(yaml.safe_dump(cfg, allow_unicode=True, sort_keys=False))
    return cfg


def train_py(trabajo: Path, *banderas: str) -> None:
    """Corre openwakeword/train.py con el YAML resuelto."""
    cmd = [sys.executable, str(AQUI / "lanzar_train.py"), "--training_config",
           str(trabajo / "oye_bb8.yml"), *banderas]
    log.info("→ %s", " ".join(cmd))
    subprocess.run(cmd, check=True, cwd=trabajo)


def paso_muestras(cfg: dict, trabajo: Path, args) -> None:
    muestras.generar_todo(cfg, trabajo, args.procesos)
    if args.mis_grabaciones:
        muestras.agregar_grabaciones(Path(args.mis_grabaciones), Path(cfg["output_dir"]) / cfg["model_name"],
                                     args.repetir)


def paso_modelo(cfg: dict, trabajo: Path) -> Path:
    train_py(trabajo, "--train_model")
    onnx = Path(cfg["output_dir"]) / f"{cfg['model_name']}.onnx"
    final = trabajo / onnx.name
    shutil.copy(onnx, final)
    log.info("✅ Modelo listo: %s", final)
    log.info("   Cópialo a partes/4-agente-movimiento/voz/modelos/%s en la Pi; el agente lo usa solo.",
             onnx.name)
    return final


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("paso", choices=["datos", "muestras", "aumentar", "modelo", "todo"])
    ap.add_argument("--trabajo", type=Path, default=Path("ww_trabajo"),
                    help="carpeta para datos, muestras y resultados (necesita ~30 GB)")
    ap.add_argument("--config", type=Path, default=AQUI / "oye_bb8.yml")
    ap.add_argument("--mis-grabaciones", help="carpeta con tus grabaciones de 'oye BB-8'")
    ap.add_argument("--repetir", type=int, default=20, help="copias de cada grabación propia")
    ap.add_argument("--audioset-partes", type=int, default=1, help="archivos de ruido de AudioSet")
    ap.add_argument("--procesos", type=int, default=None, help="procesos para generar con Piper")
    ap.add_argument("--rapido", action="store_true", help="prueba corta del pipeline (modelo inútil)")
    ap.add_argument("--rehacer", action="store_true", help="recalcula las features del paso aumentar")
    args = ap.parse_args()

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s: %(message)s")
    trabajo = args.trabajo.resolve()
    trabajo.mkdir(parents=True, exist_ok=True)
    cfg = cargar_config(args.config, trabajo, args.rapido)

    if args.paso in ("datos", "todo"):
        datos.todo(trabajo, args.audioset_partes)
    if args.paso in ("muestras", "todo"):
        paso_muestras(cfg, trabajo, args)
    if args.paso in ("aumentar", "todo"):
        # train.py se salta el paso si ya existe la primera matriz; si la última no está,
        # una corrida anterior quedó a medias y hay que rehacerlo.
        a_medias = not (Path(cfg["output_dir"]) / cfg["model_name"] / "negative_features_test.npy").exists()
        train_py(trabajo, "--augment_clips", *(["--overwrite"] if args.rehacer or a_medias else []))
    if args.paso in ("modelo", "todo"):
        paso_modelo(cfg, trabajo)


if __name__ == "__main__":
    main()
