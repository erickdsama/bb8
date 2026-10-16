# Simulador (robot dummy)

El simulador reemplaza el hardware de los dos extremos del robot, el Arduino y la
cabeza, y deja todo lo demás igual: `bb8-body` y `bb8-head` son exactamente los
binarios de Go que corren en las Pis (la cabeza con `--sim`, que reenvía a la cabeza
simulada en Python en lugar de tocar cámara, ojo y altavoz). Sirve para probar a Claude
o al agente propio manejando el robot antes de tener hardware.

Código: [`simulador/`](https://github.com/erickdsama/bb8/tree/main/simulador).

| Archivo | Hace |
| --- | --- |
| `dummy/world.py` | Habitación de 4 × 3 m con una silla y una caja |
| `dummy/arduino_sim.py` | Firmware simulado por TCP `:5555`: mismo protocolo, rampa, watchdog, frenos por ToF e inclinación |
| `dummy/run_dummy.py` | Arranca el Arduino y la cabeza simulados |
| `dummy/visor.py`, `dummy/visor.html` | Visor 2D en el navegador (`/sim`) |
| `dummy/probar.py` | Prueba de punta a punta por MCP |
| `lanzar_dummy.py` | Compila los binarios de Go y arranca dummy + `bb8-body` + `bb8-head --sim` con un solo comando |

## Instalar

Python 3.11 o más nuevo con [uv](https://docs.astral.sh/uv/), y Go 1.25 o más nuevo
([go.dev/dl](https://go.dev/dl); basta con descomprimirlo en `~/.local/go`).

```bash
git clone https://github.com/erickdsama/bb8 && cd bb8
uv venv && source .venv/bin/activate   # Windows: .venv\Scripts\activate
uv pip install -r requirements.txt
uv pip install -e .
make test                              # pruebas de Go (opcional)
```

## Correr

```bash
python simulador/lanzar_dummy.py                    # webcam 0 como cámara de la cabeza
python simulador/lanzar_dummy.py --camara sintetica # Claude ve la habitación simulada
python simulador/lanzar_dummy.py --camara 1         # otra webcam
python simulador/lanzar_dummy.py --camara ninguna   # cabeza sin cámara (take_photo falla, lo demás no)
python simulador/lanzar_dummy.py --sin-sonido       # sin pitidos en el PC
python simulador/lanzar_dummy.py --mundo casa       # casa de 8×6 m con cuartos, muebles y una persona
python simulador/lanzar_dummy.py --fov 90           # apertura de la cámara sintética (54 = OV5647 normal)
BB8_MCP_PORT=8775 python simulador/lanzar_dummy.py  # MCP y página de configuración en otro puerto
python simulador/lanzar_dummy.py --python           # stack anterior en Python (bb8.motion_api + bb8_mcp)
```

Si cambias el puerto, corre la prueba con el mismo (`BB8_MCP_PORT=8775 PYTHONPATH=simulador python -m dummy.probar`)
y registra ese MCP en Claude Code: `claude mcp add -s local --transport http bb8-pc http://127.0.0.1:8775/mcp`.

Cuando aparece `✅ BB-8 dummy listo` (la cabeza ya se conectó al cuerpo), corren tres
procesos:

| Proceso | Dirección | Hace de |
| --- | --- | --- |
| `dummy.run_dummy` | serial `socket://127.0.0.1:5555`, cabeza simulada `http://127.0.0.1:8080` | Hardware del Arduino y de la Pi Zero |
| `bin/bb8-body` | MCP y configuración `http://127.0.0.1:8765` (`/mcp`, `/`), API local `127.0.0.1:8770`, enlace `127.0.0.1:8772` | El cuerpo, igual que en la Pi 3 |
| `bin/bb8-head --sim` | se conecta al `:8772` por TLS mutuo | La cabeza, igual que en la Zero |

Los certificados del enlace son de prueba y se crean en una carpeta temporal en cada
arranque; la configuración (`config.json`, `secrets.json`) también va a una carpeta
temporal, así no toca la de tu robot. La página de configuración del dummy está en
<http://127.0.0.1:8765/> (en localhost no pide token): cámbiale algo y mira cómo
aparece al momento en otra pestaña, o pídele a BB-8 que "hable más bajo" y verás el
cambio llegar desde el agente.

Con `--camara sintetica` Claude ve paredes, una silla café a 1.6 m al frente y una
caja verde: útil para probar "mira, esquiva y sigue". Con la webcam ve tu cuarto real
aunque el robot se mueva por la habitación simulada.

### Mundos

| `--mundo` | Qué hay |
| --- | --- |
| `habitacion` (por defecto) | 4 × 3 m, una silla y una caja. Es el que usa `dummy.probar`: no lo cambies sin ajustar la prueba. |
| `casa` | 8 × 6 m: sala, cocina, recámara y estudio, con muros interiores y puertas de 90 cm (sala↔cocina, sala↔recámara, cocina↔estudio). Sofá, tele, mesita, refri, barra, comedor, cama, ropero, librero, escritorio, plantas y una pelota, de alturas distintas. Una **persona** (playera azul, cara color piel, 1.70 m) empieza en el estudio; arrástrala en el visor para jugar a "búscame". |

La cámara sintética usa por defecto los 54° horizontales de la OV5647 con lente normal;
con `--fov` imitas una versión gran angular (90–120°), que ve mucho más del cuarto desde
el piso. La cámara sintética es 3D ([`simulador/dummy/render3d.py`](https://github.com/erickdsama/bb8/blob/main/simulador/dummy/render3d.py)):
un rayo por píxel con numpy, a 35 cm del piso. Dibuja piso de madera (azulejo en la
cocina), paredes con zoclo hasta el techo, cada mueble con sus piezas (mesas con patas,
sofá con respaldo, cama con almohada, tele, librero con libros, plantas) y a la persona
con piernas, brazos y cara, siempre mirando al robot. Solo es lo que se ve: la física y
el ToF siguen usando la huella 2D de cada objeto. Un cuadro tarda ~0.1 s en la
habitación y ~0.4 s en la casa. Los mundos viven en `MUNDOS` de
`simulador/dummy/world.py`: cada uno es una función que devuelve tamaño, cajas, pose
inicial y nombres de los cuartos.

### Plano

`lanzar_dummy.py` escribe el plano del mundo elegido (sin la persona, que se mueve) en un
JSON temporal y se lo pasa a `bb8-body` con `--map`, así `get_map`
funciona igual que en el robot con el plano de tu casa. La pose en el plano sale de la
odometría simulada, con su deriva. Lo que Claude cambie con `update_map` queda en ese
JSON temporal (lo que *cree*), no en el mundo: el dummy sigue igual.

### Oído (ReSpeaker 2-Mic simulado)

En el visor, la tarjeta **Oído** hace hablar a la persona (botón *Hablar*, o
`POST /sim/hablar {"texto": "...", "x": 1, "y": 2}` desde cualquier punto). El dummy
calcula lo que daría el ReSpeaker 2-Mic HAT del cuerpo
([`simulador/dummy/oido.py`](https://github.com/erickdsama/bb8/blob/main/simulador/dummy/oido.py)):
ángulo de −90 a 90° respecto al frente, **sin distinguir adelante de atrás**, con ruido,
y 12 dB menos (y más ruido) por cada muro de por medio. Lo manda a `bb8-body`
(`POST /hearing` en la API local) y Claude lo lee con la herramienta `listen`. En el mapa salen
anillos desde quien habló y dos líneas verdes desde el robot: continua el candidato de
adelante, punteada el de atrás.

En el robot real lo mandará la cabeza por el enlace (evento `sound_dir`) cuando lea el
ángulo del ReSpeaker; eso queda pendiente hasta que llegue el HAT.

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

Con el dummy recién arrancado con `--camara sintetica`, en otra terminal con el venv activo:

```bash
PYTHONPATH=simulador python -m dummy.probar   # o todo junto: make probar
```

Recorre las herramientas, choca a propósito contra la silla (debe frenar solo a
~21 cm), comprueba el plano, el oído, que el mando gane al LLM, que `stop` cancele,
que una inclinación de 40° frene y la configuración en los dos sentidos (lo que cambia
el agente llega a la web y a la cabeza; la web con un `rev` viejo recibe 409). Guarda
la foto en `foto_dummy.jpg` y termina con `Todo bien` (30 comprobaciones). Necesita un
dummy recién arrancado: las pruebas de movimiento asumen la pose inicial.

## Calibración contra el dummy

`calibrar` también habla con el Arduino simulado, útil para practicar la Parte 3:

```bash
python -m calibrar consola --serial socket://127.0.0.1:5555
```

Detén antes `bb8-body`, que es el único dueño del serial (en el dummy, corre
`python -m dummy.run_dummy` solo).

## Problemas comunes

| Síntoma | Causa |
| --- | --- |
| `No arrancó http://127.0.0.1:8080/estado` | Otro proceso usa el puerto 8080, 5555, 8770, 8772 u 8765 |
| `Falta Go` | Instala Go 1.25+ en el PATH o en `~/.local/go` (o usa `--python`) |
| `La cabeza no se conectó al cuerpo` | `bb8-head` no llegó al `:8772`: mira el log (`link:`) |
| `ModuleNotFoundError: dummy` al correr la prueba | Falta `PYTHONPATH=simulador` |
| La prueba falla en `move` la segunda vez | El dummy no es nuevo: reinícialo |
