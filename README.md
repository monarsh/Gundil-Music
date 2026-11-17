# Gundil Music

A YouTube Music style player you host yourself. One small server streams the
YouTube Music catalogue to a web app on your phone, laptop or desktop, and it
can play through its own speaker or a **Bluetooth speaker it pairs itself**.
That turns a headless Raspberry Pi or Orange Pi into a music box you control
from your phone.

What you get, laid out like YouTube Music:

- **Home**: mood chips, Quick picks, Listen again, albums, mixes and artists
- **Explore**: new releases, charts by country, moods & genres
- **Search** with suggestions, a top result, and Songs / Videos / Albums / Artists / Playlists filters
- **Album, artist and playlist pages** with Play, Shuffle, Mix and Save
- **Player** with Up next (autoplay radio keeps the music going), synced Lyrics and Related
- **Library**: playlists, Liked Music (thumbs up), saved albums, subscribed artists, history
- **Speakers & Bluetooth**: play on this device or on the server, scan, pair and connect speakers

Gundil Music is not affiliated with YouTube or Google. It uses YouTube Music's
unofficial web API (through [ytmusicapi](https://github.com/sigma67/ytmusicapi))
and [yt-dlp](https://github.com/yt-dlp/yt-dlp) for audio, without a Google
account. Using YouTube this way may be against YouTube's Terms of Service; it's
meant for personal use. Your likes, playlists and history are stored on your
server, not in a YouTube account.

## Platforms

Downloads are on the [releases page](https://github.com/monarsh/Gundil-Music/releases/latest).

| Where | Download | Bluetooth |
|---|---|---|
| Headless Raspberry Pi 3/4/5 | `linux-arm64` (64-bit OS) or `linux-armv7`, set up by `install.sh` | Pair and connect from the app; the Pi plays to the speaker |
| Headless Orange Pi PC | `linux-armv7` (Armbian), set up by `install.sh`. Needs a USB Bluetooth dongle | Same as the Pi |
| Linux desktop | `…-x86_64.AppImage` (everything bundled), or `linux-amd64` with your own Python | Pair and connect from the app, or use the system settings |
| Windows 10/11 | `…-windows-x86_64.zip` (everything bundled): unzip, run `gundil-music.exe` | Pair in Windows settings; audio follows |
| Android | `…-android-arm64.apk` (most phones), `-armv7.apk` (older 32-bit phones) or `-universal.apk`: the player app for your server | Pair in Android settings; notification, lock-screen and headset controls work |
| Android as the server | `termux-arm64` with `install-termux.sh`, in Termux | Pair in Android settings |

The Android app plays from a Gundil Music server on your network (a Pi, an
Orange Pi or a PC). It finds servers on the Wi-Fi by itself and keeps playing
with the screen off.

Pi 1 and Pi Zero (ARMv6) are not supported for YouTube playback: no JavaScript
runtime that yt-dlp accepts runs on them.

## What the server needs

- **Python 3** with `ytmusicapi` and `yt-dlp`:
  `pip install -U ytmusicapi "yt-dlp[default]"`
- **A JavaScript runtime** for yt-dlp to unlock YouTube streams: [Deno](https://deno.com) 2.3+
  (recommended) or Node.js 22+. Most Linux distributions package an older Node, so get it
  from the project's site.
- **[mpv](https://mpv.io)**, only if the server itself should play sound (headless boxes,
  Bluetooth speakers).
- **BlueZ** (`bluetoothctl`), only for pairing Bluetooth speakers from the app (Linux).

`install.sh` sets all of this up on a Pi, and the Windows zip and the AppImage
carry Python, the libraries and Deno with them. Elsewhere, install them yourself.
**About → YouTube Music** in the app (avatar menu) shows what the server found.

## Headless Raspberry Pi / Orange Pi

On Raspberry Pi OS or Armbian, as your normal user:

```sh
curl -fsSLO https://github.com/monarsh/Gundil-Music/releases/latest/download/install.sh
bash install.sh                        # downloads the latest release
# or: bash install.sh ./gundil-music-linux-armv7
```

It installs mpv, BlueZ, PipeWire, Python with ytmusicapi and yt-dlp, and Deno
(or Node on 32-bit boards). Then it sets up a service that starts at boot
without anyone logging in, and prints the address to open. From your phone:

1. Open the app and tap the cast button at the top.
2. Put your Bluetooth speaker in pairing mode and tap **Scan**.
3. Tap **Connect**. Playback moves to the board and comes out of the speaker.

The speaker is trusted, so it reconnects on its own after a reboot.

YouTube changes often. If songs stop playing, update the libraries:
`bash scripts/update-youtube.sh`.

## Other machines

```sh
pip install -U ytmusicapi "yt-dlp[default]"
./gundil-music
```

Open the printed address (default `http://<this-machine>:8338`).

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `-addr` | `GUNDIL_ADDR` | `:8338` | Listen address |
| `-data` | `GUNDIL_DATA` | user config dir | Where your library (likes, playlists, history) lives |
| `-python` | `GUNDIL_PYTHON` | `python3` in PATH | Python that has ytmusicapi and yt-dlp |
| `-mpv` | `GUNDIL_MPV` | `mpv` in PATH | mpv for playing on the server |
| `-auth` | `GUNDIL_AUTH` | none | Require a login, as `user:password` |
| `-open` | | on for Windows | Open the app in the browser after starting |
| | `GUNDIL_REGION` | | Country code for YouTube Music results, e.g. `US`, `IN` |
| | `GUNDIL_LANG` | `en` | Language for YouTube Music results |

Without `-auth`, anyone on your network can use the player and manage
Bluetooth. Don't expose the port to the internet.

## Where the sound comes out

The cast button switches between:

- **This device**: audio plays in the browser you're holding, and Bluetooth is whatever
  that phone or computer is connected to. Audio still flows through the server, which
  fetches it from YouTube.
- **The server**: audio plays from the machine running Gundil Music through mpv. The queue
  lives on the server, so it keeps playing when you close the app, and every phone sees and
  controls the same player.

## Build

Needs Go 1.22 or newer.

```sh
go build -o gundil-music .      # this machine
./scripts/build-all.sh          # server binaries for every platform, into dist/
./scripts/package-appimage.sh   # Linux AppImage with Python and Deno bundled (needs gh)
./scripts/package-windows.sh    # Windows zip with Python and Deno bundled (run on Windows)
gradle -p android assembleRelease   # Android APKs (needs the Android SDK and Gradle 8.14)
```

CI (`.github/workflows/build.yml`) builds and smoke-tests all of these. To
publish a release, add `docs/release-notes/<version>.md`, then push a
`<version>` tag or run the workflow by hand with that version. To keep Android
updates installable over each other, add the repository secrets
`ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS` and
`ANDROID_KEY_PASSWORD`. Without them, each release is signed with a new key.

The web app in `web/` is plain HTML, CSS and JavaScript, and the YouTube helper
(`internal/yt/helper.py`) is a Python script. Both are embedded in the binary.
There is no front-end build step.

## Layout

```
main.go               flags and wiring
internal/yt           Python helper (ytmusicapi + yt-dlp) and the Go client that runs it
internal/server       HTTP API, audio stream proxy, live updates, static files
internal/player       server-side playback through mpv, with autoplay
internal/bluetooth    pairing and connecting through bluetoothctl
internal/store        likes, playlists, saved items, history (one JSON file)
web/                  the app
android/              the Android app (Java, built with Gradle)
packaging/            AppImage and Windows zip extras
scripts/              installers, YouTube updater, build and packaging scripts
```
