"""Stand-in for the YouTube helper, for testing the Android app in an
emulator: same protocol and shapes, a small made-up catalogue, and audio
served from a local folder (FAKE_AUDIO_BASE)."""
import hashlib, json, os, sys, threading, time

BASE = os.environ.get("FAKE_AUDIO_BASE", "http://127.0.0.1:8339")
LOCK = threading.Lock()


def vid(s):
    return "v" + hashlib.sha1(s.encode()).hexdigest()[:10]


ARTISTS = ["Lunar Tide", "Marigold Avenue", "The Copper Wires", "Nadia Okoro"]
WORDS = "Harbor Lights Undertow Glass Ocean Night Ferry Radio Origami Paper Moons Kite Weather Porch Swing Golden Hour".split()
SONGS = []
for i in range(24):
    a = ARTISTS[i % len(ARTISTS)]
    t = f"{WORDS[i % len(WORDS)]} {WORDS[(i * 7 + 3) % len(WORDS)]}"
    SONGS.append({"type": "song", "videoId": vid(t + str(i)), "title": t, "artists": [{"name": a, "id": "UC" + hashlib.sha1(a.encode()).hexdigest()[:22]}],
                  "album": {"name": "Test Album", "id": "MPREb_test0001"}, "duration": 20, "thumb": "", "explicit": False, "views": "", "year": "2025", "n": i % 4})
BY_ID = {s["videoId"]: s for s in SONGS}


def pub(s):
    return {k: v for k, v in s.items() if k != "n"}


M = {
    "ping": lambda a: {"ok": True, "ytmusicapi": "fake", "yt_dlp": "fake", "js": ["fake"], "python": "3"},
    "home": lambda a: {"sections": [{"title": "Quick picks", "items": [pub(s) for s in SONGS[:16]]}]},
    "moods": lambda a: {"sections": []},
    "watch": lambda a: {"tracks": [pub(BY_ID[a["videoId"]])] * (a.get("videoId") in BY_ID) + [pub(s) for s in SONGS if s["videoId"] != a.get("videoId")][:12],
                        "playlistId": "", "lyrics": "", "related": ""},
    "stream": lambda a: {"url": f"{BASE}/{BY_ID[a['videoId']]['n']}.wav", "headers": {}, "mime": "audio/wav",
                         "expires": int(time.time()) + 3600, "duration": 20, "size": 0},
}


def handle(req):
    try:
        out = {"id": req["id"], "result": M[req["method"]](req.get("args") or {})}
    except Exception as e:  # noqa: BLE001
        out = {"id": req["id"], "error": f"{type(e).__name__}: {e}"}
    with LOCK:
        sys.stdout.write(json.dumps(out) + "\n")
        sys.stdout.flush()


for line in sys.stdin:
    if line.strip():
        threading.Thread(target=handle, args=(json.loads(line),)).start()
