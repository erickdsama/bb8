#!/bin/bash
# Installs the head on the Pi Zero W (armv6, Raspberry Pi OS Lite 32-bit). Hostname: bb8-head.
# No Python: one Go binary. Built on the PC and copied over:
#   PC:    make zero && scp dist/linux-armv6/bb8-head partes/5-cabeza/eye/bb8_eye.c \
#            sistema/{instalar_zero.sh,bb8-head.service,bb8-eye.socket,bb8-eye@.service} bb8-head.local:/tmp/
#   Pi 3:  scp /etc/bb8/pki/{ca.crt,bb8-head.crt,bb8-head.key} bb8-head.local:/tmp/
#   Zero:  sudo bash /tmp/instalar_zero.sh [bb8.local]       # body address, default bb8.local
set -euo pipefail
cd "$(dirname "$0")"
BODY="${1:-bb8.local}"
BIN=bb8-head; [ -x "$BIN" ] || BIN=../dist/linux-armv6/bb8-head
[ -x "$BIN" ] || { echo "Missing bb8-head (make zero on the PC)" >&2; exit 1; }
UNIT=bb8-head.service; [ -f "$UNIT" ] || UNIT=../sistema/bb8-head.service
EYE_C=bb8_eye.c; [ -f "$EYE_C" ] || EYE_C=../partes/5-cabeza/eye/bb8_eye.c
EYE_DIR=.; [ -f bb8-eye.socket ] || EYE_DIR=../sistema
# rpi_ws281x pinned (GPIO12 = PWM0 ALT0 in its pwm.c; Pi Zero W 0x9000c1 in rpihw.c)
WS281X_COMMIT=22f2e35012704845d82e06d2fa2e10fbc810ecaa

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
apt-get install -y --no-install-recommends rpicam-apps-lite alsa-utils espeak-ng gcc git libc6-dev || \
  apt-get install -y --no-install-recommends libcamera-apps-lite alsa-utils espeak-ng gcc git libc6-dev

# Eye: WS2812 on GPIO12 (PWM0) with rpi_ws281x. bb8-head has no cgo, so a small C helper
# (bb8-eye) runs as root per connection through a systemd socket.
WS=$(mktemp -d)
git clone -q https://github.com/jgarff/rpi_ws281x "$WS/rpi_ws281x"
git -C "$WS/rpi_ws281x" checkout -q "$WS281X_COMMIT"
gcc -O2 -o /usr/local/bin/bb8-eye "$EYE_C" "$WS"/rpi_ws281x/{ws2811,pwm,pcm,dma,mailbox,rpihw}.c -I "$WS/rpi_ws281x" -lm
rm -rf "$WS"
cp "$EYE_DIR"/bb8-eye.socket "$EYE_DIR"/bb8-eye@.service /etc/systemd/system/
# SPI (the HAT's APA102 LEDs, or the ring without the HAT), I2C for the VL53L0X and the HAT codec.
raspi-config nonint do_spi 0
raspi-config nonint do_i2c 0
# Power saving: no Bluetooth. WiFi stays (it is how the body reaches it).
CONFIG=/boot/firmware/config.txt; [ -f "$CONFIG" ] || CONFIG=/boot/config.txt
grep -q "dtoverlay=disable-bt" "$CONFIG" || echo "dtoverlay=disable-bt" >> "$CONFIG"
# rpi_ws281x uses the PWM block: the onboard (PWM) audio must be off. The HAT's audio is I2S.
if grep -q "^dtparam=audio=on" "$CONFIG"; then sed -i 's/^dtparam=audio=on/dtparam=audio=off/' "$CONFIG"
else grep -q "^dtparam=audio=off" "$CONFIG" || echo "dtparam=audio=off" >> "$CONFIG"; fi

systemctl disable --now bb8-cabeza 2>/dev/null || true   # old Python head
rm -f /etc/systemd/system/bb8-cabeza.service
cp "$UNIT" /etc/systemd/system/bb8-head.service
systemctl daemon-reload
systemctl enable --now bb8-eye.socket bb8-head
echo "Done. Logs: journalctl -fu bb8-head  (it should say 'link: connected to body')"
echo "Reboot once so SPI, I2C and dtparam=audio=off apply (the eye needs PWM free)."
