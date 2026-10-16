# Parte 3 · Base andando desde la Pi

En la Pi principal corre `bb8-body`, un binario de Go
([`go/cmd/bb8-body`](../../go/cmd/bb8-body); movimiento en
[`go/internal/motion`](../../go/internal/motion)). Esta carpeta tiene la calibración y
los planos.

```
calibrar/             consola, ticks, velocidad, rutina (Python)
mapas/                planos de ejemplo (ejemplo_casa.json = mundo «casa» del dummy; claves en inglés)
bb8/                  servicio anterior en Python (lo usan calibrar y lanzar_dummy.py --python)
```

```bash
make pi                                   # en el PC: dist/linux-arm64/bb8-body
sudo bash sistema/instalar_pi.sh --sin-voz   # en la Pi (ver wiki/Instalacion-en-la-Pi.md)
uv venv && uv pip install -r requirements.txt && uv pip install -e .   # para calibrar
sudo systemctl stop bb8-body              # calibrar abre el serial directo
python -m calibrar ticks                  # pulsos por vuelta
python -m calibrar velocidad              # ruedas en el aire
sudo systemctl start bb8-body             # con BB8_GAMEPAD=1 lee el mando
python -m calibrar rutina                 # en el piso: avanza, gira 90°, vuelve
```

Los números van a la página de configuración (`http://bb8.local:8765/`, sección
`motion`, se aplican al momento) y `PULSOS_MAX_CICLO` al firmware.

**Lista cuando** avanza, gira 90° y vuelve con deriva < 5°.

Guía completa: [wiki/Parte-3-Base-andando.md](../../wiki/Parte-3-Base-andando.md)
