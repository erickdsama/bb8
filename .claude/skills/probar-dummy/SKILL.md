---
name: probar-dummy
description: Start the dummy robot (simulated Arduino and head + bb8-body + bb8-head --sim, both Go) and run the end-to-end test. Use it after changing go/, the dummy, voz or energia, or whenever you need to check the project still works.
---

# Test with the dummy

The project's end-to-end test is `dummy.probar`. It calls the tools over MCP the way a model would, and checks:
- the brakes, the gamepad and the errors;
- the floor plan and hearing;
- two-way config sync (agent → web SSE → head ack; web PATCH with a stale rev → 409).

It has **31 checks**. The Go unit tests (`make test`) cover the rest. CI runs both on every push and PR.

## 1. Environment (once)

```bash
uv venv && uv pip install -r requirements.txt && uv pip install -e .
```

You also need Go 1.25+ in PATH or `~/.local/go`. `lanzar_dummy.py` builds `bin/` itself.

## 2. Start the dummy in the background

Ports 5555, 8080, 8770, 8772 and 8765 must be free (a previous dummy holds them).

```bash
python simulador/lanzar_dummy.py --camara sintetica --sin-sonido > /tmp/bb8-dummy.log 2>&1 &
```

- Wait for the log to say `✅ BB-8 dummy listo`. That line appears once the head has connected to the body over the link.
- Always use `--camara sintetica` for the test: the chair check depends on the simulated room.
- If 8765 is taken, prefix both commands with `BB8_MCP_PORT=8775`.

## 3. Run the test

```bash
PYTHONPATH=simulador python -m dummy.probar
```

- It must end in `Todo bien` with 31 ✅ lines.
- **It needs a fresh dummy:** the move checks assume the start pose. Restart the dummy before running it again.
- It saves `foto_dummy.jpg` in the current folder (it is in `.gitignore`; delete it if it lands elsewhere).
- `make probar` does steps 2–4 in one go.
- If a check fails, the dummy log has every process mixed: `dummy.*` lines are Python, `level=… msg=…` lines are Go.

## 4. Stop

Kill the `lanzar_dummy.py` process by its PID (Ctrl+C in the foreground); it closes the others.

Careful: `pkill -f lanzar_dummy` also kills the shell whose command contains that text. Find the PID in a separate command, or kill by port: `ss -ltnp 'sport = :8770'`.

## If you added something

If the change adds a tool, a route or a reflex, add its check to `simulador/dummy/probar.py` with `check(name, ok, detail)`, in the same style.
