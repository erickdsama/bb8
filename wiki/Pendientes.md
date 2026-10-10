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

- Nada por ahora. Confirmado el 9 oct 2026: Arduino Uno, Pi principal Raspberry Pi 3,
  y la cámara OV5647 todavía no se compra (está en el [carrito](Materiales.md)).
