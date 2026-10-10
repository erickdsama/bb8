# Simulador (robot dummy)

El simulador reemplaza los dos extremos del robot, el Arduino y la cabeza, y deja
todo lo demás igual: el servicio de movimiento, el servidor MCP y el agente de voz son
exactamente el código que corre en la Pi. Sirve para probar a Claude manejando el
robot antes de tener hardware.

Código: [`simulador/`](https://github.com/erickdsama/bb8/tree/main/simulador).

| Archivo | Hace |
| --- | --- |
| `dummy/world.py` | Habitación de 4 × 3 m con una silla y una caja |
| `dummy/arduino_sim.py` | Firmware simulado por TCP `:5555`: mismo protocolo, rampa, watchdog, frenos por ToF e inclinación |
| `dummy/run_dummy.py` | Arranca el Arduino y la cabeza simulados |
| `dummy/visor.py`, `dummy/visor.html` | Visor 2D en el navegador (`/sim`) |
| `dummy/probar.py` | Prueba de punta a punta por MCP |
| `lanzar_dummy.py` | Arranca dummy + servicio de movimiento + MCP con un solo comando |

## Instalar

Python 3.11 o más nuevo. En Windows usa `py` en lugar de `python3`.

```bash
git clone https://github.com/erickdsama/bb8 && cd bb8
python3 -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
pip install -r requirements.txt
pip install -e .
```

## Correr

```bash
python simulador/lanzar_dummy.py                    # webcam 0 como cámara de la cabeza
python simulador/lanzar_dummy.py --camara sintetica # Claude ve la habitación simulada
python simulador/lanzar_dummy.py --camara 1         # otra webcam
python simulador/lanzar_dummy.py --camara ninguna   # cabeza sin cámara (take_photo falla, lo demás no)
python simulador/lanzar_dummy.py --sin-sonido       # sin pitidos en el PC
```

Cuando aparece `✅ BB-8 dummy listo`, corren tres procesos:

| Proceso | Dirección | Hace de |
| --- | --- | --- |
| `dummy.run_dummy` | serial `socket://127.0.0.1:5555`, cabeza `http://127.0.0.1:8080` | Arduino y Pi Zero |
| `bb8.motion_api` | `http://127.0.0.1:8770` | Servicio de movimiento (igual que en la Pi) |
| `bb8_mcp` | `http://127.0.0.1:8765/mcp` | Servidor MCP (igual que en la Pi) |

Con `--camara sintetica` Claude ve paredes, una silla café a 1.6 m al frente y una
caja verde: útil para probar "mira, esquiva y sigue". Con la webcam ve tu cuarto real
aunque el robot se mueva por la habitación simulada.

## Ver al robot moverse

Con el dummy corriendo, abre <http://127.0.0.1:8080/sim> en el navegador. Es una
vista desde arriba de la habitación que se actualiza 10 veces por segundo mientras
Claude (o `dummy.probar`) maneja el robot por el MCP:

- El robot a escala, con una flecha hacia donde avanza y el ojo dibujado del lado
  hacia donde mira la cabeza, en su color y con su patrón (fijo, respirar, parpadeo).
- El rayo del ToF punteado hasta lo que toca; se pone rojo por debajo de 25 cm, la
  distancia a la que el Arduino frena solo.
- La estela del camino recorrido y un globo con lo último que dijo.
- Al lado: pose, rumbo, ángulo de la cabeza, inclinación, distancia del ToF, PWM de
  cada motor (actual → objetivo), frenos activos (obstáculo, inclinación, watchdog,
  dormido, chocando) y la cámara de la cabeza una vez por segundo.
- Botones para pausar la cámara, borrar la estela y "empujar 40°" (frena por
  inclinación, como en la prueba).

El visor solo mira: no manda órdenes de movimiento. Lee
`GET /sim/mundo`, que también sirve para scripts:

```bash
curl http://127.0.0.1:8080/sim/mundo
```

Se sirve desde el mismo proceso de la cabeza simulada, así que no hay que instalar
nada más ni abrir otro puerto.

## Probar que todo funciona

Con el dummy corriendo con `--camara sintetica`, en otra terminal con el venv activo:

```bash
python -m dummy.probar
```

Recorre las ocho herramientas, choca a propósito contra la silla (debe frenar solo a
~21 cm), comprueba que el mando gane al LLM, que `stop` cancele y que una inclinación
de 40° frene. Guarda la foto en `foto_dummy.jpg` y termina con `Todo bien`
(19 comprobaciones).

## Calibración contra el dummy

`calibrar` también habla con el Arduino simulado, útil para practicar la Parte 3:

```bash
python -m calibrar consola --serial socket://127.0.0.1:5555
```

Detén antes el servicio de movimiento, que es el único dueño del serial (en el dummy,
corre `python -m dummy.run_dummy` solo).

## Problemas comunes

| Síntoma | Causa |
| --- | --- |
| `No arrancó http://127.0.0.1:8080/estado` | Otro proceso usa el puerto 8080, 5555, 8770 u 8765 |
| `ModuleNotFoundError: bb8` al usar `python -m` | Falta `pip install -e .` en el venv activo |
| `ImportError: FastMCP` | Tienes `mcp` 2.x; `requirements.txt` fija `mcp>=1.10,<2` |
| `/cara` falla | `opencv-python` 5.x quitó los detectores Haar; se fija `<5` |
