# Pendientes

## Probar en el hardware real

Ya funciona en el robot: la Pi 3 con el Arduino y el servo, y en la cabeza el micrófono (ReSpeaker v1), el ojo WS2812 y la voz por Wyoming (Piper).

- Motores y encoders con el firmware real; ajustar PWM máximo y rampa.
- ToF VL53L0X en la cabeza.
- Cámara OV5647 (llega en otro pedido; mientras, la cabeza arranca sin ella).
- Mando Bluetooth real.
- `calibrar` contra el robot.

## Por construir

- Identificación de voz sin torch (un modelo ONNX) para que quepa en la Pi 3. Hoy `voz/hablantes.py` usa `resemblyzer`.
- Un mejor modelo "oye BB-8": grabaciones de voz real y más negativas (ver [Wake word "oye BB-8"](Wake-word-oye-BB-8.md)). El actual ya despierta.
- Detección de obstáculos con la cámara a 5 fps en el servicio de movimiento (hoy frena el ToF).
- JEV como "System 1" de navegación.
- Kokoro como voz (TTS) en el servidor Wyoming.
- Base de carga inductiva (opcional, después de la Parte 6).

## Construcción

- Prueba de impresión de los gajos A y B en el FabLab (ABS) antes de la esfera completa.
- Montar la base: plataforma de MDF 12 mm y piezas de PETG (soportes de motor, RL4, bandeja de la LiPo, panel de carga).
- Probar la unión magnética cabeza–poste con los imanes de prueba.
- Las compras que faltan están en [Materiales](Materiales.md).

## Hardware confirmado

- Arduino Uno y Raspberry Pi 3 B+ (1 GB) como Pi principal.
- Raspberry Pi Zero W en la cabeza con el ReSpeaker 2-Mic v1.
- La voz (STT y TTS) corre en un servidor Wyoming en la red local, no en la Pi.
