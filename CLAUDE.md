# CLAUDE.md

BB-8 de 30–35 cm (estilo Sphero) manejado por Claude vía MCP. Dueño: Erick. Todo el
proyecto (código, comentarios, docs, mensajes de commit) está **en español**; sigue así.

## Arquitectura en una línea

Agente (Claude) → servidor MCP `bb8_mcp` (:8765/mcp) → servicio de movimiento
`bb8.motion_api` (127.0.0.1:8770, único dueño del serial) → Arduino (reflejos a 10 ms).
La cabeza es una Pi Zero con `head.server` (:8080). Detalle: `wiki/Arquitectura.md`.

## Dónde está cada cosa

| Carpeta | Paquete / contenido | Parte |
| --- | --- | --- |
| `partes/1-protoboard/arduino/` | `pruebas/p1…p5`, `bb8_firmware/bb8_firmware.ino` | 1 |
| `partes/1-protoboard/wokwi/` | firmware en el simulador Wokwi (`diagram.json`, `wokwi.toml`) | 1 |
| `partes/2-base-melamina/` | solo README (montaje) | 2 |
| `partes/3-base-andando/` | `bb8` (config, protocol, serial_link, motion, motion_api, gamepad), `calibrar` | 3 |
| `partes/4-agente-movimiento/` | `bb8_mcp` (servidor MCP), `voz` (agente de voz) | 4 |
| `partes/5-cabeza/` | `head` (servidor de la Pi Zero, backends real y simulado) | 5 |
| `partes/6-esfera-baterias/` | `energia` (reposo y batería) | 6 |
| `simulador/` | `dummy` (Arduino y cabeza simulados, `probar.py`), `lanzar_dummy.py` | — |
| `sistema/` | systemd, `instalar_pi.sh`, `instalar_zero.sh`, `bb8.env.ejemplo` | — |
| `wiki/` | documentación (fuente de verdad del protocolo: `wiki/Protocolo.md`) | — |
| `docs/` | documento técnico original y artefactos HTML (históricos) | — |

Los paquetes viven en carpetas distintas pero se importan entre sí; `pyproject.toml`
los mapea. **Cuando agregues un paquete nuevo, agrégalo a `packages` y
`package-dir` en `pyproject.toml`** o no se podrá importar.

## Comandos

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt && pip install -e .
python simulador/lanzar_dummy.py --camara sintetica --sin-sonido   # dummy + servicio + MCP
python -m dummy.probar                                             # prueba de punta a punta (17 checks)
python -m compileall -q partes simulador                           # chequeo rápido de sintaxis
```

No hay suite de pytest: **`python -m dummy.probar` contra el dummy es la prueba**. El CI
(`.github/workflows/ci.yml`) la corre en cada push y PR, y compila el firmware y las
pruebas p1…p5 para el Arduino Uno. Córrela después de tocar `bb8`, `bb8_mcp`, `head` o `dummy` (skill
`probar-dummy`). El firmware no se puede probar en hardware aquí (sí en Wokwi a mano); si lo cambias, cambia también
`dummy/arduino_sim.py` para que el simulador lo imite.

## Reglas que no se rompen

- **El LLM nunca maneja motores directo.** Toda orden pasa por el servicio de movimiento, que limita (máx. 2 m, 360°, 5 s, PWM 55 %) y arbitra con el mando. Los límites viven en `bb8/config.py` y en el firmware, no en prompts.
- **El mando Bluetooth gana** sobre el LLM si se tocó en el último segundo.
- **Los reflejos del Arduino no esperan a la Pi:** inclinación > 35°, obstáculo < 25 cm y watchdog de 500 ms frenan solos.
- **Voces desconocidas no mueven al robot**: el bloqueo de `move`/`turn`/`look_at` está en `voz/agente.py`, en código.
- **Un cambio de protocolo toca cuatro sitios a la vez:** `wiki/Protocolo.md`, `bb8/protocol.py`, `dummy/arduino_sim.py` y el firmware (o `head/server.py` + `head/backends_sim.py` para la cabeza). Skill `cambiar-protocolo`.
- Un solo dueño del serial: `bb8.motion_api`. `calibrar` abre el puerto directo solo con el servicio detenido.

## Dependencias fijadas a propósito

- `mcp>=1.10,<2`: FastMCP vive en mcp 1.x (en 2.x se renombró).
- `opencv-python<5`: 5.x quitó los detectores Haar que usa `/cara`.
- La Pi Zero solo instala `partes/5-cabeza/requirements-zero.txt`.

## Hardware (para entender el código)

Arduino Uno/Nano, un L298N (ENA D5, IN1 D7, IN2 D8, ENB D6, IN3 D11, IN4 D12),
encoders en D2/D4 y D3/D10, servo MG996R en D9, MPU6050 + VL53L0X por I2C (A4/A5),
INT del MPU en A2, corte de la Pi en A3, divisor de batería en A0. Motores JGB37-520B
12 V 319 RPM. Nada se ha probado aún en hardware real: todo se validó contra el dummy.

## MCP del robot

`.mcp.json` registra el servidor `bb8` en `http://127.0.0.1:8765/mcp` y
`.claude/settings.json` lo habilita. Solo responde si el dummy está corriendo (o un
túnel a la Pi). Con él pruebas un cambio como lo usaría Claude: `take_photo`, `move`,
`get_pose`… Skill `manejar-bb8`. Para el robot real, agrega aparte
`claude mcp add -s local --transport http bb8-pi http://<ip-de-la-pi>:8765/mcp`.

## Skills del proyecto

En `.claude/skills/`: `probar-dummy`, `cambiar-protocolo`, `agregar-herramienta-mcp`,
`firmware-arduino`, `manejar-bb8`.

## Documentación

Si cambias cómo se corre algo, actualiza la página de la wiki de esa parte
(`wiki/Parte-N-*.md`) y el README de su carpeta. Los enlaces de la wiki a código usan
URLs absolutas `https://github.com/erickdsama/bb8/blob/main/...` para que funcionen
también en la Wiki de GitHub.
