# Conectar con Claude

`bb8-body` ([`go/cmd/bb8-body`](https://github.com/erickdsama/bb8/tree/main/go/cmd/bb8-body))
expone las herramientas del robot por MCP en `http://<host>:8765/mcp`. Son las mismas
que usa el agente propio de BB-8 (OpenRouter): un solo registro de herramientas. Funciona
igual en el [simulador](Simulador.md) que en el robot.

Desde la misma máquina (127.0.0.1) no pide nada. Desde la red de casa pide el token de
`/etc/bb8/token` en la cabecera `Authorization: Bearer …`.

## Claude Code

Con el dummy corriendo (`python simulador/lanzar_dummy.py --camara sintetica`):

```bash
claude mcp add --transport http bb8 http://127.0.0.1:8765/mcp
```

Luego, en Claude Code: *"BB-8, mira al frente y acércate a lo que veas"*.

Si abres Claude Code dentro del repo no hace falta ese paso: `.mcp.json` ya registra
`bb8` en esa dirección y `.claude/settings.json` lo habilita. El repo también trae
`CLAUDE.md` (contexto del proyecto) y skills en `.claude/skills/` para probar con el
dummy, cambiar el protocolo, agregar herramientas, trabajar en el firmware y manejar al
robot.

Con el robot real, `bb8-body` ya escucha en `0.0.0.0:8765`. Desde el portátil:

```bash
claude mcp add -s local --transport http bb8-pi http://bb8.local:8765/mcp \
  --header "Authorization: Bearer $(ssh bb8.local sudo cat /etc/bb8/token)"
```

## Claude Desktop

`bb8-body` habla MCP por HTTP, no por stdio. Claude Desktop lo usa con el puente
`mcp-remote` (necesita Node.js). Con el dummy corriendo, en *Settings → Developer →
Edit Config*:

```json
{
  "mcpServers": {
    "bb8": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "http://127.0.0.1:8765/mcp"]
    }
  }
}
```

Para el robot real, cambia la URL a `http://bb8.local:8765/mcp` y agrega
`"--header", "Authorization: Bearer <token>"` a `args`. Reinicia Claude Desktop y las
herramientas de `bb8` aparecen en el menú.

## Agente propio (OpenRouter)

BB-8 también tiene su propio agente dentro de `bb8-body`: usa cualquier modelo de
[OpenRouter](https://openrouter.ai) con herramientas (por defecto
`anthropic/claude-sonnet-5.5`). Se configura en la página de configuración
(`http://bb8.local:8765/`, o <http://127.0.0.1:8765/> en el dummy):

1. `agent.api_key`: tu clave de OpenRouter (se guarda en `secrets.json`, 0600, y la página solo muestra "set").
2. `agent.model`: el modelo; `agent.vision` apagado si el modelo no ve imágenes.
3. Conectores: `connectors.weather.enabled` + `home.city` para el clima;
   `connectors.calendar.*` para Google Calendar. Para conectar Google, abre la página
   por un túnel (`ssh -L 8765:localhost:8765 bb8.local`) y entra a
   <http://localhost:8765/api/connectors/calendar/auth>.

La página tiene un chat para probarlo. El agente de voz de la
[Parte 4](Parte-4-Agente-controla-el-movimiento.md) le manda lo que oye a
`POST /agent/ask`. Sin micrófono, contra el dummy:

```bash
python -m voz.agente "da una vuelta y dime qué ves"
```

## Personalidad

Las instrucciones le dicen al modelo quién es:
- un droide de 30 cm que habla poco, en español, con frases de menos de 15 palabras;
- pita entre frases y reacciona con `express`;
- mira antes de avanzar (nunca más de 1 m sin volver a tomar foto).

Son las mismas para Claude por MCP y para el agente propio. Están en
`persona.Instructions()`
([`go/internal/persona/persona.go`](https://github.com/erickdsama/bb8/blob/main/go/internal/persona/persona.go)).
