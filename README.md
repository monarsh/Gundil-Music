# Gundil Music

A self-hosted music streaming app. One small program indexes your music
folders and serves a streaming-style web app (home, search, albums, artists,
playlists, likes, queue) to every device on your network. It can also play
through the speakers of the machine it runs on, including a Bluetooth speaker
it pairs itself, so a headless Raspberry Pi or Orange Pi becomes a music box
you control from your phone.

It plays **your own audio files** (MP3, FLAC, M4A, OGG, Opus, WAV and more).
There is no online catalogue.

## Platforms

| Where | How it runs | Bluetooth |
|---|---|---|
| Linux desktop | `gundil-music-linux-amd64`, open the web app | Pair and connect from the app (BlueZ) |
| Headless Raspberry Pi | `linux-arm64` (64-bit OS), `linux-armv7`, `linux-armv6` (Pi 1/Zero) | Pair and connect from the app; the Pi plays to the speaker |
| Headless Orange Pi PC | `linux-armv7` (Armbian) | Same as the Pi. Needs a USB Bluetooth dongle, the board has none |
| Windows | `gundil-music-windows-amd64.exe`, open the web app | Pair in Windows settings; audio follows |
| Android | Open the web app in Chrome and "Add to Home screen". Or run the server on the phone in Termux (`android-arm64`) | Pair in Android settings; lock-screen and headset buttons work |

## Quick start

Download a binary from Releases (or build one, below), then:

```sh
./gundil-music -music ~/Music
```

Open the printed address (default `http://<this-machine>:8338`) on any device.

| Flag | Environment | Default | Meaning |
|---|---|---|---|
| `-music` | `GUNDIL_MUSIC` | `~/Music` | Music folder(s). Separate several with `:` (`;` on Windows) |
| `-addr` | `GUNDIL_ADDR` | `:8338` | Listen address |
| `-data` | `GUNDIL_DATA` | user config dir | Where playlists, likes and the library cache live |
| `-auth` | `GUNDIL_AUTH` | none | Require a login, as `user:password` |
| `-mpv` | `GUNDIL_MPV` | found in `PATH` | Path to mpv |

Without `-auth`, anyone on your network can play music and manage Bluetooth.
Do not expose the port to the internet.

## Headless Raspberry Pi / Orange Pi

On Raspberry Pi OS or Armbian, as your normal user:

```sh
curl -fsSLO https://raw.githubusercontent.com/monarsh/Gundil-Music/main/scripts/install.sh
bash install.sh                      # downloads the latest release
# or: bash install.sh ./gundil-music-linux-armv7 /mnt/usb/music
```

The script installs mpv, BlueZ and PipeWire, sets up a service that starts at
boot without anyone logging in, and prints the address to open. Then, from
your phone:

1. Open the app and tap the speaker button.
2. Put your Bluetooth speaker in pairing mode and tap **Scan for devices**.
3. Tap **Connect**. Playback switches to the board and comes out of the speaker.

The speaker is trusted, so it reconnects on its own after a reboot.

## Where the sound comes out

The speaker button switches between two outputs:

- **This device**: audio plays in the browser you are holding. Bluetooth is
  whatever that phone or computer is connected to.
- **The server**: audio plays from the machine running Gundil Music (needs
  [mpv](https://mpv.io) installed there). The queue lives on the server, so it
  keeps playing when you close the app, and every device sees the same player.

On Windows, install mpv and put it in `PATH` (or pass `-mpv`) to use the
second mode.

## Build

Needs Go 1.22 or newer.

```sh
go build -o gundil-music .      # this machine
./scripts/build-all.sh          # every platform, into dist/
```

The web app in `web/` is plain HTML, CSS and JavaScript embedded into the
binary. There is no front-end build step.

## Layout

```
main.go               flags and wiring
internal/library      folder scan, tags, cover art
internal/store        playlists, likes, play history (JSON file)
internal/player       server-side playback through mpv
internal/bluetooth    pairing and connecting through bluetoothctl
internal/server       HTTP API, live updates, static files
web/                  the app
scripts/              installer and cross-compile script
```
