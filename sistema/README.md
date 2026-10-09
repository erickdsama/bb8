# Sistema

Servicios systemd e instaladores. Ambos instaladores esperan el repo en `/home/bb8/bb8`.

| Archivo | Va en | Hace |
| --- | --- | --- |
| `instalar_pi.sh` | Pi principal | Usuario `bb8`, venv, dependencias, `pip install -e .`, voz de Piper, servicios |
| `instalar_zero.sh` | Pi Zero 2 W | picamera2, venv, Piper, servicio de la cabeza |
| `bb8-motion.service` | Pi principal | `python -m bb8.motion_api --mando` (Parte 3) |
| `bb8-mcp.service` | Pi principal | `python -m bb8_mcp` (Parte 4) |
| `bb8-voz.service` | Pi principal | `python -m voz` (Parte 4) |
| `bb8-energia.service` | Pi principal | `python -m energia` (Parte 6) |
| `bb8-cabeza.service` | Pi Zero | `python -m head.server` (Parte 5) |
| `bb8.env.ejemplo` | `/etc/bb8.env` | API key y ajustes `BB8_*` |

Guía: [wiki/Instalacion-en-la-Pi.md](../wiki/Instalacion-en-la-Pi.md)
