# Sistema

systemd units and installers. The Go binaries are built on the PC (`make pi`, `make zero`)
and copied to the Pis; neither Pi needs Go.

| File | Goes on | Does |
| --- | --- | --- |
| `instalar_pi.sh` | Main Pi (Pi 3 B+, 64-bit OS) | Installs `bb8-body`, pairs the head link (`/etc/bb8/pki`), creates the token (`/etc/bb8/token`), zram; with voice: venv, Piper, `bb8-voz` and `bb8-energia` |
| `instalar_zero.sh` | Pi Zero W (armv6, 32-bit OS) | Installs `bb8-head`, the head certs, rpicam, espeak-ng, SPI and I2C; builds `bb8-eye` with rpi_ws281x and sets `dtparam=audio=off`. No Python |
| `bb8-body.service` | Main Pi | `bb8-body`: serial, motion, map, MCP + config web (:8765), local API (127.0.0.1:8770), head link (:8772) |
| `bb8-head.service` | Pi Zero W | `bb8-head`: dials the body over mutual TLS, opens no ports |
| `bb8-eye.socket`, `bb8-eye@.service` | Pi Zero W | Ayudante `bb8-eye` (C, rpi_ws281x) como root por conexión: el ojo WS2812 en GPIO12 (PWM0) |
| `bb8-voz.service` | Main Pi | `python -m voz`: wake word, STT, speaker id → `POST /agent/ask` |
| `bb8-energia.service` | Main Pi | `python -m energia` (part 6) |
| `bb8.env.ejemplo` | `/etc/bb8.env` | `BB8_*` settings of the services (agent and limits live in the config page) |

Guide: [wiki/Instalacion-en-la-Pi.md](../wiki/Instalacion-en-la-Pi.md)
