"""Arranca todo en el PC sin hardware: robot dummy + bb8-body (Go) + bb8-head --sim (Go).

    python simulador/lanzar_dummy.py                     # webcam del PC como cámara de la cabeza
    python simulador/lanzar_dummy.py --camara sintetica  # vista de la habitación simulada
    python simulador/lanzar_dummy.py --camara ninguna    # cabeza sin cámara
    python simulador/lanzar_dummy.py --mundo casa        # casa de 8×6 m con una persona
    BB8_MCP_PORT=8775 python simulador/lanzar_dummy.py   # MCP en otro puerto si el 8765 está ocupado
    python simulador/lanzar_dummy.py --python            # stack Python anterior (bb8.motion_api + bb8_mcp)
Compila los binarios de Go en bin/ (necesita Go 1.25+ en el PATH o en ~/.local/go).
Ctrl+C detiene todos los procesos.
"""
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import time
import tomllib
import urllib.request

RAIZ = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _rutas_paquetes() -> list[str]:
    """Carpetas de cada parte con paquetes de Python, leídas de pyproject.toml.

    Así el dummy corre aunque no se haya hecho `pip install -e .`.
    """
    with open(os.path.join(RAIZ, "pyproject.toml"), "rb") as f:
        mapa = tomllib.load(f)["tool"]["setuptools"]["package-dir"]
    return sorted({os.path.dirname(os.path.join(RAIZ, d)) for d in mapa.values()})


ENV = os.environ | {
    "BB8_SERIAL": "socket://127.0.0.1:5555",
    "BB8_HEAD_URL": "http://127.0.0.1:8080",
    "PYTHONPATH": os.pathsep.join([*_rutas_paquetes(), os.environ.get("PYTHONPATH", "")]),
    "PYTHONUNBUFFERED": "1",
    # run_dummy escribe aquí el plano del mundo y el servicio de movimiento lo lee
    "BB8_MAPA": os.path.join(tempfile.gettempdir(), f"bb8_mapa_{os.getpid()}.json"),
    # pose guardada de esta corrida (no la del robot real en ~/.bb8)
    "BB8_MAPA_ESTADO": os.path.join(tempfile.gettempdir(), f"bb8_pose_{os.getpid()}.json"),
}


def wait_http(url: str, seconds: float = 20) -> None:
    t0 = time.time()
    while time.time() - t0 < seconds:
        try:
            urllib.request.urlopen(url, timeout=1)
            return
        except Exception:
            time.sleep(0.3)
    raise SystemExit(f"No arrancó {url}")


def _go() -> str:
    go = shutil.which("go") or os.path.expanduser("~/.local/go/bin/go")
    if not os.path.exists(go):
        raise SystemExit("Falta Go: https://go.dev/dl (o usa --python)")
    return go


def _compilar_go() -> str:
    """Compila bb8-body y bb8-head en bin/ y crea certificados de prueba (PKI temporal)."""
    bin_dir = os.path.join(RAIZ, "bin")
    subprocess.run([_go(), "build", "-o", bin_dir + os.sep, "./go/cmd/..."], cwd=RAIZ, check=True)
    pki = os.path.join(tempfile.gettempdir(), f"bb8_pki_{os.getpid()}")
    subprocess.run([os.path.join(bin_dir, "bb8-body"), "pair", "--dir", pki], cwd=RAIZ, check=True,
                   stdout=subprocess.DEVNULL)
    return pki


def wait_head(seconds: float = 15) -> None:
    import json
    t0 = time.time()
    while time.time() - t0 < seconds:
        try:
            if json.load(urllib.request.urlopen("http://127.0.0.1:8770/head", timeout=1)).get("connected"):
                return
        except Exception:
            pass
        time.sleep(0.3)
    raise SystemExit("La cabeza no se conectó al cuerpo")


def _sigterm(*_) -> None:
    raise KeyboardInterrupt


def main() -> None:
    signal.signal(signal.SIGTERM, _sigterm)
    py = sys.executable
    args = [a for a in sys.argv[1:] if a not in ("--go", "--python")]
    python = "--python" in sys.argv[1:]
    pki = None if python else _compilar_go()
    procs = []
    try:
        procs.append(subprocess.Popen([py, "-m", "dummy.run_dummy", *args], cwd=RAIZ, env=ENV))
        wait_http("http://127.0.0.1:8080/estado")
        if python:
            procs.append(subprocess.Popen([py, "-m", "bb8.motion_api"], cwd=RAIZ, env=ENV))
            wait_http("http://127.0.0.1:8770/pose")
            procs.append(subprocess.Popen([py, "-m", "bb8_mcp"], cwd=RAIZ, env=ENV))
            time.sleep(1.5)
        else:
            b = os.path.join(RAIZ, "bin")
            # The pages: bb8-web (off the robot), no password on loopback; the body dials out to it.
            web_port = os.environ.get("BB8_WEB_PORT", "8780")
            web_data = os.path.join(tempfile.gettempdir(), f"bb8_web_{os.getpid()}")
            procs.append(subprocess.Popen([os.path.join(b, "bb8-web"), "--addr", f"127.0.0.1:{web_port}",
                                           "--data", web_data, "--no-login"], cwd=RAIZ, env=ENV))
            wait_http(f"http://127.0.0.1:{web_port}/login")
            procs.append(subprocess.Popen([os.path.join(b, "bb8-body"), "--serial", ENV["BB8_SERIAL"],
                                           "--web", f"ws://127.0.0.1:{web_port}/robot",
                                           "--web-token-file", os.path.join(web_data, "device-token"),
                                           "--map", ENV["BB8_MAPA"], "--map-state", ENV["BB8_MAPA_ESTADO"],
                                           "--pki", pki, "--link", "127.0.0.1:8772",
                                           "--data", os.path.join(tempfile.gettempdir(), f"bb8_data_{os.getpid()}")],
                                          cwd=RAIZ, env=ENV))
            wait_http("http://127.0.0.1:8770/pose")
            procs.append(subprocess.Popen([os.path.join(b, "bb8-head"), "--body", "127.0.0.1:8772",
                                           "--pki", pki, "--sim", "http://127.0.0.1:8080"], cwd=RAIZ, env=ENV))
            wait_head()
        print(f"\n✅ BB-8 dummy listo. MCP en http://127.0.0.1:{os.environ.get('BB8_MCP_PORT', '8765')}/mcp\n"
              f"   Páginas (bb8-web): http://127.0.0.1:{os.environ.get('BB8_WEB_PORT', '8780')}/\n"
              "   Míralo moverse en http://127.0.0.1:8080/sim  (Ctrl+C para salir)\n", flush=True)
        while all(p.poll() is None for p in procs):
            time.sleep(0.5)
        print("Un proceso terminó; cerrando los demás.")
    except KeyboardInterrupt:
        pass
    finally:
        for p in procs:
            p.terminate()
        for p in procs:
            try:
                p.wait(5)
            except subprocess.TimeoutExpired:
                p.kill()


if __name__ == "__main__":
    main()
