# Simulador

Robot dummy: imita al Arduino (TCP `:5555`) y a la cabeza (`:8080`) para correr el
servicio de movimiento, el MCP y el agente de voz en el PC, sin hardware.

```bash
pip install -r requirements.txt && pip install -e .     # desde la raíz del repo
python simulador/lanzar_dummy.py --camara sintetica
python -m dummy.probar                                  # en otra terminal: 19 comprobaciones
```

| Archivo | Hace |
| --- | --- |
| `lanzar_dummy.py` | Arranca dummy + `bb8.motion_api` + `bb8_mcp` |
| `dummy/world.py` | Habitación de 4 × 3 m con una silla y una caja |
| `dummy/arduino_sim.py` | Firmware simulado: protocolo, rampa, watchdog, frenos |
| `dummy/run_dummy.py` | Arduino y cabeza simulados |
| `dummy/visor.py` | Visor 2D en <http://127.0.0.1:8080/sim> |
| `dummy/probar.py` | Prueba de punta a punta por MCP |

Guía completa: [wiki/Simulador.md](../wiki/Simulador.md)
