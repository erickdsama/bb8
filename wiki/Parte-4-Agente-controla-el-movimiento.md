# Parte 4 · Agente controla el movimiento

**Entregable:** le dices "Oye Jarvis… ven acá" o "da una vuelta" y la base lo hace;
log de la conversación con las llamadas a herramientas.

El cerebro vive en `bb8-body` (Go). En la Pi también corre, en Python, el lado del
micrófono: wake word, transcripción e identificación de voz.

| Código | Hace |
| --- | --- |
| [`go/internal/tools`](https://github.com/erickdsama/bb8/tree/main/go/internal/tools), [`body/robot_tools.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/body/robot_tools.go) | Un solo registro de herramientas: `move`, `turn`, `stop`, `look_at`, `take_photo`, `set_eye_color`, `say`, `express`, `listen`, `get_map`, `set_position`, `update_map`, `get_pose`, `get_setting`, `set_setting` y las de los conectores (`weather`, `calendar_*`) |
| [`go/internal/mcpserver`](https://github.com/erickdsama/bb8/tree/main/go/internal/mcpserver) | Servidor MCP en `:8765/mcp` (token desde la red) |
| [`go/internal/agent`](https://github.com/erickdsama/bb8/tree/main/go/internal/agent) | Agente propio: cualquier modelo de OpenRouter con herramientas; `POST /agent/ask` |
| [`go/internal/persona`](https://github.com/erickdsama/bb8/blob/main/go/internal/persona/persona.go) | Personalidad y reglas: el system prompt para Claude y para el agente |
| [`go/internal/connectors`](https://github.com/erickdsama/bb8/tree/main/go/internal/connectors) | Clima (Open-Meteo) y calendario (Google), como módulos |
| `voz/escucha.py` | openWakeWord + Silero VAD (ONNX, sin torch) |
| `voz/stt.py` | faster-whisper o API en la nube |
| `voz/hablantes.py` | Registro e identificación de voces (Resemblyzer, opcional) |
| `voz/agente.py` | Manda lo transcrito a `bb8-body` (`POST /agent/ask`) con quién habló |
| `voz/salida.py` | Piper local, o el altavoz de la cabeza a través de `bb8-body`; pitidos |
| `voz/__main__.py` | El bucle; sus frases fijas (buenas noches, sin señal…) salen de `head.personalidad.frase()` |
| `voz/cara.py` | Gira la cabeza hacia la voz (Parte 5) |
| `wakeword/` | Entrenamiento de la wake word "oye BB-8" (Colab o PC, no en la Pi) |

## Paso 1: MCP desde Claude Code o Desktop

Con la [Parte 3](Parte-3-Base-andando.md) funcionando (`bb8-body` corriendo), el MCP ya
está en `http://bb8.local:8765/mcp`. Conéctalo como en
[Conectar con Claude](Conectar-con-Claude.md) y prueba con el robot en el piso:
*"avanza medio metro y gira a la derecha"*. Antes de tener el robot, todo esto funciona
igual con el [simulador](Simulador.md).

## Paso 2: el agente propio

En la página de configuración (`http://bb8.local:8765/?token=…`) pon `agent.api_key` (tu
clave de [OpenRouter](https://openrouter.ai)) y, si quieres, otro `agent.model`. Pruébalo
con el chat de la misma página: *"da una vuelta y dime qué ves"*.

## Paso 3: micrófono y altavoz

Micrófono USB (o ReSpeaker 2-Mic HAT) en la Pi principal (una Pi 3); la voz sale por su
jack de 3.5 mm → PAM8403 → bocina. `sistema/instalar_pi.sh` instala el venv, las
dependencias de voz y la voz de Piper `es_ES-davefx-medium` (a mano, de
`huggingface.co/rhasspy/piper-voices`, archivos `.onnx` y `.onnx.json`, en
`partes/4-agente-movimiento/voz/modelos/`).

## Paso 4: probar por partes

```bash
python -m voz.agente "da una vuelta y dime qué ves"   # texto, sin micrófono
python -m voz.hablantes registrar Erick               # 4 frases de ~3 s; necesita resemblyzer
python -m voz.hablantes lista
python -m voz                                         # el bucle completo
python -m voz --sin-voz-registrada                    # obedece a cualquiera
```

Ciclo: wake word → graba hasta 0.8 s de silencio → Whisper → ¿quién habla? →
`bb8-body` (`/agent/ask`: modelo de OpenRouter con las herramientas) → Piper.

## Ajustes

- **Wake word:** "hey Jarvis" de fábrica. Para "oye BB-8", entrena tu modelo con el cuaderno de `wakeword/` en Colab y copia `oye_bb8.onnx` a `voz/modelos/`; el agente lo usa solo. Ver [Wake word "oye BB-8"](Wake-word-oye-BB-8.md).
- **Modelo:** `agent.model` en la página de configuración (por defecto `anthropic/claude-sonnet-5.5`). `agent.max_steps` limita las llamadas a herramientas por orden (8) y `agent.reset_after_s` olvida la conversación tras 5 min de silencio.
- **Voces desconocidas:** pueden platicar, pero `move`, `turn` y `look_at` se bloquean en el código de `bb8-body` (`speaker_known: false`), no solo en el prompt. Mientras no registres ninguna voz, obedece a todos. No es biometría segura.
- **Ajustes por voz:** BB-8 puede cambiar algunos ajustes cuando se lo piden ("habla más bajo", "vivo en Monterrey") con `set_setting`. Solo los marcados para el agente; los límites de movimiento no. El cambio aparece al momento en la página de configuración.
- **Transcripción:** en la Pi 3, Whisper `base` local tarda ~4–8 s por frase (estimado). Con `BB8_STT_API_KEY` (Groq) se transcribe en la nube en ~1 s. Comparación y memoria en [Instalación en la Pi](Instalacion-en-la-Pi.md#transcripción-local-o-en-la-nube).
- **Identificación de voz:** `resemblyzer` instala torch (~1 GB de disco, ~350 MB de RAM). En la Pi 3 solo cabe con la transcripción en la nube; descoméntalo en `requirements-voz.txt` si lo quieres.
- **Dormir:** "BB-8, a dormir" pasa al reposo (profundo si está habilitado, Parte 6).

Todas las variables en [Instalación en la Pi](Instalacion-en-la-Pi.md#variables-de-entorno).
Para que arranque solo: servicios `bb8-body` y `bb8-voz`.

## Lista cuando

- [ ] Claude Code mueve la base por MCP
- [ ] El chat de la página de configuración responde y usa herramientas
- [ ] `python -m voz.agente "..."` responde
- [ ] Tu voz registrada; una voz desconocida no logra moverlo
- [ ] "Oye Jarvis… ven acá" y se mueve
- [ ] (Opcional) "Oye BB-8… ven acá" con tu modelo entrenado
