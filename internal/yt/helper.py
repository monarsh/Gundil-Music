"""Gundil Music YouTube helper.

The Go server runs this script and talks to it over stdin/stdout, one JSON
object per line:  {"id": 1, "method": "search", "args": {"q": "..."}}
and gets back     {"id": 1, "result": ...}  or  {"id": 1, "error": "..."}.

It uses ytmusicapi for the YouTube Music catalogue and yt-dlp to find audio
streams, and turns their output into a few small, stable shapes:

  track    {type: song|video, videoId, title, artists: [{name, id}], album: {name, id},
            duration (seconds), thumb, explicit, views, year}
  album    {type: album, id, title, subtitle, thumb, year, artists, kind, playlistId}
  artist   {type: artist, id, title, subtitle, thumb}
  playlist {type: playlist, id, title, subtitle, thumb}
  section  {title, items: [...]}
"""

import json
import os
import shutil
import sys
import threading
import traceback
from concurrent.futures import ThreadPoolExecutor
from urllib.parse import parse_qs, urlparse

MISSING = None
try:
    import yt_dlp
    from ytmusicapi import YTMusic
except ImportError as e:  # reported to the app instead of crashing
    MISSING = str(e)

LANG = os.environ.get("GUNDIL_LANG", "en")
COOKIES = os.environ.get("GUNDIL_COOKIES", "")  # cookies.txt for when YouTube asks to sign in
REGION = os.environ.get("GUNDIL_REGION", "")

_out_lock = threading.Lock()
_local = threading.local()


def yt():
    if not hasattr(_local, "yt"):
        kwargs = {"language": LANG}
        if REGION:
            kwargs["location"] = REGION
        _local.yt = YTMusic(**kwargs)
    return _local.yt


# ---------- normalising ----------

def thumb(x):
    t = x.get("thumbnails") or x.get("thumbnail") or []
    if isinstance(t, dict):
        t = t.get("thumbnails") or []
    if not isinstance(t, list) or not t:
        return ""
    return t[-1].get("url", "") if isinstance(t[-1], dict) else ""


def secs(v):
    if v is None or v == "":
        return 0
    if isinstance(v, (int, float)):
        return v
    n = 0
    try:
        for part in str(v).split(":"):
            n = n * 60 + int(part)
    except ValueError:
        return 0
    return n


def ref(name, id_=None):
    return {"name": name, "id": id_ or ""}


def artists(x):
    a = x.get("artists")
    if isinstance(a, list):
        return [ref(y.get("name", ""), y.get("id")) for y in a if isinstance(y, dict) and y.get("name")]
    for key in ("artist", "author"):
        v = x.get(key)
        if isinstance(v, str) and v:
            return [ref(v)]
        if isinstance(v, list):
            return [ref(y.get("name", ""), y.get("id")) for y in v if isinstance(y, dict) and y.get("name")]
        if isinstance(v, dict) and v.get("name"):
            return [ref(v["name"], v.get("id"))]
    return []


def album_ref(x):
    a = x.get("album")
    if isinstance(a, dict) and a.get("name"):
        return ref(a["name"], a.get("id"))
    if isinstance(a, str) and a:
        return ref(a)
    return None


VIDEO_TYPES = ("MUSIC_VIDEO_TYPE_OMV", "MUSIC_VIDEO_TYPE_UGC", "MUSIC_VIDEO_TYPE_OFFICIAL_SOURCE_MUSIC")


def is_podcast(x):
    return (x.get("videoType") or "").endswith("PODCAST_EPISODE") or x.get("resultType") in ("episode", "podcast")


def track(x, default_thumb="", album=None):
    if not isinstance(x, dict) or not x.get("videoId") or is_podcast(x):
        return None
    video = x.get("resultType") == "video" or (x.get("videoType") or "") in VIDEO_TYPES
    return {
        "type": "video" if video else "song",
        "videoId": x["videoId"],
        "title": x.get("title") or "",
        "artists": artists(x),
        "album": album or album_ref(x),
        "duration": secs(x.get("duration_seconds") or x.get("duration") or x.get("length")),
        "thumb": thumb(x) or default_thumb,
        "explicit": bool(x.get("isExplicit")),
        "views": x.get("views") or "",
        "year": str(x.get("year") or ""),
    }


def names(refs):
    return ", ".join(a["name"] for a in refs)


def join(*parts):
    return " • ".join(str(p) for p in parts if p)


