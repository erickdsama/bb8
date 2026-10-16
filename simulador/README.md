# Simulador

Robot dummy: imita el hardware del Arduino (TCP `:5555`) y de la cabeza (`:8080`) para
correr en el PC, sin hardware, los mismos binarios de Go que van en las Pis
(`bb8-body` y `bb8-head --sim`).

```bash
uv pip install -r requirements.txt && uv pip install -e .   # desde la raíz del repo; Go 1.25+ también
python simulador/lanzar_dummy.py --camara sintetica        # compila bin/ y arranca todo
python simulador/lanzar_dummy.py --camara sintetica --mundo casa   # casa con una persona
PYTHONPATH=simulador python -m dummy.probar                 # en otra terminal: 30 comprobaciones (dummy nuevo)
make probar                                                 # las dos cosas juntas
```

MCP en <http://127.0.0.1:8765/mcp>, página de configuración en <http://127.0.0.1:8765/>.

| Archivo | Hace |
| --- | --- |
| `lanzar_dummy.py` | Compila y arranca dummy + `bb8-body` + `bb8-head --sim` (`--python`: el stack anterior) |
| `dummy/world.py` | Mundos: `habitacion` (4 × 3 m, silla y caja; el de la prueba) y `casa` (8 × 6 m, cuatro cuartos y una persona) |
| `dummy/arduino_sim.py` | Firmware simulado: protocolo, rampa, watchdog, frenos |
| `dummy/run_dummy.py` | Arduino y cabeza simulados |
| `dummy/render3d.py` | Cámara sintética en 3D (un rayo por píxel con numpy) |
| `dummy/oido.py` | ReSpeaker 2-Mic simulado: ángulo de la voz (adelante/atrás ambiguo), nivel y muros |
| `dummy/visor.py` | Visor 2D en <http://127.0.0.1:8080/sim> |
| `dummy/probar.py` | Prueba de punta a punta por MCP |

Guía completa: [wiki/Simulador.md](../wiki/Simulador.md)
