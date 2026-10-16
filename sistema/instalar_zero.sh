#!/bin/bash
# Installs the head on the Pi Zero W (armv6, Raspberry Pi OS Lite 32-bit). Hostname: bb8-head.
# No Python: one Go binary. Built on the PC and copied over:
#   PC:    make zero && scp dist/linux-armv6/bb8-head sistema/{instalar_zero.sh,bb8-head.service} bb8-head.local:/tmp/
#   Pi 3:  scp /etc/bb8/pki/{ca.crt,bb8-head.crt,bb8-head.key} bb8-head.local:/tmp/
#   Zero:  sudo bash /tmp/instalar_zero.sh [bb8.local]       # body address, default bb8.local
set -euo pipefail
cd "$(dirname "$0")"
BODY="${1:-bb8.local}"
BIN=bb8-head; [ -x "$BIN" ] || BIN=../dist/linux-armv6/bb8-head
[ -x "$BIN" ] || { echo "Missing bb8-head (make zero on the PC)" >&2; exit 1; }
UNIT=bb8-head.service; [ -f "$UNIT" ] || UNIT=../sistema/bb8-head.service

id bb8 &>/dev/null || useradd -m bb8
for g in spi i2c video audio gpio; do getent group $g >/dev/null && usermod -aG $g bb8; done
install -m 755 "$BIN" /usr/local/bin/bb8-head

# Certs from 'bb8-body pair' (on the main Pi).
install -d -m 750 -o bb8 -g bb8 /etc/bb8/pki
for f in ca.crt bb8-head.crt bb8-head.key; do
  [ -f /etc/bb8/pki/$f ] || install -m 600 -o bb8 /tmp/$f /etc/bb8/pki/$f
done
echo "BB8_BODY_ADDR=$BODY:8772" > /etc/bb8.env

# Camera (rpicam-jpeg), speaker (aplay), speech (espeak-ng: Piper does not run on armv6).
apt-get install -y --no-install-recommends rpicam-apps-lite alsa-utils espeak-ng || \
  apt-get install -y --no-install-recommends libcamera-apps-lite alsa-utils espeak-ng
# SPI for the NeoPixel ring (data on GPIO10 / pin 19), I2C for the VL53L0X.
raspi-config nonint do_spi 0
raspi-config nonint do_i2c 0
# Power saving: no Bluetooth. WiFi stays (it is how the body reaches it).
CONFIG=/boot/firmware/config.txt; [ -f "$CONFIG" ] || CONFIG=/boot/config.txt
grep -q "dtoverlay=disable-bt" "$CONFIG" || echo "dtoverlay=disable-bt" >> "$CONFIG"

systemctl disable --now bb8-cabeza 2>/dev/null || true   # old Python head
rm -f /etc/systemd/system/bb8-cabeza.service
cp "$UNIT" /etc/systemd/system/bb8-head.service
systemctl daemon-reload
systemctl enable --now bb8-head
echo "Done. Logs: journalctl -fu bb8-head  (it should say 'link: connected to body')"
