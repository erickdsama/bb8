# Parte 3 · Base andando desde la Pi

El servicio de movimiento de la Pi principal y la herramienta de calibración.

```
bb8/                  paquete del servicio de movimiento
├── config.py         constantes a calibrar, límites y direcciones
├── protocol.py       líneas seriales (lo comparten la Pi y el dummy)
├── serial_link.py    una orden, una respuesta; /dev/ttyACM0 o socket://
├── motion.py         árbitro mando/LLM; move/turn con encoders y giroscopio
├── motion_api.py     API HTTP 127.0.0.1:8770
└── gamepad.py        mando Bluetooth con evdev (solo Linux)
calibrar/             consola, ticks, velocidad, rutina
```

Desde la raíz del repo, con el venv activo:

```bash
pip install -r requirements.txt evdev && pip install -e .
python -m calibrar ticks          # pulsos por vuelta (detén antes bb8.motion_api)
python -m calibrar velocidad      # ruedas en el aire
python -m bb8.motion_api --mando  # el servicio, con el mando Bluetooth
python -m calibrar rutina         # en el piso: avanza, gira 90°, vuelve
```

Copia los números a `bb8/config.py` y `PULSOS_MAX_CICLO` al firmware.

**Lista cuando** avanza, gira 90° y vuelve con deriva < 5°.

Guía completa: [wiki/Parte-3-Base-andando.md](../../wiki/Parte-3-Base-andando.md)
