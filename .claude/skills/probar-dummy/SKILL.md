---
name: probar-dummy
description: Levanta el robot dummy (Arduino y cabeza simulados + servicio de movimiento + MCP) y corre la prueba de punta a punta. Úsalo después de cambiar bb8, bb8_mcp, head, dummy o voz, o cuando haya que verificar que el proyecto sigue funcionando.
---

# Probar con el dummy

La prueba del proyecto es `dummy.probar`: llama por MCP a las ocho herramientas como
lo haría Claude y comprueba frenos, mando y errores. No hay pytest; el CI
(`.github/workflows/ci.yml`) corre esta misma prueba en cada push y PR.

## 1. Entorno (una vez)

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt && pip install -e .
```

Comprueba `pip show mcp`: debe ser 1.x. Con mcp 2.x falla el import de FastMCP.

## 2. Arrancar el dummy en segundo plano

Antes, que los puertos 5555, 8080, 8770 y 8765 estén libres (un dummy anterior los
ocupa):

```bash
python simulador/lanzar_dummy.py --camara sintetica --sin-sonido > /tmp/bb8-dummy.log 2>&1 &
```

Espera a que el log diga `✅ BB-8 dummy listo` (no basta "Servicio de movimiento
listo": el MCP arranca después). Usa `--camara sintetica` siempre en la prueba: la
comprobación de la silla depende de la habitación simulada.

## 3. Correr la prueba

```bash
python -m dummy.probar
```

Debe terminar en `Todo bien` con 17 líneas ✅. Guarda `foto_dummy.jpg` en la carpeta
actual (está en `.gitignore`; bórrala si cae en otro sitio).

Si una falla, el log del dummy (`/tmp/bb8-dummy.log`) tiene los tres procesos
mezclados con su prefijo (`dummy.arduino`, `bb8.api`, `bb8.mcp`).

## 4. Apagar

Mata el proceso de `lanzar_dummy.py` por su PID (Ctrl+C si está en primer plano);
cierra los otros tres. Ojo: `pkill -f lanzar_dummy` también mata a la shell cuyo
comando contiene ese texto; busca el PID con `pgrep -f "simulador/lanzar_dummy.py"`
en un comando aparte.

## Si agregaste algo

Si el cambio agrega una herramienta, una ruta o un reflejo, agrega su comprobación a
`simulador/dummy/probar.py` con `check(nombre, ok, detalle)`, en el mismo estilo.
