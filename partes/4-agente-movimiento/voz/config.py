"""Ajustes del agente de voz. Todo se puede cambiar con variables de entorno."""
import os
from pathlib import Path

from bb8 import config as C

AQUI = Path(__file__).resolve().parent

# bb8-body local API: agent (/agent/ask), tools (/tools/<name>), hearing. The head
# is only reachable through it (secure link).
MOTION_URL = C.MOTION_URL
ENERGIA_URL = os.environ.get("BB8_ENERGIA_URL", "http://127.0.0.1:8771")

# The model, its key, step cap and conversation reset live in the bb8-body config
# (agent.* in the web page), not here.

# --- Micrófono y wake word ---------------------------------------------------
TASA = 16000
BLOQUE = 1280                 # 80 ms, lo que espera openWakeWord
# "cabeza": el ReSpeaker de la cabeza, por el enlace (GET /audio/stream de bb8-body).
# "local": un micrófono en la Pi (BB8_MICROFONO elige cuál).
MIC = os.environ.get("BB8_MIC", "cabeza")
MICROFONO = os.environ.get("BB8_MICROFONO") or None   # nombre o índice de sounddevice; None = el de por defecto
# Modelo de wake word: un nombre de los preentrenados (hey_jarvis, alexa, hey_mycroft)
# o la ruta a un .onnx. Sin BB8_WAKEWORD usa "oye BB-8" si ya copiaste el modelo
# entrenado con partes/4-agente-movimiento/wakeword a voz/modelos/, y si no "hey Jarvis".
WAKEWORD_PROPIA = AQUI / "modelos" / "oye_bb8.onnx"
WAKEWORD_RESPALDO = "hey_jarvis"
WAKEWORD = os.environ.get("BB8_WAKEWORD") or (
    str(WAKEWORD_PROPIA) if WAKEWORD_PROPIA.exists() else WAKEWORD_RESPALDO)
WAKEWORD_UMBRAL = float(os.environ.get("BB8_WAKEWORD_UMBRAL", "0.5"))
VAD_UMBRAL = 0.5
SILENCIO_FIN_S = 0.8          # deja de grabar tras 0.8 s de silencio
GRABACION_MAX_S = 10.0
ESPERA_VOZ_S = 4.0            # si no empieza a hablar en 4 s tras la wake word, vuelve a esperar

# --- Transcripción -----------------------------------------------------------
# "local": faster-whisper en la Pi. "nube": API de transcripción compatible con OpenAI
# (Groq, OpenAI o whisper.cpp en tu PC). Con BB8_STT_API_KEY puesta, nube por defecto.
STT_API_KEY = os.environ.get("BB8_STT_API_KEY", "")
STT = os.environ.get("BB8_STT") or ("nube" if STT_API_KEY else "local")
STT_URL = os.environ.get("BB8_STT_URL", "https://api.groq.com/openai/v1/audio/transcriptions")
STT_MODELO = os.environ.get("BB8_STT_MODELO", "whisper-large-v3-turbo")
# Pi 3 (1 GB): base cabe en RAM y entiende español aceptable; small no cabe junto a lo demás.
WHISPER = os.environ.get("BB8_WHISPER", "base")
# 3 de 4 núcleos: el cuarto queda para el ciclo de 50 ms del servicio de movimiento.
WHISPER_HILOS = int(os.environ.get("BB8_WHISPER_HILOS", "3"))

# --- Quién habla -------------------------------------------------------------
VOCES_JSON = Path(os.environ.get("BB8_VOCES", AQUI / "voces.json"))
VOZ_UMBRAL = 0.75

# --- Salida de voz -----------------------------------------------------------
# "local": Piper y altavoz USB en la Pi (Parte 4). "cabeza": altavoz de la Zero vía bb8-body (Parte 5).
SALIDA = os.environ.get("BB8_VOZ_SALIDA", "local")
PIPER_MODELO = os.environ.get("BB8_PIPER_MODELO", str(AQUI / "modelos" / "es_ES-davefx-medium.onnx"))

# --- Mirar a quien habla (Parte 5) ------------------------------------------
# Turn the head toward the voice (ReSpeaker direction from GET /hearing).
BUSCAR_CARA = os.environ.get("BB8_BUSCAR_CARA", "0") == "1"
