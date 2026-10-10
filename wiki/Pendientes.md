# Pendientes

## Probar en el hardware real

- El firmware compila para el Arduino Uno, pero solo se ha ejercitado su contraparte simulada.
- `head/backends_real.py` (cámara, ToF, NeoPixel, Piper en la Zero) no se ha corrido en la Zero.
- El agente de voz se probó contra el dummy con la API simulada, sin micrófono, Whisper ni Piper reales.
- Medir en la Pi 3 la RAM y la espera reales de Whisper `base` (hoy son estimados).

## Por construir

- Detección de obstáculos con la cámara a 5 fps en el servicio de movimiento (hoy frena el ToF).
- Entrenar el modelo "oye BB-8" en Colab (el pipeline está listo, ver [Wake word "oye BB-8"](Wake-word-oye-BB-8.md)); hasta entonces despierta con "hey Jarvis".
- Base de carga inductiva (opcional, después de la Parte 6).
- Identificación de voz sin torch (un modelo ONNX), para que quepa en la Pi 3 con Whisper local.

## Hardware confirmado

- Arduino Uno y Raspberry Pi 3 (1 GB) como Pi principal.
- La cámara OV5647 llega en otro pedido; mientras, la cabeza arranca sin ella.
