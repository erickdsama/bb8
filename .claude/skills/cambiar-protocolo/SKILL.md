---
name: cambiar-protocolo
description: Change or add a Pi↔Arduino serial command, a status-line field, a head-link message (body↔head over mutual TLS), or a bb8-body HTTP route (local API, config API). Use it whenever a change crosses from one board or process to another.
---

# Change the protocol

`wiki/Protocolo.md` is the source of truth. A half-done protocol change leaves the dummy saying one thing and the Arduino another, while the test keeps passing. So **every** side changes in the same commit.

## Serial Pi ↔ Arduino

| Place | What to touch |
| --- | --- |
| `wiki/Protocolo.md` § 1 | Command table, `?` status line, errors, reflexes; bump the version and date |
| `go/internal/protocol/protocol.go` | Command builders, `ParseReply`, `ParseStatus` (+ `protocol_test.go`) |
| `go/internal/motion/motion.go` | Who sends it and what to do with the reply |
| `simulador/dummy/arduino_sim.py` | The same command with the same behavior and errors (it uses the Python `bb8/protocol.py`; keep that in sync too) |
| `partes/1-protoboard/arduino/bb8_firmware/bb8_firmware.ino` | Command parser and reply (skill `firmware-arduino`) |

Protocol rules that stay:

- One line ending in `\n` and exactly one reply: `OK …` or `ERR <reason>`.
- 115200 baud. Only `M` feeds the 500 ms watchdog.
- Reflexes (tilt, obstacle, watchdog, critical battery) are decided on the Arduino, not the Pi.
- A new `ERR` reason needs its mapping to a `result` in `motion.go` (`blocked`, `error`…) and a row in the error table.

## Head link (mutual TLS, :8772)

| Place | What to touch |
| --- | --- |
| `wiki/Protocolo.md` § 3 | Message table: type, direction, body, reply |
| `go/internal/head/head.go` | `handle` (requests) or `pump` / `onEvent` (events) |
| `go/internal/head/real.go`, `sim.go` | Both backends (`Backend` interface) |
| `go/internal/body/head.go` | Client method or `onEvent` case |
| `go/internal/link` | Only if framing or TLS change (+ `link_test.go`) |

## bb8-body HTTP

| API | Places |
| --- | --- |
| Local API 127.0.0.1:8770 | `wiki/Protocolo.md` § 2.2, `go/internal/body/api.go`; clients `voz/*.py`, `energia/__main__.py` |
| Config / agent :8765 | § 2.3, `go/internal/config/web.go` (pages in `go/internal/webui`), `go/internal/agent/agent.go` (`Routes`) |
| Power manager :8771 | `energia/__main__.py` and `wiki/Parte-6-Esfera-y-baterias.md` |

If the model uses a new route, continue with skill `agregar-herramienta-mcp`.

## Verify

1. `make test`, and `python -m compileall -q partes simulador`.
2. Add a check to `simulador/dummy/probar.py` that exercises the change through MCP or HTTP.
3. Run skill `probar-dummy` on a fresh dummy.
4. If you touched the firmware, compile it (skill `firmware-arduino`) even if you cannot upload it.
