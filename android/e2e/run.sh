#!/usr/bin/env bash
# Runs inside an Android emulator job: installs the debug APK, connects it to
# a Gundil Music test server on the host (10.0.2.2), plays a song, then
# minimises the app and turns the screen off, checking the app survives and
# keeps playing. Logs and screenshots go to android/e2e/out.
set -uo pipefail
cd "$(dirname "$0")"
out=out
mkdir -p "$out"
pkg=com.gundil.music
fail=0

shot() { adb exec-out screencap -p > "$out/$1.png" || true; }
alive() { adb shell pidof "$pkg" >/dev/null 2>&1; }
forward() {
  # Wait for the app's WebView debugging socket, then forward it to :9222.
  local pid sock i
  for i in $(seq 30); do
    pid=$(adb shell pidof "$pkg" | tr -d '\r')
    sock=$(adb shell cat /proc/net/unix | tr -d '\r' | grep -o "webview_devtools_remote_${pid:-x}" | head -1)
    [ -n "$sock" ] && break
    sleep 2
  done
  adb forward --remove-all >/dev/null 2>&1 || true
  [ -n "$sock" ] && adb forward tcp:9222 "localabstract:$sock" >/dev/null
}

adb install -r ../app/build/outputs/apk/debug/app-universal-debug.apk || adb install -r ../app/build/outputs/apk/debug/*.apk
adb shell pm grant "$pkg" android.permission.POST_NOTIFICATIONS 2>/dev/null || true
adb logcat -c
adb shell am start -n "$pkg/.MainActivity" --es server http://10.0.2.2:8338
sleep 15
shot 1-home
forward || echo "no devtools socket"

echo "== play a song"
node drive.js play || fail=1
sleep 3
shot 2-playing
alive && echo "app alive after play" || { echo "::error::app died after starting playback"; fail=1; }

echo "== minimise (home button)"
adb shell input keyevent KEYCODE_HOME
sleep 10
alive && echo "app alive while minimised" || { echo "::error::app died while minimised"; fail=1; }
forward; node drive.js state || true

echo "== screen off"
adb shell input keyevent KEYCODE_SLEEP
sleep 10
alive && echo "app alive with screen off" || { echo "::error::app died with screen off"; fail=1; }
forward; node drive.js state || true
adb shell input keyevent KEYCODE_WAKEUP
sleep 2
adb shell cmd statusbar expand-notifications 2>/dev/null || true
sleep 2
shot 3-notification

adb shell dumpsys activity services "$pkg" > "$out/services.txt" 2>&1 || true
adb logcat -d > "$out/logcat.txt"
echo "== crashes"
grep -n -A30 -E "FATAL EXCEPTION|AndroidRuntime: |ForegroundService|did not then call|RenderProcessGone|GundilMusic" "$out/logcat.txt" | head -150 || echo "none"
exit $fail
