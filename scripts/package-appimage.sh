#!/usr/bin/env bash
# Builds Gundil_Music-<version>-x86_64.AppImage: the app plus Python, the
# YouTube libraries and Deno in one file for Linux desktops.
#
#   VERSION=v1.1 scripts/package-appimage.sh
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
app=build/AppDir
rm -rf "$app" && mkdir -p "$app/usr/bin" dist

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$app/usr/bin/gundil-music" .
scripts/fetch-runtime.sh "$app/usr/bin" linux

cp packaging/appimage/AppRun "$app/AppRun" && chmod +x "$app/AppRun"
cp packaging/appimage/gundil-music.desktop "$app/"
# The icon at the top (what the AppImage shows) and in the standard icon
# folders, which start-menu integration tools copy from.
cp web/icon-512.png "$app/gundil-music.png"
ln -sf gundil-music.png "$app/.DirIcon"
mkdir -p "$app/usr/share/icons" "$app/usr/share/applications"
cp -r packaging/icons/hicolor "$app/usr/share/icons/"
cp packaging/appimage/gundil-music.desktop "$app/usr/share/applications/"

tool=build/appimagetool
[ -x "$tool" ] || { curl -fsSL https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage -o "$tool"; chmod +x "$tool"; }
# Without VERSION in its environment: appimagetool would otherwise write it into
# the shortcut (X-AppImage-Version), and menus would show "Gundil Music (v1.0)".
env -u VERSION ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$tool" --no-appstream "$app" "dist/Gundil_Music-$VERSION-x86_64.AppImage"
ls -lh dist/*.AppImage
