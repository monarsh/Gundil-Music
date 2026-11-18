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
- **Player** with Up next (autoplay radio keeps the music going), Lyrics that follow the song (with a full-screen view) and Related
- **Song / Video switch**: watch the music video in step with the song, full screen if you like
- **Drag to reorder** Up next and your playlists, and a **sleep timer**
- **Settings**: audio and video quality, region and language, daily YouTube updates, clearing history
- **Library**: playlists, Liked Music (thumbs up), saved albums, subscribed artists, history
- **Import playlists** from YouTube Music or YouTube by link, or your private ones and Liked Music from your linked account
- **Accounts**: optional sign-in, so each person gets their own library, playlists and history
- **Speakers & Bluetooth**: play on this device or on the server, scan, pair and connect speakers
- **Background play**: keeps playing when the app is minimised, swiped away from Recents or the screen is off, for free
- **Downloads**: keep songs, albums and playlists on your phone or computer and play them with no connection
- **Crossfade** between songs (2–12 seconds, per device)
- **Server or client**: every Windows, Linux desktop, AppImage and Pi build can be the server or connect to another Gundil Music server

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

YouTube changes often; the server updates yt-dlp by itself daily (see
"Keeping YouTube working" below).

## Closing and quitting

On Windows and the Linux desktop (AppImage), Gundil Music runs in the
background and shows its window in your browser. **Closing the browser window
doesn't stop it**, so phones and other devices can keep using it, and music
playing on this computer's speaker carries on.

- **Starting Gundil Music again** while it's running just opens its window again.
- **To stop it**, use the avatar menu → **Quit Gundil Music**. It's shown only on
  the computer running it; other devices can't quit it.

## Start-menu shortcut and icon

The AppImage carries its name and icon for start-menu integration
(AppImageLauncher, Gear Lever and the like): it shows as **Gundil Music** with
the Gundil Music icon. If an older shortcut shows a version in its name or no
icon, remove it and integrate the new AppImage again. On Windows,
`gundil-music.exe` has the icon too, so a shortcut you make to it (right-click →
Send to → Desktop, or Pin to Start) shows it.

## Server or client

Every build except the Android app can work two ways:

- **Server** (the default): it fetches YouTube Music and keeps your library.
  Phones and other computers connect to it.
- **Client**: it uses another Gundil Music server on your network, for example a
  PC that plays from the Raspberry Pi in the living room. Your library, likes and
  playlists are the server's, and nothing is fetched from YouTube on the client.
  It needs no Python. In the cast menu you can play on the computer you're
  using, on the server's speaker, or on its Bluetooth speaker.

To switch, open **Settings → This computer → Change** in the app, or go to
`http://localhost:8338/_gundil/`. Choose **Connect to another server**, then either
type the server's address or pick it from **Find servers on this network**.
Command line, for headless boxes:

```sh
gundil-music -connect 192.168.1.20      # become a client of that server (remembered)
gundil-music -mode server               # back to being the server
```

The choice is saved in `<data>/mode.json`. Only the computer itself can change
it. If the server has a password (`-auth`), anyone who knows it can change it
from another device too. If the server can't be reached, the app shows the
connect page with the error so you can pick another one.

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
| `-connect` | `GUNDIL_CONNECT` | | Be a client of the Gundil Music server at this address (remembered) |
| `-mode` | `GUNDIL_MODE` | `server` | `server` or `client` (remembered; also in Settings) |
| `-open` | | on for Windows | Open the app in the browser after starting |
| `-auto-update` | `GUNDIL_AUTO_UPDATE` | `true` | Update yt-dlp and ytmusicapi daily when newer versions exist |
| `-update-youtube` | | | Update yt-dlp and ytmusicapi now, then exit |
| `-diagnose-youtube [video id]` | | | Try every way of getting streams from YouTube on this network, print what works, then exit |
| `-reset-password` | | | Set an account's password, as `name:newpassword`, then exit |
| `-cookies` | `GUNDIL_COOKIES` | `<data>/cookies.txt` if present | YouTube cookies, for when YouTube asks the server to sign in |
| | `GUNDIL_REGION` | | Country code for YouTube Music results, e.g. `US`, `IN` |
| | `GUNDIL_LANG` | `en` | Language for YouTube Music results |

Region, language, the server's audio quality and the daily update can also be
changed in the app under **Settings**. They're saved in `<data>/settings.json`
and take priority over these options.

## Keeping YouTube working

