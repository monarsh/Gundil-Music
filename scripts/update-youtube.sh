#!/usr/bin/env bash
# YouTube changes often. If songs stop playing, update the YouTube libraries.
set -euo pipefail
VENV="$HOME/.local/share/gundil-music/venv"
"$VENV/bin/pip" install --upgrade yt-dlp ytmusicapi yt-dlp-ejs
systemctl --user restart gundil-music
echo "Updated and restarted."
