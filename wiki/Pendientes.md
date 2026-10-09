# Pendientes

## Probar en el hardware real

- El firmware compila para Uno y Nano, pero solo se ha ejercitado su contraparte simulada.
- `head/backends_real.py` (cámara, ToF, NeoPixel, Piper en la Zero) no se ha corrido en la Zero.
- El agente de voz se probó contra el dummy con la API simulada, sin micrófono, Whisper ni Piper reales.

## Por construir

- Detección de obstáculos con la cámara a 5 fps en el servicio de movimiento (hoy frena el ToF).
- Modelo de wake word propio "oye BB-8" (hoy "hey Jarvis").
- Base de carga inductiva (opcional, después de la Parte 6).
- Imprimir y probar las piezas de [Diseño 3D](Diseno-3D.md): la bayoneta de la junta y el ajuste de los ball transfers no se han probado; las medidas marcadas MEDIR salen de hojas de vendedores.

## Por confirmar

- Modelo exacto del Arduino (se asume Uno o Nano).
- Modelo de la Pi principal: una Pi 5 necesita un buck de 5 A.
