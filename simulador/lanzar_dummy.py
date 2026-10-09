"""Arranca todo en el PC sin hardware: robot dummy + servicio de movimiento + MCP.

    python simulador/lanzar_dummy.py                     # webcam del PC como cámara de la cabeza
    python simulador/lanzar_dummy.py --camara sintetica  # vista de la habitación simulada
Ctrl+C detiene los tres procesos.
"""
import os
import signal
import subprocess
import sys
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


def _sigterm(*_) -> None:
    raise KeyboardInterrupt


def main() -> None:
    signal.signal(signal.SIGTERM, _sigterm)
    py = sys.executable
    procs = []
    try:
        procs.append(subprocess.Popen([py, "-m", "dummy.run_dummy", *sys.argv[1:]], cwd=RAIZ, env=ENV))
        wait_http("http://127.0.0.1:8080/estado")
        procs.append(subprocess.Popen([py, "-m", "bb8.motion_api"], cwd=RAIZ, env=ENV))
        wait_http("http://127.0.0.1:8770/pose")
        procs.append(subprocess.Popen([py, "-m", "bb8_mcp"], cwd=RAIZ, env=ENV))
        time.sleep(1.5)
        print("\n✅ BB-8 dummy listo. MCP en http://127.0.0.1:8765/mcp  (Ctrl+C para salir)\n", flush=True)
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
