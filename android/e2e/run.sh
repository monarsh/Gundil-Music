#!/usr/bin/env bash
# Runs inside an Android emulator job: installs the debug APK, connects it to
# a Gundil Music test server on the host (10.0.2.2), plays a song, then
# minimises the app, turns the screen off and swipes it away from Recents,
# checking it keeps playing throughout. Then it downloads a song, stops the
# server, and checks the app opens offline and plays the download. Logs and screenshots go to android/e2e/out.
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

echo "== swipe the app away from Recents (music must keep playing)"
adb shell input keyevent KEYCODE_BACK # close the notification shade
adb shell am start -n "$pkg/.MainActivity"
sleep 5
forward; node drive.js check || { echo "::error::not playing before the swipe"; fail=1; }
adb shell input keyevent KEYCODE_APP_SWITCH
sleep 4
shot 4-recents
size=$(adb shell wm size | tr -d '\r' | grep -o '[0-9]*x[0-9]*' | tail -1)
w=${size%x*}; h=${size#*x}
adb shell input swipe $((w / 2)) $((h / 2)) $((w / 2)) $((h / 12)) 300
sleep 4
adb shell input keyevent KEYCODE_HOME
sleep 4
if adb shell dumpsys activity activities | tr -d '\r' | grep -q "$pkg/.MainActivity"; then
  echo "::warning::the swipe didn't remove the app from Recents; the test can't check this step"
  adb shell dumpsys activity recents > "$out/recents.txt" 2>&1 || true
else
  echo "app's screen is gone (swiped away)"
  alive && echo "app process alive after the swipe" || { echo "::error::the app was killed when swiped away"; fail=1; }
  forward; node drive.js check && echo "still playing after the swipe" || { echo "::error::music stopped when the app was swiped away"; fail=1; }
  shot 5-after-swipe
  echo "== open the app again: the same page, still playing"
  adb shell am start -n "$pkg/.MainActivity"
  sleep 5
  shot 6-reopened
  forward; node drive.js check && echo "reopened on the playing page" || { echo "::error::reopening the app lost the music"; fail=1; }
fi

echo "== turn the phone sideways: full-screen video controls must fit the screen"
adb shell settings put system accelerometer_rotation 0
adb shell settings put system user_rotation 1
sleep 5
forward; node drive.js fullscreen || echo "::warning::the full-screen video controls don't fit the screen sideways"
shot 8-fullscreen-landscape
adb shell settings put system user_rotation 0
sleep 4

echo "== download a song in the app, then play it with the server off"
forward; node drive.js download || { echo "::error::downloading a song failed in the app"; fail=1; }
pkill -x gundil-music && echo "test server stopped"
sleep 2
forward; node drive.js offline || { echo "::error::the app didn't play downloads without the server"; fail=1; }
shot 7-offline

adb shell dumpsys activity services "$pkg" > "$out/services.txt" 2>&1 || true
adb logcat -d > "$out/logcat.txt"
echo "== crashes"
grep -n -A30 -E "FATAL EXCEPTION|AndroidRuntime: |ForegroundService|did not then call|RenderProcessGone|GundilMusic" "$out/logcat.txt" | head -150 || echo "none"
exit $fail
