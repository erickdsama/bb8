"""Prueba de punta a punta contra el dummy, como lo haría Claude: por MCP.

    python simulador/lanzar_dummy.py --camara sintetica   # en otra terminal
    python -m dummy.probar
"""
import asyncio
import json
import sys
import urllib.request

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

URL = "http://127.0.0.1:8765/mcp"
fallos = 0


def check(nombre: str, ok: bool, detalle) -> None:
    global fallos
    fallos += not ok
    print(f"{'✅' if ok else '❌'} {nombre}: {detalle}")


def mundo() -> dict:
    return json.load(urllib.request.urlopen("http://127.0.0.1:8080/sim/mundo"))


def post(url: str, body: dict) -> dict:
    req = urllib.request.Request(url, json.dumps(body).encode(), {"Content-Type": "application/json"})
    return json.load(urllib.request.urlopen(req))


async def main() -> None:
    async with streamablehttp_client(URL) as (r, w, _), ClientSession(r, w) as s:
        await s.initialize()

        async def call(tool, **args):
            res = await s.call_tool(tool, args)
            if res.isError:
                return {"error": res.content[0].text}
            if res.structuredContent:
                return res.structuredContent.get("result", res.structuredContent)
            if res.content and res.content[0].type == "text":
                return json.loads(res.content[0].text)
            return res.content

        tools = [t.name for t in (await s.list_tools()).tools]
        check("herramientas", set(tools) >= {"move", "turn", "stop", "look_at", "take_photo",
                                             "set_eye_color", "say", "express", "get_pose"}, tools)

        foto = await call("take_photo")
        img = next((c for c in foto if c.type == "image"), None) if isinstance(foto, list) else None
        check("take_photo", img is not None and img.mimeType == "image/jpeg", img.mimeType if img else foto)
        if img:
            import base64
            open("foto_dummy.jpg", "wb").write(base64.b64decode(img.data))
            print("   foto guardada en foto_dummy.jpg")

        w0 = mundo()
        res = await call("move", metros=0.5)
        w1 = mundo()
        real = w1["x_m"] - w0["x_m"]
        check("move 0.5 m", res["resultado"] == "done" and abs(real - 0.5) < 0.05,
              f"{res} · en el mundo avanzó {real:.3f} m")

        res = await call("turn", grados=90)
        w2 = mundo()
        check("turn +90 (derecha)", res["resultado"] == "done" and abs(w2["rumbo_deg"] - w1["rumbo_deg"] - 90) < 5,
              f"{res} · rumbo real {w2['rumbo_deg']}")
        res = await call("turn", grados=-90)
        check("turn -90", res["resultado"] == "done", res)

        # La silla está a ~1.0 m al frente: debe frenar sola a ~25 cm.
        res = await call("move", metros=2.0)
        w3 = mundo()
        check("move contra la silla → blocked", res["resultado"] == "blocked" and not w3["chocando"],
              f"{res} · x={w3['x_m']}")
        res = await call("move", metros=0.3)
        check("insistir hacia la silla → blocked sin moverse", res["resultado"] == "blocked", res)
        res = await call("move", metros=-0.4)
        check("retroceder sí se permite", res["resultado"] == "done", res)

        res = await call("look_at", grados=60)
        check("look_at 60", res["resultado"] == "done" and abs(res["grados"] - 60) <= 1, res)
        await call("look_at", grados=0)

        res = await call("set_eye_color", color="verde", patron="respirar")
        check("set_eye_color", res.get("ok") is True, res)
        res = await call("set_eye_color", color="fucsia")
        check("color inválido da error claro", res.get("ok") is False, res)
        res = await call("say", texto="Hola Erick, ya llegué")
        check("say texto", res.get("ok") is True, res)
        res = await call("say", sonido="feliz")
        check("say sonido", res.get("ok") is True, res)
        res = await call("express", emocion="curioso")
        check("express curioso", res.get("ok") is True and res.get("ojo") == "cian respirar", res)
        res = await call("express", emocion="enojadísimo")
        check("emoción inválida da error", "error" in res, res)

        pose = await call("get_pose")
        check("get_pose", "bateria_v" in pose and pose["control"] == "idle", pose)

        # El mando manda: una orden manual bloquea al LLM durante 1 s.
        post("http://127.0.0.1:8770/manual", {"izq": 0, "der": 0})
        res = await call("move", metros=0.2)
        check("mando activo → manual_override", res["resultado"] == "manual_override", res)
        await asyncio.sleep(1.1)

        # stop cancela un movimiento en curso
        task = asyncio.create_task(call("turn", grados=360))
        await asyncio.sleep(0.6)
        await call("stop")
        res = await task
        check("stop cancela", res["resultado"] == "cancelled", res)

        # Inclinación > 35°: el Arduino frena solo
        post("http://127.0.0.1:8080/sim/empujar", {"grados": 40, "segundos": 1.5})
        await asyncio.sleep(0.2)
        res = await call("move", metros=0.3)
        check("inclinado → error tilt", res.get("motivo") == "tilt", res)
        await asyncio.sleep(1.5)

    print(f"\n{'Todo bien' if not fallos else f'{fallos} fallo(s)'}")
    sys.exit(1 if fallos else 0)


if __name__ == "__main__":
    asyncio.run(main())
