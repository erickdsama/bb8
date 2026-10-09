"""Muestras sintéticas en español para entrenar "oye BB-8".

openWakeWord genera sus muestras con una voz inglesa (piper-sample-generator), que no sabe
decir "oye". Aquí las hacemos con las voces de Piper en español, variando velocidad,
entonación y tono, y las dejamos donde `openwakeword/train.py` las espera:

    <salida>/<model_name>/positive_train, positive_test, negative_train, negative_test

Es reanudable: cuenta lo que ya hay y solo genera lo que falta.
"""
from __future__ import annotations

import json
import logging
import multiprocessing as mp
import random
import uuid
from pathlib import Path

import numpy as np
import scipy.io.wavfile
import scipy.signal

log = logging.getLogger("bb8.wakeword.muestras")

TASA = 16000
REPO_VOCES = "rhasspy/piper-voices"


# --- audio ---------------------------------------------------------------------

def a_16k(audio: np.ndarray, tasa: int) -> np.ndarray:
    """Mono float32 a 16 kHz."""
    audio = np.asarray(audio, dtype=np.float32)
    if audio.ndim > 1:
        audio = audio.mean(axis=1)
    if tasa != TASA:
        g = np.gcd(int(tasa), TASA)
        audio = scipy.signal.resample_poly(audio, TASA // g, int(tasa) // g).astype(np.float32)
    return audio


def recortar(audio: np.ndarray, umbral_db: float = -40.0, margen_s: float = 0.05,
             pausa_max_s: float | None = 0.2) -> np.ndarray:
    """Quita el silencio de las orillas (openWakeWord coloca la frase al final de la ventana)
    y acorta las pausas internas a `pausa_max_s` (algunas voces de Piper hacen pausas de un
    segundo entre palabras, y nadie dice así "oye BB-8")."""
    paso = 160  # 10 ms
    n = len(audio) // paso
    if n == 0:
        return audio
    audio = audio[: n * paso]
    energia = np.sqrt(np.mean(audio.reshape(n, paso) ** 2, axis=1) + 1e-12)
    activo = energia > energia.max() * 10 ** (umbral_db / 20)
    idx = np.flatnonzero(activo)
    if not len(idx):
        return audio[:0]
    m = int(margen_s * TASA) // paso
    ini, fin = max(0, idx[0] - m), min(n, idx[-1] + 1 + m)
    if pausa_max_s is not None:
        tope = int(pausa_max_s * TASA) // paso
        quedan = np.ones(n, bool)
        i = ini
        while i < fin:
            if not activo[i]:
                j = i
                while j < fin and not activo[j]:
                    j += 1
                if j - i > tope:
                    quedan[i + tope // 2: j - (tope - tope // 2)] = False
                i = j
            else:
                i += 1
        quedan[:ini] = quedan[fin:] = False
        return audio.reshape(n, paso)[quedan].reshape(-1)
    return audio[ini * paso: fin * paso]


def guardar(ruta: Path, audio: np.ndarray) -> None:
    pico = float(np.abs(audio).max()) or 1.0
    audio = audio / pico * random.uniform(0.3, 0.95)
    scipy.io.wavfile.write(ruta, TASA, (audio * 32767).astype(np.int16))


def cambiar_tono(audio: np.ndarray, factor: float) -> np.ndarray:
    """Reproduce más rápido/lento: sube o baja tono y velocidad juntos (voz más aguda/grave)."""
    if abs(factor - 1.0) < 1e-3:
        return audio
    n = max(1, int(round(len(audio) / factor)))
    return scipy.signal.resample(audio, n).astype(np.float32)


# --- voces de Piper ------------------------------------------------------------

def voces_espanol(lista="auto") -> list[str]:
    """Nombres de las voces de Piper a usar ("auto" = todas las de español)."""
    if lista != "auto":
        return list(lista)
    from huggingface_hub import hf_hub_download

    indice = json.loads(Path(hf_hub_download(REPO_VOCES, "voices.json")).read_text())
    return sorted(k for k, v in indice.items() if v.get("language", {}).get("family") == "es")


def bajar_voz(nombre: str, carpeta: Path) -> Path:
    """Descarga <nombre>.onnx y .onnx.json a carpeta (si no están) y devuelve la ruta del .onnx."""
    onnx = carpeta / f"{nombre}.onnx"
    if onnx.exists() and onnx.with_suffix(".onnx.json").exists():
        return onnx
    from huggingface_hub import hf_hub_download

    idioma, region_voz = nombre.split("_", 1)            # es, ES-davefx-medium
    region = f"{idioma}_{region_voz.split('-')[0]}"        # es_ES
    _, voz, calidad = nombre.split("-")
    base = f"{idioma}/{region}/{voz}/{calidad}/{nombre}"
    carpeta.mkdir(parents=True, exist_ok=True)
    for ext in (".onnx", ".onnx.json"):
        hf_hub_download(REPO_VOCES, base + ext, local_dir=carpeta)
        (carpeta / (base + ext)).replace(carpeta / f"{nombre}{ext}")
    return onnx


# --- generación ----------------------------------------------------------------

def _rango(r) -> float:
    return random.uniform(float(r[0]), float(r[1]))


def _trabajo(args) -> int:
    """Genera `n` clips con una voz. Corre en un proceso aparte."""
    ruta_voz, frases, n, destino, cfg, semilla, cuda = args
    from piper import PiperVoice, SynthesisConfig

    random.seed(semilla)
    voz = PiperVoice.load(str(ruta_voz), use_cuda=cuda)
    hablantes = max(1, voz.config.num_speakers)
    hechos = intentos = 0
    while hechos < n and intentos < n * 3:
        intentos += 1
        sc = SynthesisConfig(
            speaker_id=random.randrange(hablantes) if hablantes > 1 else None,
            length_scale=_rango(cfg["length_scale"]),
            noise_scale=_rango(cfg["noise_scale"]),
            noise_w_scale=_rango(cfg["noise_w"]),
        )
        trozos = list(voz.synthesize(random.choice(frases), sc))
        if not trozos:
            continue
        audio = np.concatenate([t.audio_float_array for t in trozos])
        audio = recortar(a_16k(audio, trozos[0].sample_rate))
        audio = cambiar_tono(audio, _rango(cfg["tono"]))
        if not (0.25 <= len(audio) / TASA <= float(cfg["duracion_max_s"])):
            continue
        guardar(destino / f"{uuid.uuid4().hex}.wav", audio)
        hechos += 1
    return hechos


def hay_cuda() -> bool:
    try:
        import onnxruntime

        return "CUDAExecutionProvider" in onnxruntime.get_available_providers()
    except ImportError:
        return False


def generar(destino: Path, frases: list[str], total: int, voces: list[Path], cfg: dict,
            procesos: int | None = None) -> None:
    """Llena `destino` hasta tener `total` clips, repartidos entre las voces."""
    destino.mkdir(parents=True, exist_ok=True)
    cuda = hay_cuda()
    procesos = procesos or (2 if cuda else max(1, mp.cpu_count()))
    for _ronda in range(3):   # lo descartado por largo se repone en la siguiente ronda
        faltan = total - len(list(destino.glob("*.wav")))
        if faltan <= 0:
            break
        log.info("%s: generando %d clips con %d voces%s", destino.name, faltan, len(voces),
                 " en GPU" if cuda else "")
        # tandas de hasta 100 clips para repartir el trabajo entre todos los procesos
        tareas = []
        for i, voz in enumerate(voces):
            n = faltan // len(voces) + (1 if i < faltan % len(voces) else 0)
            while n > 0:
                tareas.append((voz, frases, min(n, 100), destino, cfg, random.randrange(2**31), cuda))
                n -= 100
        random.shuffle(tareas)
        with mp.get_context("spawn").Pool(procesos) as pool:
            hechos = 0
            for h in pool.imap_unordered(_trabajo, tareas):
                hechos += h
                print(f"\r  {destino.name}: {hechos}/{faltan}", end="", flush=True)
        print()
    log.info("%s: %d clips", destino.name, len(list(destino.glob("*.wav"))))


def agregar_grabaciones(origen: Path, carpeta_modelo: Path, repetir: int = 20) -> None:
    """Copia tus grabaciones reales de "oye BB-8" (wav/flac, cualquier tasa) a las positivas.

    El 80 % va a entrenamiento, repetido `repetir` veces (el aumento de datos le pone otro
    ruido y otro eco a cada copia); el 20 % a validación, sin repetir."""
    import soundfile as sf

    archivos = sorted(p for p in origen.rglob("*") if p.suffix.lower() in (".wav", ".flac", ".ogg"))
    if not archivos:
        log.warning("No hay grabaciones en %s", origen)
        return
    random.shuffle(archivos)
    corte = max(1, len(archivos) // 5) if len(archivos) >= 5 else 0
    for i, p in enumerate(archivos):
        datos, tasa = sf.read(p, dtype="float32")
        audio = recortar(a_16k(datos, tasa))
        if not len(audio):
            continue
        prueba = i < corte
        destino = carpeta_modelo / ("positive_test" if prueba else "positive_train")
        destino.mkdir(parents=True, exist_ok=True)
        for _ in range(1 if prueba else repetir):
            guardar(destino / f"grabacion_{p.stem}_{uuid.uuid4().hex[:8]}.wav", audio)
    log.info("Agregué %d grabaciones (%d a validación)", len(archivos), corte)


def generar_todo(cfg: dict, trabajo: Path, procesos: int | None = None) -> None:
    """Las cuatro carpetas que espera train.py, según el YAML ya resuelto."""
    bb8 = cfg["bb8"]
    carpeta = Path(cfg["output_dir"]) / cfg["model_name"]
    nombres = voces_espanol(bb8.get("voces", "auto"))
    log.info("Voces: %s", ", ".join(nombres))
    voces = [bajar_voz(n, trabajo / "voces_piper") for n in nombres]
    # Validación con parámetros más tranquilos, como hace openWakeWord.
    cfg_val = dict(bb8, length_scale=[0.85, 1.2], tono=[0.94, 1.06])
    generar(carpeta / "positive_train", bb8["positivas"], cfg["n_samples"], voces, bb8, procesos)
    generar(carpeta / "positive_test", bb8["positivas"], cfg["n_samples_val"], voces, cfg_val, procesos)
    generar(carpeta / "negative_train", bb8["negativas"], cfg["n_samples"], voces, bb8, procesos)
    generar(carpeta / "negative_test", bb8["negativas"], cfg["n_samples_val"], voces, cfg_val, procesos)
