# Conectar con Claude

El servidor MCP `bb8-motion` ([`partes/4-agente-movimiento/bb8_mcp`](https://github.com/erickdsama/bb8/tree/main/partes/4-agente-movimiento/bb8_mcp))
expone las ocho herramientas del robot. Hay tres formas de usarlo; las tres hablan
con el mismo servidor, en el [simulador](Simulador.md) o en el robot.

## Claude Code

Con el dummy corriendo (`python simulador/lanzar_dummy.py`):

```bash
claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp
```

Luego, en Claude Code: *"BB-8, mira al frente y acércate a lo que veas"*.

Si abres Claude Code dentro del repo no hace falta ese paso: `.mcp.json` ya registra
`bb8` en esa dirección y `.claude/settings.json` lo habilita. El repo también trae
`CLAUDE.md` (contexto del proyecto) y skills en `.claude/skills/` para probar con el
dummy, cambiar el protocolo, agregar herramientas MCP, trabajar en el firmware y
manejar al robot.

Con el robot real, arranca el MCP en la Pi con `--host 0.0.0.0` (o en
`sistema/bb8-mcp.service`) y desde el portátil:

```bash
claude mcp add -s local --transport http bb8-pi http://<ip-de-la-pi>:8765/mcp
```

Sin contraseña: úsalo solo en la WiFi de casa.

## Claude Desktop

Claude Desktop lanza el servidor MCP él mismo por stdio, así que deja corriendo solo
el dummy y el servicio de movimiento:

1. Terminal 1: `python -m dummy.run_dummy --camara sintetica`
2. Terminal 2: `BB8_SERIAL=socket://127.0.0.1:5555 BB8_HEAD_URL=http://127.0.0.1:8080 python -m bb8.motion_api`
   (PowerShell: `$env:BB8_SERIAL="socket://127.0.0.1:5555"; $env:BB8_HEAD_URL="http://127.0.0.1:8080"; python -m bb8.motion_api`)
3. En Claude Desktop: *Settings → Developer → Edit Config*, y agrega (cambia la ruta):

```json
{
  "mcpServers": {
    "bb8": {
      "command": "/ruta/a/bb8/.venv/bin/python",
      "args": ["-m", "bb8_mcp", "--stdio"],
      "env": {
        "BB8_HEAD_URL": "http://127.0.0.1:8080"
      }
    }
  }
}
```

En Windows el `command` es `C:\\ruta\\a\\bb8\\.venv\\Scripts\\python.exe`. Hace falta
haber corrido `pip install -e .` en ese venv; si no, agrega
`"PYTHONPATH": "/ruta/a/bb8/partes/3-base-andando:/ruta/a/bb8/partes/4-agente-movimiento"`
(en Windows separa con `;`). Reinicia Claude Desktop y las herramientas de `bb8`
aparecen en el menú.

## Agente de voz

El agente de voz de la [Parte 4](Parte-4-Agente-controla-el-movimiento.md) es otro
cliente MCP, con micrófono y altavoz. Para probarlo sin micrófono, contra el dummy:

```bash
pip install -r partes/4-agente-movimiento/requirements-voz.txt
export ANTHROPIC_API_KEY=sk-ant-...
python -m voz.agente "da una vuelta y dime qué ves"
```

## Personalidad

Las instrucciones del servidor MCP le dicen a Claude quién es: un droide de 30 cm que
habla poco, en español, con frases de menos de 15 palabras, pita entre frases y mira
antes de avanzar (nunca más de 1 m sin volver a tomar foto). Están en
`INSTRUCTIONS` dentro de `bb8_mcp/__main__.py`.
