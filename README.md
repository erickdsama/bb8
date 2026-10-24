# BB-8 con agente LLM

Un BB-8 de 30–35 cm que rueda, escucha y ve, manejado por Claude (MCP) o por su propio
agente (OpenRouter, con clima y calendario). Funciona como un Sphero: una base de dos
ruedas rueda por dentro de la esfera y un poste con imanes sujeta la cabeza a través del
casco. El modelo nunca toca los motores: pide movimientos acotados, `bb8-body` decide si
obedece y el Arduino frena solo si algo sale mal.

```
voz (Python) ─┐
Claude (MCP) ─┼─► bb8-body (Go, Pi 3) ─► Arduino ─► motores, servo
web (config) ─┘     ▲  ▲       │
   Mando Bluetooth ─┘  │       └─► OpenRouter, clima, calendario
                       └── TLS mutuo ── bb8-head (Go, Pi Zero W: cámara, ToF, ojo, voz)
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
| 3 · Base andando | [`partes/3-base-andando`](partes/3-base-andando) | `calibrar` y planos; el cuerpo es `bb8-body` ([`go/`](go)) | [Parte 3](wiki/Parte-3-Base-andando.md) |
| 4 · Agente controla el movimiento | [`partes/4-agente-movimiento`](partes/4-agente-movimiento) | Micrófono `voz` (wake word, Whisper) → agente de `bb8-body` | [Parte 4](wiki/Parte-4-Agente-controla-el-movimiento.md) |
| 5 · Agente controla la cabeza | [`partes/5-cabeza`](partes/5-cabeza) | Pitidos y personalidad; la cabeza es `bb8-head` ([`go/`](go), Pi Zero W) | [Parte 5](wiki/Parte-5-Agente-controla-la-cabeza.md) |
| 6 · Esfera y baterías | [`partes/6-esfera-baterias`](partes/6-esfera-baterias) | Gestor de energía `energia` (reposo y batería baja) | [Parte 6](wiki/Parte-6-Esfera-y-baterias.md) |

Además:

| Carpeta | Qué hay |
| --- | --- |
| [`go/`](go) | `bb8-body` y `bb8-head`: movimiento, plano, MCP, agente, conectores, configuración, enlace TLS, cabeza |
| [`cad/`](cad) | Piezas imprimibles en OpenSCAD paramétrico: esfera, base, poste y cabeza, con STL y renders |
| [`simulador/`](simulador) | Robot dummy: Arduino y cabeza simulados para correr todo en el PC sin hardware, y la prueba de punta a punta |
| [`sistema/`](sistema) | Servicios systemd, instaladores para la Pi y la Zero, ejemplo de `/etc/bb8.env` (`make release` compila los binarios) |
| [`docs/`](docs) | Documento técnico original y los artefactos (diagramas, avance, mecánica, animación) |
| [`wiki/`](wiki) | La wiki: guías por parte, arquitectura, protocolo, materiales, instalación |

## Pruébalo en tu PC en 2 minutos

Necesitas Python 3.11 o más nuevo con [uv](https://docs.astral.sh/uv/) y Go 1.25 o más
nuevo ([go.dev/dl](https://go.dev/dl)).

```bash
git clone https://github.com/erickdsama/bb8 && cd bb8
uv venv && source .venv/bin/activate   # Windows: .venv\Scripts\activate
uv pip install -r requirements.txt
uv pip install -e .                    # registra los paquetes de Python de cada parte
python simulador/lanzar_dummy.py --camara sintetica   # compila bb8-body y bb8-head y arranca todo
```

En otra terminal, con el venv activo: `PYTHONPATH=simulador python -m dummy.probar` recorre
las herramientas, los frenos de seguridad y la configuración en vivo, y termina con
`Todo bien` (31 comprobaciones; `make probar` hace todo junto). La página de configuración
está en <http://127.0.0.1:8780/> (`bb8-web`, sin contraseña en el dummy). Para hablarle desde Claude Code:
`claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp`.
Dentro del repo, Claude Code ya lo encuentra por `.mcp.json`, y `CLAUDE.md` y `.claude/skills/` le explican el proyecto.
Detalles en [Simulador](wiki/Simulador.md) y [Conectar con Claude](wiki/Conectar-con-Claude.md).

## Go y Python

- **Go** (`go/`): lo que corre siempre en el robot, `bb8-body` en la Pi 3 y `bb8-head` en la
  Zero W. Son binarios estáticos de ~10–15 MB de RAM. `make test` corre sus pruebas y
  `make release` compila para arm64 y armv6.
- **Python**: el micrófono (`voz`: wake word, Whisper, identificación de voz), el gestor
  de energía, `calibrar` y el simulador. El código está repartido en la carpeta de cada
  parte; `pyproject.toml` mapea cada paquete a su carpeta, así que después de
  `uv pip install -e .` funcionan `python -m voz`, `python -m energia`,
  `python -m calibrar` y `python -m dummy.probar` desde cualquier carpeta.

## Estado

Partes 1–6 tienen código; el firmware compila para el Arduino Uno y todo se ha probado
contra el simulador, todavía no en el hardware real. Lo que falta está en
[Pendientes](wiki/Pendientes.md) y el avance en el
[tracker](docs/artefactos/avance-del-proyecto.html).
