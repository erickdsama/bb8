"""Parte 3 · calibración y prueba de la base desde la Pi.

Con el puerto serial directo (detén antes bb8.motion_api, que es el único dueño del serial):

    python -m calibrar consola            # escribir órdenes a mano: M 80 80, ?, S...
    python -m calibrar ticks              # pulsos por vuelta (giras la rueda a mano)
    python -m calibrar velocidad          # ruedas en el aire: velocidad máxima y PWM mínimo

Con el servicio de movimiento corriendo (base en el piso, 1 m libre al frente):

    python -m calibrar rutina             # avanza, gira 90°, vuelve; mide deriva

--serial cambia el puerto (por defecto BB8_SERIAL o /dev/ttyACM0); con el dummy:
--serial socket://127.0.0.1:5555
"""
from __future__ import annotations

import argparse
import math
import sys
import time

import httpx

from bb8 import config as C
from bb8 import protocol as P
from bb8.serial_link import SerialLink


def _abrir(url: str) -> SerialLink:
    link = SerialLink(url, C.SERIAL_BAUD)
    print(f"Abriendo {url}...")
    print(link.open())
    return link


def _estado(link: SerialLink) -> P.Status:
    return P.parse_status(link.send(P.CMD_STATUS))


def consola(link: SerialLink) -> None:
    print("Escribe órdenes del protocolo (M 80 80, S, H 30, ?, I). Línea vacía o Ctrl+C para salir.")
    print("Ojo: M sin repetir frena a los 500 ms por el watchdog; es lo esperado.")
    try:
        while line := input("> ").strip():
            try:
                print("OK " + link.send(line))
            except P.ProtocolError as e:
                print(e.line)
            except TimeoutError as e:
                print(e)
    except (KeyboardInterrupt, EOFError):
        pass
    link.send(P.CMD_STOP)


def ticks(link: SerialLink) -> None:
    vueltas = 10
    input(f"Marca la rueda IZQUIERDA con cinta. Enter, gírala {vueltas} vueltas HACIA ADELANTE y Enter otra vez.")
    e0 = _estado(link).enc_left
    input("...girando. Enter al terminar.")
    izq = (_estado(link).enc_left - e0) / vueltas
    input(f"Ahora la DERECHA: Enter, {vueltas} vueltas hacia adelante y Enter.")
    e0 = _estado(link).enc_right
    input("...girando. Enter al terminar.")
    der = (_estado(link).enc_right - e0) / vueltas
    print(f"\nPulsos por vuelta: izquierda {izq:.0f}, derecha {der:.0f} (config actual {C.TICKS_PER_REV})")
    for nombre, v in (("izquierdo", izq), ("derecho", der)):
        if v < 0:
            print(f"  El encoder {nombre} cuenta al revés: INVERTIR_ENC_{'IZQ' if nombre == 'izquierdo' else 'DER'} 1 en el firmware.")
    if izq > 0 and der > 0:
        print(f"  Pon en bb8/config.py: TICKS_PER_REV = {round((izq + der) / 2)}")


def _correr(link: SerialLink, izq: int, der: int, segundos: float) -> tuple[float, float]:
    """Manda M cada 100 ms (watchdog) y devuelve pulsos por segundo de cada rueda en el último 60 %."""
    t0 = time.monotonic()
    marca = None
    while (t := time.monotonic() - t0) < segundos:
        try:
            link.send(P.cmd_motors(izq, der))
        except P.ProtocolError as e:
            link.send(P.CMD_STOP)
            sys.exit(f"El firmware frenó: {e.reason}. ¿Ruedas en el aire y nada frente al ToF?")
        if marca is None and t > segundos * 0.4:
            st = _estado(link)
            marca = (t, st.enc_left, st.enc_right)
        time.sleep(0.1)
    st = _estado(link)
    t1 = time.monotonic() - t0
    link.send(P.CMD_STOP)
    assert marca
    dt = t1 - marca[0]
    return (st.enc_left - marca[1]) / dt, (st.enc_right - marca[2]) / dt