def item(x):
    """Turns any card-like result into a track, album, artist or playlist."""
    if not isinstance(x, dict) or is_podcast(x):
        return None
    rt = x.get("resultType")
    if rt in ("profile", "upload"):
        return None
    if x.get("videoId") and rt not in ("album", "playlist", "artist"):
        return track(x)
    bid = x.get("browseId") or ""
    if rt == "album" or bid.startswith("MPRE"):
        a = artists(x)
        year = str(x.get("year") or "")
        kind = x.get("type") or "Album"
        return {
            "type": "album", "id": bid, "title": x.get("title") or "", "thumb": thumb(x),
            "year": year, "artists": a, "kind": kind, "playlistId": x.get("audioPlaylistId") or "",
            "subtitle": join(kind, names(a), year if year != names(a) else ""),
            "explicit": bool(x.get("isExplicit")),
        }
    if rt == "artist" or bid.startswith("UC") or ("subscribers" in x and bid and not x.get("playlistId")):
        subs = x.get("subscribers") or ""
        if subs and "subscriber" not in subs:
            subs += " subscribers"
        return {"type": "artist", "id": bid, "title": x.get("artist") or x.get("title") or "",
                "subtitle": subs or "Artist", "thumb": thumb(x)}
    pid = x.get("playlistId") or (bid[2:] if bid.startswith("VL") else "")
    if rt == "playlist" or pid:
        sub = x.get("description") or ""
        if not sub or len(sub) > 80:
            author = names(artists(x))
            count = x.get("itemCount") or x.get("count") or ""
            sub = join("Playlist", author, f"{count} songs" if str(count).isdigit() else count)
        return {"type": "playlist", "id": pid or bid, "title": x.get("title") or "", "subtitle": sub, "thumb": thumb(x)}
    return None


def items(lst):
    out = []
    for x in lst or []:
        it = item(x)
        if it:
            out.append(it)
    return out


def sections(raw):
    out = []
    for s in raw or []:
        if not isinstance(s, dict):
            continue
        contents = s.get("contents")
        if isinstance(contents, str):
            out.append({"title": s.get("title") or "", "text": contents, "items": []})
            continue
        its = items(contents)
        if its:
            out.append({"title": s.get("title") or "", "items": its})
    return out


def strip_vl(pid):
    return pid[2:] if isinstance(pid, str) and pid.startswith("VL") else (pid or "")


# ---------- methods ----------

def js_runtimes():
    """JavaScript runtimes yt-dlp can use (it needs one to unlock YouTube streams)."""
    try:
        from yt_dlp.utils import _jsruntime as jr
        found = []
        for cls in (jr.DenoJsRuntime, jr.NodeJsRuntime, jr.BunJsRuntime, jr.QuickJsRuntime):
            info = cls().info
            if info and info.supported:
                found.append(f"{info.name} {info.version}")
        return found
    except Exception:  # older or newer yt-dlp: fall back to looking in PATH
        return [n for n in ("deno", "node", "bun", "qjs") if shutil.which(n)]


def m_ping(args):
    return {
        "ok": MISSING is None,
        "ytmusicapi": getattr(sys.modules.get("ytmusicapi"), "__version__", ""),
        "yt_dlp": getattr(getattr(sys.modules.get("yt_dlp"), "version", None), "__version__", ""),
        "js": js_runtimes() if MISSING is None else [],
        "python": sys.version.split()[0],
    }


def m_home(args):
    return {"sections": sections(yt().get_home(limit=int(args.get("limit") or 10)))}


def m_moods(args):
    raw = yt().get_mood_categories()
    return {"sections": [{"title": title, "items": [{"title": c.get("title", ""), "params": c.get("params", "")} for c in cats]}
                         for title, cats in raw.items()]}


def m_mood(args):
    return {"items": items(yt().get_mood_playlists(args["params"]))}


def m_explore(args):
    raw = yt().get_explore()
    out = {"sections": []}
    if raw.get("new_releases"):
        out["sections"].append({"title": "New albums & singles", "items": items(raw["new_releases"])})
    for key, title in (("top_songs", "Top songs"), ("trending", "Trending")):
        block = raw.get(key)
        if isinstance(block, dict) and block.get("items"):
            out["sections"].append({"title": title, "items": items(block["items"]),
                                    "playlistId": strip_vl(block.get("playlist"))})
    if raw.get("new_videos"):
        out["sections"].append({"title": "New music videos", "items": items(raw["new_videos"])})
    out["moods"] = [{"title": m.get("title", ""), "params": m.get("params", "")} for m in raw.get("moods_and_genres") or []]
    return out


