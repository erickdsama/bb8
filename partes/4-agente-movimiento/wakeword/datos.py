"""Descarga lo que necesita el entrenamiento, igual que el cuaderno oficial de openWakeWord.

- mit_rirs/        respuestas al impulso de cuartos reales (eco), 16 kHz
- fondo_16k/       ruido de fondo de AudioSet, 16 kHz
- openwakeword_features_ACAV100M_2000_hrs_16bit.npy   negativos genéricos (~2000 h, ~17 GB)
- validation_set_features.npy                          ~11 h para medir falsas activaciones
- los modelos de features de openWakeWord (melspectrogram y embedding)

Todo sale de Hugging Face; cada paso se salta si ya está hecho.
"""
from __future__ import annotations

import logging
import tarfile
from pathlib import Path

import numpy as np
import scipy.io.wavfile

from muestras import a_16k

log = logging.getLogger("bb8.wakeword.datos")

FEATURES_REPO = "davidscripka/openwakeword_features"
FEATURES_ACAV = "openwakeword_features_ACAV100M_2000_hrs_16bit.npy"
FEATURES_VAL = "validation_set_features.npy"
RIRS_REPO = "davidscripka/MIT_environmental_impulse_responses"
AUDIOSET_REPO = "agkphysics/AudioSet"
EXT_AUDIO = (".wav", ".flac", ".mp3", ".ogg")


def _escribir_16k(destino: Path, datos: np.ndarray, tasa: int) -> None:
    audio = np.clip(a_16k(datos, tasa), -1, 1)
    scipy.io.wavfile.write(destino, 16000, (audio * 32767).astype(np.int16))


def _convertir_carpeta(origen: Path, destino: Path) -> int:
    """Todo el audio de `origen` (wav/flac, o parquet de datasets de HF) a wav 16 kHz mono."""
    import soundfile as sf

    destino.mkdir(parents=True, exist_ok=True)
    n = 0
    for p in sorted(origen.rglob("*")):
        if p.suffix.lower() in EXT_AUDIO:
            try:
                datos, tasa = sf.read(p, dtype="float32")
            except Exception as e:
                log.warning("No pude leer %s: %s", p.name, e)
                continue
            _escribir_16k(destino / f"{p.stem}.wav", datos, tasa)
            n += 1
        elif p.suffix == ".parquet":
            import io

            import pyarrow.parquet as pq

            for fila in pq.read_table(p).to_pylist():
                audio = fila.get("audio") or {}
                if not audio.get("bytes"):
                    continue
                datos, tasa = sf.read(io.BytesIO(audio["bytes"]), dtype="float32")
                nombre = Path(audio.get("path") or f"clip_{n}").stem
                _escribir_16k(destino / f"{nombre}.wav", datos, tasa)
                n += 1
    return n


def rirs(trabajo: Path) -> None:
    destino = trabajo / "mit_rirs"
    if destino.exists() and any(destino.glob("*.wav")):
        log.info("mit_rirs listo")
        return
    from huggingface_hub import snapshot_download

    crudo = Path(snapshot_download(RIRS_REPO, repo_type="dataset", local_dir=trabajo / "_crudo" / "rirs"))
    log.info("mit_rirs: %d respuestas al impulso", _convertir_carpeta(crudo, destino))


def fondo(trabajo: Path, partes: int = 1) -> None:
    """Ruido de fondo: `partes` archivos .tar de AudioSet (~1–2 GB cada uno)."""
    destino = trabajo / "fondo_16k"
    if destino.exists() and any(destino.glob("*.wav")):
        log.info("fondo_16k listo")
        return
    from huggingface_hub import hf_hub_download

    crudo = trabajo / "_crudo" / "audioset"
    for i in range(partes):   # bal_train09 es el que usa el cuaderno oficial; luego 08, 07…
        tar = Path(hf_hub_download(AUDIOSET_REPO, f"data/bal_train{9 - i:02d}.tar", repo_type="dataset",
                                   local_dir=crudo))
        with tarfile.open(tar) as t:
            t.extractall(crudo / tar.stem, filter="data")
        tar.unlink()
    log.info("fondo_16k: %d clips", _convertir_carpeta(crudo, destino))


def features(trabajo: Path) -> None:
    from huggingface_hub import hf_hub_download

    for nombre in (FEATURES_VAL, FEATURES_ACAV):
        if (trabajo / nombre).exists():
            log.info("%s listo", nombre)
            continue
        log.info("Bajando %s…", nombre)
        hf_hub_download(FEATURES_REPO, nombre, repo_type="dataset", local_dir=trabajo)


def modelos_openwakeword() -> None:
    """melspectrogram y embedding (los usa el aumento de datos para sacar features)."""
    from openwakeword.utils import download_models

    download_models(["hey_jarvis"])   # baja siempre los de features; hey_jarvis es el respaldo


def todo(trabajo: Path, partes_audioset: int = 1) -> None:
    trabajo.mkdir(parents=True, exist_ok=True)
    modelos_openwakeword()
    rirs(trabajo)
    fondo(trabajo, partes_audioset)
    features(trabajo)
