"""Prueba de punta a punta contra el dummy, como lo haría Claude: por MCP.

    python simulador/lanzar_dummy.py --camara sintetica   # en otra terminal
    python -m dummy.probar
Si el 8765 está ocupado, arranca ambos con BB8_MCP_PORT=<otro puerto>.
"""
import asyncio
import json
import os
import sys
import urllib.error
import urllib.request

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

URL = f"http://127.0.0.1:{os.environ.get('BB8_MCP_PORT', '8765')}/mcp"
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
            if any(c.type == "image" for c in res.content):
                return res.content
            if res.structuredContent:
                return res.structuredContent
            if res.content and res.content[0].type == "text":
                return json.loads(res.content[0].text)
            return res.content

        tools = [t.name for t in (await s.list_tools()).tools]
        check("herramientas", set(tools) >= {"move", "turn", "stop", "look_at", "take_photo",
                                             "set_eye_color", "say", "express", "get_pose", "listen",
                                             "get_map", "set_position", "update_map"}, tools)

        foto = await call("take_photo")
        img = next((c for c in foto if c.type == "image"), None) if isinstance(foto, list) else None
        check("take_photo", img is not None and img.mimeType == "image/jpeg", img.mimeType if img else foto)
        if img:
            import base64
            open("foto_dummy.jpg", "wb").write(base64.b64decode(img.data))
            print("   foto guardada en foto_dummy.jpg")

        w0 = mundo()
        res = await call("move", meters=0.5)
        w1 = mundo()
        real = w1["x_m"] - w0["x_m"]
        check("move 0.5 m", res["result"] == "done" and abs(real - 0.5) < 0.05,
              f"{res} · en el mundo avanzó {real:.3f} m")

        res = await call("turn", degrees=90)
        w2 = mundo()
        check("turn +90 (derecha)", res["result"] == "done" and abs(w2["rumbo_deg"] - w1["rumbo_deg"] - 90) < 5,
              f"{res} · rumbo real {w2['rumbo_deg']}")
        res = await call("turn", degrees=-90)
        check("turn -90", res["result"] == "done", res)

        # La silla está a ~1.0 m al frente: debe frenar sola a ~25 cm.
        res = await call("move", meters=2.0)
        w3 = mundo()
        check("move contra la silla → blocked", res["result"] == "blocked" and not w3["chocando"],
              f"{res} · x={w3['x_m']}")
        res = await call("move", meters=0.3)
        check("insistir hacia la silla → blocked sin moverse", res["result"] == "blocked", res)
        res = await call("move", meters=-0.4)
        check("retroceder sí se permite", res["result"] == "done", res)

        res = await call("look_at", degrees=60)
        check("look_at 60", res["result"] == "done" and abs(res["degrees"] - 60) <= 1, res)
        await call("look_at", degrees=0)

        res = await call("set_eye_color", color="green", pattern="breathe")
        check("set_eye_color", res.get("result") == "done", res)
        res = await call("set_eye_color", color="fuchsia")
        check("color inválido da error claro", res.get("result") == "error", res)
        res = await call("say", text="Hola Erick, ya llegué")
        check("say texto", res.get("result") == "done", res)
        res = await call("say", sound="happy")
        check("say sonido", res.get("result") == "done", res)
        res = await call("express", emotion="curious")
        check("express curioso", res.get("result") == "done" and res.get("eye") == "cyan breathe", res)
        res = await call("express", emotion="furious")
        check("emoción inválida da error", "error" in res, res)

        pose = await call("get_pose")
        check("get_pose", "battery_v" in pose and pose["control"] == "idle" and pose["head"]["connected"], pose)

        # Oído: alguien habla a 1.2 m, 40° a la derecha del frente. El ReSpeaker 2-Mic da
        # el ángulo y su gemelo de atrás (140°); listen con espera_s lo recibe al momento.
        import math
        w = mundo()
        ang = math.radians(w["theta_deg"] - 40)
        voz = {"texto": "BB-8, ven", "x": w["x_m"] + 1.2 * math.cos(ang), "y": w["y_m"] + 1.2 * math.sin(ang)}
        task = asyncio.create_task(call("listen", wait_s=3))
        await asyncio.sleep(0.3)
        post("http://127.0.0.1:8080/sim/hablar", voz)
        res = await task
        ok = (res.get("heard") and res.get("text") == "BB-8, ven" and abs(res["angle_deg"] - 40) < 15
              and abs(res["candidates_deg"][1] - 140) < 15)
        check("listen: voz a 40° derecha", ok, res)
        await call("turn", degrees=40)
        res = await call("listen")
        check("listen tras girar 40°: candidato al frente", abs(res["candidates_deg"][0]) < 15
              and abs(res["turned_since_deg"] - 40) < 8, res)

        # Los micrófonos van en la cabeza: con la cabeza a 60°, una voz a 40° a la derecha
        # del cuerpo llega a −20° de la cabeza, y bb8-body la devuelve a 40° del cuerpo.
        await call("look_at", degrees=60)
        w = mundo()
        ang = math.radians(w["theta_deg"] - 40)
        voz = {"texto": "aquí estoy", "x": w["x_m"] + 1.2 * math.cos(ang), "y": w["y_m"] + 1.2 * math.sin(ang)}
        task = asyncio.create_task(call("listen", wait_s=3))
        await asyncio.sleep(0.3)
        post("http://127.0.0.1:8080/sim/hablar", voz)
        res = await task
        await call("look_at", degrees=0)
        check("listen con la cabeza girada 60°", res.get("heard") and abs(res["angle_deg"] + 20) < 15
              and abs(res["head_deg"] - 60) < 3 and abs(res["candidates_deg"][0] - 40) < 15, res)

        # Plano: tras moverse y girar, la odometría llevada al plano coincide con el mundo
        # (los ejes del plano son los del mundo; el rumbo del plano es horario: −theta).
        res = await call("get_map")
        w = mundo()
        r = res.get("robot", {})
        err = math.hypot(r.get("x", 99) - w["x_m"], r.get("y", 99) - w["y_m"])
        err_r = abs((r.get("heading_deg", 999) + w["theta_deg"] + 180) % 360 - 180)
        check("get_map: posición en el plano", err < 0.15 and err_r < 8 and r.get("room") == "habitacion"
              and len(res.get("sketch", [])) > 5, f"{r} · mundo x={w['x_m']} y={w['y_m']} θ={w['theta_deg']}")
        res = await call("set_position", x=2.0, y=1.0, heading_deg=90)
        r = (await call("get_map"))["robot"]
        check("set_position corrige el plano", abs(r["x"] - 2) < 0.02 and abs(r["y"] - 1) < 0.02
              and abs(r["heading_deg"] - 90) < 1, r)
        await call("set_position", x=w["x_m"], y=w["y_m"], heading_deg=-w["theta_deg"])
        res = await call("set_position", x=9.0, y=1.0)
        check("set_position fuera de la casa → error", res.get("result") == "error", res)

        # Claude anota en el plano lo que vio: un mueble nuevo y luego lo quita
        res = await call("update_map", action="add_furniture", name="maceta", x=3.5, y=0.5,
                         width_m=0.3, length_m=0.3, height_m=0.6)
        muebles = [m["name"] for m in (await call("get_map"))["furniture"]]
        check("update_map agrega un mueble", res.get("result") == "done" and "maceta" in muebles, muebles)
        res = await call("update_map", action="remove_furniture", name="maceta")
        muebles = [m["name"] for m in (await call("get_map"))["furniture"]]
        check("update_map quita un mueble", res.get("result") == "done" and "maceta" not in muebles, muebles)

        # Configuración en los dos sentidos: lo que cambia el agente llega a la web (SSE) y
        # a la cabeza (config_ack); lo que cambia la web llega al robot. Los límites de
        # seguridad no los puede cambiar el agente.
        web = URL.removesuffix("mcp")
        eventos: list[str] = []

        def escuchar_sse() -> None:
            with urllib.request.urlopen(web + "api/config/events", timeout=10) as r:
                for linea in r:
                    eventos.append(linea.decode())
                    if linea.startswith(b"data:") and b"head.volume" in linea:
                        return

        import threading
        hilo = threading.Thread(target=escuchar_sse, daemon=True)
        hilo.start()
        await asyncio.sleep(0.3)
        res = await call("set_setting", key="head.volume", value=0.35)
        hilo.join(3)
        cfg = json.load(urllib.request.urlopen(web + "api/config"))
        await asyncio.sleep(0.3)
        cabeza = json.load(urllib.request.urlopen("http://127.0.0.1:8770/head"))
        check("set_setting → web (SSE) y cabeza", res.get("result") == "done" and cfg["values"]["head.volume"] == 0.35
              and any('"source":"agent"' in e for e in eventos) and cabeza.get("config_rev") == cfg["rev"],
              f"{res} · rev web {cfg['rev']} · rev cabeza {cabeza.get('config_rev')}")
        res = await call("set_setting", key="motion.pwm_cruise", value=160)
        check("el agente no cambia límites de seguridad", res.get("result") == "error" or "error" in res, res)

        def patch(body: dict) -> tuple[int, dict]:
            req = urllib.request.Request(web + "api/config", json.dumps(body).encode(),
                                         {"Content-Type": "application/json"}, method="PATCH")
            try:
                with urllib.request.urlopen(req) as r:
                    return r.status, json.load(r)
            except urllib.error.HTTPError as e:
                return e.code, json.load(e)

        viejo, _ = patch({"baseRev": cfg["rev"] - 1, "values": {"motion.pwm_cruise": 120}})
        nuevo, cfg2 = patch({"baseRev": cfg["rev"], "values": {"motion.pwm_cruise": 120}})
        check("web: rev viejo → 409, rev actual → aplicado", viejo == 409 and nuevo == 200
              and cfg2["values"]["motion.pwm_cruise"] == 120, f"{viejo} {nuevo}")
        patch({"values": {"motion.pwm_cruise": 140}})

        # El mando manda: una orden manual bloquea al LLM durante 1 s.
        post("http://127.0.0.1:8770/manual", {"left": 0, "right": 0})
        res = await call("move", meters=0.2)
        check("mando activo → manual_override", res["result"] == "manual_override", res)
        await asyncio.sleep(1.1)

        # stop cancela un movimiento en curso
        task = asyncio.create_task(call("turn", degrees=360))
        await asyncio.sleep(0.6)
        await call("stop")
        res = await task
        check("stop cancela", res["result"] == "cancelled", res)

        # Inclinación > 35°: el Arduino frena solo
        post("http://127.0.0.1:8080/sim/empujar", {"grados": 40, "segundos": 1.5})
        await asyncio.sleep(0.2)
        res = await call("move", meters=0.3)
        check("inclinado → error tilt", res.get("reason") == "tilt", res)
        res = (await call("get_map"))["position"]
        check("inclinado → posición incierta en el plano", res["uncertain"] is True, res)
        await asyncio.sleep(1.5)

    print(f"\n{'Todo bien' if not fallos else f'{fallos} fallo(s)'}")
    sys.exit(1 if fallos else 0)


if __name__ == "__main__":
    asyncio.run(main())