def m_charts(args):
    raw = yt().get_charts(country=args.get("country") or "ZZ")
    out = {"sections": [], "countries": (raw.get("countries") or {}).get("options") or [],
           "country": ((raw.get("countries") or {}).get("selected") or {}).get("text", "")}
    for key, title in (("songs", "Top songs"), ("videos", "Video charts"), ("trending", "Trending"),
                       ("genres", "Top genres"), ("languages", "Languages")):
        block = raw.get(key)
        its = items(block.get("items") if isinstance(block, dict) else block)
        if its:
            out["sections"].append({"title": title, "items": its})
    artists_ = items(raw.get("artists") if not isinstance(raw.get("artists"), dict) else raw["artists"].get("items"))
    if artists_:
        out["sections"].append({"title": "Top artists", "items": artists_, "ranked": True})
    return out


def m_search(args):
    q = (args.get("q") or "").strip()
    if not q:
        return {"sections": []}
    flt = args.get("filter") or None
    raw = yt().search(q, filter=flt, limit=40 if flt else 20)
    if flt:
        return {"items": items(raw)}
    order, groups = [], {}
    for r in raw:
        it = item(r)
        if not it:
            continue
        cat = r.get("category") or "Results"
        if cat not in groups:
            groups[cat] = []
            order.append(cat)
        groups[cat].append(it)
    return {"sections": [{"title": c, "items": groups[c]} for c in order]}


def m_suggest(args):
    q = (args.get("q") or "").strip()
    return {"items": yt().get_search_suggestions(q) if q else []}


def m_album(args):
    a = yt().get_album(args["id"])
    art = thumb(a)
    me = ref(a.get("title") or "", args["id"])
    tracks = []
    for t in a.get("tracks") or []:
        tr = track(t, art, me)
        if tr:
            tr["number"] = t.get("trackNumber") or 0
            tr["available"] = t.get("isAvailable", True) is not False
            tr["plays"] = t.get("views") or ""
            tracks.append(tr)
    return {
        "id": args["id"], "title": a.get("title") or "", "kind": a.get("type") or "Album",
        "artists": artists(a), "year": str(a.get("year") or ""), "thumb": art,
        "description": a.get("description") or "", "trackCount": a.get("trackCount") or len(tracks),
        "duration": a.get("duration") or "", "playlistId": a.get("audioPlaylistId") or "",
        "explicit": bool(a.get("isExplicit")), "tracks": tracks, "other": items(a.get("other_versions")),
    }


def m_artist(args):
    a = yt().get_artist(args["id"])
    songs = a.get("songs") or {}
    out = {
        "id": args["id"], "name": a.get("name") or "", "description": a.get("description") or "",
        "subscribers": a.get("subscribers") or "", "views": a.get("views") or "",
        "monthly": a.get("monthlyListeners") or "", "thumb": thumb(a),
        "shuffleId": a.get("shuffleId") or "", "radioId": a.get("radioId") or "",
        "songsPlaylist": strip_vl(songs.get("browseId")),
        "songs": [t for t in (track(s) for s in songs.get("results") or []) if t],
        "sections": [],
    }
    for key, title in (("albums", "Albums"), ("singles", "Singles & EPs"), ("videos", "Videos"),
                       ("playlists", "Featured on"), ("related", "Fans might also like")):
        block = a.get(key)
        if isinstance(block, dict):
            its = items(block.get("results"))
            if key == "videos":
                for v in its:
                    v["type"] = "video"
            if its:
                out["sections"].append({"title": title, "items": its,
                                        "playlistId": strip_vl(block.get("browseId")) if key == "videos" else ""})
    return out


def m_playlist(args):
    p = yt().get_playlist(strip_vl(args["id"]), limit=int(args.get("limit") or 200))
    tracks = [t for t in (track(x) for x in p.get("tracks") or []) if t]
    author = p.get("author")
    if isinstance(author, dict):
        author = author.get("name") or ""
    elif isinstance(author, list):
        author = names(artists({"author": author}))
    return {
        "id": strip_vl(args["id"]), "title": p.get("title") or "", "author": author or "",
        "description": p.get("description") or "", "thumb": thumb(p), "year": str(p.get("year") or ""),
        "trackCount": p.get("trackCount") or len(tracks), "duration": p.get("duration") or "",
        "views": p.get("views") or "", "tracks": tracks,
    }


