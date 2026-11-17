Gundil Music for Windows
========================

Double-click gundil-music.exe. It opens Gundil Music in your browser at
http://localhost:8338. Keep the black window open while you listen; close it
to stop.

Other devices on your network (phones, the Android app) can connect to this
PC on port 8338. Windows may ask whether to allow that the first time.

Everything it needs is in this folder: Python with ytmusicapi and yt-dlp
(python), and Deno (runtime). YouTube changes often, so Gundil Music updates
yt-dlp and ytmusicapi by itself once a day. To update right away, use
About > "Update YouTube support" in the app, or run

    gundil-music.exe -update-youtube

To play through this PC's speakers from your phone, put mpv.exe next to
gundil-music.exe (from https://mpv.io).

Bluetooth speakers: pair them in Windows settings; sound follows.
