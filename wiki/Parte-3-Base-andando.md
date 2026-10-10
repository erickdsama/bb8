# Parte 3 · Base andando desde la Pi

**Entregable:** la base se maneja por órdenes desde la Pi por USB, con el mando
Bluetooth y con una rutina automática: avanza, gira 90°, vuelve y frena en el piso sin
la esfera. Video de 30 s.

Código: [`partes/3-base-andando`](https://github.com/erickdsama/bb8/tree/main/partes/3-base-andando).

| Archivo | Hace |
| --- | --- |
| `bb8/config.py` | Constantes a calibrar, límites de seguridad y direcciones |
| `bb8/protocol.py` | Arma y lee las líneas seriales (lo comparten la Pi y el dummy) |
| `bb8/serial_link.py` | Una orden, una respuesta; abre `/dev/ttyACM0` o `socket://` |
| `bb8/motion.py` | Servicio de movimiento: árbitro mando/LLM, `move`/`turn` con encoders y giroscopio |
| `bb8/motion_api.py` | API HTTP local en `127.0.0.1:8770` |
| `bb8/gamepad.py` | Mando Bluetooth con `evdev` (solo Linux) |
| `calibrar/` | Consola serial, pulsos por vuelta, velocidad máxima y rutina de prueba |

## Preparar la Pi

Raspberry Pi 3 con Raspberry Pi OS Lite de 64 bits, Arduino Uno por USB con el firmware de la Parte 1.

```bash
git clone https://github.com/erickdsama/bb8 && cd bb8
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt evdev
pip install -e .
```

Empareja el mando con `bluetoothctl` (`scan on`, `pair`, `trust`, `connect`). Tu usuario
necesita los grupos `dialout` (serial) e `input` (mando).

## Calibrar

Con el puerto serial directo (detén antes `bb8.motion_api`, que es el único dueño del
serial):

```bash
python -m calibrar ticks        # gira la rueda 10 vueltas a mano → pulsos por vuelta
python -m calibrar velocidad    # ruedas en el aire → velocidad máxima y PWM mínimo
python -m calibrar consola      # órdenes a mano: M 80 80, ?, S, H 30…
```

`--serial` cambia el puerto (por defecto `BB8_SERIAL` o `/dev/ttyACM0`).

Copia los números a [`bb8/config.py`](https://github.com/erickdsama/bb8/blob/main/partes/3-base-andando/bb8/config.py)
y `PULSOS_MAX_CICLO` al firmware:

| Constante | Cómo medirla |
| --- | --- |
| `TICKS_PER_REV` | Girar la rueda 10 vueltas a mano, leer `E` con `?`, dividir entre 10 (de fábrica 1320) |
| `SLIP_FACTOR` | Mandar `move(1.0)`, medir con cinta; `SLIP_FACTOR = metros_ordenados / metros_reales` |
| `TRACK_WIDTH_M` | Distancia entre centros de las ruedas (de fábrica 0.17) |
| `V_MAX_MPS` | Velocidad a PWM 255 con la fuente de 12 V (~0.9 m/s con el L298N) |
| `PWM_CRUISE`, `PWM_MIN` | Subir hasta que cabecee y bajar un 20 %; mínimo con el que arranca desde parado |
| `PULSOS_MAX_CICLO` (firmware) | Lo que diga `calibrar velocidad` |

## Manejarla

```bash
python -m bb8.motion_api --mando      # /dev/ttyACM0, cabeza en bb8-head.local
```

El mando manda si se tocó en el último segundo (`MANUAL_HOLD_S`), con PWM máximo 150.
Desde otra terminal puedes probar la API:

```bash
curl -X POST localhost:8770/move -d '{"metros": 0.5}'
curl -X POST localhost:8770/turn -d '{"grados": 90}'
curl localhost:8770/pose
```

Rutas completas en [Protocolo § 2](Protocolo.md#2-http-del-servicio-de-movimiento-pi-1270018770).

## Rutina de prueba

Con el servicio corriendo, base en el piso y 1 m libre al frente:

```bash
python -m calibrar rutina    # avanza, gira 90°, vuelve; mide la deriva con la IMU
```

Para que arranque solo al encender: servicio `bb8-motion` en
[Instalación en la Pi](Instalacion-en-la-Pi.md).

## Lista cuando

- [ ] `calibrar ticks` y `velocidad` hechos; números copiados a `config.py` y al firmware
- [ ] El mando la conduce y suelta el control al segundo de no tocarlo
- [ ] `calibrar rutina` vuelve con deriva < 5°
- [ ] Frena sola ante una caja a 25 cm y al inclinarla más de 35°
- [ ] Video de 30 s
