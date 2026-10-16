# CLAUDE.md

A 30–35 cm BB-8 (Sphero style) driven by an LLM through MCP and its own OpenRouter agent. Owner: Erick.

**Language.**
- New code, identifiers, comments and technical docs (protocol, architecture, skills) are **in English**.
- What BB-8 says out loud stays **in Spanish**, and so do the build guides (`wiki/Parte-N-*.md`, part READMEs).
- Older Python code and its comments are in Spanish. Leave them unless you are rewriting that code.

## Architecture in one line

The model reaches `bb8-body` through MCP (`:8765/mcp`) or the agent (`/agent/ask`). `bb8-body` is a Go binary on the Pi 3 B+ and the only owner of the serial port; it drives the Arduino (10 ms reflexes). `bb8-head` (Go, Pi Zero W) dials the body over mutual TLS (:8772) and opens no ports.

Details: `wiki/Arquitectura.md`. Wire formats: `wiki/Protocolo.md` (the source of truth).

## Where things are

| Folder | Content |
| --- | --- |
| `go/cmd/bb8-body`, `go/cmd/bb8-head` | The two binaries (`bb8-body pair` creates the link certs) |
| `go/internal/` | `protocol`, `serial`, `motion`, `floorplan`, `tools`, `mcpserver`, `body`, `link`, `head`, `config` (+ `web/index.html`), `agent`, `connectors/{weather,calendar}`, `gamepad`, `persona` |
| `partes/1-protoboard/arduino/` | `pruebas/p1…p5`, `bb8_firmware/bb8_firmware.ino` |
| `partes/1-protoboard/wokwi/` | Firmware in the Wokwi simulator |
| `partes/3-base-andando/` | Legacy Python `bb8` (used by `calibrar` and `--python`); `mapas/` (floor plans, English keys) |
| `partes/4-agente-movimiento/` | `voz`: Python voice front end (wake word, STT, speaker id → `POST /agent/ask`); `wakeword/`; legacy `bb8_mcp` |
| `partes/5-cabeza/` | Legacy Python `head`. `head.sounds` and `head.personalidad` are still used by `voz` and the sim |
| `partes/6-esfera-baterias/` | `energia` (Python power manager, uses the local API) |
| `cad/` | Parametric OpenSCAD. `exportar.sh` writes `stl/`, `png/`, `plantillas/` |
| `simulador/` | `dummy` (simulated Arduino + head, `probar.py`), `lanzar_dummy.py` |
| `sistema/` | systemd units (`bb8-body`, `bb8-head`, `bb8-voz`, `bb8-energia`), `instalar_pi.sh`, `instalar_zero.sh` |
| `wiki/` | Documentation |

Python packages are mapped in `pyproject.toml`. **If you add a Python package, add it to `packages` and `package-dir`** or it will not import.

## Commands

```bash
uv venv && uv pip install -r requirements.txt && uv pip install -e .   # Python (always uv)
make test        # go vet + go test (Go in PATH or ~/.local/go)
make             # bin/bb8-body, bin/bb8-head for this PC
make release     # dist/linux-arm64 (Pi 3) and dist/linux-armv6 (Zero W)
python simulador/lanzar_dummy.py --camara sintetica --sin-sonido   # builds Go, dummy + bb8-body + bb8-head --sim
PYTHONPATH=simulador python -m dummy.probar                        # end-to-end test, 30 checks
make probar      # both of the above
```

- Config page on the dummy: http://127.0.0.1:8765/. Loopback needs no token.
- If 8765 is taken, start the dummy and run probar with `BB8_MCP_PORT=8775`.
- **probar needs a fresh dummy.** It assumes the start pose; a second run in a row fails the move checks.
- CI (`.github/workflows/ci.yml`) runs:
  - `make test` and `make release`;
  - probar against the dummy;
  - the firmware and p1…p5 builds.
- After touching `go/`, `dummy` or `voz`, run probar (skill `probar-dummy`).
- The firmware cannot be tested on hardware here. If you change it, also change `dummy/arduino_sim.py`.

## Rules that do not break

- **The LLM never drives motors directly.** Every order goes through `bb8-body`, which clamps it and arbitrates with the gamepad.
  - Limits live in the config (`motion.*`), in hard ceilings in `motion.Params.Clamp`, and in the firmware. Never in prompts.
  - The agent cannot write them: only fields with `Agent: true` are writable from `set_setting`.
- **The gamepad wins** over the LLM if it was touched within `motion.manual_hold_s` (1 s).
- **Arduino reflexes do not wait for the Pi:** tilt > 35°, obstacle < 25 cm and the 500 ms watchdog brake by themselves.
- **Unknown voices do not move the robot.** Tools with `Moves: true` are refused in `tools.Registry.Call`, in code.
- **The head is only reachable through the body**, over mutual TLS. Nothing talks to the Zero over plain HTTP.
- **A protocol change touches every side at once.**
  - Serial: `wiki/Protocolo.md`, `go/internal/protocol`, `dummy/arduino_sim.py` and the firmware.
  - Head link: `go/internal/link` + `go/internal/head` + `go/internal/body/head.go`.
  - Skill `cambiar-protocolo`.
- One serial owner: `bb8-body`. `calibrar` opens the port directly only with `bb8-body` stopped.

## Pinned on purpose

- `github.com/modelcontextprotocol/go-sdk v1.8.0`. Tools use the raw `Server.AddTool` with the registry's JSON schema.
- `mcp>=1.10,<2` (Python): only for `dummy.probar` and the legacy `bb8_mcp`.
- The Go binaries are built with `CGO_ENABLED=0` (static) so they cross-compile to armv6 without a toolchain.

## Hardware (to understand the code)

- **Arduino Uno and L298N:** ENA D5, IN1 D7, IN2 D8, ENB D6, IN3 D11, IN4 D12.
- **Encoders:** D2/D4 and D3/D10.
- **Servo:** MG996R on D9.
- **I2C (A4/A5):** MPU6050 + VL53L0X. MPU INT on A2, Pi power cut on A3, battery divider on A0.
- **Motors:** JGB37-520B, 12 V, 319 RPM, 6 mm D-shaft. 65 mm RC wheels on a 12 mm hex coupling.
- **Main Pi:** Raspberry Pi 3 B+, 1 GB, 64-bit OS, hostname `bb8`.
- **Head:** the original Raspberry Pi **Zero W** (armv6, 512 MB, hostname `bb8-head`), not a Zero 2 W. Piper does not run there; the head speaks with `espeak-ng`.
  - The NeoPixel data line is on SPI0 MOSI (GPIO10).
  - The OV5647 camera (wide angle) and the ReSpeaker 2-Mic HAT are pending.

Shopping list: `wiki/Materiales.md`. Nothing has been tested on real hardware yet; everything was validated against the dummy.

## Robot MCP

- `.mcp.json` registers the server `bb8` at `http://127.0.0.1:8765/mcp`, and `.claude/settings.json` enables it.
- It only answers while the dummy runs (or through a tunnel to the Pi). Skill `manejar-bb8`.
- For the real robot: `claude mcp add -s local --transport http bb8-pi http://bb8.local:8765/mcp --header "Authorization: Bearer <token>"`.

## Project skills

In `.claude/skills/`: `probar-dummy`, `cambiar-protocolo`, `agregar-herramienta-mcp`, `firmware-arduino`, `manejar-bb8`.

## Documentation

- If you change how something runs, update that part's wiki page (`wiki/Parte-N-*.md`) and its folder README.
- Wiki links to code use absolute `https://github.com/erickdsama/bb8/blob/main/...` URLs, so they also work in the GitHub Wiki.
