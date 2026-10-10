#!/bin/bash
# Instala el BB-8 en la Pi principal: Raspberry Pi 3 (o mejor) con Raspberry Pi OS Lite de 64 bits.
# Clona el repo en /home/bb8/bb8 (git clone https://github.com/erickdsama/bb8 /home/bb8/bb8) y corre:
#   sudo bash sistema/instalar_pi.sh
set -euo pipefail
cd "$(dirname "$0")/.."
# faster-whisper (ctranslate2) y onnxruntime solo publican ruedas aarch64, no armv7: hace falta el OS de 64 bits.
if [ "$(uname -m)" != aarch64 ]; then
  echo "Este sistema es $(uname -m). Instala Raspberry Pi OS Lite de 64 bits (la Pi 3 lo soporta)." >&2
  exit 1
fi
id bb8 &>/dev/null || useradd -m -G dialout,audio,input bb8
usermod -aG dialout,audio,input bb8
chown -R bb8: .   # el venv y los modelos se crean como bb8
apt-get install -y python3-venv libportaudio2 libopenblas0 alsa-utils zram-tools
# 1 GB de RAM (Pi 3): swap comprimida en RAM para los picos al cargar modelos, sin gastar la microSD.
if [ "$(awk '/MemTotal/ {print $2}' /proc/meminfo)" -lt 2000000 ]; then
  printf 'ALGO=zstd\nPERCENT=50\nPRIORITY=100\n' > /etc/default/zramswap
  systemctl restart zramswap || true
  # Sin monitor ni cámara en la Pi principal: deja solo 16 MB a la GPU (aplica al reiniciar).
  CONFIG=/boot/firmware/config.txt; [ -f "$CONFIG" ] || CONFIG=/boot/config.txt
  grep -q "^gpu_mem=" "$CONFIG" || echo "gpu_mem=16" >> "$CONFIG"
fi
sudo -u bb8 python3 -m venv .venv
sudo -u bb8 .venv/bin/pip install --no-cache-dir -r requirements.txt -r partes/4-agente-movimiento/requirements-voz.txt evdev
# Registra los paquetes de cada parte (bb8, bb8_mcp, voz, energia...) para que python -m los encuentre
sudo -u bb8 .venv/bin/pip install --no-deps -e .
# Voz de Piper en español
sudo -u bb8 mkdir -p partes/4-agente-movimiento/voz/modelos
for ext in onnx onnx.json; do
  [ -f partes/4-agente-movimiento/voz/modelos/es_ES-davefx-medium.$ext ] || sudo -u bb8 curl -fsSL -o partes/4-agente-movimiento/voz/modelos/es_ES-davefx-medium.$ext \
    "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/davefx/medium/es_ES-davefx-medium.$ext"
done
[ -f /etc/bb8.env ] || install -m 600 -o bb8 sistema/bb8.env.ejemplo /etc/bb8.env
# El gestor de energía apaga la Pi en el reposo profundo
echo "bb8 ALL=(root) NOPASSWD: /usr/bin/systemctl poweroff" > /etc/sudoers.d/bb8-poweroff
chmod 440 /etc/sudoers.d/bb8-poweroff
cp sistema/bb8-{motion,mcp,voz,energia}.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable bb8-motion bb8-mcp bb8-energia
echo "Listo. Pon tu ANTHROPIC_API_KEY en /etc/bb8.env y luego: sudo systemctl enable --now bb8-voz"
echo "Arranca: sudo systemctl start bb8-motion bb8-mcp bb8-energia   Logs: journalctl -fu bb8-voz"
