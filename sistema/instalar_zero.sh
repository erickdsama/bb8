#!/bin/bash
# Instala la cabeza en la Pi Zero 2 W. Hostname: bb8-head (sudo raspi-config).
# Clona el repo en /home/bb8/bb8 (git clone https://github.com/erickdsama/bb8 /home/bb8/bb8).
# Activa I2C y la cámara en raspi-config. Luego:  sudo bash sistema/instalar_zero.sh
set -euo pipefail
cd "$(dirname "$0")/.."
id bb8 &>/dev/null || useradd -m bb8
apt-get install -y python3-venv python3-picamera2 python3-opencv alsa-utils
python3 -m venv --system-site-packages .venv
.venv/bin/pip install -r partes/5-cabeza/requirements-zero.txt
.venv/bin/pip install --no-deps -e .   # registra el paquete head
# Piper para la voz en la cabeza (binario arm64)
if ! command -v piper >/dev/null; then
  .venv/bin/pip install piper-tts
  ln -sf "$PWD/.venv/bin/piper" /usr/local/bin/piper
fi
mkdir -p /opt/piper
for ext in onnx onnx.json; do
  [ -f /opt/piper/es_ES-davefx-medium.$ext ] || curl -fsSL -o /opt/piper/es_ES-davefx-medium.$ext \
    "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/davefx/medium/es_ES-davefx-medium.$ext"
done
# Ahorro en reposo: sin Bluetooth ni HDMI, CPU en powersave. El WiFi se queda (es como la despiertan).
grep -q "dtoverlay=disable-bt" /boot/firmware/config.txt || echo "dtoverlay=disable-bt" >> /boot/firmware/config.txt
cp sistema/bb8-cabeza.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now bb8-cabeza
echo "Listo: curl http://bb8-head.local:8080/estado"
