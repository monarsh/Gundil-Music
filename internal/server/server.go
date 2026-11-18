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
	"os"
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

var streamClient = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
	TLSHandshakeTimeout:   15 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	MaxIdleConnsPerHost:   8,
}}

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
	mux.HandleFunc("POST /api/prefetch/{id}", func(w http.ResponseWriter, r *http.Request) {
		if id := r.PathValue("id"); model.ValidID(id) {
			f := s.format(r)
			go s.YT.StreamFormat(yt.WithCookies(context.Background(), s.accountCookies(r)), id, f, false)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{"settings": s.Settings.Get()}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		if data, err := s.YT.Cached(ctx, "options", nil, 24*time.Hour); err == nil {
			out["options"] = json.RawMessage(data)
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
		s.Broadcast("settings", now)
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
	})
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

// stream proxies a video's audio from YouTube, passing Range through so the
// browser and mpv can seek.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !model.ValidID(id) {
		http.NotFound(w, r)
		return
	}
	clientRange := r.Header.Get("Range")
	f := s.format(r)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		st, err := s.YT.StreamFormat(yt.WithCookies(r.Context(), s.accountCookies(r)), id, f, attempt > 0)
		if err != nil {
			lastErr = err
			break
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, st.URL, nil)
		if err != nil {
			lastErr = err
			break
		}
		for k, v := range st.Headers {
			req.Header.Set(k, v)
		}
		if clientRange != "" {
			req.Header.Set("Range", clientRange)
		} else {
			req.Header.Set("Range", "bytes=0-")
		}
		resp, err := streamClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			lastErr = fmt.Errorf("YouTube refused the stream (%s)", resp.Status)
			continue
		}
		defer resp.Body.Close()

		h := w.Header()
		mime := st.Mime
		if mime == "" {
			mime = resp.Header.Get("Content-Type")
		}
		h.Set("Content-Type", mime)
		h.Set("Accept-Ranges", "bytes")
		h.Set("Cache-Control", "no-store")
		status := resp.StatusCode
		if status == http.StatusPartialContent && clientRange == "" {
			// We asked for the whole file as a range; answer as a plain 200.
			status = http.StatusOK
			if _, total, ok := strings.Cut(resp.Header.Get("Content-Range"), "/"); ok && total != "*" {
				h.Set("Content-Length", total)
			}
		} else {
			if v := resp.Header.Get("Content-Range"); v != "" {
				h.Set("Content-Range", v)
			}
			if v := resp.Header.Get("Content-Length"); v != "" {
				h.Set("Content-Length", v)
			}
		}
		w.WriteHeader(status)
		io.Copy(w, resp.Body)
		return
	}
	if r.Context().Err() == nil {
		log.Printf("stream %s: %v", id, lastErr)
		fail(w, 502, lastErr)
	}
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
		return yt.Format{Kind: "video", Height: h}
	}
	switch v := q.Get("q"); v {
	case "low", "normal", "high":
		return yt.Format{Quality: v}
	}
	return yt.Format{Quality: s.YT.AudioQuality()}
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
