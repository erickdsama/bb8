# Parte 4 · Agente controla el movimiento

**Entregable:** le dices "Oye Jarvis… ven acá" o "da una vuelta" y la base lo hace;
log de la conversación con las llamadas a herramientas.

Código: [`partes/4-agente-movimiento`](https://github.com/erickdsama/bb8/tree/main/partes/4-agente-movimiento).

| Archivo | Hace |
| --- | --- |
| `bb8_mcp/` | Servidor MCP `bb8-motion` (FastMCP, `:8765/mcp`): `move`, `turn`, `stop`, `look_at`, `take_photo`, `set_eye_color`, `say`, `express`, `get_pose`. Su system prompt suma la personalidad de `head/personalidad.py` |
| `voz/escucha.py` | openWakeWord + Silero VAD (ONNX, sin torch) |
| `voz/stt.py` | faster-whisper |
| `voz/hablantes.py` | Registro e identificación de voces (Resemblyzer, opcional) |
| `voz/agente.py` | Claude con las herramientas MCP |
| `voz/salida.py` | Piper local o en la cabeza, y pitidos |
| `voz/__main__.py` | El bucle; sus frases fijas (buenas noches, sin señal…) salen de `head.personalidad.frase()` |
| `voz/cara.py` | Gira la cabeza hacia quien habla (Parte 5) |
| `requirements-voz.txt` | Dependencias del agente de voz |

## Paso 1: el servidor MCP desde Claude Code o Desktop

Con la [Parte 3](Parte-3-Base-andando.md) funcionando (`bb8.motion_api` corriendo):

```bash
python -m bb8_mcp                    # 127.0.0.1:8765/mcp
python -m bb8_mcp --host 0.0.0.0     # para conectarte desde el portátil (solo WiFi de casa)
python -m bb8_mcp --stdio            # para Claude Desktop
```

Conéctalo como en [Conectar con Claude](Conectar-con-Claude.md) y prueba con el robot
en el piso: *"avanza medio metro y gira a la derecha"*. Antes de tener el robot,
todo esto funciona igual con el [simulador](Simulador.md).

## Paso 2: micrófono y altavoz

Micrófono USB (o ReSpeaker 2-Mic HAT) en la Pi principal (una Pi 3); la voz sale por su jack de 3.5 mm → PAM8403 → bocina.

```bash
sudo apt install libportaudio2 libopenblas0
pip install -r partes/4-agente-movimiento/requirements-voz.txt
export ANTHROPIC_API_KEY=sk-ant-...
```

La voz de Piper `es_ES-davefx-medium` va en `partes/4-agente-movimiento/voz/modelos/`
(el instalador de la Pi la descarga; a mano, de
`huggingface.co/rhasspy/piper-voices`, archivos `.onnx` y `.onnx.json`).

## Paso 3: probar por partes

```bash
python -m voz.agente "da una vuelta y dime qué ves"   # texto, sin micrófono
python -m voz.hablantes registrar Erick               # 4 frases de ~3 s; necesita resemblyzer
python -m voz.hablantes lista
python -m voz                                         # el bucle completo
python -m voz --sin-voz-registrada                    # obedece a cualquiera
```

Ciclo: wake word → graba hasta 0.8 s de silencio → Whisper → ¿quién habla? → Claude con
las herramientas MCP → Piper.

## Ajustes

- **Wake word:** "hey Jarvis" de fábrica. Para "oye BB-8" entrena tu modelo con el cuaderno de openWakeWord (genera voces sintéticas, ~1 h en Colab) y pon la ruta del `.onnx` en `BB8_WAKEWORD`.
- **Modelo:** `claude-opus-5-5` con esfuerzo `low` para que conteste rápido (`BB8_MODELO`, `BB8_ESFUERZO`). Lleva activado el respaldo del servidor: si un clasificador rechaza una petición, la API la reintenta con otro modelo.
- **Voces desconocidas:** pueden platicar, pero `move`, `turn` y `look_at` se bloquean en el código, no solo en el prompt. Mientras no registres ninguna voz, obedece a todos. No es biometría segura.
- **Whisper:** `small` tarda 2–4 s en una Pi 4; `BB8_WHISPER=base` es más rápido.
- **Identificación de voz:** `resemblyzer` instala torch (~1 GB); descoméntalo en `requirements-voz.txt` si lo quieres.
- **Dormir:** "BB-8, a dormir" pasa al reposo (profundo si está habilitado, Parte 6).

Todas las variables en [Instalación en la Pi](Instalacion-en-la-Pi.md#variables-de-entorno).
Para que arranque solo: servicios `bb8-mcp` y `bb8-voz`.

## Lista cuando

- [ ] Claude Code mueve la base por MCP
- [ ] `python -m voz.agente "..."` responde y usa herramientas
- [ ] Tu voz registrada; una voz desconocida no logra moverlo
- [ ] "Oye Jarvis… ven acá" y se mueve
