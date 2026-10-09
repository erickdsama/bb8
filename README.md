# BB-8 con agente LLM

Un BB-8 de 30–35 cm que rueda, escucha y ve, manejado por Claude. Funciona como un
Sphero: una base de dos ruedas rueda por dentro de la esfera y un poste con imanes
sujeta la cabeza a través del casco. Claude nunca toca los motores: pide movimientos
acotados a un servidor MCP, el servicio de movimiento decide si obedece y el Arduino
frena solo si algo sale mal.

```
voz ─► Agente (Pi) ─► Claude ─► MCP bb8-motion ─► Servicio de movimiento ─► Arduino ─► motores, servo
                                     │                    ▲
                                     └─► Pi Zero (cabeza: cámara, ToF, ojo, voz)
                                          Mando Bluetooth ┘ (gana sobre el LLM)
```

**Toda la documentación está en la [wiki](wiki/Home.md).** Empieza por
[Arquitectura](wiki/Arquitectura.md) y luego sigue las partes en orden.

## El repo, parte por parte

El proyecto se construye en seis partes, en orden. Cada carpeta de `partes/` tiene
el código de esa parte y un README con qué hacer y cuándo está lista.

| Parte | Carpeta | Qué hay | Guía |
| --- | --- | --- | --- |
| 1 · Protoboard | [`partes/1-protoboard`](partes/1-protoboard) | Sketches de prueba `p1`…`p5` y el firmware completo del Arduino | [Parte 1](wiki/Parte-1-Protoboard.md) |
| 2 · Base de melamina | [`partes/2-base-melamina`](partes/2-base-melamina) | Montaje mecánico, sin código nuevo | [Parte 2](wiki/Parte-2-Base-de-melamina.md) |
| 3 · Base andando | [`partes/3-base-andando`](partes/3-base-andando) | Servicio de movimiento `bb8` (Pi) y `calibrar` | [Parte 3](wiki/Parte-3-Base-andando.md) |
| 4 · Agente controla el movimiento | [`partes/4-agente-movimiento`](partes/4-agente-movimiento) | Servidor MCP `bb8_mcp` y agente de voz `voz` | [Parte 4](wiki/Parte-4-Agente-controla-el-movimiento.md) |
| 5 · Agente controla la cabeza | [`partes/5-cabeza`](partes/5-cabeza) | Servidor de la cabeza `head` (Pi Zero 2 W) | [Parte 5](wiki/Parte-5-Agente-controla-la-cabeza.md) |
| 6 · Esfera y baterías | [`partes/6-esfera-baterias`](partes/6-esfera-baterias) | Gestor de energía `energia` (reposo y batería baja) | [Parte 6](wiki/Parte-6-Esfera-y-baterias.md) |

Además:

| Carpeta | Qué hay |
| --- | --- |
| [`simulador/`](simulador) | Robot dummy: Arduino y cabeza simulados para correr todo en el PC sin hardware, y la prueba de punta a punta |
| [`sistema/`](sistema) | Servicios systemd, instaladores para la Pi y la Zero, ejemplo de `/etc/bb8.env` |
| [`docs/`](docs) | Documento técnico original y los artefactos (diagramas, avance, mecánica, animación) |
| [`wiki/`](wiki) | La wiki: guías por parte, arquitectura, protocolo, materiales, instalación |

## Pruébalo en tu PC en 2 minutos

Necesitas Python 3.11 o más nuevo. En Windows usa `py` en lugar de `python3`.

```bash
git clone https://github.com/erickdsama/bb8 && cd bb8
python3 -m venv .venv
source .venv/bin/activate              # Windows: .venv\Scripts\activate
pip install -r requirements.txt
pip install -e .                       # registra los paquetes de cada parte
python simulador/lanzar_dummy.py --camara sintetica
```

En otra terminal, con el venv activo: `python -m dummy.probar` recorre las ocho
herramientas MCP y los frenos de seguridad, y termina con `Todo bien`. Para hablarle
desde Claude Code: `claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp`.
Detalles en [Simulador](wiki/Simulador.md) y [Conectar con Claude](wiki/Conectar-con-Claude.md).

## Por qué `pip install -e .`

El código de Python vive repartido en la carpeta de cada parte, pero los paquetes se
importan entre sí (`voz` usa `bb8.config`, el dummy usa `head`). `pyproject.toml`
mapea cada paquete a su carpeta, así que después de `pip install -e .` funcionan
`python -m bb8.motion_api`, `python -m bb8_mcp`, `python -m voz`, `python -m head.server`,
`python -m energia`, `python -m calibrar` y `python -m dummy.probar` desde cualquier
carpeta. `simulador/lanzar_dummy.py` funciona incluso sin ese paso.

## Estado

Partes 1–6 tienen código; el firmware compila para Uno y Nano y todo se ha probado
contra el simulador, todavía no en el hardware real. Lo que falta está en
[Pendientes](wiki/Pendientes.md) y el avance en el
[tracker](docs/artefactos/avance-del-proyecto.html).
