#!/usr/bin/env bash
# Installs Gundil Music on a Debian-based Linux machine: Raspberry Pi OS,
# Armbian (Orange Pi PC and friends), Debian, Ubuntu. Works headless.
#
#   ./scripts/install.sh [path/to/gundil-music] [music folder]
#
# Without a binary path it downloads the latest release for this CPU.
# Run it as the user that should play the music, not as root.
set -euo pipefail

REPO="monarsh/Gundil-Music"
BIN_SRC="${1:-}"
MUSIC_DIR="${2:-$HOME/Music}"
PORT="${GUNDIL_PORT:-8338}"

if [ "$(id -u)" = 0 ]; then
  echo "Run this as your normal user (it uses sudo where needed)." >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64) ARCH=amd64 ;;
  aarch64 | arm64) ARCH=arm64 ;;
  armv7l | armv8l) ARCH=armv7 ;;
  armv6l) ARCH=armv6 ;;
  *) echo "Unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

echo "==> Installing mpv and the Bluetooth audio stack"
if command -v apt-get >/dev/null; then
  sudo apt-get update
  sudo apt-get install -y mpv bluez curl pipewire pipewire-pulse wireplumber libspa-0.2-bluetooth
else
  echo "    apt-get not found: install mpv, bluez and PipeWire (with Bluetooth support) yourself."
fi

echo "==> Installing the gundil-music binary"
TMP="$(mktemp)"
if [ -n "$BIN_SRC" ]; then
  cp "$BIN_SRC" "$TMP"
else
  curl -fL "https://github.com/$REPO/releases/latest/download/gundil-music-linux-$ARCH" -o "$TMP"
fi
sudo install -m 0755 "$TMP" /usr/local/bin/gundil-music
rm -f "$TMP"

echo "==> Letting $USER use Bluetooth and audio without a login session"
sudo usermod -aG bluetooth,audio "$USER" || true
sudo loginctl enable-linger "$USER"
sudo systemctl enable --now bluetooth || true

# WirePlumber only enables Bluetooth for the "active" login seat by default.
# A headless board has no such seat, so turn that check off (0.5 and 0.4 syntax).
mkdir -p "$HOME/.config/wireplumber/wireplumber.conf.d" "$HOME/.config/wireplumber/bluetooth.lua.d"
cat > "$HOME/.config/wireplumber/wireplumber.conf.d/80-gundil-headless-bluetooth.conf" <<'CONF'
wireplumber.profiles = {
  main = {
    monitor.bluez.seat-monitoring = disabled
  }
}
CONF
cat > "$HOME/.config/wireplumber/bluetooth.lua.d/80-gundil-headless-bluetooth.lua" <<'CONF'
bluez_monitor.properties["with-logind"] = false
CONF

echo "==> Creating the service"
mkdir -p "$MUSIC_DIR" "$HOME/.config/systemd/user"
cat > "$HOME/.config/systemd/user/gundil-music.service" <<UNIT
[Unit]
Description=Gundil Music
After=pipewire.service wireplumber.service network-online.target

[Service]
ExecStart=/usr/local/bin/gundil-music -music "$MUSIC_DIR" -addr :$PORT
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
UNIT

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
systemctl --user daemon-reload
systemctl --user enable pipewire pipewire-pulse wireplumber 2>/dev/null || true
systemctl --user restart wireplumber 2>/dev/null || true
systemctl --user enable --now gundil-music

IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "Done. Put music in $MUSIC_DIR and open http://${IP:-localhost}:$PORT from any device."
echo "Logs: journalctl --user -u gundil-music -f"