YouTube changes often, and yt-dlp releases fixes quickly. Gundil Music keeps
up by itself: once a day the server checks PyPI and, when newer yt-dlp or
ytmusicapi versions exist, installs them into `<data folder>/pylib`. That
folder takes priority over the bundled copies, so this works for the
AppImage, the Windows zip, a Pi and Termux alike. To update right away:

- in the app: avatar menu → **Settings** → **Update** (or **About** → **Update YouTube support**)
- AppImage: `./Gundil_Music-…-x86_64.AppImage -update-youtube`
- Windows: `gundil-music.exe -update-youtube`
- Pi / Orange Pi: `bash update-youtube.sh` (updates, then restarts the service)
- anywhere else: `gundil-music -update-youtube`

After the daily update or the button, nothing needs restarting or reloading.
The server switches to a fresh YouTube helper straight away and lets the old one
finish whatever it was fetching, so songs that are playing carry on. Open apps
are told about it: they show the new versions, reload a page that had failed,
and retry a song that couldn't be played. After `-update-youtube` or
`update-youtube.sh` from the command line, restart the server (the script does
this for you). Turn the daily check off in Settings or with `-auto-update=false`.

### Songs won't play: "YouTube refused the stream" (403)

Run the diagnosis on the server's computer, with Gundil Music quit:

- AppImage: `./Gundil_Music-…-x86_64.AppImage -diagnose-youtube`
- Windows: `gundil-music.exe -diagnose-youtube`
- anywhere else: `gundil-music -diagnose-youtube`

It looks one video up as each YouTube player, over IPv4 and IPv6 (and with a
linked account, if there is one), tries a little of each kind of stream, and
ends with what works on this network. Add a video id to try another video.

### "Sign in to confirm you're not a bot"

Sometimes YouTube challenges a server's network before it plays songs. This is
common on cloud servers and VPNs and rare at home. Gundil Music handles it as far
as it can, by itself:

1. It first asks YouTube again, as other YouTube players that are challenged less often.
2. If that fails too, it plays the song signed in with a **linked YouTube Music account**
   (see below): the account of whoever is playing, then an admin's.

Once the check has come up, the server remembers it for an hour: songs (and
the next song, looked up ahead of time) go straight to the linked account, so
they start in the usual time instead of failing the other ways first. After the
hour it tries without signing in again, and stops using the account as soon as
YouTube lets it.

When the check is on and nobody's account is linked, or YouTube has signed the
linked account out, the app tells you once, with a **Link account** button.
**About** shows whether YouTube is asking right now.

So the one-time fix is to link an account in **Settings → YouTube Music account**.
No app can switch this check off for good; it's YouTube's decision. A
server-wide `cookies.txt` in the data folder (or `-cookies <file>`) still works
too, and is used for every song.

## Accounts

Out of the box everyone using a server shares one library and nobody signs in.
To give each person their own playlists, likes, saved albums, history and searches:

1. Open **Settings → Accounts → Set up accounts** and choose your name and password.
   This first account is an **admin**, and keeps the library used so far.
2. Add an account for each person under **Settings → Accounts → Add**.
3. Everyone else signs in on their phone or computer. The Android app and
   browsers stay signed in for a year.

