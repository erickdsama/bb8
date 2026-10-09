"""Constantes del robot y direcciones de los servicios.

Todo lo que se calibra está aquí. Las direcciones se pueden cambiar con
variables de entorno sin tocar el código.
"""
import math
import os

# --- Direcciones -----------------------------------------------------------
# Puerto serial del Arduino. En el dummy: socket://127.0.0.1:5555
SERIAL_URL = os.environ.get("BB8_SERIAL", "/dev/ttyACM0")
SERIAL_BAUD = 115200
# Cabeza (Pi Zero 2 W). En el dummy: http://127.0.0.1:8080
HEAD_URL = os.environ.get("BB8_HEAD_URL", "http://bb8-head.local:8080")
# Servicio de movimiento (solo localhost)
MOTION_HOST = os.environ.get("BB8_MOTION_HOST", "127.0.0.1")
MOTION_PORT = int(os.environ.get("BB8_MOTION_PORT", "8770"))
MOTION_URL = os.environ.get("BB8_MOTION_URL", f"http://{MOTION_HOST}:{MOTION_PORT}")
# Servidor MCP
MCP_HOST = os.environ.get("BB8_MCP_HOST", "127.0.0.1")
MCP_PORT = int(os.environ.get("BB8_MCP_PORT", "8765"))

# --- Mecánica (calibrar en la Parte 3) -------------------------------------
WHEEL_DIAMETER_M = 0.065
# JGB37-520 319 RPM: 11 pulsos/vuelta del motor × ~1:30 × 4 flancos. Verificar
# girando la rueda 10 vueltas a mano y leyendo E con "?".
TICKS_PER_REV = 1320
# >1 si la rueda patina dentro de la esfera (avanza menos de lo que cuenta).
SLIP_FACTOR = 1.0
TICKS_PER_M = TICKS_PER_REV / (math.pi * WHEEL_DIAMETER_M) * SLIP_FACTOR
TRACK_WIDTH_M = 0.17  # distancia entre ruedas
# Velocidad a PWM 255. 319 RPM × π × 0.065 / 60 = 1.08 m/s a 12 V; el L298N
# deja ~10 V al motor, así que ~0.9 m/s.
V_MAX_MPS = 0.9

# --- Límites de seguridad (un solo sitio) ----------------------------------
PWM_CRUISE = 140        # ~55 % → ~0.5 m/s
PWM_MIN = 45            # por debajo los motores no vencen la fricción
PWM_TURN = 90
PWM_TURN_MIN = 40
PWM_MANUAL_MAX = 150
MOVE_MAX_M = 2.0
TURN_MAX_DEG = 360.0
HEAD_MAX_DEG = 90
ORDER_MAX_S = 5.0       # duración máxima de una orden
MANUAL_HOLD_S = 1.0     # el mando manda si tocó algo en el último segundo
GAMEPAD_DEADZONE = 10   # en escala 0..255

# --- Ciclos ----------------------------------------------------------------
POLL_S = 0.05           # sondeo de estado "?" (50 ms)
KEEPALIVE_S = 0.1       # reenvío de M mientras hay movimiento
TOF_POLL_S = 0.1        # lectura del ToF de la cabeza
TOF_FORWARD_MAX_HEAD_DEG = 15  # más allá, el ToF no mira al camino
OBSTACLE_MM = 250
