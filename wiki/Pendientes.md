# Pendientes

## Probar en el hardware real

- El firmware compila para Uno y Nano (lo comprueba el CI), pero solo se ha ejercitado su contraparte simulada y Wokwi.
- `head/backends_real.py` (cámara, ToF, NeoPixel, Piper en la Zero) no se ha corrido en la Zero.
- El agente de voz se probó contra el dummy con la API simulada, sin micrófono, Whisper ni Piper reales.

## Por construir

- Detección de obstáculos con la cámara a 5 fps en el servicio de movimiento (hoy frena el ToF).
- Modelo de wake word propio "oye BB-8" (hoy "hey Jarvis").
- Base de carga inductiva (opcional, después de la Parte 6).

## Por confirmar

- Modelo exacto del Arduino (se asume Uno o Nano).
- Modelo de la Pi principal: una Pi 5 necesita un buck de 5 A.
