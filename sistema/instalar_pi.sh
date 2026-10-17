#!/bin/bash
# Installs the BB-8 on the main Pi: Raspberry Pi 3 B+ with Raspberry Pi OS Lite 64-bit.
# Repo in /home/bb8/bb8 (git clone https://github.com/erickdsama/bb8 /home/bb8/bb8).
#
# The Go binaries are built on the PC and copied over (the Pi does not need Go):
#   PC:  make pi && scp dist/linux-arm64/bb8-body bb8.local:/home/bb8/bb8/dist/linux-arm64/
#   Pi:  sudo bash sistema/instalar_pi.sh            # add --sin-voz to skip the Python voice stack
set -euo pipefail
cd "$(dirname "$0")/.."
VOZ=1; [ "${1:-}" = "--sin-voz" ] && VOZ=0
# faster-whisper (ctranslate2) and onnxruntime only ship aarch64 wheels: 64-bit OS needed.
if [ "$(uname -m)" != aarch64 ]; then
  echo "This system is $(uname -m). Install Raspberry Pi OS Lite 64-bit (the Pi 3 supports it)." >&2
  exit 1
fi
BIN=dist/linux-arm64/bb8-body
if [ ! -x "$BIN" ]; then
  if command -v go >/dev/null; then make pi; else
    echo "Missing $BIN. On the PC: make pi && scp $BIN bb8.local:$PWD/dist/linux-arm64/" >&2; exit 1
  fi
fi
id bb8 &>/dev/null || useradd -m -G dialout,audio,input bb8
usermod -aG dialout,audio,input bb8
install -m 755 "$BIN" /usr/local/bin/bb8-body

# Secure head link: private CA + body/head certs (once). The head files go to the Zero.
install -d -m 750 -o bb8 -g bb8 /etc/bb8
if [ ! -f /etc/bb8/pki/ca.crt ]; then
  /usr/local/bin/bb8-body pair --dir /etc/bb8/pki --host "$(hostname).local" --host "$(hostname -I | awk '{print $1}')"
  chown -R bb8: /etc/bb8/pki
fi
# Bearer token for MCP and the config page from the LAN.
if [ ! -f /etc/bb8/token ]; then
  head -c 24 /dev/urandom | base64 | tr -d '/+=' > /etc/bb8/token
  chown bb8: /etc/bb8/token; chmod 600 /etc/bb8/token
fi
[ -f /etc/bb8.env ] || install -m 600 -o bb8 sistema/bb8.env.ejemplo /etc/bb8.env

apt-get install -y alsa-utils zram-tools
# 1 GB RAM (Pi 3): compressed swap in RAM for model loading peaks, without wearing the microSD.
if [ "$(awk '/MemTotal/ {print $2}' /proc/meminfo)" -lt 2000000 ]; then
  printf 'ALGO=zstd\nPERCENT=50\nPRIORITY=100\n' > /etc/default/zramswap
  systemctl restart zramswap || true
  # No monitor or camera on the main Pi: 16 MB for the GPU (applies after reboot).
  CONFIG=/boot/firmware/config.txt; [ -f "$CONFIG" ] || CONFIG=/boot/config.txt
  grep -q "^gpu_mem=" "$CONFIG" || echo "gpu_mem=16" >> "$CONFIG"
fi

# Python only for the voice front end (wake word, STT, speaker id) and the power manager.
if [ $VOZ = 1 ]; then
  chown -R bb8: .
  apt-get install -y python3-venv libportaudio2 libopenblas0
  sudo -u bb8 python3 -m venv .venv
  sudo -u bb8 .venv/bin/pip install --no-cache-dir -r requirements.txt -r partes/4-agente-movimiento/requirements-voz.txt
  sudo -u bb8 .venv/bin/pip install --no-deps -e .
  sudo -u bb8 mkdir -p partes/4-agente-movimiento/voz/modelos
  for ext in onnx onnx.json; do
    [ -f partes/4-agente-movimiento/voz/modelos/es_ES-davefx-medium.$ext ] || sudo -u bb8 curl -fsSL -o partes/4-agente-movimiento/voz/modelos/es_ES-davefx-medium.$ext \
      "https://huggingface.co/rhasspy/piper-voices/resolve/main/es/es_ES/davefx/medium/es_ES-davefx-medium.$ext"
  done
  # The power manager shuts the Pi down in deep sleep.
  echo "bb8 ALL=(root) NOPASSWD: /usr/bin/systemctl poweroff" > /etc/sudoers.d/bb8-poweroff
  chmod 440 /etc/sudoers.d/bb8-poweroff
  cp sistema/bb8-{voz,energia}.service /etc/systemd/system/
fi

# Old Python services (bb8-motion, bb8-mcp) are replaced by bb8-body.
systemctl disable --now bb8-motion bb8-mcp 2>/dev/null || true
rm -f /etc/systemd/system/bb8-{motion,mcp}.service
cp sistema/bb8-body.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now bb8-body
[ $VOZ = 1 ] && systemctl enable bb8-energia
cat <<MSG
Done. bb8-body is running.
  Config page:  http://$(hostname).local:8765/?token=$(cat /etc/bb8/token)
                (set agent.api_key for OpenRouter, home.city…)
  Head certs:   scp /etc/bb8/pki/{ca.crt,bb8-head.crt,bb8-head.key} bb8-head.local:/tmp/  then run instalar_zero.sh there
  Voice:        sudo systemctl enable --now bb8-voz bb8-energia
  Logs:         journalctl -fu bb8-body
MSG
