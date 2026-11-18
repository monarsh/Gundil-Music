// Package server exposes the HTTP API and serves the web app.
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/monarsh/gundil-music/internal/bluetooth"
	"github.com/monarsh/gundil-music/internal/model"
	"github.com/monarsh/gundil-music/internal/player"
	"github.com/monarsh/gundil-music/internal/settings"
	"github.com/monarsh/gundil-music/internal/store"
	"github.com/monarsh/gundil-music/internal/users"
	"github.com/monarsh/gundil-music/internal/yt"
)

type Server struct {
	YT      *yt.Client
	Store   *store.Store // the shared library, used until accounts are made
	Users   *users.Store
	DataDir string
	Player  *player.Player
	BT      *bluetooth.Manager
	Web     fs.FS
	Version string
	Auth    string // "user:password", empty for no auth
	Port    int

	Settings *settings.Store
	// OnSettings applies changed settings (region, language, quality...).
	OnSettings func(old, now settings.Settings)

	token string // lets mpv on this machine fetch streams when auth is on

	mu         sync.Mutex
	goodPlayer map[string]string      // stream kind -> YouTube player whose streams play
	relayFirst bool                   // fetch streams through yt-dlp's connection first: YouTube refused ours
	clients    map[chan string]string // live-update channel -> account ("" without accounts)
	sentQVer   int
	libs       map[string]*store.Store
	playerUser string // whose library the server's speaker plays for (history, dislikes)
	fails      map[string]*loginFails
}

// endpoint describes a YouTube Music method the app may call.
type endpoint struct {
	args []string
	ttl  time.Duration
}

var endpoints = map[string]endpoint{
	"home":     {nil, 20 * time.Minute},
	"explore":  {nil, 30 * time.Minute},
	"charts":   {[]string{"country"}, 30 * time.Minute},
	"moods":    {nil, 6 * time.Hour},
	"mood":     {[]string{"params"}, 30 * time.Minute},
	"search":   {[]string{"q", "filter"}, 10 * time.Minute},
	"suggest":  {[]string{"q"}, 10 * time.Minute},
	"album":    {[]string{"id"}, time.Hour},
	"artist":   {[]string{"id"}, time.Hour},
	"playlist": {[]string{"id"}, 30 * time.Minute},
	"watch":    {[]string{"videoId", "playlistId", "radio", "shuffle"}, 10 * time.Minute},
	"lyrics":   {[]string{"id"}, 24 * time.Hour},
	"related":  {[]string{"id"}, time.Hour},
}

// Warm fetches the pages people usually open first, so they're ready.
func (s *Server) Warm() {
	for _, m := range []string{"home", "explore", "moods"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		s.YT.Cached(ctx, m, map[string]string{}, endpoints[m].ttl)
		cancel()
	}
}

// YouTube ties a stream link to the internet address that looked it up
// (its ip= part) and refuses it (403) from any other. On a computer with
// both IPv4 and IPv6, yt-dlp may look a link up over one and the server
// fetch it over the other, so streams are fetched over the same kind of
// address the link names: each kind has its own client (and connections).
func streamTransport(network string) *http.Client {
	d := &net.Dialer{Timeout: 15 * time.Second}
	return &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return d.DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConnsPerHost:   8,
	}}
}

var (
	streamClient  = streamTransport("tcp")
	streamClient4 = streamTransport("tcp4")
	streamClient6 = streamTransport("tcp6")
)

// clientFor picks the client that fetches over the kind of address a
// stream link is for, and names that kind ("" when the link doesn't say).
func clientFor(link string) (*http.Client, string) {
	u, err := url.Parse(link)
	if err != nil {
		return streamClient, ""
	}
	ip, err := netip.ParseAddr(u.Query().Get("ip"))
	switch {
	case err != nil:
		return streamClient, ""
	case ip.Is4() || ip.Is4In6():
		return streamClient4, "IPv4"
	default:
		return streamClient6, "IPv6"
	}
}

