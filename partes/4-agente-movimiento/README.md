# Parte 4 · Agente controla el movimiento

The tools, the MCP server and the OpenRouter agent live in `bb8-body` (Go):
[`go/internal/tools`](../../go/internal/tools), [`mcpserver`](../../go/internal/mcpserver),
[`agent`](../../go/internal/agent), [`connectors`](../../go/internal/connectors). This folder
holds the microphone side (Python).

```
voz/                    wake word → VAD → Whisper → ¿quién habla? → bb8-body /agent/ask → Piper
├── escucha.py          openWakeWord + Silero VAD
├── stt.py              faster-whisper local o API en la nube (BB8_STT)
├── hablantes.py        registro de voces (Resemblyzer, opcional)
├── agente.py           cliente de POST /agent/ask (con speaker_known)
├── salida.py           Piper local o el altavoz de la cabeza vía bb8-body, pitidos
└── cara.py             girar la cabeza hacia la voz (Parte 5)
wakeword/               entrenar la wake word "oye BB-8" (Colab o PC, no en la Pi)
bb8_mcp/                servidor MCP anterior en Python (solo lanzar_dummy.py --python)
requirements-voz.txt
```

Desde la raíz del repo, con `bb8-body` corriendo (o el dummy) y `agent.api_key` puesta en
la página de configuración:

```bash
claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp   # Claude Code
uv pip install -r partes/4-agente-movimiento/requirements-voz.txt
python -m voz.agente "da una vuelta y dime qué ves"   # sin micrófono
python -m voz.hablantes registrar Erick
python -m voz                                         # el bucle completo
```

**Lista cuando** "Oye Jarvis… ven acá" y se mueve. Para "oye BB-8", entrena el modelo con
[`wakeword/`](wakeword) y copia `oye_bb8.onnx` a `voz/modelos/`.

Guías: [wiki/Parte-4-Agente-controla-el-movimiento.md](../../wiki/Parte-4-Agente-controla-el-movimiento.md) ·
[wiki/Conectar-con-Claude.md](../../wiki/Conectar-con-Claude.md) ·
[wiki/Wake-word-oye-BB-8.md](../../wiki/Wake-word-oye-BB-8.md)
