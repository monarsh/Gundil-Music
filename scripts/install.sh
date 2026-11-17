#!/usr/bin/env bash
# Installs Gundil Music on a Debian-based Linux machine: Raspberry Pi OS,
# Armbian (Orange Pi PC and friends), Debian, Ubuntu. Works headless.
#
#   ./scripts/install.sh [path/to/gundil-music]
#
# Without a binary path it downloads the latest release for this CPU.
# Run it as the user that should play the music, not as root.
set -euo pipefail

REPO="monarsh/Gundil-Music"
BIN_SRC="${1:-}"
APP="$HOME/.local/share/gundil-music"
VENV="$APP/venv"
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

echo "==> Installing mpv, Python and the Bluetooth audio stack"
if command -v apt-get >/dev/null; then
  sudo apt-get update
  sudo apt-get install -y mpv bluez curl unzip xz-utils python3 python3-venv pipewire pipewire-pulse wireplumber libspa-0.2-bluetooth
else
  echo "    apt-get not found: install mpv, bluez, curl, unzip, xz, python3 (with venv) and PipeWire yourself."
fi

echo "==> Installing the YouTube Music libraries (ytmusicapi, yt-dlp)"
python3 -m venv "$VENV"
"$VENV/bin/pip" install --upgrade pip
"$VENV/bin/pip" install --upgrade ytmusicapi "yt-dlp[default]"

# yt-dlp needs a JavaScript runtime (Deno 2.3+ or Node 22+) to unlock YouTube
# streams. Distribution packages are too old, so fetch an official build.
echo "==> Installing a JavaScript runtime for yt-dlp"
mkdir -p "$APP/bin"
case "$ARCH" in
  amd64 | arm64)
    target=x86_64-unknown-linux-gnu
    [ "$ARCH" = arm64 ] && target=aarch64-unknown-linux-gnu
    curl -fL "https://github.com/denoland/deno/releases/latest/download/deno-$target.zip" -o "$APP/deno.zip"
    unzip -o -q "$APP/deno.zip" -d "$APP/bin" && rm -f "$APP/deno.zip"
    "$APP/bin/deno" --version | head -1
    ;;
  armv7)
    # Deno has no 32-bit ARM build; Node.js still publishes one.
    file="$(curl -fsSL https://nodejs.org/dist/latest-v22.x/SHASUMS256.txt | awk '/linux-armv7l\.tar\.xz$/ {print $2}')"
    curl -fL "https://nodejs.org/dist/latest-v22.x/$file" -o "$APP/node.tar.xz"
    rm -rf "$APP/node" && mkdir -p "$APP/node"
    tar -xJf "$APP/node.tar.xz" -C "$APP/node" --strip-components=1 && rm -f "$APP/node.tar.xz"
    ln -sf "$APP/node/bin/node" "$APP/bin/node"
    "$APP/bin/node" --version
    ;;
  armv6)
    echo "    No supported JavaScript runtime exists for ARMv6 (Pi 1 / Zero). Most YouTube songs will not play." ;;
esac

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
mkdir -p "$HOME/.config/systemd/user"
cat > "$HOME/.config/systemd/user/gundil-music.service" <<UNIT
[Unit]
Description=Gundil Music
After=pipewire.service wireplumber.service network-online.target

[Service]
Environment=PATH=$APP/bin:/usr/local/bin:/usr/bin:/bin
ExecStart=/usr/local/bin/gundil-music -python "$VENV/bin/python" -addr :$PORT
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
echo "Done. Open http://${IP:-localhost}:$PORT from any device."
echo "Keep YouTube working by updating yt-dlp now and then: $VENV/bin/pip install -U yt-dlp ytmusicapi"
echo "Logs: journalctl --user -u gundil-music -f"
