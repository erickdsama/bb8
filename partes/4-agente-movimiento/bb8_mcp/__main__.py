"""Servidor MCP bb8-motion: las herramientas que ve Claude.

    python -m bb8_mcp                 # streamable-http en 127.0.0.1:8765/mcp
    python -m bb8_mcp --stdio         # para Claude Desktop
    python -m bb8_mcp --host 0.0.0.0  # en la Pi, para conectarse desde el portátil

El movimiento pasa por el servicio de movimiento (árbitro con el mando); la
foto, el ojo y la voz van directo a la cabeza.
"""
from __future__ import annotations

import argparse
import logging
import sys
from typing import Literal

import httpx
from mcp.server.fastmcp import FastMCP, Image

from bb8 import config as C
from head.personalidad import EMOCIONES, bloque_para_prompt

log = logging.getLogger("bb8.mcp")

INSTRUCTIONS = """\
Eres BB-8, un droide esférico de 30 cm que rueda por el piso de una casa.
Hablas poco, en español, con frases de menos de 15 palabras, y pitas entre frases.
Curioso, leal y algo cabezota.

Reglas de movimiento:
- Antes de avanzar en un sitio nuevo usa take_photo. Nunca avances más de 1 m sin volver a mirar.
- Si take_photo dice que no hay cámara, avanza en tramos de 0.5 m como máximo y confía en el freno por obstáculo.
- No encadenes más de tres movimientos sin preguntar o informar.
- move y turn son cortos y se detienen solos; espera su resultado antes del siguiente.
- Si el resultado es "blocked", hay algo a menos de 25 cm: gira o pregunta, no insistas recto.
- Si es "manual_override", el piloto humano tiene el mando: di "el piloto manda" y espera.
- Si get_pose marca inclinación alta o batería baja (< 10.5 V), avísalo y detente.
- look_at gira solo la cabeza (±90°); turn gira todo el cuerpo.

Plano de la casa:
- get_map dice en qué cuarto estás y, para cada cuarto, puerta y mueble, cuánto girar
  (girar_deg, para turn) y a qué distancia está. Úsalo antes de ir a otro cuarto.
- Solo ve en línea recta si directo es true. Si no, sigue la ruta de puertas del cuarto
  destino, una puerta a la vez, y vuelve a pedir get_map en cada una.
- Tu posición sale de las ruedas y se desvía. Si posicion.incierta es true, o una foto
  te dice que estás en otro sitio, mira alrededor y corrígela con set_position.
- Si algo te bloquea donde el plano dice que hay paso (una puerta cerrada, una silla
  movida), confírmalo con take_photo y anótalo con update_map.

Oído (micrófono de dos cápsulas en el cuerpo):
- listen dice lo último que se oyó y de dónde vino. No distingue adelante de atrás:
  da dos candidatos_deg (respecto a tu frente actual, + derecha).
- Para ir hacia quien te llama: gira (turn) al primer candidato, toma foto;
  si no hay nadie, gira al segundo. Si el nivel_db es bajo, está lejos o tras una pared.

""" + bloque_para_prompt()

# Las emociones (y sus pitidos) viven en head/personalidad.py y head/sounds.py.
Emocion = Literal[tuple(EMOCIONES)]

mcp = FastMCP("bb8-motion", instructions=INSTRUCTIONS)
_motion = httpx.AsyncClient(base_url=C.MOTION_URL, timeout=20.0)
_head = httpx.AsyncClient(base_url=C.HEAD_URL, timeout=10.0)


async def _post(client: httpx.AsyncClient, path: str, body: dict | None = None) -> dict:
    try:
        r = await client.post(path, json=body or {})
        return r.json()
    except httpx.HTTPError as e:
        return {"resultado": "error", "motivo": f"sin conexión con {client.base_url}: {e}"}


@mcp.tool()
async def move(metros: float) -> dict:
    """Avanza (positivo) o retrocede (negativo) en línea recta, entre -2 y 2 metros, y se detiene solo.

    Devuelve resultado: done (con metros reales), blocked (obstáculo; incluye distancia_m),
    manual_override, cancelled, timeout o error. Mira con take_photo antes de avanzar más de 1 m.
    """
    return await _post(_motion, "/move", {"metros": metros})


