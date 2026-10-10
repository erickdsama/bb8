# Parte 1 · Componentes en protoboard

Todo lo que corre en el Arduino: cinco sketches de prueba para verificar cada
componente y el firmware completo del robot.

```
arduino/
├── pruebas/
│   ├── p1_i2c/                 ¿están vivos el MPU6050 (0x68) y el VL53L0X (0x29)?
│   ├── p2_motores_encoders/    cada motor en los dos sentidos; su encoder cuenta hacia arriba
│   ├── p3_servo/               barrido del MG996R en D9
│   ├── p4_mpu6050/             inclinación y rumbo
│   └── p5_vl53l0x/             distancia
└── bb8_firmware/               protocolo serial, PID con encoders, reflejos, reposo profundo
wokwi/                          el firmware en el simulador Wokwi (diagram.json, wokwi.toml)
```

1. Cablea según la tabla de pines de la guía (un L298N para los dos motores, jumpers ENA/ENB fuera, buck a 5.1 V en vacío).
2. Sube `p1`…`p5` en orden y abre el monitor serial a 115200 con "Nueva línea".
3. Instala la librería **VL53L0X de Pololu**, ajusta `CONFIGURACIÓN` arriba de `bb8_firmware.ino` y súbelo.
4. Desde la Pi o el PC: `python -m calibrar consola --serial /dev/ttyACM0` y prueba `I`, `?`, `M 80 80`.

Sin protoboard todavía: [wokwi/](wokwi/) corre el firmware en el simulador Wokwi con el
MPU6050, el servo, los encoders y LEDs en lugar de los motores.

**Lista cuando** `I` contesta `OK BB8 fw=0.1` y motores, servo, IMU y ToF responden.

Guía completa: [wiki/Parte-1-Protoboard.md](../../wiki/Parte-1-Protoboard.md) ·
Protocolo serial: [wiki/Protocolo.md](../../wiki/Protocolo.md)
