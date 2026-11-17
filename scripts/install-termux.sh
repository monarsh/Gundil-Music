#!/data/data/com.termux/files/usr/bin/bash
# Runs the Gundil Music server on an Android phone, inside Termux
# (install Termux from F-Droid). Most people don't need this: the Gundil
# Music APK connects to a server elsewhere. This is for when the phone
# itself should be the server; the APK finds it on this phone.
#
#   bash install-termux.sh [path/to/gundil-music-termux-arm64]
set -euo pipefail

REPO="monarsh/Gundil-Music"

echo "==> Installing Python, Node.js (for yt-dlp) and mpv"
pkg update -y
pkg install -y python nodejs-lts curl mpv

echo "==> Installing the YouTube Music libraries"
pip install -U ytmusicapi yt-dlp yt-dlp-ejs

echo "==> Installing gundil-music"
if [ -n "${1:-}" ]; then
  cp "$1" "$PREFIX/bin/gundil-music"
else
  curl -fL "https://github.com/$REPO/releases/latest/download/gundil-music-termux-arm64" -o "$PREFIX/bin/gundil-music"
fi
chmod 755 "$PREFIX/bin/gundil-music"

echo
echo "Done. Start it with:  gundil-music"
echo "Then open http://localhost:8338 in Chrome on this phone, or this phone's"
echo "Wi-Fi address on port 8338 from another device. Keep Termux open (or take a"
echo "wake lock from its notification) while it runs."
