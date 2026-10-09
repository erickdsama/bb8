"""Wake word con openWakeWord y fin de frase con Silero VAD (el ONNX que trae openWakeWord, sin torch)."""
from __future__ import annotations

import logging
import queue
from pathlib import Path

import numpy as np

from . import config as V
from .audio import Microfono

log = logging.getLogger("bb8.voz.escucha")


def _parece_ruta(nombre: str) -> bool:
    return "/" in nombre or nombre.endswith((".onnx", ".tflite"))


def _preentrenados(nombre: str) -> list[str]:
    import openwakeword

    try:  # sin argumento, openWakeWord ≥ 0.5 solo lista los .tflite
        rutas = openwakeword.get_pretrained_model_paths("onnx")
    except TypeError:
        rutas = openwakeword.get_pretrained_model_paths()
    return [p for p in rutas if Path(p).name.startswith(nombre) and Path(p).exists()]


def _cargar_wakeword(nombre: str):
    from openwakeword.model import Model

    if _parece_ruta(nombre) and not Path(nombre).exists():
        log.warning("No existe el modelo de wake word %s; uso %s", nombre, V.WAKEWORD_RESPALDO)
        nombre = V.WAKEWORD_RESPALDO
    ruta = nombre
    if Path(nombre).exists():
        try:  # un modelo propio necesita los de features (melspectrogram, embedding)
            from openwakeword.utils import download_models
            download_models([V.WAKEWORD_RESPALDO])
        except Exception as e:  # 0.4 los trae dentro; sin red, quizá ya estén
            log.debug("download_models: %s", e)
    else:
        candidatos = _preentrenados(nombre)
        if not candidatos:
            try:  # openWakeWord ≥ 0.5 descarga los modelos aparte
                from openwakeword.utils import download_models
                download_models([nombre])
                candidatos = _preentrenados(nombre)
            except Exception as e:
                log.warning("No pude descargar %s: %s", nombre, e)
        if not candidatos:
            raise SystemExit(f"No encuentro el modelo de wake word {nombre!r}")
        onnx = [p for p in candidatos if p.endswith(".onnx")]
        ruta = (onnx or candidatos)[0]
    try:
        modelo = Model(wakeword_models=[ruta], inference_framework="onnx")   # openWakeWord ≥ 0.5
    except TypeError:
        modelo = Model(wakeword_model_paths=[ruta])                          # 0.4
    log.info("Wake word: %s", Path(ruta).stem)
    return modelo


class Escucha:
    def __init__(self, mic: Microfono):
        from openwakeword.vad import VAD

        self.mic = mic
        self.ww = _cargar_wakeword(V.WAKEWORD)
        self.vad = VAD()

    def esperar_wakeword(self) -> float:
        """Bloquea hasta oír la wake word. Devuelve la puntuación."""
        self.ww.reset()
        while True:
            pred = self.ww.predict(self.mic.bloque())
            score = max(pred.values()) if pred else 0.0
            if score >= V.WAKEWORD_UMBRAL:
                log.info("Wake word (%.2f)", score)
                return score

    def grabar_frase(self) -> np.ndarray | None:
        """Graba desde ya hasta 0.8 s de silencio. None si nadie habló."""
        self.vad.reset_states()
        bloques: list[np.ndarray] = []
        hablo = False
        silencio = 0.0
        dur = V.BLOQUE / V.TASA
        while True:
            try:
                b = self.mic.bloque(timeout=1.0)
            except queue.Empty:
                continue
            bloques.append(b)
            voz = self.vad.predict(b, frame_size=640) >= V.VAD_UMBRAL
            if voz:
                hablo, silencio = True, 0.0
            else:
                silencio += dur
            t = len(bloques) * dur   # tiempo de audio, no de reloj
            if not hablo and t > V.ESPERA_VOZ_S:
                return None
            if hablo and silencio >= V.SILENCIO_FIN_S:
                break
            if t > V.GRABACION_MAX_S:
                break
        return np.concatenate(bloques)
