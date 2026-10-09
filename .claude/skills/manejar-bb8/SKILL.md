---
name: manejar-bb8
description: Usar las herramientas del MCP bb8 (move, turn, look_at, take_photo, get_pose…) para manejar el robot, en el dummy o en el real, por ejemplo para probar un cambio como lo vería Claude.
---

# Manejar a BB-8 por MCP

El servidor `bb8` está en `.mcp.json` (`http://127.0.0.1:8765/mcp`).
Solo responde si algo está corriendo detrás:

- **Dummy:** `python simulador/lanzar_dummy.py --camara sintetica --sin-sonido` en segundo plano (skill `probar-dummy`, pasos 1–2). Si las herramientas `bb8` no aparecen, el servidor no estaba arriba al iniciar la sesión: arranca el dummy y reconecta con `/mcp`.
- **Robot real:** solo si el usuario lo pide. El MCP de la Pi debe escuchar en `0.0.0.0`; regístralo aparte con `claude mcp add -s local --transport http bb8-pi http://<ip-de-la-pi>:8765/mcp`.

## Cómo manejar

Sigue las mismas reglas que el servidor le da a BB-8:

- `take_photo` antes de avanzar en un sitio nuevo; nunca más de 1 m sin volver a mirar.
- Espera el resultado de cada `move`/`turn` antes del siguiente; no encadenes más de tres sin informar.
- `blocked` = algo a menos de 25 cm: gira, no insistas recto.
- `manual_override` = el humano tiene el mando: detente.
- `get_pose` con inclinación alta o batería < 10.5 V: detente y avisa.
- `look_at` gira solo la cabeza (±90°); `turn` gira todo el cuerpo (+ derecha).

En el robot real, cada movimiento mueve un objeto de 2 kg en una casa: confirma con el
usuario antes de la primera orden de movimiento de la sesión.

## En el dummy

- Habitación de 4 × 3 m; silla café a ~1.6 m al frente y una caja verde.
- Posición real: `curl http://127.0.0.1:8080/sim/mundo`.
- Para simular que lo inclinan o lo empujan, mira las rutas `/sim/*` en `simulador/dummy/run_dummy.py`.
