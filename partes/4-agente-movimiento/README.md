# Parte 4 · Agente controla el movimiento

El servidor MCP que le da a Claude las herramientas del robot y el agente de voz.

```
bb8_mcp/                servidor MCP bb8-motion (FastMCP, :8765/mcp)
voz/                    wake word → VAD → Whisper → Claude + MCP → Piper
├── escucha.py          openWakeWord + Silero VAD
├── stt.py              faster-whisper local o API en la nube (BB8_STT)
├── hablantes.py        registro de voces (Resemblyzer, opcional)
├── agente.py           Claude con las herramientas MCP
├── salida.py           Piper local o en la cabeza, pitidos
└── cara.py             girar hacia quien habla (Parte 5)
requirements-voz.txt
```

Desde la raíz del repo, con la Parte 3 corriendo:

```bash
python -m bb8_mcp                                     # luego: claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp
pip install -r partes/4-agente-movimiento/requirements-voz.txt
export ANTHROPIC_API_KEY=sk-ant-...
python -m voz.agente "da una vuelta y dime qué ves"   # sin micrófono
python -m voz.hablantes registrar Erick
python -m voz                                         # el bucle completo
```

**Lista cuando** "Oye Jarvis… ven acá" y se mueve.

Guías: [wiki/Parte-4-Agente-controla-el-movimiento.md](../../wiki/Parte-4-Agente-controla-el-movimiento.md) ·
[wiki/Conectar-con-Claude.md](../../wiki/Conectar-con-Claude.md)
