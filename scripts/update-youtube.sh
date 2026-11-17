#!/usr/bin/env bash
# Updates yt-dlp and ytmusicapi now and restarts Gundil Music. The server also
# does this by itself once a day, and About → "Update YouTube support" in the
# app does the same. Use this on a Pi/Orange Pi set up by install.sh.
set -euo pipefail
VENV="$HOME/.local/share/gundil-music/venv"
PATH="$HOME/.local/share/gundil-music/bin:$PATH" gundil-music -python "$VENV/bin/python" -update-youtube
systemctl --user restart gundil-music
echo "Restarted."
