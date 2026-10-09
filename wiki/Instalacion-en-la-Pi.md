# Instalación en la Pi

Los instaladores y servicios están en [`sistema/`](https://github.com/erickdsama/bb8/tree/main/sistema).
Ambos esperan el repo clonado en `/home/bb8/bb8`.

## Pi principal

Raspberry Pi OS Lite de 64 bits, Arduino conectado por USB.

```bash
sudo git clone https://github.com/erickdsama/bb8 /home/bb8/bb8
cd /home/bb8/bb8
sudo bash sistema/instalar_pi.sh
sudoedit /etc/bb8.env          # pon tu ANTHROPIC_API_KEY
sudo systemctl start bb8-motion bb8-mcp bb8-energia
sudo systemctl enable --now bb8-voz
```

El instalador crea el usuario `bb8` (grupos `dialout`, `audio`, `input`), el venv en
`.venv`, instala `requirements.txt` + `partes/4-agente-movimiento/requirements-voz.txt`
+ `evdev`, registra los paquetes con `pip install -e .`, descarga la voz de Piper
`es_ES-davefx-medium`, copia `/etc/bb8.env` y habilita los servicios.

| Servicio | Comando | Parte |
| --- | --- | --- |
| `bb8-motion` | `python -m bb8.motion_api --mando` | 3 |
| `bb8-mcp` | `python -m bb8_mcp` | 4 |
| `bb8-voz` | `python -m voz` | 4 |
| `bb8-energia` | `python -m energia` | 6 |

Logs: `journalctl -fu bb8-voz` (o el servicio que sea).

## Pi Zero 2 W (cabeza)

Hostname `bb8-head` y I2C + cámara activados en `sudo raspi-config`.

```bash
sudo git clone https://github.com/erickdsama/bb8 /home/bb8/bb8
cd /home/bb8/bb8
sudo bash sistema/instalar_zero.sh
curl http://bb8-head.local:8080/estado
```

Instala `python3-picamera2`, `python3-opencv`, `alsa-utils`, el venv con
`--system-site-packages`, `partes/5-cabeza/requirements-zero.txt`, Piper y su voz en
`/opt/piper`, apaga Bluetooth para ahorrar y habilita `bb8-cabeza` (corre como root
por el NeoPixel en GPIO18).

## Actualizar

```bash
cd /home/bb8/bb8 && sudo -u bb8 git pull
sudo systemctl restart bb8-motion bb8-mcp bb8-energia bb8-voz
```

## Variables de entorno

Van en `/etc/bb8.env` (ejemplo en `sistema/bb8.env.ejemplo`).

| Variable | Por defecto | Para qué |
| --- | --- | --- |
| `ANTHROPIC_API_KEY` | — | Agente de voz |
| `BB8_SERIAL` | `/dev/ttyACM0` | Puerto del Arduino; `socket://127.0.0.1:5555` en el dummy |
| `BB8_HEAD_URL` | `http://bb8-head.local:8080` | Cabeza |
| `BB8_MOTION_HOST` / `BB8_MOTION_PORT` / `BB8_MOTION_URL` | `127.0.0.1` / `8770` | Servicio de movimiento |
| `BB8_MCP_HOST` / `BB8_MCP_PORT` | `127.0.0.1` / `8765` | Servidor MCP |
| `BB8_MCP_URL` | `http://127.0.0.1:8765/mcp` | A dónde se conecta el agente de voz |
| `BB8_ENERGIA_URL` | `http://127.0.0.1:8771` | Gestor de energía |
| `BB8_MODELO` | `claude-opus-5-5` | Modelo del agente de voz |
| `BB8_ESFUERZO` | `low` | Esfuerzo del modelo; `medium` si planea mal los movimientos |
| `BB8_WAKEWORD` | `voz/modelos/oye_bb8.onnx` si existe, si no `hey_jarvis` | Nombre de un modelo preentrenado o ruta a un `.onnx` ([Wake word "oye BB-8"](Wake-word-oye-BB-8.md)) |
| `BB8_WAKEWORD_UMBRAL` | `0.5` | Sensibilidad de la wake word |
| `BB8_MICROFONO` | el de por defecto | Nombre o índice de `sounddevice` |
| `BB8_WHISPER` | `small` | `base` es más rápido en una Pi 4 |
| `BB8_WHISPER_HILOS` | `4` | Hilos de faster-whisper |
| `BB8_VOCES` | `voz/voces.json` | Voces registradas |
| `BB8_VOZ_SALIDA` | `local` | `local` (altavoz USB de la Pi, Parte 4) o `cabeza` (Parte 5) |
| `BB8_PIPER_MODELO` | `voz/modelos/es_ES-davefx-medium.onnx` | Voz de Piper |
| `BB8_BUSCAR_CARA` | `0` | `1` para girar la cabeza hacia quien habla (Parte 5) |
| `BB8_REPOSO_PROFUNDO` | `0` | `1` con el P-MOSFET de A3 instalado (Parte 6) |
| `BB8_SIN_SONIDO` | — | En el dummy, no reproducir pitidos |
