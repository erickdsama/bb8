# Parte 1 · Componentes en protoboard

**Entregable:** los dos motores giran en ambos sentidos desde el Arduino con sus
encoders contando, el servo barre, la IMU y el ToF responden, y el firmware completo
contesta `OK BB8 fw=0.1`.

Código: [`partes/1-protoboard/arduino`](https://github.com/erickdsama/bb8/tree/main/partes/1-protoboard/arduino).

## Materiales

Arduino Uno o Nano, 1 L298N (el segundo queda de repuesto), 2 motorreductores
JGB37-520B 12 V 319 RPM con encoder, servo MG996R, MPU6050, VL53L0X, buck LM2596,
condensador 1000 µF 16 V, adaptador de 12 V 2 A con jack, protoboard + cables, Raspberry
Pi con su cargador. Todo viene en el [pedido UNIT 377466](Materiales.md).

## Cableado

| Pin Arduino | Va a | Uso |
| --- | --- | --- |
| D5 · D6 | L298N ENA · ENB | PWM izq · der (Timer0) |
| D7 · D8 | L298N IN1 · IN2 | Sentido motor izquierdo |
| D11 · D12 | L298N IN3 · IN4 | Sentido motor derecho |
| D2 · D4 | Encoder izq C1 · C2 | D2 = INT0 |
| D3 · D10 | Encoder der C1 · C2 | D3 = INT1 |
| D9 | Servo MG996R (naranja) | La librería Servo toma Timer1 |
| A4 · A5 | MPU6050 (0x68) + VL53L0X (0x29) | I2C SDA · SCL |
| A2 | MPU6050 INT | Despertar del reposo |
| A0 | Divisor de batería | Opcional, Parte 6 |
| A3 | MOSFET de la Pi | Parte 6, reposo profundo |
| USB | Raspberry Pi | Serial 115200 y 5 V |

- Adaptador 12 V → L298N (+12 V, GND) y → LM2596.
- LM2596 ajustado a **5.1 V sin nada conectado** → condensador 1000 µF (pata larga a +) → servo (rojo 5.1 V, café GND).
- En el L298N: quita los jumpers ENA y ENB, deja el jumper 5V-EN. No uses su salida de 5 V.
- Encoders: VCC 5 V y GND del Arduino.
- **Un solo riel de GND** para fuente, L298N, buck, servo y Arduino; la Pi llega por el USB.
- El Arduino come 5 V del USB de la Pi, así que una caída de los 12 V no lo reinicia.

Diagrama completo en el artefacto [Diagramas](Artefactos.md#diagramas), sección 1.

> Con el adaptador de 2 A no frenes una rueda con la mano mientras se mueve el servo:
> un motor trabado más el servo a tope pasan de 2 A.

## Sketches de prueba, en orden

Ábrelos en el Arduino IDE, sube, y abre el monitor serial a 115200 con "Nueva línea".
Ruedas en el aire la primera vez.

| Sketch | Qué prueba | Debe pasar |
| --- | --- | --- |
| `pruebas/p1_i2c` | Escáner I2C | Ve `0x68` (MPU6050) y `0x29` (VL53L0X) |
| `pruebas/p2_motores_encoders` | Cada motor en los dos sentidos. Teclas: `a`/`z` izq, `k`/`m` der, `b` ambos, espacio freno, `+`/`-` PWM, `r` reinicia contadores | "Adelante" hace avanzar al robot y su encoder cuenta **hacia arriba**. Si no, apunta qué invertir |
| `pruebas/p3_servo` | Barre −90…+90° y luego acepta un ángulo por el monitor (`30`, `-45`) | Llega a los extremos sin zumbar; servo alimentado del buck, nunca del pin 5V |
| `pruebas/p4_mpu6050` | Inclinación y rumbo | El rumbo sube al girar a la derecha; si baja, cambia `SIGNO_GIRO_Z` |
| `pruebas/p5_vl53l0x` | Distancia (librería VL53L0X de Pololu) | Sigue tu mano; alcance útil ~1.2 m en interiores, "fuera de rango" mirando al vacío es normal |

`p2_motores_encoders` también sirve para medir pulsos por vuelta: `r`, gira la rueda
10 vueltas a mano, divide `E` entre 10 y anótalo para `TICKS_PER_REV` (Parte 3).

## Firmware completo

`bb8_firmware/bb8_firmware.ino`. Instala la librería **VL53L0X de Pololu** desde el
Gestor de librerías y súbelo. Lo que se ajusta está arriba del archivo, en
`CONFIGURACIÓN`:

| Ajuste | Cuándo cambiarlo |
| --- | --- |
| `INVERTIR_MOTOR_IZQ` / `_DER`, `INVERTIR_ENC_IZQ` / `_DER` | Según lo que viste en `p2_motores_encoders` |
| `SIGNO_GIRO_Z` | Si `p4_mpu6050` baja el rumbo al girar a la derecha |
| `SERVO_INVERTIDO` | Si `H 40` gira la cabeza a la izquierda |
| `PULSOS_MAX_CICLO` | Con lo que diga `python -m calibrar velocidad` (Parte 3) |
| `USAR_TOF_LOCAL` | `0` cuando el ToF se mude a la cabeza (Parte 5) |
| `USAR_PID` | `0` si un encoder falla y quieres seguir probando en lazo abierto |
| `HAY_DIVISOR_BAT` | `1` con el divisor 10 kΩ / 4.7 kΩ de la batería en A0 (Parte 6) |

Con `arduino-cli`:

```bash
arduino-cli lib install VL53L0X
arduino-cli compile --fqbn arduino:avr:uno partes/1-protoboard/arduino/bb8_firmware
arduino-cli upload -p /dev/ttyACM0 --fqbn arduino:avr:uno partes/1-protoboard/arduino/bb8_firmware
```

(Nano: `arduino:avr:nano`, o `arduino:avr:nano:cpu=atmega328old` si es un clon con bootloader viejo.)

El robot tiene que estar **quieto el primer segundo** mientras calibra el giroscopio.
Si `?` responde `F=imu`, el MPU6050 no contestó al arrancar.

## Probar el firmware en Wokwi

Antes de cablear, el firmware corre en el simulador [Wokwi](https://wokwi.com) con el
MPU6050, el servo, dos encoders KY-040 y LEDs en lugar del L298N. El VL53L0X no existe
en Wokwi; el obstáculo se simula con la orden `D`. Instrucciones y qué probar en
[`partes/1-protoboard/wokwi/`](https://github.com/erickdsama/bb8/blob/main/partes/1-protoboard/wokwi/README.md).

El CI compila el firmware para Uno y Nano en cada push; el `.hex` queda como artefacto
`firmware-uno` / `firmware-nano` de la corrida.

## Probar el firmware a mano

Desde la Pi o el PC, con el repo instalado (`pip install -e .`):

```bash
python -m calibrar consola --serial /dev/ttyACM0
```

Escribe órdenes del [protocolo](Protocolo.md): `I`, `?`, `M 80 80`, `H 40`, `S`.
Sin órdenes `M` durante 500 ms, el watchdog frena solo.

## Lista cuando

- [ ] `p1`…`p5` pasan
- [ ] `I` contesta `OK BB8 fw=0.1` y `?` no muestra `F=imu`
- [ ] `M 80 80` mueve las dos ruedas hacia adelante y se detienen solas a los 500 ms
- [ ] Foto del cableado tomada (sirve en la Parte 2 para no perder conexiones)
