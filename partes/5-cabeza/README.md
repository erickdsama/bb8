# Parte 5 · Agente controla la cabeza

El servidor HTTP de la cabeza, que corre en la Pi Zero 2 W.

```
head/
├── server.py           API :8080 (/estado /foto /tof /ojo /hablar /cara /reposo)
├── backends_real.py    picamera2, VL53L0X/L1X, NeoPixel en GPIO18, Piper (sin probar en hardware)
├── backends_sim.py     webcam del PC o vista sintética (la usa el simulador)
├── colors.py           colores y patrones del ojo
└── sounds.py           pitidos
requirements-zero.txt
```

En la Zero (hostname `bb8-head`, I2C y cámara activados):

```bash
sudo git clone https://github.com/erickdsama/bb8 /home/bb8/bb8
cd /home/bb8/bb8 && sudo bash sistema/instalar_zero.sh
curl http://bb8-head.local:8080/estado
```

En la Pi principal, `/etc/bb8.env`: `BB8_VOZ_SALIDA=cabeza` y `BB8_BUSCAR_CARA=1`.
Si el ToF se muda a la cabeza, `USAR_TOF_LOCAL 0` en el firmware.

**Lista cuando** gira la cabeza hacia ti y describe lo que ve.

Guía completa: [wiki/Parte-5-Agente-controla-la-cabeza.md](../../wiki/Parte-5-Agente-controla-la-cabeza.md)
