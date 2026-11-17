#!/usr/bin/env bash
# Builds gundil-music-<version>-windows-x86_64.zip: the app plus Python, the
# YouTube libraries and Deno. Run on Windows (Git Bash), so pip installs
# Windows packages.
#
#   VERSION=v1.1 scripts/package-windows.sh
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
dir="build/Gundil Music"
rm -rf build/win && mkdir -p build/win dist
rm -rf "$dir" && mkdir -p "$dir"

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$dir/gundil-music.exe" .
scripts/fetch-runtime.sh "$dir" windows
cp packaging/windows/README.txt "$dir/README.txt"

out="dist/gundil-music-$VERSION-windows-x86_64.zip"
rm -f "$out"
(cd build && 7z a -tzip -mx=7 "../$out" "Gundil Music" >/dev/null)
ls -lh "$out"
