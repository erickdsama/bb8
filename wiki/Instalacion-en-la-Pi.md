# Instalación en la Pi

Los instaladores y servicios están en [`sistema/`](https://github.com/erickdsama/bb8/tree/main/sistema).

**Las Pis no necesitan Go.** Los binarios se compilan en tu PC y se copian:

```bash
make release        # dist/linux-arm64/bb8-body (Pi 3) y dist/linux-armv6/bb8-head (Zero W)
```

(También salen como artefacto `bb8-binaries` de cada corrida del CI.)

## Pi principal (cuerpo)

Raspberry Pi 3 B+ (1 GB) con Raspberry Pi OS Lite **de 64 bits** y el Arduino Uno por USB.
El sistema de 32 bits no sirve: faster-whisper y onnxruntime solo tienen ruedas `aarch64`,
y el instalador se detiene si ve `armv7l`. Fuente de 5 V 2.5 A.

```bash
# En la Pi
sudo git clone https://github.com/erickdsama/bb8 /home/bb8/bb8
sudo mkdir -p /home/bb8/bb8/dist/linux-arm64
# En el PC
scp dist/linux-arm64/bb8-body bb8.local:/tmp/ && ssh bb8.local sudo mv /tmp/bb8-body /home/bb8/bb8/dist/linux-arm64/
# En la Pi
cd /home/bb8/bb8
sudo bash sistema/instalar_pi.sh            # --sin-voz para instalar solo bb8-body
```

El instalador:

- instala `bb8-body` en `/usr/local/bin` y crea el usuario `bb8` (grupos `dialout`, `audio`, `input`);
- **empareja el enlace con la cabeza**: crea la CA privada y los certificados en
  `/etc/bb8/pki` (`bb8-body pair`). Solo se hace la primera vez;
- crea el token de acceso de la red en `/etc/bb8/token`;
- con menos de 2 GB de RAM activa swap comprimida en RAM (`zram-tools`, 50 %, zstd) y deja
  16 MB a la GPU (`gpu_mem=16`, aplica al reiniciar);
- con voz (por defecto): venv con `requirements.txt` +
  `partes/4-agente-movimiento/requirements-voz.txt`, voz de Piper `es_ES-davefx-medium`, y
  los servicios `bb8-voz` y `bb8-energia`;
- desactiva y borra los servicios viejos de Python `bb8-motion` y `bb8-mcp`, y arranca
  `bb8-body`.

Al terminar imprime la dirección de la página de configuración con el token:

```
http://bb8.local:8765/?token=…
```

Para manejarlo con botones abre **`http://bb8.local:8765/control?token=…`** (o el enlace
*Control* de esa página): flechas para avanzar y girar, STOP, la cabeza, `say` y el ojo, y
el estado en vivo. La página es un cliente MCP más: cada botón es una llamada a una
herramienta (`move`, `turn`, `look_at`…), con los mismos límites que tiene Claude.

Ábrela y pon ahí la clave de OpenRouter (`agent.api_key`), el modelo, tu ciudad
(`home.city`) y los conectores (clima, calendario). Todo eso vive en
`/var/lib/bb8/config.json` y `secrets.json` (0600), no en `/etc/bb8.env`. Los cambios se
aplican al momento y se ven en cualquier otra pestaña abierta.

Luego enciende la voz:

```bash
sudo systemctl enable --now bb8-voz bb8-energia
```

### Memoria en una Pi 3

Estimado, salvo `bb8-body` (medido en el PC; `free -m` lo confirmará en la Pi):

| Proceso | RAM aprox. |
| --- | --- |
| Raspberry Pi OS Lite + Bluetooth | ~120 MB |
| `bb8-body` (Go: movimiento, plano, MCP, agente, configuración, enlace) | ~15 MB (antes `bb8-motion` + `bb8-mcp`: ~100 MB) |
| `bb8-energia` | ~35 MB |
| `bb8-voz`: openWakeWord + VAD (onnxruntime) | ~120 MB |
| `bb8-voz`: Whisper `base` int8 (local) | ~200 MB (`tiny` ~120, `small` ~500: no cabe) |
| `bb8-voz`: Piper `es_ES-davefx-medium` | ~100 MB (0 con `BB8_VOZ_SALIDA=cabeza`) |
| **Total** | **~590 MB** de ~920 MB libres; ~390 MB con transcripción en la nube |

Resemblyzer (identificación de voz) trae torch, ~350 MB más. Si la quieres, usa la
transcripción en la nube.

`bb8-body` corre con `Nice=-5` y `OOMScoreAdjust=-500`; `bb8-voz` con `Nice=5`, y el OOM
mata primero a la voz. Si falta memoria, el robot sigue frenando y obedeciendo al mando.

### Transcripción: local o en la nube

| | Local (`BB8_STT=local`) | Nube (`BB8_STT=nube`) |
| --- | --- | --- |
| Modelo | faster-whisper `base` int8, 3 hilos | `whisper-large-v3-turbo` (Groq) |
| Espera por frase en la Pi 3 | ~4–8 s (estimado) | ~0.5–1 s más la red |
| Español | aceptable | muy bueno |
| Necesita | nada | internet y una clave |

Para la nube:
- Crea una clave en [console.groq.com](https://console.groq.com) (tiene plan gratuito) y
  ponla en `BB8_STT_API_KEY`. Con la clave puesta, la nube se usa sola.
- Sirve cualquier API compatible con `/v1/audio/transcriptions` de OpenAI, incluido un
  servidor `whisper.cpp` en tu PC (`BB8_STT_URL`).
- El audio de cada orden sale de la casa.

| Servicio | Comando | Parte |
| --- | --- | --- |
| `bb8-body` | `/usr/local/bin/bb8-body` | 3, 4 |
| `bb8-voz` | `python -m voz` (manda el texto a `bb8-body`, `POST /agent/ask`) | 4 |
| `bb8-energia` | `python -m energia` | 6 |

Logs: `journalctl -fu bb8-body` (o el servicio que sea).

Para ver la voz en vivo abre `http://bb8.local:8765/voice?token=…`:
- volumen del micrófono y puntaje de «Oye BB-8» frente a su umbral;
- estado (esperando, escuchando, pensando, hablando);
- lo que se transcribió y lo que contestó BB-8;
- con **voice → Keep recent voice clips** activado en la configuración, las últimas 20 frases
  con su audio (solo en RAM).

## Pi Zero W (cabeza)

Pi Zero W original (armv6) con Raspberry Pi OS Lite de 32 bits y hostname `bb8-head`.
**No lleva Python**: es un solo binario de Go.

```bash
# En el PC
scp dist/linux-armv6/bb8-head partes/5-cabeza/eye/bb8_eye.c \
    sistema/{instalar_zero.sh,bb8-head.service,bb8-eye.socket,bb8-eye@.service} bb8-head.local:/tmp/
# En la Pi 3: los certificados de la cabeza
sudo scp /etc/bb8/pki/{ca.crt,bb8-head.crt,bb8-head.key} bb8-head.local:/tmp/
# En la Zero
sudo bash /tmp/instalar_zero.sh bb8.local
rm /tmp/bb8-head.key
journalctl -fu bb8-head        # debe decir "link: connected to body"
```

El instalador:

- instala `bb8-head` y los certificados en `/etc/bb8/pki`;
- instala `rpicam-apps-lite` (cámara), `alsa-utils` (micrófono y pitidos) y `espeak-ng`
  (voz: Piper no corre en armv6);
- activa el ReSpeaker 2-Mic HAT v1 (códec WM8960, I2C 0x1a) con `dtparam=i2s=on` y
  `dtoverlay=wm8960-soundcard`, que ya vienen con el kernel. Tras reiniciar, `arecord -l`
  debe mostrar `wm8960soundcard`. La v2.0 (0x18) usa en cambio el overlay
  `respeaker-2mic-v2_0` de [seeed-linux-dtoverlays](https://github.com/Seeed-Studio/seeed-linux-dtoverlays)
  y `--mic plughw:CARD=seeed2micvoicec`;
- activa **SPI** (anillo NeoPixel) e **I2C** (VL53L0X) y apaga Bluetooth para ahorrar;
- habilita `bb8-head`, que corre como `bb8` con los grupos `spi i2c video audio gpio`, no
  como root.

La cabeza **no abre ningún puerto**: marca al cuerpo en `bb8.local:8772` y solo acepta y
presenta certificados de la CA del BB-8. Si el cuerpo cambia de IP no pasa nada: se
verifica por nombre (`bb8-body`), no por IP.

**El cable de datos del ojo va a GPIO12 (PWM0), en el puerto Grove digital del ReSpeaker**,
con 5 V y GND del power bank.
- El instalador clona rpi_ws281x (en un commit fijo) y compila en la Zero el ayudante `bb8-eye` (`partes/5-cabeza/eye/bb8_eye.c`), por eso instala `gcc` y `git`.
- Deja `bb8-eye.socket` activo y pone `dtparam=audio=off` para que el PWM quede libre.
- Hay que reiniciar una vez.

Sin la cámara OV5647 conectada, la cabeza arranca igual: `take_photo` responde "la
cabeza no tiene cámara" y lo demás funciona. Sin VL53L0X en la cabeza (en las Partes 1–4
está en el Arduino) no manda ToF y el Arduino usa el suyo.

## Actualizar

```bash
# PC
make release
scp dist/linux-arm64/bb8-body bb8.local:/tmp/ && ssh bb8.local 'sudo install -m 755 /tmp/bb8-body /usr/local/bin/ && sudo systemctl restart bb8-body'
scp dist/linux-armv6/bb8-head bb8-head.local:/tmp/ && ssh bb8-head.local 'sudo install -m 755 /tmp/bb8-head /usr/local/bin/ && sudo systemctl restart bb8-head'
# Pi 3, para la parte en Python
cd /home/bb8/bb8 && sudo -u bb8 git pull && sudo systemctl restart bb8-voz bb8-energia
```

## Variables de entorno

Van en `/etc/bb8.env` (ejemplo en `sistema/bb8.env.ejemplo`). El agente, los límites de
movimiento, la ciudad y los conectores **no** van aquí: están en la página de
configuración.

| Variable | Por defecto | Para qué |
| --- | --- | --- |
| `BB8_SERIAL` | `/dev/ttyACM0` | Puerto del Arduino; `socket://127.0.0.1:5555` en el dummy |
| `BB8_MAP` | — | Plano de la casa para `get_map` |
| `BB8_GAMEPAD` | `0` | `1` para leer el mando Bluetooth en `bb8-body` |
| `BB8_BODY_ADDR` | `bb8.local:8772` | En la Zero: a dónde se conecta la cabeza |
| `BB8_MOTION_URL` | `http://127.0.0.1:8770` | API local de `bb8-body` (la usan `voz` y `energia`) |
| `BB8_ENERGIA_URL` | `http://127.0.0.1:8771` | Gestor de energía |
| `BB8_WAKEWORD` | `voz/modelos/oye_bb8.onnx` si existe, si no `hey_jarvis` | Modelo preentrenado o ruta a un `.onnx` ([Wake word "oye BB-8"](Wake-word-oye-BB-8.md)) |
| `BB8_WAKEWORD_UMBRAL` | `0.5` | Sensibilidad de la wake word |
| `BB8_MIC` | `cabeza` | `cabeza`: el ReSpeaker de la cabeza por el enlace (`GET /audio/stream`); `local`: un micrófono en la Pi 3 |
| `BB8_MICROFONO` | el de por defecto | Con `BB8_MIC=local`: nombre o índice de `sounddevice` |
| `BB8_HEAD_MIC`, `BB8_MIC_SWAP`, `BB8_EYE` (en la Zero) | `plughw:CARD=wm8960soundcard`, `0`, `ws2812-pwm` con micrófono | Dispositivo ALSA del ReSpeaker, canales al revés, tipo de ojo (`ws2812-pwm`, `apa102`, `ws2812`, `none`) |
| `BB8_STT` | `wyoming` si hay `BB8_STT_WYOMING`; si no `nube` con `BB8_STT_API_KEY`; si no `local` | Dónde se transcribe |
| `BB8_STT_WYOMING` | — | `host:puerto` de un servidor Wyoming de transcripción en tu red (p. ej. `wyoming-faster-whisper` con `large-v3-turbo` en un PC con GPU, puerto 10300). Es lo que mejor entiende a 1 m |
| `BB8_TTS_WYOMING` (en la Zero) | — | `host:puerto` de un servidor Wyoming de voz (Piper). Sin él, la cabeza habla con `espeak-ng` |
| `BB8_ESPERA_SEGUIMIENTO_S` | `8` | Si BB-8 termina con una pregunta, escucha esta cantidad de segundos sin «Oye BB-8» (máximo 3 seguidas) |
| `BB8_GUARDAR_FRASES` | — | Carpeta donde guardar cada frase grabada como WAV (depuración) |
| `BB8_STT_API_KEY` | — | Clave de la API de transcripción (Groq, OpenAI…) |
| `BB8_STT_URL` | `https://api.groq.com/openai/v1/audio/transcriptions` | Endpoint compatible con OpenAI |
| `BB8_STT_MODELO` | `whisper-large-v3-turbo` | Modelo de la API (`whisper-1` en OpenAI) |
| `BB8_WHISPER` | `base` | Modelo local: `tiny` es ~2× más rápido y entiende peor; `small` no cabe en 1 GB |
| `BB8_WHISPER_HILOS` | `3` | Hilos de faster-whisper; el cuarto núcleo queda para el movimiento |
| `BB8_VOCES` | `voz/voces.json` | Voces registradas |
| `BB8_VOZ_SALIDA` | `local` | `local` (altavoz USB de la Pi, Parte 4) o `cabeza` (Parte 5, a través de `bb8-body`) |
| `BB8_PIPER_MODELO` | `voz/modelos/es_ES-davefx-medium.onnx` | Voz de Piper |
| `BB8_BUSCAR_CARA` | `0` | `1` para girar la cabeza hacia la voz (Parte 5) |
| `BB8_REPOSO_PROFUNDO` | `0` | `1` con el P-MOSFET de A3 instalado (Parte 6) |
| `BB8_SIN_SONIDO` | — | En el dummy, no reproducir pitidos |
