# Pendientes

## Probar en el hardware real

- El firmware compila para Uno y Nano, pero solo se ha ejercitado su contraparte simulada.
- `head/backends_real.py` (cámara, ToF, NeoPixel, Piper en la Zero) no se ha corrido en la Zero.
- El agente de voz se probó contra el dummy con la API simulada, sin micrófono, Whisper ni Piper reales.

## Por construir

- Detección de obstáculos con la cámara a 5 fps en el servicio de movimiento (hoy frena el ToF).
- Entrenar el modelo "oye BB-8" en Colab (el pipeline está listo, ver [Wake word "oye BB-8"](Wake-word-oye-BB-8.md)); hasta entonces despierta con "hey Jarvis".
- Base de carga inductiva (opcional, después de la Parte 6).

## Por confirmar

- Modelo exacto del Arduino (se asume Uno o Nano).
- Modelo de la Pi principal: una Pi 5 necesita un buck de 5 A.
