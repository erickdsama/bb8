# Parte 3 · Base andando desde la Pi

**Entregable:** la base se maneja por órdenes desde la Pi por USB, con el mando
Bluetooth y con una rutina automática: avanza, gira 90°, vuelve y frena en el piso sin
la esfera. Video de 30 s.

En la Pi corre `bb8-body`, un binario de Go
([`go/cmd/bb8-body`](https://github.com/erickdsama/bb8/tree/main/go/cmd/bb8-body)).

| Código | Hace |
| --- | --- |
| [`go/internal/protocol`](https://github.com/erickdsama/bb8/tree/main/go/internal/protocol), [`serial`](https://github.com/erickdsama/bb8/tree/main/go/internal/serial) | Arma y lee las líneas seriales; una orden, una respuesta; abre `/dev/ttyACM0` o `socket://` |
| [`go/internal/motion`](https://github.com/erickdsama/bb8/tree/main/go/internal/motion) | Árbitro mando/LLM, `move`/`turn` con encoders y giroscopio, límites (`Params`, con topes fijos en `Clamp`) |
| [`go/internal/gamepad`](https://github.com/erickdsama/bb8/tree/main/go/internal/gamepad) | Mando Bluetooth leyendo `/dev/input` (evdev) directo |
| [`go/internal/body/api.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/body/api.go) | API local en `127.0.0.1:8770` |
| [`partes/3-base-andando/calibrar/`](https://github.com/erickdsama/bb8/tree/main/partes/3-base-andando/calibrar) | Consola serial, pulsos por vuelta, velocidad máxima y rutina de prueba (Python) |
| `partes/3-base-andando/mapas/` | Planos de la casa |
| `partes/3-base-andando/bb8/` | El servicio anterior en Python; lo usan `calibrar` y `lanzar_dummy.py --python` |

## Preparar la Pi

Raspberry Pi 3 con Raspberry Pi OS Lite de 64 bits y el Arduino Uno por USB con el
firmware de la Parte 1. Compila `bb8-body` en el PC (`make pi`) e instálalo con
`sistema/instalar_pi.sh --sin-voz` ([Instalación en la Pi](Instalacion-en-la-Pi.md)).
Para `calibrar` hace falta el venv de Python:

```bash
cd /home/bb8/bb8
uv venv && uv pip install -r requirements.txt && uv pip install -e .
```

Empareja el mando con `bluetoothctl` (`scan on`, `pair`, `trust`, `connect`). Tu usuario
necesita los grupos `dialout` (serial) e `input` (mando).

## Calibrar

Con el puerto serial directo (detén antes `bb8-body`, que es el único dueño del
serial: `sudo systemctl stop bb8-body`):

```bash
python -m calibrar ticks        # gira la rueda 10 vueltas a mano → pulsos por vuelta
python -m calibrar velocidad    # ruedas en el aire → velocidad máxima y PWM mínimo
python -m calibrar consola      # órdenes a mano: M 80 80, ?, S, H 30…
```

`--serial` cambia el puerto (por defecto `BB8_SERIAL` o `/dev/ttyACM0`).

Escribe los números en la página de configuración (`bb8-web`, sección
`motion`): se aplican al momento, sin reiniciar. `PULSOS_MAX_CICLO` va al firmware:

| Ajuste | Cómo medirlo |
| --- | --- |
| `motion.ticks_per_rev` | Girar la rueda 10 vueltas a mano, leer `E` con `?`, dividir entre 10 (de fábrica 1320) |
| `motion.wheel_diameter_m` | Diámetro de la llanta (65 mm las de RC) |
| `motion.slip_factor` | Mandar `move(1.0)`, medir con cinta; `slip_factor = metros_ordenados / metros_reales` |
| `motion.track_width_m` | Distancia entre centros de las ruedas (de fábrica 0.17) |
| `motion.pwm_cruise`, `motion.pwm_min` | Subir hasta que cabecee y bajar un 20 %; mínimo con el que arranca desde parado. Tope fijo: 160 |
| `PULSOS_MAX_CICLO` (firmware) | Lo que diga `calibrar velocidad` |

## Manejarla

```bash
sudo systemctl start bb8-body          # o a mano: bb8-body --gamepad
journalctl -fu bb8-body
```

Con `BB8_GAMEPAD=1` en `/etc/bb8.env` lee el mando. El mando manda si se tocó en el último
segundo (`motion.manual_hold_s`), con PWM máximo `motion.pwm_manual_max` (150). Desde la
Pi puedes probar la API local:

```bash
curl -X POST localhost:8770/tools/move -d '{"meters": 0.5}'
curl -X POST localhost:8770/tools/turn -d '{"degrees": 90}'
curl localhost:8770/pose
```

Rutas completas en [Protocolo § 2](Protocolo.md).

### El plano de tu casa

Para que Claude sepa en qué cuarto está (`get_map`), mide la casa y escribe un JSON como
[`mapas/ejemplo_casa.json`](https://github.com/erickdsama/bb8/blob/main/partes/3-base-andando/mapas/ejemplo_casa.json):
rectángulos de cuartos, muros y muebles, el centro de cada puerta y
`start`, el punto y el rumbo donde arranca el robot (su base). Las claves van en inglés
(`rooms`, `doors`, `walls`, `furniture`). Ejes: x al este, y al norte, en metros desde la
esquina suroeste; rumbo horario desde el este. Ponlo en la Pi y en `/etc/bb8.env`:

```bash
BB8_MAP=/home/bb8/bb8/partes/3-base-andando/mapas/mi_casa.json
```

La posición sale de las ruedas y se desvía; Claude la corrige con `set_position` cuando
una foto le dice dónde está. `bb8-body` guarda la última pose en
`/var/lib/bb8/pose.json` (`--map-state`): tras un reinicio o el reposo profundo arranca desde ahí, marcada como
incierta hasta que Claude la confirme. El plano no es fijo: Claude lo actualiza con
`update_map` (una puerta cerrada, una silla nueva) y el servicio reescribe el JSON
dejando la versión anterior en `mi_casa.json.bak`; también puedes editarlo a mano con el
servicio andando, se recarga solo. Formato completo en [Protocolo § 4](Protocolo.md).

### Que llegue sola: `navigate_to` y JEV

Con el plano, BB-8 puede ir sola a un cuarto o a un mueble (`navigate_to`, *"ve a la
cocina"*). Avanza en pasos cortos: gira hacia la siguiente puerta, avanza, y si algo le
estorba barre la cabeza (`scan`) y le da la vuelta. Cada paso lo decide un planificador
rápido. El LLM solo le dice a dónde ir.

- Con `navigation.jev_api_key` (tu clave de [TypeSafe](https://console.typesafe.ai)),
  decide **JEV** (70–500 ms por paso). Si duda (confianza < `navigation.min_confidence`)
  o falla, ese paso lo deciden las reglas locales.
- Sin clave, solo las reglas locales.
- La pestaña **Activity** de `/control` muestra cada paso, quién lo decidió y su confianza.

### El sensor de distancia va en la cabeza

El VL53L0X mira al frente desde la cabeza (34 cm de alto, 10° hacia abajo), así que gira
con ella:

- Antes de avanzar, la cabeza se endereza sola.
- Si no hay lectura (la cabeza desconectada), avanza como máximo 30 cm
  (`motion.blind_max_m`).
- Las cosas bajas (un zapato, un juguete) se ven desde ~60 cm y luego quedan por debajo
  del cono. BB-8 se acuerda de ellas y frena a 25 cm aunque ya no las vea
  (`motion.obstacle_memory_m`).

## Rutina de prueba

Con el servicio corriendo, base en el piso y 1 m libre al frente:

```bash
python -m calibrar rutina    # avanza, gira 90°, vuelve; mide la deriva con la IMU
```

Para que arranque solo al encender: servicio `bb8-body` en
[Instalación en la Pi](Instalacion-en-la-Pi.md).

## Lista cuando

- [ ] `calibrar ticks` y `velocidad` hechos; números en la página de configuración y en el firmware
- [ ] El mando la conduce y suelta el control al segundo de no tocarlo
- [ ] `calibrar rutina` vuelve con deriva < 5°
- [ ] Frena sola ante una caja a 25 cm y al inclinarla más de 35°
- [ ] Video de 30 s
