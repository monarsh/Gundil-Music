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

# Packages updated by Gundil Music ("Update YouTube support", daily auto-update)
# live in a writable folder and take priority over the bundled or system ones.
PYLIB = os.environ.get("GUNDIL_PYLIB", "")
if PYLIB and os.path.isdir(PYLIB):
    sys.path.insert(0, PYLIB)

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


_auth_lock = threading.Lock()
_auth = {}


def yt_auth(path):
    """YouTube Music signed in with a cookies.txt export of the account."""
    import http.cookiejar
    key = (path, os.path.getmtime(path), threading.get_ident())
    with _auth_lock:
        if key in _auth:
            return _auth[key]
    jar = http.cookiejar.MozillaCookieJar(path)
    jar.load(ignore_discard=True, ignore_expires=True)
    cookies = {}
    for c in jar:
        if c.domain.lstrip(".").endswith("youtube.com"):
            cookies[c.name] = c.value
    if "__Secure-3PAPISID" not in cookies:
        raise RuntimeError("these cookies are not signed in to YouTube")
    headers = {
        "cookie": "; ".join(f"{k}={v}" for k, v in cookies.items()),
        "authorization": "SAPISIDHASH 0_0",  # recomputed by ytmusicapi for each request
        "x-goog-authuser": "0",
        "origin": "https://music.youtube.com",
        "x-origin": "https://music.youtube.com",
        "user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
        "accept": "*/*",
        "content-type": "application/json",
    }
    kwargs = {"language": LANG}
    if REGION:
        kwargs["location"] = REGION
    api = YTMusic(auth=headers, **kwargs)
    with _auth_lock:
        _auth[key] = api
    return api


def yt_for(args):
    return yt_auth(args["cookies"]) if args.get("cookies") else yt()


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
        "updated": bool(PYLIB) and any(
            (getattr(sys.modules.get(m), "__file__", "") or "").startswith(PYLIB) for m in ("yt_dlp", "ytmusicapi")),
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
    limit = int(args.get("limit") or 200)
    p = yt_for(args).get_playlist(strip_vl(args["id"]), limit=limit if limit > 0 else None)
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


def m_account(args):
    """Who the linked Google account is."""
    a = yt_auth(args["cookies"]).get_account_info()
    return {"name": a.get("accountName") or "", "handle": a.get("channelHandle") or "", "photo": a.get("accountPhotoUrl") or ""}


