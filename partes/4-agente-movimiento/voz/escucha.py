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
    def __init__(self, mic: Microfono, panel=None):
        from openwakeword.vad import VAD

        self.mic = mic
        self.panel = panel
        self.ww = _cargar_wakeword(V.WAKEWORD)
        self.vad = VAD()

    def esperar_wakeword(self) -> float:
        """Bloquea hasta oír la wake word. Devuelve la puntuación."""
        self.ww.reset()
        pico, n = 0.0, 0
        while True:
            pred = self.ww.predict(self.mic.bloque())
            score = float(max(pred.values())) if pred else 0.0
            pico, n = max(pico, score), n + 1
            if self.panel and n >= 3:   # ~4 per second to the /voice page
                self.panel.evento("wake", score=round(pico, 2), threshold=V.WAKEWORD_UMBRAL)
                pico, n = 0.0, 0
            if score >= V.WAKEWORD_UMBRAL:
                log.info("Wake word (%.2f)", score)
                if self.panel:
                    self.panel.evento("wake", score=round(score, 2), threshold=V.WAKEWORD_UMBRAL)
                    self.panel.evento("wake_fired", score=round(score, 2))
                return score

    def grabar_frase(self, espera_s: float = V.ESPERA_VOZ_S) -> np.ndarray | None:
        """Graba desde ya hasta una pausa. None si nadie habló.

        The tail of "Oye BB-8" is still arriving when the wake word fires: it must
        not count as the command, or a short pause before speaking ended the phrase
        (1 s phrases transcribed as "Gracias."). So the command starts only after
        MIN_VOZ_S of speech, and the user gets ESPERA_VOZ_S to begin.
        """
        self.vad.reset_states()
        bloques: list[np.ndarray] = []
        voz_s = 0.0
        silencio = 0.0
        dur = V.BLOQUE / V.TASA
        while True:
            try:
                b = self.mic.bloque(timeout=1.0)
            except queue.Empty:
                continue
            bloques.append(b)
            relleno = getattr(self.mic, "relleno", None)
            if relleno is not None:
                # Head mic: its VAD tracks the noise floor and only streams while
                # someone talks. Silero missed quiet far speech; a fixed loudness
                # threshold never ended with a high mic gain.
                voz = not relleno
            else:
                fuerte = float(np.sqrt(np.mean(b.astype(np.float32) ** 2))) > V.VOZ_RMS
                voz = fuerte or self.vad.predict(b, frame_size=640) >= V.VAD_UMBRAL
            if voz:
                voz_s, silencio = voz_s + dur, 0.0
            else:
                silencio += dur
            hablo = voz_s >= V.MIN_VOZ_S
            t = len(bloques) * dur   # tiempo de audio, no de reloj
            if not hablo and t > espera_s:
                log.info("Nobody spoke within %.0f s", espera_s)
                if self.panel:
                    self.panel.evento("nobody")
                return None
            if hablo and silencio >= V.SILENCIO_FIN_S:
                break
            if t > V.GRABACION_MAX_S:
                break
        log.info("Phrase: %.1f s of audio", len(bloques) * dur)
        audio = np.concatenate(bloques)
        if V.GUARDAR_FRASES:   # debugging: what the transcriber gets
            import time
            import wave
            V.GUARDAR_FRASES.mkdir(parents=True, exist_ok=True)
            with wave.open(str(V.GUARDAR_FRASES / f"frase-{time.strftime('%H%M%S')}.wav"), "wb") as w:
                w.setnchannels(1)
                w.setsampwidth(2)
                w.setframerate(V.TASA)
                w.writeframes(audio.astype(np.int16).tobytes())
        return audio
