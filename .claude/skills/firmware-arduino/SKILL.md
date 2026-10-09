---
name: firmware-arduino
description: Editar, compilar o subir el firmware del Arduino (bb8_firmware) o los sketches de prueba de la Parte 1; ajustes de CONFIGURACIÓN, pinout y reflejos.
---

# Firmware del Arduino

Archivo: `partes/1-protoboard/arduino/bb8_firmware/bb8_firmware.ino` (la carpeta y
el `.ino` deben llamarse igual). Placa: Uno o Nano (ATmega328P), 2 KB de RAM.

## Antes de editar

- Ajustes de usuario arriba, en el bloque `CONFIGURACIÓN` (`USAR_TOF_LOCAL`, `USAR_PID`, `HAY_DIVISOR_BAT`, `INVERTIR_*`, `SIGNO_GIRO_Z`, `SERVO_INVERTIDO`, `PULSOS_MAX_CICLO`, umbrales). Si agregas uno, va ahí con un comentario de cuándo cambiarlo, y en la tabla de `wiki/Parte-1-Protoboard.md`.
- Pinout fijo (no lo cambies sin actualizar la wiki y los diagramas): ENA D5, IN1 D7, IN2 D8, ENB D6, IN3 D11, IN4 D12; encoders izq D2/D4, der D3/D10; servo D9; I2C A4/A5; MPU INT A2; corte Pi A3; batería A0.
- La librería Servo toma Timer1: no hay PWM en D9 ni D10. Solo D2 y D3 tienen interrupción externa; C2 se lee con pin-change.
- Sin `String` ni `malloc` en el bucle; buffers fijos. Nada bloqueante en el ciclo de 10 ms (sin `delay` salvo en `setup`).
- Los reflejos (tilt > 35°, obstáculo < 250 mm, watchdog 500 ms, batería < 9.9 V) se quedan en el firmware.
- Si cambias una orden o una respuesta, es un cambio de protocolo: skill `cambiar-protocolo`, y refleja el comportamiento en `simulador/dummy/arduino_sim.py`.

## Compilar

Con `arduino-cli` (librería VL53L0X de Pololu):

```bash
arduino-cli core install arduino:avr
arduino-cli lib install VL53L0X
arduino-cli compile --fqbn arduino:avr:uno  partes/1-protoboard/arduino/bb8_firmware
arduino-cli compile --fqbn arduino:avr:nano partes/1-protoboard/arduino/bb8_firmware
```

El CI compila el firmware y `pruebas/p1…p5` para Uno y Nano en cada push y PR; si agregas
un sketch o una librería, cámbialo también en `.github/workflows/ci.yml`. Si cambias
pines, actualiza `partes/1-protoboard/wokwi/diagram.json`.

Compila para las dos placas y fíjate en el uso de RAM que imprime: por encima del
~75 % empiezan los cuelgues raros. Si `arduino-cli` no está disponible, dilo en vez de
afirmar que compila.

## Subir (solo con la placa conectada)

```bash
arduino-cli upload -p /dev/ttyACM0 --fqbn arduino:avr:uno partes/1-protoboard/arduino/bb8_firmware
python -m calibrar consola --serial /dev/ttyACM0     # I → OK BB8 fw=…, ?, M 80 80
```

Detén `bb8.motion_api` antes: es el único dueño del serial. El robot debe estar quieto
el primer segundo (calibra el giroscopio). Sube `FW_VERSION` cuando cambie el
comportamiento.