**Letting people create their own account.** Turn on **Settings → Accounts → Let
people create their own account** (Settings is in the avatar menu, top right; the
Accounts section is near the bottom), and the sign-in screen gets a **Create an
account** link. If accounts aren't set up yet, turning it on first asks for your
own name and password, which becomes the admin account. With an **Invite code** set there, only people who know it can
sign up (it isn't case-sensitive); leave it empty and anyone who can reach the
server can. Accounts made this way are never admins. No approval is needed:
they can use Gundil Music right away.

Admins add and delete accounts, reset passwords, make other admins, and change
the server's settings (region, language, speaker quality, YouTube updates).
Everyone can change their own password. Playing on the server's speaker or
Bluetooth is shared: songs played there go into the history of whoever started
them.

Forgot the admin password? On the server run
`gundil-music -reset-password "name:new password"`.

Accounts are stored in `<data>/users.json` (passwords are hashed), and each
person's files in `<data>/users/`.

Google sign-in ("Sign in with Google") can't be used for Gundil Music accounts.
Google only allows it for apps on a public `https://` web address registered
with Google, and a home server at `http://192.168.x.x:8338` can't be. Gundil
Music accounts use a name and password instead.

## Importing playlists

- **By link**: paste a YouTube Music or YouTube playlist or album link into
  Search, or use **Library → Import playlist**. Every song (no 200-song limit) is
  copied into a playlist of your own that you can reorder and edit. Unlisted
  playlists work too.
- **From a YouTube or YouTube Music page in the app**: ⋮ → **Import as my playlist**.
- **Your private playlists and Liked Music**: link your YouTube Music account
  first. **Import playlist** then lists your YouTube Music library, one tap each.

## Linking your YouTube Music account

Gundil Music signs in to YouTube Music with your browser's cookies (it can't
use Google's sign-in page, as above). In **Settings → YouTube Music account → Link**:

1. On a computer, open a **private (incognito) window** and sign in at music.youtube.com.
2. Export its cookies in Netscape format with a "cookies.txt" browser extension,
   such as *Get cookies.txt LOCALLY*.
3. Close the private window **without signing out**, so YouTube doesn't replace
   those cookies.
4. Choose the file in the Link dialog, or paste its text.

Each account links its own YouTube Music account. The cookies are kept on the
server (`google-cookies.txt` in that person's folder, readable only by the
server's user) and give access to that Google account, so link only on a server
you trust. **Unlink** removes them. If YouTube signs the cookies out later,
Settings says so: link again with a fresh export.

## Lyrics

The Lyrics tab follows the song when timings are known: the current line lights
up, and the lyrics scroll to keep it in the middle. Dots fill in during the intro
and instrumental breaks, and tapping a line jumps there. Scroll by hand to read
ahead: following pauses until you tap **Back to current line**, or for a few seconds.
The ⛶ button shows them full screen.

Timings come from YouTube Music. When it only has plain lyrics, or none, the
server asks [LRCLIB](https://lrclib.net), a free open database of timed lyrics. It
only uses lyrics for a song of the same length (within 3 seconds), so a live or
extended version's timings aren't used for the original. That lookup sends the
song's title, artist, album and length to lrclib.net, nothing else.

## Downloads and offline

Download a song with ⋮ → **Download**, or a whole album or playlist with the
download button next to Play. Songs are kept on the device you're using, inside
the app (or the browser on a computer), at that device's audio quality, with a ✓
next to them. Find them under **Library → Downloads**, where you can also remove
them. They play from the device even when the server can be reached.

Away from home, or with the server off, the app still opens, goes to your
Downloads and plays them. It offers to reload when the server can be reached
again. Each device has its own downloads. Settings shows how much space they use.

In a computer's browser, opening the app with the server off needs the page to
come from `localhost` or `https://` (browsers only keep a page for offline use
there). Over plain `http://` on your network, downloads still play while the
page is open. The Android app always opens offline.

## Touch gestures (phones)

- **Mini player:** swipe up to open the player; swipe left or right for the next
  or previous song. A tap still opens it.
- **Player:** swipe down to close it; swipe the cover left or right to change song.
- **Up next / Lyrics / Related:** swipe the tabs up to open the panel, and down
  (or pull down from the top of the list) to close it.
- **Menus** that open from the bottom: pull down to close.
- **Any page:** pull down at the top to load it again.

## When YouTube is slow or fails

The server keeps the pages it fetched (Home, Explore, albums, artists,
playlists, searches, lyrics), also on disk across restarts:

- A page fetched before shows at once, even when it's a bit old, while a fresh
  copy is fetched behind the scenes for next time.
- When YouTube doesn't answer or a connection drops, the server asks again
  (up to three times, a few seconds apart), and the app asks the server again
  once. If YouTube still fails, a copy up to a week old is shown instead of an
  error.
- Home and Explore are fetched when the server starts, so they're ready.
- A page that hangs is given up after 30 seconds and asked again, rather than
  waiting a minute and a half.

Songs that won't play are a different problem: usually YouTube's sign-in check
(see above) or an out-of-date yt-dlp (**Update YouTube support** in About).

## Crossfade

**Settings → Crossfade** fades each song into the next, over 2 to 12 seconds
(off by default). It's set per device, for music playing on that device. It
isn't used in Video mode, with repeat one, or when the server's speaker plays.

## Security

Without `-auth` or accounts, anyone on your network can use the player and
manage Bluetooth. Don't expose the port to the internet: it is plain HTTP.

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
internal/settings     server settings chosen in the app
web/                  the app
android/              the Android app (Java, built with Gradle)
packaging/            AppImage and Windows zip extras
scripts/              installers, YouTube updater, build and packaging scripts
```
