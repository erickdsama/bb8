# Instalación en la Pi

Los instaladores y servicios están en [`sistema/`](https://github.com/erickdsama/bb8/tree/main/sistema).
Ambos esperan el repo clonado en `/home/bb8/bb8`.

## Pi principal

Raspberry Pi 3 (1 GB) con Raspberry Pi OS Lite **de 64 bits**, Arduino Uno conectado por
USB. El de 32 bits no sirve: faster-whisper y onnxruntime solo tienen ruedas `aarch64`,
y el instalador se detiene si ve `armv7l`. Fuente de 5 V 2.5 A.

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
`es_ES-davefx-medium`, copia `/etc/bb8.env` y habilita los servicios. Con menos de 2 GB
de RAM también activa swap comprimida en RAM (`zram-tools`, 50 %, zstd) y deja 16 MB a la
GPU (`gpu_mem=16`, aplica al reiniciar).

### Memoria en una Pi 3

Estimado, sin medir en la Pi todavía (`free -m` y `systemctl status bb8-voz` lo dirán):

| Proceso | RAM aprox. |
| --- | --- |
| Raspberry Pi OS Lite + Bluetooth | ~120 MB |
| `bb8-motion` + `bb8-mcp` + `bb8-energia` | ~130 MB |
| `bb8-voz`: openWakeWord + VAD (onnxruntime) | ~120 MB |
| `bb8-voz`: Whisper `base` int8 (local) | ~200 MB (`tiny` ~120, `small` ~500: no cabe) |
| `bb8-voz`: Piper `es_ES-davefx-medium` | ~100 MB (0 con `BB8_VOZ_SALIDA=cabeza`) |
| **Total** | **~670 MB** de ~920 MB libres; ~470 MB con transcripción en la nube |

Resemblyzer (identificación de voz) trae torch, ~350 MB más: en la Pi 3 no cabe junto a
Whisper local. Si la quieres, usa la transcripción en la nube.

`bb8-motion` corre con `Nice=-5` y `bb8-voz` con `Nice=5`, y el OOM mata primero a la voz:
si falta memoria el robot sigue frenando y obedeciendo al mando.

### Transcripción: local o en la nube

| | Local (`BB8_STT=local`) | Nube (`BB8_STT=nube`) |
| --- | --- | --- |
| Modelo | faster-whisper `base` int8, 3 hilos | `whisper-large-v3-turbo` (Groq) |
| Espera por frase en la Pi 3 | ~4–8 s (estimado) | ~0.5–1 s más la red |
| Español | aceptable | muy bueno |
| Necesita | nada | internet y una clave |

Para la nube, crea una clave en [console.groq.com](https://console.groq.com) (tiene plan
gratuito) y ponla en `BB8_STT_API_KEY`; con la clave puesta la nube se usa sola. Sirve
cualquier API compatible con `/v1/audio/transcriptions` de OpenAI, incluido un
`whisper.cpp` server en tu PC (`BB8_STT_URL`). El audio de cada orden sale de la casa.

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

Sin la cámara OV5647 conectada el servidor arranca igual: `/estado` dice
`"camara":"ninguna"` y `/foto` y `/cara` responden 503.

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
| `BB8_WAKEWORD` | `hey_jarvis` | Nombre de un modelo preentrenado o ruta a tu `.onnx` |
| `BB8_WAKEWORD_UMBRAL` | `0.5` | Sensibilidad de la wake word |
| `BB8_MICROFONO` | el de por defecto | Nombre o índice de `sounddevice` |
| `BB8_STT` | `local`, o `nube` si hay `BB8_STT_API_KEY` | Dónde se transcribe |
| `BB8_STT_API_KEY` | — | Clave de la API de transcripción (Groq, OpenAI…) |
| `BB8_STT_URL` | `https://api.groq.com/openai/v1/audio/transcriptions` | Endpoint compatible con OpenAI |
| `BB8_STT_MODELO` | `whisper-large-v3-turbo` | Modelo de la API (`whisper-1` en OpenAI) |
| `BB8_WHISPER` | `base` | Modelo local; `tiny` es ~2× más rápido y entiende peor; `small` no cabe en 1 GB |
| `BB8_WHISPER_HILOS` | `3` | Hilos de faster-whisper; el cuarto núcleo queda para el movimiento |
| `BB8_VOCES` | `voz/voces.json` | Voces registradas |
| `BB8_VOZ_SALIDA` | `local` | `local` (altavoz USB de la Pi, Parte 4) o `cabeza` (Parte 5) |
| `BB8_PIPER_MODELO` | `voz/modelos/es_ES-davefx-medium.onnx` | Voz de Piper |
| `BB8_BUSCAR_CARA` | `0` | `1` para girar la cabeza hacia quien habla (Parte 5) |
| `BB8_REPOSO_PROFUNDO` | `0` | `1` con el P-MOSFET de A3 instalado (Parte 6) |
| `BB8_SIN_SONIDO` | — | En el dummy, no reproducir pitidos |
