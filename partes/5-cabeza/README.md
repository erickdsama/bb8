# Parte 5 · Agente controla la cabeza

En la Pi Zero W corre `bb8-head`, un binario de Go ([`go/cmd/bb8-head`](../../go/cmd/bb8-head),
código en [`go/internal/head`](../../go/internal/head)). Se conecta al cuerpo por TLS mutuo
(`:8772`) y no abre puertos. Esta carpeta tiene la cabeza anterior en Python, de la que
`voz` y el simulador todavía usan los pitidos y la personalidad.

```
head/
├── sounds.py           pitidos de droide (15 emociones + balbuceo); los usan voz y el dummy
├── personalidad.py     emoción → pitido + ojo y frases cortas en español (las usa voz)
├── backends_sim.py     webcam del PC o vista sintética (la usa el simulador)
├── server.py, backends_real.py, colors.py   cabeza anterior en Python (ya no se instala)
requirements-zero.txt   solo para la cabeza anterior
```

Instalar (ver [wiki/Instalacion-en-la-Pi.md](../../wiki/Instalacion-en-la-Pi.md#pi-zero-w-cabeza)):

```bash
make zero                                   # en el PC: dist/linux-armv6/bb8-head
sudo bash /tmp/instalar_zero.sh bb8.local   # en la Zero, con el binario y los certificados en /tmp
```

**El cable de datos del ojo va a GPIO12 (PWM0), en el puerto Grove digital del ReSpeaker**,
con 5 V y GND del power bank. Lo maneja `bb8-eye` ([`eye/bb8_eye.c`](eye/bb8_eye.c), rpi_ws281x),
que el instalador compila en la Zero. En la Pi principal, `/etc/bb8.env`: `BB8_VOZ_SALIDA=cabeza` y `BB8_BUSCAR_CARA=1`.
Si el ToF se muda a la cabeza, `USAR_TOF_LOCAL 0` en el firmware.

Casco, plato y soportes impresos de la cabeza: [`cad/`](../../cad).
Escuchar los pitidos en el PC (solo librería estándar):

```bash
python -m head.sounds                       # todos, con su descripción
python -m head.sounds --texto "¿Quién anda ahí?"
python -m head.sounds --html sonidos.html   # página para oírlos en el navegador
```

**Lista cuando** gira la cabeza hacia ti y describe lo que ve.

Guía completa: [wiki/Parte-5-Agente-controla-la-cabeza.md](../../wiki/Parte-5-Agente-controla-la-cabeza.md)
