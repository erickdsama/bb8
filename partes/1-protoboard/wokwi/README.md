# Firmware en Wokwi

Simula el Arduino Uno con `bb8_firmware` en el navegador o en VS Code, antes de tener
el protoboard armado. Sirve para probar el protocolo serial y los reflejos a mano.

| Pieza real | En Wokwi | Pines |
| --- | --- | --- |
| L298N + 2 motores | 6 LEDs: `ENA`/`ENB` (brillo = PWM), `IN1`/`IN3` adelante, `IN2`/`IN4` atrás | D5 D7 D8 · D6 D11 D12 |
| Encoders de los motores | 2 perillas KY-040 (CLK = C1, DT = C2) | D2/D4 izq · D3/D10 der |
| MG996R del poste | `wokwi-servo` | D9 |
| MPU6050 | `wokwi-mpu6050` (con controles de aceleración y giro) | A4/A5, INT en A2 |
| Corte de la Pi | LED amarillo | A3 |
| Divisor de la batería | Potenciómetro (0–5 V ≈ 0–15.6 V) | A0 |
| VL53L0X | **no existe en Wokwi**: el firmware arranca sin él | — |

El anillo NeoPixel no está aquí: lo maneja la cabeza (Pi Zero), no el Arduino.

## En VS Code

1. Instala la extensión **Wokwi Simulator** (pide una licencia gratuita la primera vez).
2. Compila con los binarios exportados (los deja en `arduino/bb8_firmware/build/`, que git ignora):

   ```bash
   arduino-cli compile --fqbn arduino:avr:uno --export-binaries partes/1-protoboard/arduino/bb8_firmware
   ```

   O descarga el artefacto `firmware-uno` de la última corrida de CI y descomprímelo ahí.
3. Abre `partes/1-protoboard/wokwi/wokwi.toml` y ejecuta **Wokwi: Start Simulator**
   (si no lo encuentra, **Wokwi: Select Config File** y elige este `wokwi.toml`).

## En wokwi.com

Crea un proyecto nuevo de Arduino Uno y reemplaza sus archivos: pega
`bb8_firmware.ino` en `sketch.ino`, este `diagram.json` y este `libraries.txt`.

## Qué probar

Escribe en el monitor serial (115200, termina en salto de línea):

- `I` → `OK BB8 fw=0.1`, y `?` → la línea de estado. `F=` no debe incluir `imu`.
- `H 45` → el servo gira. `H -45` al otro lado.
- `M 120 120` → se encienden `ENA`, `ENB`, `IN1` e `IN3`, y se apagan solos a los 500 ms
  (watchdog: tecleando a mano no da tiempo de mandar otra `M`).
- Inclina el MPU6050 más de 35° (controles `accelX`/`accelY`) → `?` muestra `F=tilt` y `M` responde `ERR tilt`.
- `D 200` → hay un obstáculo a 20 cm durante 300 ms: `?` muestra `F=obstacle` y una `M` hacia
  adelante frena. Sustituye al ToF que Wokwi no tiene; como caduca rápido, pega `D 200` y `?` de una vez.
- Gira las perillas → los contadores `E=izq,der` de `?` cambian.

Ojo con el PID: los encoders no giran solos con los "motores", así que con `USAR_PID 1`
el lazo sube el PWM al máximo. Para ver el PWM directo pon `USAR_PID 0` en
`CONFIGURACIÓN`. Para probar la batería pon `HAY_DIVISOR_BAT 1` y baja el
potenciómetro: tras 5 s por debajo de ~3.2 V en A0 (9.9 V), `M` responde `ERR bateria`.