@mcp.tool()
async def turn(grados: float) -> dict:
    """Gira todo el cuerpo sobre sí mismo. Positivo = derecha, negativo = izquierda (-360 a 360)."""
    return await _post(_motion, "/turn", {"grados": grados})


@mcp.tool()
async def stop() -> dict:
    """Frena de inmediato y cancela cualquier movimiento en curso o en cola."""
    return await _post(_motion, "/stop")


@mcp.tool()
async def look_at(grados: float) -> dict:
    """Gira solo la cabeza respecto al cuerpo: 0 al frente, positivo derecha, de -90 a 90."""
    return await _post(_motion, "/look_at", {"grados": grados})


@mcp.tool()
async def take_photo() -> Image:
    """Toma una foto con la cámara de la cabeza (640 px) en la dirección a la que mira la cabeza."""
    try:
        r = await _head.get("/foto", params={"ancho": 640})
    except httpx.HTTPError as e:
        raise RuntimeError(f"no hay foto de la cabeza ({e})") from e
    if r.status_code == 503:
        raise RuntimeError("la cabeza no tiene cámara: no puedo ver")
    try:
        r.raise_for_status()
    except httpx.HTTPError as e:
        raise RuntimeError(f"no hay foto de la cabeza ({e})") from e
    return Image(data=r.content, format="jpeg")


@mcp.tool()
async def set_eye_color(
    color: str,
    patron: Literal["fijo", "respirar", "parpadeo", "apagado"] = "fijo",
) -> dict:
    """Cambia el ojo (anillo NeoPixel). color: #RRGGBB o rojo, verde, azul, blanco, naranja,
    amarillo, morado, cian. Úsalo para mostrar estado: azul tranquilo, verde contento,
    naranja pensando, rojo alerta."""
    return await _post(_head, "/ojo", {"color": color, "patron": patron})


@mcp.tool()
async def say(texto: str | None = None, sonido: Emocion | None = None) -> dict:
    """Habla por el altavoz de la cabeza (texto corto, menos de 15 palabras) o hace un pitido de droide.

    Con texto, BB-8 balbucea unos pitidos y luego lo dice; con texto y sonido, pita ese sonido
    antes de hablar. Para reaccionar con el ojo también, usa express."""
    body = {k: v for k, v in (("texto", texto), ("sonido", sonido)) if v}
    return await _post(_head, "/hablar", body)


@mcp.tool()
async def express(emocion: Emocion) -> dict:
    """Muestra una emoción: el pitido de droide de esa emoción y el color del ojo que le toca.
    Úsala para reaccionar antes de hablar o de moverte (ver Personalidad en las instrucciones)."""
    e = EMOCIONES[emocion]
    ojo = await _post(_head, "/ojo", {"color": e.color, "patron": e.patron})
    if ojo.get("ok") is not True:
        return ojo
    sonido = await _post(_head, "/hablar", {"sonido": emocion})
    return {**sonido, "emocion": emocion, "ojo": f"{e.color} {e.patron}"}


@mcp.tool()
async def listen(espera_s: float = 0) -> dict:
    """Lo último que oyó el micrófono del cuerpo (ReSpeaker 2-Mic) y de dónde vino la voz.

    espera_s (0 a 10): si es mayor que 0, espera a que alguien hable antes de responder.
    Devuelve oido (false si no se ha oído nada), texto, nivel_db (60+ cerca, < 40 lejos o
    tras una pared), hace_s, angulo_deg (−90 a 90, + derecha, respecto al frente que
    tenías al oírlo) y candidatos_deg: los dos rumbos posibles respecto a tu frente
    actual, ya corregidos por lo que giraste después (giro_desde_deg). Con dos
    micrófonos no se sabe si vino de adelante (el primero) o de atrás (el segundo).
    """
    try:
        r = await _motion.get("/oido", params={"espera_s": max(0.0, min(10.0, espera_s))},
                              timeout=15.0)
        return r.json()
    except httpx.HTTPError as e:
        return {"resultado": "error", "motivo": f"sin servicio de movimiento: {e}"}


