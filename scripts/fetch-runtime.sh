#!/usr/bin/env bash
# Downloads a portable Python (python-build-standalone) with ytmusicapi and
# yt-dlp installed, plus Deno, into <dest>/python and <dest>/runtime.
# gundil-music finds both next to itself, so nothing else needs installing.
#
#   scripts/fetch-runtime.sh <dest> linux|windows
#
# Needs curl, tar, unzip and the GitHub CLI (gh, with GH_TOKEN set).
set -euo pipefail
dest="$1"; os="$2"
case "$os" in
  linux) triple=x86_64-unknown-linux-gnu; deno=deno-x86_64-unknown-linux-gnu.zip; py="$dest/python/bin/python3" ;;
  windows) triple=x86_64-pc-windows-msvc; deno=deno-x86_64-pc-windows-msvc.zip; py="$dest/python/python.exe" ;;
  *) echo "unknown os $os" >&2; exit 1 ;;
esac
mkdir -p "$dest/runtime"

repo=astral-sh/python-build-standalone
tag="$(gh release view --repo "$repo" --json tagName -q .tagName)"
name="$(gh release view "$tag" --repo "$repo" --json assets -q '.assets[].name' \
  | grep -E "^cpython-3\.12\.[0-9]+\+[0-9]+-$triple-install_only\.tar\.gz$" | head -1)"
[ -n "$name" ] || { echo "no Python build found for $triple in $tag" >&2; exit 1; }
echo "Python: $name"
curl -fsSL "https://github.com/$repo/releases/download/$tag/${name//+/%2B}" | tar -xz -C "$dest"

"$py" -m pip install --no-cache-dir --upgrade pip >/dev/null
"$py" -m pip install --no-cache-dir --upgrade ytmusicapi "yt-dlp[default]"

echo "Deno: $deno"
curl -fsSL "https://github.com/denoland/deno/releases/latest/download/$deno" -o deno.zip
if command -v unzip >/dev/null; then unzip -o -q deno.zip -d "$dest/runtime"; else 7z x -y -o"$dest/runtime" deno.zip >/dev/null; fi
rm -f deno.zip

# Tests, caches and headers aren't needed at run time.
find "$dest/python" -type d \( -name __pycache__ -o -name tests -o -name test \) -prune -exec rm -rf {} + 2>/dev/null || true
rm -rf "$dest/python/include" "$dest/python/share" 2>/dev/null || true