// StreamURL is what the server's own player opens for a video.
func (s *Server) StreamURL(id string) (string, bool) {
	if !model.ValidID(id) {
		return "", false
	}
	return fmt.Sprintf("http://127.0.0.1:%d/api/stream/%s?k=%s", s.Port, id, s.token), true
}

// Broadcast sends a server-sent event to every connected client.
func (s *Server) Broadcast(event string, v any) {
	data, _ := json.Marshal(v)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	s.mu.Lock()
	for ch := range s.clients {
		select {
		case ch <- msg:
		default: // slow client, drop the update
		}
	}
	s.mu.Unlock()
}

// BroadcastPlayer sends the player state, with the queue only when it changed.
func (s *Server) BroadcastPlayer() {
	st := s.Player.State()
	s.mu.Lock()
	if st.QueueVersion == s.sentQVer {
		st.Queue = nil
	}
	s.sentQVer = st.QueueVersion
	s.mu.Unlock()
	s.Broadcast("player", st)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readBody(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 8<<20)).Decode(v)
}

func (s *Server) Handler() http.Handler {
	s.clients = map[chan string]string{}
	s.libs = map[string]*store.Store{}
	s.fails = map[string]*loginFails{}
	b := make([]byte, 16)
	rand.Read(b)
	s.token = hex.EncodeToString(b)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/info", s.info)
	mux.HandleFunc("GET /api/yt/{method}", s.ytCall)
	mux.HandleFunc("GET /api/lyrics", s.lyricsFor)
	mux.HandleFunc("POST /api/youtube/update", func(w http.ResponseWriter, r *http.Request) {
		if !s.requireAdmin(w, r) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		res, err := s.YT.Update(ctx, r.URL.Query().Get("force") == "1")
		if err != nil {
			fail(w, 502, err)
			return
		}
		if res.Updated {
			s.Broadcast("info", map[string]bool{"youtubeUpdated": true})
		}
		writeJSON(w, res)
	})
	mux.HandleFunc("GET /api/stream/{id}", s.stream)
	mux.HandleFunc("POST /api/video-error/{id}", func(w http.ResponseWriter, r *http.Request) {
		// A device couldn't show a video: note why, to tell what to fix.
		q := r.URL.Query()
		clip := func(v string) string {
			return strings.Map(func(c rune) rune {
				if c < ' ' {
					return -1
				}
				return c
			}, v[:min(len(v), 200)])
		}
		log.Printf("video %s didn't play on a device: %s (codecs %s, %sp, %s)", clip(r.PathValue("id")), clip(q.Get("error")), clip(q.Get("codecs")), clip(q.Get("h")), clip(r.UserAgent()))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/prefetch/{id}", func(w http.ResponseWriter, r *http.Request) {
		if id := r.PathValue("id"); model.ValidID(id) {
			f := s.format(r)
			go s.YT.StreamFormat(yt.WithCookies(context.Background(), s.accountCookies(r)), id, f, false)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		st := s.Settings.Get()
		if !s.isAdmin(r) {
			st = st.Public()
		}
		out := map[string]any{"settings": st}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if s.YT != nil {
			if data, err := s.YT.Cached(ctx, "options", nil, 24*time.Hour); err == nil {
				out["options"] = json.RawMessage(data)
			}
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		if !s.requireAdmin(w, r) {
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			fail(w, 400, err)
			return
		}
		var bad error
		old, now, err := s.Settings.Update(func(st *settings.Settings) { bad = json.Unmarshal(body, st) })
		if bad != nil {
			fail(w, 400, bad)
			return
		}
		if err != nil {
			fail(w, 400, err)
			return
		}
		if s.OnSettings != nil && old != now {
			s.OnSettings(old, now)
		}
		s.Broadcast("settings", now.Public())
		writeJSON(w, map[string]any{"settings": now})
	})

	mux.HandleFunc("GET /api/state", s.state)
	mux.HandleFunc("POST /api/rating", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Track  model.Track `json:"track"`
			Rating string      `json:"rating"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.lib(r).SetRating(b.Track, b.Rating); err != nil {
			fail(w, 400, err)
			return
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("POST /api/history", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Track model.Track `json:"track"`
		}
		if readBody(r, &b) == nil {
			s.lib(r).AddHistory(b.Track)
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("DELETE /api/history", func(w http.ResponseWriter, r *http.Request) {
		s.lib(r).ClearHistory()
		s.changedState(w, r)
	})
	mux.HandleFunc("POST /api/searches", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Q string }
		if readBody(r, &b) == nil {
			s.lib(r).AddSearch(b.Q)
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("DELETE /api/searches", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q != "" {
			s.lib(r).RemoveSearch(q)
		} else {
			s.lib(r).ClearSearches()
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("POST /api/saved", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Item  store.Saved `json:"item"`
			Saved bool        `json:"saved"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.lib(r).SetSaved(b.Item, b.Saved); err != nil {
			fail(w, 400, err)
			return
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("POST /api/playlists", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name        string        `json:"name"`
			Description string        `json:"description"`
			Tracks      []model.Track `json:"tracks"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		p, err := s.lib(r).CreatePlaylist(b.Name, b.Description, b.Tracks)
		if err != nil {
			fail(w, 400, err)
			return
		}
		s.stateChanged(r)
		writeJSON(w, map[string]any{"id": p.ID, "state": json.RawMessage(s.lib(r).JSON())})
	})
	mux.HandleFunc("PUT /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name        *string        `json:"name"`
			Description *string        `json:"description"`
			Tracks      *[]model.Track `json:"tracks"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.lib(r).UpdatePlaylist(r.PathValue("id"), b.Name, b.Description, b.Tracks); err != nil {
			fail(w, 404, err)
			return
		}
		s.changedState(w, r)
	})
	mux.HandleFunc("POST /api/playlists/{id}/add", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Tracks []model.Track `json:"tracks"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		n, err := s.lib(r).AddToPlaylist(r.PathValue("id"), b.Tracks)
		if err != nil {
			fail(w, 404, err)
			return
		}
		s.stateChanged(r)
		writeJSON(w, map[string]any{"added": n, "state": json.RawMessage(s.lib(r).JSON())})
	})
	mux.HandleFunc("DELETE /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.lib(r).DeletePlaylist(r.PathValue("id")); err != nil {
			fail(w, 404, err)
			return
		}
		s.changedState(w, r)
	})

	mux.HandleFunc("GET /api/player", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.Player.State()) })
	mux.HandleFunc("GET /api/player/devices", func(w http.ResponseWriter, r *http.Request) {
		list, current, err := s.Player.AudioDevices()
		if err != nil {
			fail(w, 503, err)
			return
		}
		writeJSON(w, map[string]any{"devices": list, "current": current})
	})
	mux.HandleFunc("POST /api/player/{action}", s.playerAction)

	mux.HandleFunc("GET /api/bluetooth", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.BT.Status()) })
	mux.HandleFunc("POST /api/bluetooth/{action}", s.bluetoothAction)

	mux.HandleFunc("GET /api/events", s.events)

	static := http.FileServer(http.FS(s.Web))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	}))

	s.accountRoutes(mux)
	var h http.Handler = s.accounts(mux)
	if s.Auth == "" {
		return h
	}
	user, pass, _ := strings.Cut(s.Auth, ":")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/stream/") &&
			subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("k")), []byte(s.token)) == 1 {
			h.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Gundil Music"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	writeJSON(w, map[string]any{
		"name":       "Gundil Music",
		"version":    s.Version,
		"hostname":   host,
		"player":     s.Player.Available(),
		"bluetooth":  s.BT.Available(),
		"youtube":    s.YT.Status(ctx),
		"lastUpdate": s.YT.LastUpdate(),
		// YouTube's "confirm you're not a bot" check, and whether this
		// person has a YouTube Music account songs can play with.
		"signInCheck":   s.YT.SignInCheckState(),
		"accountLinked": len(s.accountCookies(r)) > 0,
	})
}

// PlayerPrefetch gets the stream of the song the server's own player plays
// next ready, signed in like the player when YouTube asks for that.
func (s *Server) PlayerPrefetch(id string) {
	if !model.ValidID(id) {
		return
	}
	s.YT.Stream(yt.WithCookies(context.Background(), s.playerCookies()), id, false)
}

func (s *Server) ytCall(w http.ResponseWriter, r *http.Request) {
	method := r.PathValue("method")
	ep, ok := endpoints[method]
	if !ok {
		fail(w, 404, errors.New("unknown method"))
		return
	}
	args := map[string]string{}
	q := r.URL.Query()
	for _, a := range ep.args {
		if v := strings.TrimSpace(q.Get(a)); v != "" {
			if len(v) > 500 {
				fail(w, 400, errors.New(a+" is too long"))
				return
			}
			args[a] = v
		}
	}
	ttl := ep.ttl
	if method == "watch" && args["radio"] != "" {
		ttl = 0 // a radio is different every time
	}
	data, err := s.YT.Cached(r.Context(), method, args, ttl)
	if err != nil {
		if r.Context().Err() == nil {
			fail(w, 502, err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// streamChunk is the most asked of YouTube in one request. YouTube refuses
// (403) a single request for much more of a stream than this, so longer
// streams (videos above all) are fetched piece by piece, as yt-dlp does.
const streamChunk = 10 << 20

// parseRange reads a "bytes=a-b" or "bytes=a-" request. end is -1 for "to the end".
func parseRange(h string) (start, end int64, ok bool) {
	spec, found := strings.CutPrefix(h, "bytes=")
	if !found || strings.Contains(spec, ",") {
		return 0, -1, false
	}
	a, b, _ := strings.Cut(spec, "-")
	if _, err := fmt.Sscan(a, &start); err != nil || start < 0 {
		return 0, -1, false
	}
	end = -1
	if b != "" {
		if _, err := fmt.Sscan(b, &end); err != nil || end < start {
			return 0, -1, false
		}
	}
	return start, end, true
}

// totalOf reads the full size from a "bytes a-b/total" Content-Range.
func totalOf(contentRange string) int64 {
	_, t, ok := strings.Cut(contentRange, "/")
	var n int64
	if !ok || t == "*" {
		return -1
	}
	if _, err := fmt.Sscan(t, &n); err != nil {
		return -1
	}
	return n
}

// stream proxies a video's audio (or picture) from YouTube, passing Range
// through so the browser and mpv can seek. The bytes asked for are fetched
// from YouTube in pieces of at most streamChunk.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !model.ValidID(id) {
		http.NotFound(w, r)
		return
	}
	clientRange := r.Header.Get("Range")
	start, end, ranged := parseRange(clientRange)
	if !ranged {
		start, end = 0, -1
	}
	f := s.format(r)
	ctx := yt.WithCookies(r.Context(), s.accountCookies(r))

	// get asks YouTube for bytes from..to of the stream. If YouTube refuses
	// the link, it gets a fresh one, then asks as other YouTube players
	// (the links some of them give are refused), and remembers which works.
	var st *yt.Stream
	kind := f.Kind
	if kind == "" {
		kind = "audio"
	}
	tries := s.playerTries(kind)
	try := 0
	warned := map[*yt.Stream]bool{}
	get := func(from, to int64) (*http.Response, error) {
		var lastErr error
		for ; try < len(tries); try++ {
			t := tries[try]
			if st == nil || t.refresh || try > 0 && tries[try-1].clients != t.clients {
				g := f
				g.Clients = t.clients
				var err error
				if st, err = s.YT.StreamFormat(ctx, id, g, t.refresh); err != nil {
					if t.clients == "" {
						return nil, err // the usual lookup failed: not a refusal
					}
					lastErr = err
					log.Printf("stream %s (%s): asking as YouTube player %s failed: %v", id, kind, t.clients, err)
					continue
				}
			}
			// The server's own connection first, then yt-dlp's (or the other
			// way round once ours has been refused and yt-dlp's wasn't).
			s.mu.Lock()
			ways := []bool{s.relayFirst, !s.relayFirst}
			s.mu.Unlock()
			refused := false
			for _, viaRelay := range ways {
				resp, how, err := s.fetchPiece(r.Context(), st, from, to, viaRelay)
				if err != nil {
					lastErr = err
					if viaRelay {
						log.Printf("stream %s (%s): fetching through yt-dlp failed: %v", id, kind, err)
					}
					continue
				}
				switch resp.StatusCode {
				case http.StatusForbidden, http.StatusGone, http.StatusNotFound:
					resp.Body.Close()
					refused = true
					lastErr = fmt.Errorf("YouTube refused the stream (%s)", resp.Status)
					log.Printf("stream %s (%s, format %s, YouTube player %s): YouTube refused it (%s; %s)", id, kind, st.Format, orDefault(t.clients), resp.Status, how)
					if !warned[st] && len(st.Warnings) > 0 {
						warned[st] = true
						log.Printf("stream %s: yt-dlp warned while looking it up: %s", id, strings.Join(st.Warnings, " | "))
					}
					continue
				case http.StatusRequestedRangeNotSatisfiable:
					resp.Body.Close()
					return nil, errRangeEnd
				case http.StatusOK, http.StatusPartialContent:
					s.notePlayer(kind, t.clients, true)
					s.noteRelay(viaRelay, refused)
					return resp, nil
				default:
					resp.Body.Close()
					return nil, fmt.Errorf("YouTube answered %s", resp.Status)
				}
			}
			if refused {
				s.notePlayer(kind, t.clients, false)
			}
		}
		return nil, lastErr
	}

	firstTo := start + streamChunk - 1
	if end >= 0 && end < firstTo {
		firstTo = end
	}
	resp, err := get(start, firstTo)
	if err != nil {
		if r.Context().Err() == nil {
			if errors.Is(err, errRangeEnd) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			log.Printf("stream %s: %v", id, err)
			fail(w, 502, err)
		}
		return
	}
	defer resp.Body.Close()
	if f.Kind == "video" && start == 0 {
		log.Printf("video %s: format %s, %s, %dp, %s, YouTube player %s (device plays %s)", id, st.Format, st.VCodec, st.Height, st.Protocol, orDefault(tries[min(try, len(tries)-1)].clients), f.Codecs)
	}

	h := w.Header()
	mime := st.Mime
	if mime == "" {
		mime = resp.Header.Get("Content-Type")
	}
	h.Set("Content-Type", mime)
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "no-store")

	total := totalOf(resp.Header.Get("Content-Range"))
	if resp.StatusCode == http.StatusOK || total < 0 {
		// YouTube sent the whole thing (or won't say its size): pass it on as is.
		if v := resp.Header.Get("Content-Length"); v != "" && !ranged {
			h.Set("Content-Length", v)
		}
		w.WriteHeader(http.StatusOK)
		io.Copy(w, resp.Body)
		return
	}
	if end < 0 || end >= total {
		end = total - 1
	}
	if ranged {
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		h.Set("Content-Length", fmt.Sprint(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		h.Set("Content-Length", fmt.Sprint(total))
		w.WriteHeader(http.StatusOK)
	}
	n, err := io.Copy(w, resp.Body)
	resp.Body.Close()
	for pos := start + n; err == nil && pos <= end; {
		to := min(pos+streamChunk-1, end)
		next, gerr := get(pos, to)
		if gerr != nil {
			if r.Context().Err() == nil {
				log.Printf("stream %s at byte %d: %v", id, pos, gerr)
			}
			return // the player asks again from where it stopped
		}
		n, err = io.Copy(w, next.Body)
		next.Body.Close()
		if n == 0 {
			return
		}
		pos += n
	}
}

var errRangeEnd = errors.New("past the end of the stream")

// relayClient talks to the helper's relay on this machine (never a proxy).
var relayClient = &http.Client{Transport: &http.Transport{
	Proxy:                 nil,
	ResponseHeaderTimeout: 45 * time.Second,
	MaxIdleConnsPerHost:   4,
}}

// fetchPiece asks YouTube for bytes from..to of a stream link: over the
// server's own connection, or through yt-dlp's (the helper's relay). It
// also says how it fetched, for the log.
func (s *Server) fetchPiece(ctx context.Context, st *yt.Stream, from, to int64, viaRelay bool) (*http.Response, string, error) {
	rng := fmt.Sprintf("bytes=%d-%d", from, to)
	if viaRelay {
		addr, token, err := s.YT.Relay(ctx)
		if err != nil {
			return nil, "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/", nil)
		if err != nil {
			return nil, "", err
		}
		hdr, _ := json.Marshal(st.Headers)
		req.Header.Set("X-Gundil-Token", token)
		req.Header.Set("X-Gundil-Url", st.URL)
		req.Header.Set("X-Gundil-Headers", string(hdr))
		req.Header.Set("Range", rng)
		resp, err := relayClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusBadGateway {
			msg, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
			resp.Body.Close()
			return nil, "", errors.New(strings.TrimSpace(string(msg)))
		}
		return resp, "fetched through yt-dlp's connection", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, st.URL, nil)
	if err != nil {
		return nil, "", err
	}
	for k, v := range st.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Range", rng)
	client, family := clientFor(st.URL)
	var local string
	req = req.WithContext(httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GotConn: func(c httptrace.GotConnInfo) { local = c.Conn.LocalAddr().String() },
	}))
	resp, err := client.Do(req)
	return resp, fmt.Sprintf("link for %s, fetched by the server from %s", orUnknown(family), local), err
}

// noteRelay remembers whether streams play through yt-dlp's connection
// rather than the server's own, so the next piece goes the working way first.
func (s *Server) noteRelay(viaRelay, afterRefusal bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if viaRelay != s.relayFirst && (afterRefusal || !viaRelay) {
		s.relayFirst = viaRelay
		if viaRelay {
			log.Printf("streams now play through yt-dlp's connection: YouTube refused the server's own")
		} else {
			log.Printf("streams play over the server's own connection again")
		}
	}
}

// Other YouTube players to ask when YouTube refuses the stream links yt-dlp's
// usual ones give: the links of some players are refused without proof only
// YouTube's own apps can give.
var otherPlayers = []string{"android_vr", "tv", "web_embedded", "mweb"}

type playerTry struct {
	clients string // "" = yt-dlp's choice
	refresh bool
}

// playerTries lists how to look a stream up: the YouTube player that worked
// last for this kind of stream (or yt-dlp's choice), a fresh link from it,
// then the other players.
func (s *Server) playerTries(kind string) []playerTry {
	s.mu.Lock()
	good := s.goodPlayer[kind]
	s.mu.Unlock()
	tries := []playerTry{{good, false}, {good, true}}
	for _, p := range append([]string{""}, otherPlayers...) {
		if p != good {
			tries = append(tries, playerTry{p, false})
		}
	}
	return tries
}

// notePlayer remembers the YouTube player whose streams play, for the next
// stream of the same kind; one that's refused is forgotten.
func (s *Server) notePlayer(kind, clients string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.goodPlayer == nil {
		s.goodPlayer = map[string]string{}
	}
	old := s.goodPlayer[kind]
	if ok && clients != old {
		s.goodPlayer[kind] = clients
		log.Printf("%s streams now play as YouTube player %s", kind, orDefault(clients))
	} else if !ok && clients == old && old != "" {
		delete(s.goodPlayer, kind)
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "an unnamed address"
	}
	return s
}

func orDefault(clients string) string {
	if clients == "" {
		return "(yt-dlp's choice)"
	}
	return clients
}

// format reads which stream a request wants: ?q=low|normal|high for audio,
// or ?kind=video&h=720 for a video picture. Audio defaults to the server's quality.
func (s *Server) format(r *http.Request) yt.Format {
	q := r.URL.Query()
	if q.Get("kind") == "video" {
		h := q.Get("h")
		switch h {
		case "360", "480", "720", "1080":
		default:
			h = "720"
		}
		return yt.Format{Kind: "video", Height: h, Codecs: videoCodecs(q.Get("codecs"))}
	}
	switch v := q.Get("q"); v {
	case "low", "normal", "high":
		return yt.Format{Quality: v}
	}
	return yt.Format{Quality: s.YT.AudioQuality()}
}

// videoCodecs keeps the known codec names of a device's list, in its order.
func videoCodecs(list string) string {
	var out []string
	for _, c := range strings.Split(list, ",") {
		if c = strings.TrimSpace(c); (c == "avc1" || c == "vp9" || c == "av01") && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return strings.Join(out, ",")
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.lib(r).JSON())
}

// changedState tells other devices the library changed and returns it.
func (s *Server) changedState(w http.ResponseWriter, r *http.Request) {
	s.stateChanged(r)
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.lib(r).JSON())
}

func (s *Server) playerAction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Tracks   []model.Track `json:"tracks"`
		Index    int           `json:"index"`
		Play     bool          `json:"play"`
		Next     bool          `json:"next"`
		Source   string        `json:"source"`
		Position float64       `json:"position"`
		Volume   float64       `json:"volume"`
		Mode     string        `json:"mode"`
		Name     string        `json:"name"`
		On       bool          `json:"on"`
		From     int           `json:"from"`
		To       int           `json:"to"`
		Minutes  float64       `json:"minutes"`
		End      bool          `json:"end"`
	}
	if r.ContentLength != 0 {
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
	}
	p := s.Player
	var err error
	if a := r.PathValue("action"); a == "queue" || a == "insert" || a == "jump" {
		// The speaker now plays for this account: its history and dislikes.
		s.mu.Lock()
		s.playerUser = s.user(r).Name
		s.mu.Unlock()
	}
	switch r.PathValue("action") {
	case "queue":
		err = p.SetQueue(b.Tracks, b.Index, b.Play, b.Source)
	case "insert":
		p.Insert(b.Tracks, b.Next)
	case "remove":
		err = p.Remove(b.Index)
	case "move":
		err = p.Move(b.From, b.To)
	case "sleep":
		p.SetSleep(time.Duration(b.Minutes*float64(time.Minute)), b.End)
	case "jump":
		err = p.Jump(b.Index)
	case "pause":
		err = p.SetPaused(true)
	case "resume":
		err = p.SetPaused(false)
	case "toggle":
		err = p.Toggle()
	case "next":
		p.Next()
	case "prev":
		err = p.Prev()
	case "stop":
		p.Stop()
	case "seek":
		err = p.Seek(b.Position)
	case "volume":
		err = p.SetVolume(b.Volume)
	case "repeat":
		err = p.SetRepeat(b.Mode)
	case "autoplay":
		p.SetAutoplay(b.On)
	case "device":
		err = p.SetAudioDevice(b.Name)
	default:
		fail(w, 404, errors.New("unknown player action"))
		return
	}
	if err != nil {
		fail(w, 503, err)
		return
	}
	writeJSON(w, p.State())
}

func (s *Server) bluetoothAction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		MAC string `json:"mac"`
	}
	if r.ContentLength != 0 {
		readBody(r, &b)
	}
	var err error
	switch r.PathValue("action") {
	case "scan":
		err = s.BT.Scan(12*time.Second, func() { s.Broadcast("bluetooth", map[string]bool{"scanning": false}) })
	case "connect":
		err = s.BT.Connect(b.MAC)
	case "disconnect":
		err = s.BT.Disconnect(b.MAC)
	case "remove":
		err = s.BT.Remove(b.MAC)
	default:
		fail(w, 404, errors.New("unknown bluetooth action"))
		return
	}
	if err != nil {
		fail(w, 502, err)
		return
	}
	writeJSON(w, s.BT.Status())
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := make(chan string, 64)
	s.mu.Lock()
	s.clients[ch] = s.user(r).Name
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	data, _ := json.Marshal(s.Player.State()) // full state, queue included
	fmt.Fprintf(w, "event: player\ndata: %s\n\n", data)
	flusher.Flush()

	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
