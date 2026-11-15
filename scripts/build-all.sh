#!/usr/bin/env bash
# Cross-compiles every supported target into ./dist.
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
mkdir -p dist
build() { # name GOOS GOARCH [GOARM]
  echo "building $1"
  CGO_ENABLED=0 GOOS="$2" GOARCH="$3" GOARM="${4:-}" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "dist/$1" .
}
build gundil-music-linux-amd64 linux amd64
build gundil-music-linux-arm64 linux arm64          # Raspberry Pi 3/4/5 (64-bit OS)
build gundil-music-linux-armv7 linux arm 7          # Orange Pi PC, Raspberry Pi 2/3/4 (32-bit OS)
build gundil-music-linux-armv6 linux arm 6          # Raspberry Pi 1 / Zero
build gundil-music-windows-amd64.exe windows amd64
build gundil-music-android-arm64 android arm64      # Termux
ls -lh dist