def m_watch(args):
    radio = str(args.get("radio") or "").lower() in ("1", "true", "yes")
    shuffle = str(args.get("shuffle") or "").lower() in ("1", "true", "yes")
    kw = {"limit": int(args.get("limit") or 25), "radio": radio}
    if args.get("videoId"):
        kw["videoId"] = args["videoId"]
    if args.get("playlistId"):
        kw["playlistId"] = strip_vl(args["playlistId"])
        if shuffle and not radio:
            kw["shuffle"] = True
    w = yt().get_watch_playlist(**kw)
    return {
        "tracks": [t for t in (track(x) for x in w.get("tracks") or []) if t],
        "playlistId": w.get("playlistId") or "", "lyrics": w.get("lyrics") or "", "related": w.get("related") or "",
    }


def m_lyrics(args):
    try:
        lyr = yt().get_lyrics(args["id"], timestamps=True)
    except Exception:
        lyr = yt().get_lyrics(args["id"])
    if not lyr:
        return {"text": "", "lines": None, "source": ""}
    if lyr.get("hasTimestamps"):
        lines = [{"text": l.text, "start": l.start_time / 1000, "end": l.end_time / 1000} for l in lyr["lyrics"]]
        return {"text": "\n".join(l["text"] for l in lines), "lines": lines, "source": lyr.get("source") or ""}
    return {"text": lyr.get("lyrics") or "", "lines": None, "source": lyr.get("source") or ""}


def m_related(args):
    return {"sections": sections(yt().get_song_related(args["id"]))}


_ydl = None
_ydl_lock = threading.Lock()


def m_stream(args):
    global _ydl
    vid = args["videoId"]
    with _ydl_lock:
        if _ydl is None:
            _ydl = yt_dlp.YoutubeDL({
                "quiet": True, "no_warnings": True, "noplaylist": True, "skip_download": True,
                "format": "bestaudio[ext=m4a]/bestaudio[acodec^=mp4a]/bestaudio/best",
                "js_runtimes": {"deno": {}, "node": {}, "bun": {}, "quickjs": {}},
                **({"cookiefile": COOKIES} if COOKIES else {}),
            })
        ydl = _ydl
    info = ydl.extract_info("https://www.youtube.com/watch?v=" + vid, download=False)
    url = info.get("url")
    if not url and info.get("requested_formats"):
        url = info["requested_formats"][0].get("url")
    if not url:
        raise RuntimeError("no playable audio stream found")
    ext = info.get("ext") or ""
    mime = {"m4a": "audio/mp4", "mp4": "audio/mp4", "webm": "audio/webm", "mp3": "audio/mpeg", "opus": "audio/ogg"}.get(ext, "")
    expire = parse_qs(urlparse(url).query).get("expire", ["0"])[0]
    return {
        "url": url, "headers": info.get("http_headers") or {}, "mime": mime, "ext": ext,
        "expires": int(expire) if expire.isdigit() else 0, "duration": info.get("duration") or 0,
        "size": info.get("filesize") or info.get("filesize_approx") or 0,
    }


METHODS = {name[2:]: fn for name, fn in globals().items() if name.startswith("m_")}


def send(obj):
    data = json.dumps(obj, ensure_ascii=False, default=str)
    with _out_lock:
        sys.stdout.write(data + "\n")
        sys.stdout.flush()


def handle(req):
    rid = req.get("id")
    method = req.get("method")
    try:
        if method != "ping" and MISSING:
            raise RuntimeError(f"Python packages are missing ({MISSING}). Run: pip install -U ytmusicapi \"yt-dlp[default]\"")
        fn = METHODS.get(method)
        if not fn:
            raise RuntimeError(f"unknown method {method}")
        send({"id": rid, "result": fn(req.get("args") or {})})
    except Exception as e:  # noqa: BLE001 - every failure goes back to the caller
        msg = str(e) or e.__class__.__name__
        if "yt_dlp" in sys.modules and isinstance(e, getattr(yt_dlp.utils, "DownloadError", ())):
            msg = msg.replace("ERROR: ", "")
            if "confirm you" in msg and "not a bot" in msg:
                msg = ("YouTube wants this server to sign in before it plays songs (this happens on some networks). "
                       "Export your YouTube cookies to a cookies.txt file and start Gundil Music with -cookies <file>. "
                       "See the README.")
        traceback.print_exc(file=sys.stderr)
        send({"id": rid, "error": msg[:500]})


def main():
    pool = ThreadPoolExecutor(max_workers=8)
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except ValueError:
            continue
        pool.submit(handle, req)


if __name__ == "__main__":
    main()
