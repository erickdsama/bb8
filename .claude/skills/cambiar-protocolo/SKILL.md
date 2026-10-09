---
name: cambiar-protocolo
description: Cambiar o agregar una orden serial Pi↔Arduino, un campo de la línea de estado, una ruta HTTP del servicio de movimiento o de la cabeza. Úsalo siempre que el cambio cruce de una placa a otra.
---

# Cambiar el protocolo

`wiki/Protocolo.md` es la fuente de verdad. Un cambio de protocolo a medias deja al
dummy diciendo una cosa y al Arduino otra, y la prueba sigue pasando. Por eso se
cambian **todos** los sitios en el mismo commit.

## Serial Pi ↔ Arduino

| Sitio | Qué tocar |
| --- | --- |
| `wiki/Protocolo.md` § 1 | Tabla de órdenes, línea de estado `?`, errores, reflejos; sube la versión y fecha del encabezado |
| `partes/3-base-andando/bb8/protocol.py` | Constantes `CMD_*`, armado de la línea y parseo de la respuesta/estado |
| `simulador/dummy/arduino_sim.py` | La misma orden con el mismo comportamiento y errores |
| `partes/1-protoboard/arduino/bb8_firmware/bb8_firmware.ino` | El parser de órdenes y la respuesta (skill `firmware-arduino`) |
| `partes/3-base-andando/bb8/motion.py` | Quién envía la orden y qué hace con la respuesta |

Reglas del protocolo que se mantienen:

- Una línea terminada en `\n`, exactamente una respuesta: `OK …` o `ERR <motivo>`.
- 115200 baudios. Solo `M` alimenta el watchdog de 500 ms.
- Los reflejos (tilt, obstáculo, watchdog, batería crítica) se deciden en el Arduino, no en la Pi.
- Un motivo de `ERR` nuevo necesita su traducción a `resultado` en `motion.py` (`blocked`, `error`…) y una línea en la tabla de errores.

## HTTP

| Enlace | Sitios |
| --- | --- |
| Servicio de movimiento :8770 | `wiki/Protocolo.md` § 2, `bb8/motion_api.py` (ruta), `bb8/motion.py` (lógica) |
| Cabeza :8080 | `wiki/Protocolo.md` § 3, `head/server.py` (ruta), `head/backends_real.py` y `head/backends_sim.py` (los dos backends) |
| Energía :8771 | `energia/__main__.py` y `wiki/Parte-6-Esfera-y-baterias.md` |

Si una ruta nueva la usa Claude, sigue con la skill `agregar-herramienta-mcp`.

## Verificar

1. `python -m compileall -q partes simulador`
2. Agrega una comprobación en `simulador/dummy/probar.py` que ejercite la orden nueva por MCP o HTTP.
3. Corre la skill `probar-dummy`.
4. Si tocaste el firmware, compílalo (skill `firmware-arduino`) aunque no puedas subirlo.
