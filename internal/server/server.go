// Package server exposes the HTTP API and serves the web app.
package server

import (
	"bytes"
	"compress/gzip"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/monarsh/gundil-music/internal/bluetooth"
	"github.com/monarsh/gundil-music/internal/library"
	"github.com/monarsh/gundil-music/internal/player"
	"github.com/monarsh/gundil-music/internal/store"
)

type Server struct {
	Lib     *library.Library
	Store   *store.Store
	Player  *player.Player
	BT      *bluetooth.Manager
	Web     fs.FS
	Version string
	Auth    string // "user:password", empty for no auth

	mu      sync.Mutex
	clients map[chan string]struct{}
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

func (s *Server) BroadcastPlayer() { s.Broadcast("player", s.Player.State()) }

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
	s.clients = map[chan string]struct{}{}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/info", s.info)
	mux.HandleFunc("GET /api/library", s.library)
	mux.HandleFunc("POST /api/library/rescan", func(w http.ResponseWriter, r *http.Request) {
		go s.Lib.Scan()
		writeJSON(w, map[string]bool{"scanning": true})
	})
	mux.HandleFunc("GET /api/stream/{id}", s.stream)
	mux.HandleFunc("GET /api/art/{id}", s.art)
	mux.HandleFunc("POST /api/tracks/{id}/duration", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Duration float64 }
		if readBody(r, &b) == nil {
			s.Lib.SetDuration(r.PathValue("id"), b.Duration)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/state", s.state)
	mux.HandleFunc("POST /api/played/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.Store.RecordPlay(r.PathValue("id"))
		s.state(w, r)
	})
	mux.HandleFunc("POST /api/liked/{id}", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Liked bool }
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		s.Store.SetLiked(r.PathValue("id"), b.Liked)
		s.state(w, r)
	})
	mux.HandleFunc("POST /api/playlists", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name   string
			Tracks []string
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.Store.CreatePlaylist(b.Name, b.Tracks); err != nil {
			fail(w, 400, err)
			return
		}
		s.state(w, r)
	})
	mux.HandleFunc("PUT /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name   *string
			Tracks *[]string
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if err := s.Store.UpdatePlaylist(r.PathValue("id"), b.Name, b.Tracks); err != nil {
			fail(w, 404, err)
			return
		}
		s.state(w, r)
	})
	mux.HandleFunc("DELETE /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.DeletePlaylist(r.PathValue("id")); err != nil {
			fail(w, 404, err)
			return
		}
		s.state(w, r)
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

	if s.Auth == "" {
		return mux
	}
	user, pass, _ := strings.Cut(s.Auth, ":")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Gundil Music"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	host, _ := os.Hostname()
	writeJSON(w, map[string]any{
		"name":      "Gundil Music",
		"version":   s.Version,
		"hostname":  host,
		"player":    s.Player.Available(),
		"bluetooth": s.BT.Available(),
		"folders":   s.Lib.Roots(),
	})
}

func (s *Server) library(w http.ResponseWriter, r *http.Request) {
	data, _ := json.Marshal(map[string]any{"tracks": s.Lib.Tracks(), "scanning": s.Lib.Scanning()})
	w.Header().Set("Content-Type", "application/json")
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		gz.Write(data)
		gz.Close()
		return
	}
	w.Write(data)
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	path, ok := s.Lib.Path(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path) // handles Range requests for seeking
}

func (s *Server) art(w http.ResponseWriter, r *http.Request) {
	data, mime, ok := s.Lib.Art(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(s.Store.JSON())
}

func (s *Server) playerAction(w http.ResponseWriter, r *http.Request) {
	var b struct {
		IDs      []string `json:"ids"`
		Index    int      `json:"index"`
		Play     bool     `json:"play"`
		Position float64  `json:"position"`
		Volume   float64  `json:"volume"`
		Mode     string   `json:"mode"`
		Name     string   `json:"name"`
	}
	if r.ContentLength != 0 {
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
	}
	p := s.Player
	var err error
	switch r.PathValue("action") {
	case "queue":
		err = p.SetQueue(b.IDs, b.Index, b.Play)
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

	ch := make(chan string, 32)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	data, _ := json.Marshal(s.Player.State())
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