def m_library_playlists(args):
    """The linked account's playlists in YouTube Music."""
    out = [{"id": "LM", "title": "Liked Music", "count": "", "thumb": ""}]
    for p in yt_auth(args["cookies"]).get_library_playlists(limit=None) or []:
        pid = strip_vl(p.get("playlistId") or "")
        if not pid or pid == "LM" or pid.startswith("SE"):
            continue
        out.append({"id": pid, "title": p.get("title") or "", "count": str(p.get("count") or ""), "thumb": thumb(p)})
    return out


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
    tracks = []
    for x in w.get("tracks") or []:
        t = track(x)
        if not t:
            continue
        # The song's music video, or the video's song: YouTube Music's Song/Video switch.
        alt = track(x.get("counterpart"), default_thumb=t["thumb"]) if x.get("counterpart") else None
        if alt:
            alt["type"] = "song" if t["type"] == "video" else "video"
            t["counterpart"] = alt
        tracks.append(t)
    return {
        "tracks": tracks,
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


_ydl = {}
_ydl_lock = threading.Lock()

# What to fetch for each stream kind. Audio formats play in every browser and in mpv;
# video-only streams are muted pictures the app keeps in step with the audio.
AUDIO_FORMATS = {
    "low": "worstaudio[abr>=40][ext=m4a]/bestaudio[abr<=70]/worstaudio[abr>=40]/bestaudio[ext=m4a]/bestaudio/best",
    "normal": "bestaudio[ext=m4a]/bestaudio[acodec^=mp4a]/bestaudio/best",
    "high": "bestaudio/best",
}


VIDEO_CODECS = {"avc1": "[vcodec^=avc1][ext=mp4]", "vp9": "[vcodec^=vp9]", "av01": "[vcodec^=av01]"}


def video_format(height, codecs=""):
    """A video-only picture the device can play: only the codecs it says it
    decodes (H.264 first: every phone has it in hardware), and only plain
    https downloads, never HLS or DASH manifests a <video> can't open."""
    h = int(height) if str(height).isdigit() else 720
    wanted = [c for c in (codecs or "avc1,vp9").split(",") if c in VIDEO_CODECS] or ["avc1"]
    plain = "[protocol^=http][protocol!*=m3u8][protocol!*=dash]"
    tries = [f"bestvideo[height<={h}]{VIDEO_CODECS[c]}{plain}" for c in wanted]
    # Nothing small enough: the smallest that is, in a playable codec.
    tries += [f"worstvideo{VIDEO_CODECS[c]}{plain}" for c in wanted]
    return "/".join(tries)


# Other YouTube players to ask when the usual ones get "confirm you're not a bot".
OTHER_CLIENTS = ["tv_samsung", "visionos", "web_embedded"]  # players YouTube serves without a PO token


IPV4 = os.environ.get("GUNDIL_YT_IPV6", "") != "1"  # GUNDIL_YT_IPV6=1 on IPv6-only networks


class _WarningLog:
    """Keeps yt-dlp's warnings for the stream being looked up (the server logs
    them when YouTube refuses the stream); errors come back as exceptions."""

    def debug(self, msg):
        pass

    info = debug
    error = debug

    def warning(self, msg):
        w = getattr(_local, "warnings", None)
        if w is not None and msg not in w and len(w) < 8:
            w.append(str(msg)[:300])


_WARNINGS = _WarningLog()


def ydl_for(fmt, cookies="", clients=None):
    cookies = cookies or COOKIES
    key = (fmt, cookies, tuple(clients or ()))
    with _ydl_lock:
        if key not in _ydl:
            opts = {
                "quiet": True, "noplaylist": True, "skip_download": True,
                "format": fmt, "logger": _WARNINGS,
                "js_runtimes": {"deno": {}, "node": {}, "bun": {}, "quickjs": {}},
            }
            if IPV4:
                # Stream links only work from the address that looked them up. A
                # home connection has one public IPv4 address but often several
                # changing IPv6 ones, so look links up over IPv4 (the server then
                # fetches them over IPv4 too).
                opts["source_address"] = "0.0.0.0"
            if cookies:
                opts["cookiefile"] = cookies
            if clients:
                opts["extractor_args"] = {"youtube": {"player_client": list(clients)}}
            _ydl[key] = yt_dlp.YoutubeDL(opts)
        return _ydl[key]


def sign_in_check(e):
    m = str(e).lower()
    return "confirm you" in m and "bot" in m


def m_stream(args):
    vid = args["videoId"]
    kind = args.get("kind") or "audio"
    if kind == "video":
        fmt = video_format(args.get("height") or "720", args.get("codecs") or "")
    else:
        fmt = AUDIO_FORMATS.get(args.get("quality") or "normal", AUDIO_FORMATS["normal"])
    url = "https://www.youtube.com/watch?v=" + vid
    cookies = args.get("cookies") or ""
    # Which YouTube players to ask (yt-dlp's choice when empty). The server
    # tries others when YouTube refuses the streams one of them gives.
    clients = [c for c in (args.get("clients") or "").split(",") if c.strip()] or None
    _local.warnings = []
    try:
        try:
            info = ydl_for(fmt, cookies, clients).extract_info(url, download=False)
        except Exception as e:  # noqa: BLE001
            m = str(e).lower()
            if not IPV4 or not any(w in m for w in ("network is unreachable", "no route to host", "address family",
                                                    "cannot assign requested address", "errno 101", "errno 99")):
                raise
            # No IPv4 here after all: look it up over any address.
            globals()["IPV4"] = False
            with _ydl_lock:
                _ydl.clear()
            info = ydl_for(fmt, cookies, clients).extract_info(url, download=False)
    except Exception as e:  # noqa: BLE001
        if not sign_in_check(e):
            raise
        # Some YouTube players are asked less often; try them before giving up.
        info = ydl_for(fmt, cookies, OTHER_CLIENTS).extract_info(url, download=False)
    url = info.get("url")
    if not url and info.get("requested_formats"):
        url = info["requested_formats"][0].get("url")
    if not url:
        raise RuntimeError("no playable %s stream found" % ("video" if kind == "video" else "audio"))
    ext = info.get("ext") or ""
    if kind == "video":
        mime = {"mp4": "video/mp4", "webm": "video/webm"}.get(ext, "")
    else:
        mime = {"m4a": "audio/mp4", "mp4": "audio/mp4", "webm": "audio/webm", "mp3": "audio/mpeg", "opus": "audio/ogg"}.get(ext, "")
    expire = parse_qs(urlparse(url).query).get("expire", ["0"])[0]
    return {
        "url": url, "headers": info.get("http_headers") or {}, "mime": mime, "ext": ext,
        "expires": int(expire) if expire.isdigit() else 0, "duration": info.get("duration") or 0,
        "size": info.get("filesize") or info.get("filesize_approx") or 0,
        "height": info.get("height") or 0, "abr": info.get("abr") or 0,
        "format": info.get("format_id") or "", "vcodec": info.get("vcodec") or "", "protocol": info.get("protocol") or "",
        "warnings": _local.warnings,
    }


# ---------- diagnosis ----------
# gundil-music -diagnose-youtube: looks one video up as each YouTube player,
# over IPv4 and IPv6, and tries a little of each kind of stream, so one run
# shows which way (if any) YouTube still serves this network.

def m_diagnose(args):
    from yt_dlp.networking import Request
    from urllib.parse import urljoin
    vid = args.get("videoId") or "60ItHLz5WEA"
    client = args.get("client") or ""
    family = args.get("family") or "IPv4"
    opts = {
        "quiet": True, "logger": _WARNINGS, "skip_download": True, "noplaylist": True,
        "format": "bestaudio/best", "ignore_no_formats_error": True,
        "js_runtimes": {"deno": {}, "node": {}, "bun": {}, "quickjs": {}},
        "source_address": "::" if family == "IPv6" else "0.0.0.0",
    }
    if args.get("cookies"):
        opts["cookiefile"] = args["cookies"]
    if client:
        opts["extractor_args"] = {"youtube": {"player_client": [client]}}
    _local.warnings = []
    ydl = yt_dlp.YoutubeDL(opts)

    def short(msg):
        msg = str(msg).replace("ERROR: ", "").replace("[youtube] %s: " % vid, "")
        return msg.split(". ")[0][:110]

    def tried(link, headers):
        try:
            r = ydl.urlopen(Request(link, headers={**(headers or {}), "Range": "bytes=0-1023"}))
            r.read(1024)
            r.close()
            return str(r.status)
        except Exception as e:  # noqa: BLE001
            return str(getattr(e, "status", "") or short(e)[:40])

    try:
        info = ydl.extract_info("https://www.youtube.com/watch?v=" + vid, download=False)
    except Exception as e:  # noqa: BLE001
        return {"line": "lookup failed: " + short(e), "works": []}
    fmts = info.get("formats") or []
    https = [f for f in fmts if f.get("protocol") == "https"]
    hls = [f for f in fmts if "m3u8" in (f.get("protocol") or "")]
    audio = sorted((f for f in https if f.get("vcodec") == "none" and f.get("acodec") not in (None, "none")),
                   key=lambda f: f.get("abr") or f.get("tbr") or 0)
    video = sorted((f for f in https if f.get("vcodec") not in (None, "none") and f.get("acodec") in (None, "none")
                    and (f.get("height") or 0) <= 720), key=lambda f: f.get("height") or 0)
    parts, works = ["%3d formats (https %d, hls %d)" % (len(fmts), len(https), len(hls))], []
    for name, pick in (("audio", audio), ("video", video)):
        if pick:
            f = pick[-1]
            st = tried(f["url"], f.get("http_headers"))
            parts.append("%s %s: %s" % (name, f.get("format_id"), st))
            if st in ("200", "206"):
                works.append(name)
    if hls:
        f = hls[0]
        st = "?"
        try:
            r = ydl.urlopen(Request(f["url"], headers=f.get("http_headers") or {}))
            text = r.read().decode("utf-8", "replace")
            r.close()
            seg = next((ln.strip() for ln in text.splitlines() if ln.strip() and not ln.startswith("#")), "")
            st = tried(urljoin(f["url"], seg), f.get("http_headers")) if seg else "empty playlist"
        except Exception as e:  # noqa: BLE001
            st = "playlist " + str(getattr(e, "status", "") or short(e)[:40])
        parts.append("hls %s: %s" % (f.get("format_id"), st))
        if st in ("200", "206"):
            works.append("hls")
    line = " | ".join(parts)
    if _local.warnings:
        line += "\n        warned: " + " / ".join(dict.fromkeys(short(w) for w in _local.warnings))
    return {"line": line, "works": works}


# ---------- relay ----------
# When YouTube refuses a stream the server fetches itself, the server asks for
# it through here instead: over yt-dlp's own connection (its network library,
# address and settings), the way yt-dlp downloads.

_relay = {}
_relay_lock = threading.Lock()


def _relay_ydl():
    with _relay_lock:
        if "ydl" not in _relay:
            opts = {"quiet": True, "no_warnings": True, "logger": _WARNINGS}
            if IPV4:
                opts["source_address"] = "0.0.0.0"
            _relay["ydl"] = yt_dlp.YoutubeDL(opts)
        return _relay["ydl"]


def m_relay(args):
    """Starts the relay (once) and says where it listens."""
    import secrets
    from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

    with _relay_lock:
        if "port" in _relay:
            return {"port": _relay["port"], "token": _relay["token"]}
        token = secrets.token_hex(16)

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_GET(self):
                if self.headers.get("X-Gundil-Token") != token:
                    self.send_error(403)
                    return
                from yt_dlp.networking import Request
                from yt_dlp.networking.exceptions import HTTPError
                link = self.headers.get("X-Gundil-Url") or ""
                try:
                    headers = json.loads(self.headers.get("X-Gundil-Headers") or "{}") or {}
                except ValueError:
                    headers = {}
                if not isinstance(headers, dict):
                    headers = {}
                if self.headers.get("Range"):
                    headers["Range"] = self.headers["Range"]
                try:
                    resp = _relay_ydl().urlopen(Request(link, headers=headers))
                except HTTPError as e:
                    self.send_response(e.status)
                    self.send_header("Content-Length", "0")
                    self.end_headers()
                    return
                except Exception as e:  # noqa: BLE001
                    body = (str(e) or e.__class__.__name__)[:300].encode()
                    self.send_response(502)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                    return
                try:
                    self.send_response(resp.status)
                    for h in ("Content-Type", "Content-Length", "Content-Range"):
                        if resp.headers.get(h):
                            self.send_header(h, resp.headers[h])
                    self.end_headers()
                    while True:
                        chunk = resp.read(64 << 10)
                        if not chunk:
                            break
                        self.wfile.write(chunk)
                except OSError:
                    pass  # the server stopped reading (the listener skipped ahead)
                finally:
                    resp.close()

        srv = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        srv.daemon_threads = True
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        _relay.update(port=srv.server_address[1], token=token)
        return {"port": _relay["port"], "token": token}


def m_options(args):
    """Languages and countries YouTube Music results can be shown for."""
    from ytmusicapi.constants import SUPPORTED_LANGUAGES, SUPPORTED_LOCATIONS
    return {"languages": sorted(SUPPORTED_LANGUAGES), "locations": sorted(SUPPORTED_LOCATIONS),
            "language": LANG, "location": REGION}


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
            if "confirm you" in msg and "bot" in msg:
                # Keep "sign in to confirm": the server looks for it to retry with a linked account.
                msg = ("YouTube asked this server to sign in to confirm you're not a bot (this happens on some networks). "
                       "Link your Google account in Settings and Gundil Music will use it whenever this happens.")
        if "yt_dlp" in sys.modules and isinstance(e, getattr(yt_dlp.utils, "DownloadError", ())):
            pass  # YouTube's answer: the server logs it in one line, no traceback needed
        else:
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
