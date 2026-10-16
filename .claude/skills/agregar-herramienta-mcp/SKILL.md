---
name: agregar-herramienta-mcp
description: Add or change a robot tool (what the model can do with BB-8) in the Go tool registry, end to end — MCP and the OpenRouter agent see it, plus dummy, test and wiki. Also for connector tools (weather, calendar, Nest…).
---

# Add a tool

One registry (`go/internal/tools`) feeds both the MCP server (`go/internal/mcpserver`) and the agent (`go/internal/agent`). A tool is a `tools.Tool{Name, Description, Schema, Moves, Handler}`.

| Kind | Where |
| --- | --- |
| Robot (body or head) | `go/internal/body/robot_tools.go` (`RobotTools`) |
| Settings | `go/internal/body/settings.go` (`SettingTools`) |
| Connector (external service) | `go/internal/connectors/<id>/` implementing `connectors.Connector` (`ID`, `Fields`, `Tools`, `Status`), registered in `go/cmd/bb8-body/main.go` (`connectors.NewManager`) |

## Steps

1. **Decide who does the work.**
   - Anything that moves the body or head goes through `motion.Service`, which arbitrates with the gamepad and clamps.
   - Camera, eye, speech and ToF go through `body.Head` over the secure link.
   - Never open the serial port from a tool.
2. **Backend.**
   - A new head request needs a type in `go/internal/head/head.go` (`handle`), both backends (`real.go`, `sim.go`), and the client in `go/internal/body/head.go`. Skill `cambiar-protocolo`.
3. **The tool.**
   - Short English name like the others (`move`, `look_at`). Arguments in English with units (`meters`, `degrees`, `wait_s`).
   - The `Description` is what the model reads: give the range, units, sign (+ right) and every `result` value.
   - Build the schema with `tools.Object`, `tools.Num`, `tools.Str` and `tools.Enum`; decode with `tools.Bind`.
   - Return `{"result": "done"|"error"|…, "reason": …}` maps for outcomes the model should react to. Return a Go `error` only when the call itself is wrong; it becomes an MCP `isError`.
   - Validate ranges in the service too, not only in the schema.
4. **Voice safety.** If it moves the body or head, set `Moves: true`. `Registry.Call` then refuses it for unknown voices.
5. **Instructions.** If BB-8 should behave differently, adjust `persona.Instructions()` in `go/internal/persona/persona.go`.
6. **Settings.** If it needs a setting, add a `config.Field`. Use `Agent: true` only if BB-8 may change it by voice; never for safety limits.
7. **Test.**
   - Go unit test against a fake (see `connectors/weather/weather_test.go`).
   - Add the tool to the expected list in `simulador/dummy/probar.py`, plus a `check` that calls it.
   - Run skill `probar-dummy` on a fresh dummy.
8. **Docs.** Tool tables in `wiki/Protocolo.md` § 2.1 and `wiki/Arquitectura.md`, and the list in `wiki/Parte-4-Agente-controla-el-movimiento.md`.
