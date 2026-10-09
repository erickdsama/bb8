---
name: agregar-herramienta-mcp
description: Agregar o cambiar una herramienta del servidor MCP bb8-motion (lo que Claude puede hacer con el robot), de punta a punta hasta el dummy, la prueba y la wiki.
---

# Agregar una herramienta MCP

Las herramientas están en `partes/4-agente-movimiento/bb8_mcp/__main__.py`, con
`@mcp.tool()`. Son delgadas: no tienen lógica de robot, solo llaman por HTTP al
servicio de movimiento (`_motion`) o a la cabeza (`_head`) con `_post`.

## Pasos

1. **Decide a quién llama.** Todo lo que mueve cuerpo o cabeza va al servicio de movimiento (que arbitra con el mando y aplica límites). Cámara, ojo, voz y ToF van a la cabeza. Nunca abras el serial desde el MCP.
2. **Backend.** Agrega la ruta y su lógica donde corresponda (skill `cambiar-protocolo` para la tabla de sitios). Si es de la cabeza, impleméntala en `backends_sim.py` y en `backends_real.py`.
3. **Herramienta.** En `bb8_mcp/__main__.py`:
   - Nombre en inglés corto como las existentes (`move`, `look_at`); parámetros y docstring en español.
   - El docstring es lo que Claude lee: di rango, unidades, signo (+ derecha) y cada `resultado` posible.
   - Devuelve el `dict` del backend tal cual; errores de red ya los convierte `_post`.
   - Si mueve algo, valida el rango también en el servicio, no solo en el MCP.
4. **Seguridad de voz.** Si la herramienta mueve el cuerpo o la cabeza, agrégala a `MOVIMIENTO` en `partes/4-agente-movimiento/voz/agente.py` para que una voz desconocida no pueda usarla.
5. **Instrucciones.** Si cambia cómo debe comportarse BB-8, ajusta `INSTRUCTIONS` en el mismo archivo (frases cortas, en español).
6. **Prueba.** Actualiza la lista esperada de herramientas en `simulador/dummy/probar.py` y agrega un `check` que la llame. Corre la skill `probar-dummy`.
7. **Docs.** Tabla de herramientas en `wiki/Arquitectura.md`, lista en `wiki/Parte-4-Agente-controla-el-movimiento.md` y `partes/4-agente-movimiento/README.md`.