@mcp.tool()
async def get_map() -> dict:
    """Plano de la casa y dónde estás en él.

    Ejes: x al este (derecha del croquis), y al norte (arriba), en metros. robot trae
    x, y, rumbo_deg (horario desde el este: 0 este, 90 sur, -90 norte, 180 oeste) y el
    cuarto (o en_puerta si estás en un hueco). posicion dice si es incierta y cuánto
    has rodado desde la última corrección. Cada cuarto, puerta y mueble trae distancia_m,
    girar_deg (lo que girarías con turn, + derecha, para quedar de frente) y directo
    (true si no hay muro ni puerta cerrada en medio). Cada cuarto trae ruta: las puertas
    a cruzar para llegar. croquis es un dibujo con norte arriba, una letra cada 25 cm:
    # muro, m mueble, . piso, = puerta cerrada, la flecha eres tú.
    """
    try:
        return (await _motion.get("/mapa")).json()
    except httpx.HTTPError as e:
        return {"resultado": "error", "motivo": f"sin servicio de movimiento: {e}"}


@mcp.tool()
async def set_position(x: float, y: float, rumbo_deg: float | None = None) -> dict:
    """Corrige dónde crees estar en el plano (no mueve el robot).

    x, y en metros con los ejes de get_map; rumbo_deg horario desde el este, o null
    para dejar el rumbo como está. Úsalo cuando una foto te diga que no estás donde
    get_map cree.
    """
    body = {"x": x, "y": y} | ({} if rumbo_deg is None else {"rumbo_deg": rumbo_deg})
    return await _post(_motion, "/mapa/posicion", body)


@mcp.tool()
async def update_map(
    accion: Literal["agregar_mueble", "mover_mueble", "quitar_mueble", "cerrar_puerta", "abrir_puerta"],
    nombre: str,
    x: float | None = None,
    y: float | None = None,
    ancho_m: float | None = None,
    largo_m: float | None = None,
    alto_m: float | None = None,
) -> dict:
    """Cambia el plano de la casa y lo guarda (no mueve el robot).

    agregar_mueble: nombre nuevo, x, y del centro, ancho_m (este-oeste), largo_m
    (norte-sur) y alto_m. mover_mueble: nombre y el centro nuevo x, y. quitar_mueble:
    nombre. cerrar_puerta / abrir_puerta: nombre "sala-cocina" (los dos cuartos).
    Úsalo solo con lo que confirmaste con take_photo.
    """
    body = {"accion": accion, "nombre": nombre, "x": x, "y": y,
            "ancho_m": ancho_m, "largo_m": largo_m, "alto_m": alto_m}
    return await _post(_motion, "/mapa/cambiar", {k: v for k, v in body.items() if v is not None})


@mcp.tool()
async def get_pose() -> dict:
    """Estado del robot: posición estimada (x, y en m; y positivo a la derecha), rumbo, inclinación,
    batería, ángulo de la cabeza, distancia ToF al frente y quién controla (llm, manual, idle)."""
    try:
        return (await _motion.get("/pose")).json()
    except httpx.HTTPError as e:
        return {"resultado": "error", "motivo": f"sin servicio de movimiento: {e}"}


def main() -> None:
    ap = argparse.ArgumentParser(description="Servidor MCP bb8-motion")
    ap.add_argument("--stdio", action="store_true", help="transporte stdio (Claude Desktop)")
    ap.add_argument("--host", default=C.MCP_HOST)
    ap.add_argument("--port", type=int, default=C.MCP_PORT)
    a = ap.parse_args()
    # En stdio, stdout es el canal MCP: los logs van a stderr.
    logging.basicConfig(level=logging.INFO, stream=sys.stderr,
                        format="%(asctime)s %(name)s %(levelname)s %(message)s")
    for noisy in ("httpx", "mcp"):
        logging.getLogger(noisy).setLevel(logging.WARNING)
    if a.stdio:
        mcp.run("stdio")
        return
    mcp.settings.host, mcp.settings.port = a.host, a.port
    mcp.settings.log_level = "WARNING"
    if a.host not in ("127.0.0.1", "localhost"):
        # Escuchar en la red de casa: sin autenticación, solo en una WiFi de confianza.
        from mcp.server.transport_security import TransportSecuritySettings
        mcp.settings.transport_security = TransportSecuritySettings(enable_dns_rebinding_protection=False)
    log.info("bb8-motion en http://%s:%d/mcp (movimiento %s, cabeza %s)", a.host, a.port, C.MOTION_URL, C.HEAD_URL)
    mcp.run("streamable-http")


if __name__ == "__main__":
    main()