def velocidad(link: SerialLink) -> None:
    input("Ruedas EN EL AIRE (base sobre un bote). Enter para empezar.")
    d = _estado(link).tof_mm
    if 0 < d < 300:
        sys.exit(f"El ToF ve algo a {d} mm y el firmware no dejará avanzar. Despeja el frente del sensor.")
    print("PWM 255 durante 3 s...")
    izq, der = _correr(link, 255, 255, 3.0)
    por_ciclo = (izq + der) / 2 / 100
    v = (izq + der) / 2 / C.TICKS_PER_M
    print(f"  pulsos/s izq {izq:.0f}, der {der:.0f}  → {por_ciclo:.1f} pulsos cada 10 ms, {v:.2f} m/s")
    if min(izq, der) > 0 and abs(izq - der) / max(izq, der) > 0.1:
        print("  Las ruedas difieren más de 10 %: el PID lo compensa, pero revisa engranes y cables.")
    print(f"  Firmware: PULSOS_MAX_CICLO = {por_ciclo:.0f}   bb8/config.py: V_MAX_MPS = {v:.2f}")

    print("\nBuscando el PWM mínimo que arranca las dos ruedas...")
    for pwm in range(20, 160, 5):
        izq, der = _correr(link, pwm, pwm, 1.0)
        print(f"  PWM {pwm}: izq {izq:.0f} pulsos/s, der {der:.0f}")
        if izq > 50 and der > 50:
            print(f"  bb8/config.py: PWM_MIN = {pwm + 5}  (en el piso súmale ~10)")
            break
        time.sleep(0.3)


def rutina(url: str) -> None:
    print("Base en el piso, 1 m libre al frente. Empieza en 3 s (Ctrl+C para cancelar).")
    time.sleep(3)
    cli = httpx.Client(base_url=url, timeout=20)

    def paso(ruta: str, cuerpo: dict | None = None) -> dict:
        r = cli.post(ruta, json=cuerpo or {}).json()
        print(f"  {ruta} {cuerpo or ''} → {r}")
        return r

    p0 = cli.get("/pose").json()
    paso("/move", {"metros": 0.5})
    paso("/turn", {"grados": 90})
    paso("/turn", {"grados": -90})
    paso("/move", {"metros": -0.5})
    time.sleep(0.5)
    p1 = cli.get("/pose").json()
    deriva = p1["rumbo_deg"] - p0["rumbo_deg"]
    dist = math.hypot(p1["x_m"] - p0["x_m"], p1["y_m"] - p0["y_m"])
    print("\nDebería volver al inicio con el mismo rumbo.")
    print(f"  Rumbo final − inicial: {deriva:+.1f}°  (bien si |x| < 5°)")
    print(f"  Distancia al inicio según odometría: {dist * 100:.0f} cm")
    print("  Mide con cinta dónde quedó de verdad. Si avanzó menos de lo ordenado,")
    print("  SLIP_FACTOR = metros_ordenados / metros_reales en bb8/config.py.")
    print(f"  Inclinación máx. vista: revisa que nunca pasara de ~15°; si cabecea, baja PWM_CRUISE ({C.PWM_CRUISE}).")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("modo", choices=["consola", "ticks", "velocidad", "rutina"])
    ap.add_argument("--serial", default=C.SERIAL_URL)
    ap.add_argument("--motion", default=C.MOTION_URL, help="URL del servicio de movimiento (rutina)")
    a = ap.parse_args()
    if a.modo == "rutina":
        rutina(a.motion)
        return
    try:
        link = _abrir(a.serial)
    except Exception as e:
        sys.exit(f"No pude abrir {a.serial}: {e}\n¿Está corriendo bb8.motion_api? Detenlo primero.")
    try:
        {"consola": consola, "ticks": ticks, "velocidad": velocidad}[a.modo](link)
    finally:
        try:
            link.send(P.CMD_STOP)
        finally:
            link.close()


if __name__ == "__main__":
    main()
