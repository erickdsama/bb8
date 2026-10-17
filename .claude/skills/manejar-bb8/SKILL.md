---
name: manejar-bb8
description: Use the bb8 MCP tools (move, turn, look_at, take_photo, get_pose, get_map…) to drive the robot, in the dummy or the real one, e.g. to try a change the way the model would see it.
---

# Drive BB-8 over MCP

The `bb8` server is in `.mcp.json` (`http://127.0.0.1:8765/mcp`). It only answers if something runs behind it.

- **Dummy:** start `python simulador/lanzar_dummy.py --camara sintetica --sin-sonido` in the background (skill `probar-dummy`, steps 1–2). It builds and starts `bb8-body` and `bb8-head --sim`.
  - If the `bb8` tools do not show up, the server was not up when the session started: start the dummy and reconnect with `/mcp`.
  - If port 8765 is taken, use `BB8_MCP_PORT=8775` and the local entry `bb8-pc`.
- **Real robot:** only if the user asks.
  - `bb8-body` listens on `0.0.0.0:8765` and the LAN needs the token: `claude mcp add -s local --transport http bb8-pi http://bb8.local:8765/mcp --header "Authorization: Bearer <token>"`.
  - The token is in `/etc/bb8/token` on the Pi.

## How to drive

Follow the same rules the server gives BB-8:

- `take_photo` before moving into a new place; never more than 1 m without looking again.
- Wait for each `move`/`turn` result before the next. Do not chain more than three without reporting.
- `blocked` means something is closer than 25 cm: turn, do not insist straight.
- `manual_override` means the human has the gamepad: stop.
- `get_pose` with high tilt or battery < 10.5 V: stop and say so.
- `look_at` turns only the head (±90°); `turn` rotates the body (+ right).
- `set_setting` only changes voice/eye/home settings; motion limits are in the config page.

On the real robot every move pushes a 2 kg object around a house. Confirm with the user before the first movement order of the session.

## In the dummy

- **Worlds:**
  - default: a 4 × 3 m room with a brown chair ~1.6 m ahead and a green box;
  - `--mundo casa`: an 8 × 6 m house with a person.
- **Real pose:** `curl http://127.0.0.1:8080/sim/mundo`. Erick can watch live at <http://127.0.0.1:8080/sim>; config page at <http://127.0.0.1:8765/>.
- **Tilting or pushing:** see the `/sim/*` routes in `simulador/dummy/visor.py`.
