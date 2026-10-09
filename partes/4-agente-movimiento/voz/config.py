"""Ajustes del agente de voz. Todo se puede cambiar con variables de entorno."""
import os
from pathlib import Path

from bb8 import config as C

AQUI = Path(__file__).resolve().parent

MCP_URL = os.environ.get("BB8_MCP_URL", f"http://127.0.0.1:{C.MCP_PORT}/mcp")
HEAD_URL = C.HEAD_URL
MOTION_URL = C.MOTION_URL
ENERGIA_URL = os.environ.get("BB8_ENERGIA_URL", "http://127.0.0.1:8771")

# --- Claude ------------------------------------------------------------------
MODELO = os.environ.get("BB8_MODELO", "claude-opus-5-5")
# low = respuestas rápidas para conversar; súbelo a medium si planea mal los movimientos.
ESFUERZO = os.environ.get("BB8_ESFUERZO", "low")
MAX_PASOS = 8                 # llamadas a herramientas por orden, como tope
CONVERSACION_CADUCA_S = 300   # tras 5 min de silencio empieza una conversación nueva

# --- Micrófono y wake word ---------------------------------------------------
TASA = 16000
BLOQUE = 1280                 # 80 ms, lo que espera openWakeWord
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
WHISPER = os.environ.get("BB8_WHISPER", "small")   # base es más rápido en Pi 4; small entiende mejor
WHISPER_HILOS = int(os.environ.get("BB8_WHISPER_HILOS", "4"))

# --- Quién habla -------------------------------------------------------------
VOCES_JSON = Path(os.environ.get("BB8_VOCES", AQUI / "voces.json"))
VOZ_UMBRAL = 0.75

# --- Salida de voz -----------------------------------------------------------
# "local": Piper y altavoz USB en la Pi (Parte 4). "cabeza": POST /hablar a la Zero (Parte 5).
SALIDA = os.environ.get("BB8_VOZ_SALIDA", "local")
PIPER_MODELO = os.environ.get("BB8_PIPER_MODELO", str(AQUI / "modelos" / "es_ES-davefx-medium.onnx"))

# --- Mirar a quien habla (Parte 5) ------------------------------------------
BUSCAR_CARA = os.environ.get("BB8_BUSCAR_CARA", "0") == "1"
CAMARA_FOV_H = 54.0           # OV5647: ~54° de campo horizontal
