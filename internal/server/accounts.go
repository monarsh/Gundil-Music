package server

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/monarsh/gundil-music/internal/model"
	"github.com/monarsh/gundil-music/internal/store"
	"github.com/monarsh/gundil-music/internal/users"
)

// ---------- who is asking ----------

type ctxUser struct{}

const sessionCookie = "gundil_session"

type loginFails struct {
	n    int
	last time.Time
}

var libMu sync.Mutex

func (s *Server) accountsOn() bool { return s.Users != nil && s.Users.Enabled() }

// user is the account signed in for this request (empty without accounts).
func (s *Server) user(r *http.Request) users.User {
	u, _ := r.Context().Value(ctxUser{}).(users.User)
	return u
}

func (s *Server) isAdmin(r *http.Request) bool { return !s.accountsOn() || s.user(r).Admin }

// requireAdmin answers 403 unless the request may change server-wide things.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.isAdmin(r) {
		return true
	}
	fail(w, http.StatusForbidden, errors.New("only an admin account can change this"))
	return false
}

// userDir is where an account keeps its files: the data folder itself
// without accounts.
func (s *Server) userDir(name string) string {
	if name == "" {
		return s.DataDir
	}
	return filepath.Join(s.DataDir, "users", users.DirName(name))
}

// Library returns an account's library (the shared one without accounts).
func (s *Server) Library(name string) *store.Store {
	if name == "" || !s.accountsOn() {
		return s.Store
	}
	key := strings.ToLower(name)
	libMu.Lock()
	defer libMu.Unlock()
	if l, ok := s.libs[key]; ok {
		return l
	}
	dir := s.userDir(name)
	os.MkdirAll(dir, 0o700)
	l := store.New(filepath.Join(dir, "library.json"))
	s.libs[key] = l
	return l
}

func (s *Server) lib(r *http.Request) *store.Store { return s.Library(s.user(r).Name) }

// playerAccount is the account the server's speaker is playing for.
func (s *Server) playerAccount() string {
	s.mu.Lock()
	name := s.playerUser
	s.mu.Unlock()
	if name != "" && s.accountsOn() {
		if _, ok := s.Users.Get(name); ok {
			return name
		}
	}
	if s.accountsOn() {
		if a := s.Users.Admins(); len(a) > 0 {
			return a[0]
		}
	}
	return ""
}

// PlayerLibrary is the library of whoever the speaker plays for.
func (s *Server) PlayerLibrary() *store.Store { return s.Library(s.playerAccount()) }

// BroadcastTo sends an event to one account's devices.
func (s *Server) BroadcastTo(name, event string, v any) {
	data, _ := json.Marshal(v)
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	s.mu.Lock()
	for ch, u := range s.clients {
		if strings.EqualFold(u, name) {
			select {
			case ch <- msg:
			default:
			}
		}
	}
	s.mu.Unlock()
}

// stateChanged tells this account's other devices that its library changed.
func (s *Server) stateChanged(r *http.Request) { s.BroadcastTo(s.user(r).Name, "state", nil) }

// accounts makes people sign in once accounts exist. The app's files and
// the few calls needed to sign in stay open.
func (s *Server) accounts(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.accountsOn() {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil {
			if u, ok := s.Users.Session(c.Value); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxUser{}, u))
				next.ServeHTTP(w, r)
				return
			}
		}
		p := r.URL.Path
		open := !strings.HasPrefix(p, "/api/") ||
			(r.Method == "GET" && (p == "/api/info" || p == "/api/session")) ||
			(r.Method == "POST" && (p == "/api/login" || p == "/api/logout" || p == "/api/signup"))
		if strings.HasPrefix(p, "/api/stream/") &&
			subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("k")), []byte(s.token)) == 1 {
			open = true // the server's own player
		}
		if open {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "Sign in to Gundil Music", "signin": true})
	})
}

func (s *Server) setSession(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// signUp says whether people may create their own account, and whether
// they need the invite code for it.
func (s *Server) signUp() (open, needsCode bool) {
	if s.Settings == nil || !s.accountsOn() {
		return false, false
	}
	st := s.Settings.Get()
	return st.SignUp, st.SignUp && st.InviteCode != ""
}

// tooManyFails says whether this address got a password or code wrong too often.
func (s *Server) tooManyFails(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fails[ip]
	if f != nil && time.Since(f.last) > 10*time.Minute {
		delete(s.fails, ip)
		return false
	}
	return f != nil && f.n >= 5 && time.Since(f.last) < time.Minute
}

func (s *Server) noteFail(ip string, failed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !failed {
		delete(s.fails, ip)
		return
	}
	if s.fails[ip] == nil {
		s.fails[ip] = &loginFails{}
	}
	s.fails[ip].n++
	s.fails[ip].last = time.Now()
}

func (s *Server) session(r *http.Request) map[string]any {
	open, code := s.signUp()
	out := map[string]any{"accounts": s.accountsOn(), "user": nil, "admin": s.isAdmin(r), "signUp": open, "needsCode": code}
	if u := s.user(r); u.Name != "" {
		out["user"] = u
	}
	return out
}

// ---------- routes ----------

func (s *Server) accountRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		// The accounts layer only adds the user for signed-in requests;
		// look the cookie up here too, as this route is open.
		if c, err := r.Cookie(sessionCookie); err == nil && s.user(r).Name == "" {
			if u, ok := s.Users.Session(c.Value); ok {
				r = r.WithContext(context.WithValue(r.Context(), ctxUser{}, u))
			}
		}
		writeJSON(w, s.session(r))
	})

	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Name, Password string }
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		ip := clientIP(r)
		s.mu.Lock()
		f := s.fails[ip]
		if f != nil && time.Since(f.last) > 10*time.Minute {
			f = nil
			delete(s.fails, ip)
		}
		blocked := f != nil && f.n >= 5 && time.Since(f.last) < time.Minute
		s.mu.Unlock()
		if blocked {
			fail(w, http.StatusTooManyRequests, errors.New("too many wrong passwords; wait a minute and try again"))
			return
		}
		u, token, ok := s.Users.Login(b.Name, b.Password)
		s.mu.Lock()
		if !ok {
			if s.fails[ip] == nil {
				s.fails[ip] = &loginFails{}
			}
			s.fails[ip].n++
			s.fails[ip].last = time.Now()
		} else {
			delete(s.fails, ip)
		}
		s.mu.Unlock()
		if !ok {
			time.Sleep(500 * time.Millisecond)
			fail(w, http.StatusUnauthorized, errors.New("wrong name or password"))
			return
		}
		s.setSession(w, token, int(users.SessionLife.Seconds()))
		r = r.WithContext(context.WithValue(r.Context(), ctxUser{}, u))
		writeJSON(w, s.session(r))
	})

	// People creating their own account, when an admin allows it.
	mux.HandleFunc("POST /api/signup", func(w http.ResponseWriter, r *http.Request) {
		open, needsCode := s.signUp()
		if !open {
			fail(w, http.StatusForbidden, errors.New("this server doesn't let people create their own account; ask an admin to add you"))
			return
		}
		var b struct{ Name, Password, Code string }
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		ip := clientIP(r)
		if s.tooManyFails(ip) {
			fail(w, http.StatusTooManyRequests, errors.New("too many wrong codes; wait a minute and try again"))
			return
		}
		if needsCode {
			want := s.Settings.Get().InviteCode
			ok := subtle.ConstantTimeCompare([]byte(strings.ToLower(strings.TrimSpace(b.Code))), []byte(strings.ToLower(want))) == 1
			s.noteFail(ip, !ok)
			if !ok {
				time.Sleep(500 * time.Millisecond)
				fail(w, http.StatusForbidden, errors.New("wrong invite code; ask an admin for it"))
				return
			}
		}
		u, err := s.Users.Create(b.Name, b.Password, false)
		if err != nil {
			fail(w, 400, err)
			return
		}
		_, token, _ := s.Users.Login(u.Name, b.Password)
		s.setSession(w, token, int(users.SessionLife.Seconds()))
		s.Broadcast("accounts", nil) // admins' account lists update
		r = r.WithContext(context.WithValue(r.Context(), ctxUser{}, u))
		writeJSON(w, s.session(r))
	})

	mux.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			s.Users.Logout(c.Value)
		}
		s.setSession(w, "", -1)
		w.WriteHeader(http.StatusNoContent)
	})

	// The first account: the shared library becomes this account's.
	mux.HandleFunc("POST /api/accounts/setup", func(w http.ResponseWriter, r *http.Request) {
		if s.accountsOn() {
			fail(w, http.StatusConflict, errors.New("accounts are already set up"))
			return
		}
		var b struct{ Name, Password string }
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		u, err := s.Users.Create(b.Name, b.Password, true)
		if err != nil {
			fail(w, 400, err)
			return
		}
		dir := s.userDir(u.Name)
		os.MkdirAll(dir, 0o700)
		// Copy the library (keep the old file, in case accounts are undone by hand).
		if data := s.Store.JSON(); len(data) > 0 {
			os.WriteFile(filepath.Join(dir, "library.json"), data, 0o600)
		}
		if old := s.cookiesPath(""); fileExists(old) {
			os.Rename(old, s.cookiesPath(u.Name))
		}
		_, token, _ := s.Users.Login(u.Name, b.Password)
		s.setSession(w, token, int(users.SessionLife.Seconds()))
		s.Broadcast("accounts", nil)
		r = r.WithContext(context.WithValue(r.Context(), ctxUser{}, u))
		writeJSON(w, s.session(r))
	})

	mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		if !s.accountsOn() || !s.requireAdmin(w, r) {
			if !s.accountsOn() {
				writeJSON(w, []users.User{})
			}
			return
		}
		writeJSON(w, s.Users.List())
	})
	mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		if !s.accountsOn() {
			fail(w, 400, errors.New("set up accounts first"))
			return
		}
		if !s.requireAdmin(w, r) {
			return
		}
		var b struct {
			Name, Password string
			Admin          bool
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if _, err := s.Users.Create(b.Name, b.Password, b.Admin); err != nil {
			fail(w, 400, err)
			return
		}
		writeJSON(w, s.Users.List())
	})
	mux.HandleFunc("PUT /api/users/{name}", func(w http.ResponseWriter, r *http.Request) {
		me := s.user(r)
		target, ok := s.Users.Get(r.PathValue("name"))
		if !ok {
			fail(w, 404, users.ErrNotFound)
			return
		}
		self := strings.EqualFold(target.Name, me.Name)
		if !self && !me.Admin {
			fail(w, http.StatusForbidden, errors.New("only an admin account can change other accounts"))
			return
		}
		var b struct {
			Password string `json:"password"`
			Current  string `json:"current"`
			Admin    *bool  `json:"admin"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		if b.Admin != nil {
			if !me.Admin {
				fail(w, http.StatusForbidden, errors.New("only an admin account can do that"))
				return
			}
			if err := s.Users.SetAdmin(target.Name, *b.Admin); err != nil {
				fail(w, 400, err)
				return
			}
		}
		if b.Password != "" {
			if self && !s.Users.Verify(me.Name, b.Current) {
				fail(w, 400, errors.New("your current password is wrong"))
				return
			}
			keep := ""
			if c, err := r.Cookie(sessionCookie); err == nil && self {
				keep = c.Value
			}
			if err := s.Users.SetPassword(target.Name, b.Password, keep); err != nil {
				fail(w, 400, err)
				return
			}
		}
		s.Broadcast("accounts", nil)
		if me.Admin {
			writeJSON(w, s.Users.List())
		} else {
			writeJSON(w, []users.User{})
		}
	})
	mux.HandleFunc("DELETE /api/users/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !s.requireAdmin(w, r) {
			return
		}
		target, ok := s.Users.Get(r.PathValue("name"))
		if !ok {
			fail(w, 404, users.ErrNotFound)
			return
		}
		if err := s.Users.Delete(target.Name); err != nil {
			fail(w, 400, err)
			return
		}
		libMu.Lock()
		delete(s.libs, strings.ToLower(target.Name))
		libMu.Unlock()
		os.RemoveAll(s.userDir(target.Name))
		s.Broadcast("accounts", nil)
		writeJSON(w, s.Users.List())
	})

	// ---------- the account's Google (YouTube Music) sign-in ----------

	mux.HandleFunc("GET /api/google", func(w http.ResponseWriter, r *http.Request) {
		p := s.cookiesPath(s.user(r).Name)
		if !fileExists(p) {
			writeJSON(w, map[string]any{"linked": false})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		acc, err := s.googleAccount(ctx, p, false)
		if err != nil {
			writeJSON(w, map[string]any{"linked": true, "error": firstLine(err.Error())})
			return
		}
		writeJSON(w, map[string]any{"linked": true, "account": acc})
	})
	mux.HandleFunc("PUT /api/google", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		if err != nil {
			fail(w, 400, err)
			return
		}
		text, err := cleanCookies(string(body))
		if err != nil {
			fail(w, 400, err)
			return
		}
		p := s.cookiesPath(s.user(r).Name)
		os.MkdirAll(filepath.Dir(p), 0o700)
		tmp := p + ".new"
		if err := os.WriteFile(tmp, []byte(text), 0o600); err != nil {
			fail(w, 500, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		acc, err := s.googleAccount(ctx, tmp, true)
		if err != nil {
			os.Remove(tmp)
			fail(w, 400, fmt.Errorf("YouTube Music didn't accept these cookies: %s", firstLine(err.Error())))
			return
		}
		if err := os.Rename(tmp, p); err != nil {
			fail(w, 500, err)
			return
		}
		googleCache.Delete(p)
		writeJSON(w, map[string]any{"linked": true, "account": acc})
	})
	mux.HandleFunc("DELETE /api/google", func(w http.ResponseWriter, r *http.Request) {
		p := s.cookiesPath(s.user(r).Name)
		os.Remove(p)
		googleCache.Delete(p)
		writeJSON(w, map[string]any{"linked": false})
	})
	mux.HandleFunc("GET /api/google/playlists", func(w http.ResponseWriter, r *http.Request) {
		p := s.cookiesPath(s.user(r).Name)
		if !fileExists(p) {
			fail(w, 400, errors.New("link your Google account in Settings first"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		data, err := s.YT.Call(ctx, "library_playlists", map[string]string{"cookies": p})
		if err != nil {
			fail(w, 502, errors.New(firstLine(err.Error())))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// ---------- importing a playlist or album into the library ----------

	mux.HandleFunc("POST /api/import", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		kind, id, err := ParseLink(b.URL)
		if err != nil {
			fail(w, 400, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		cookies := s.cookiesPath(s.user(r).Name)
		if !fileExists(cookies) {
			cookies = ""
		}
		var got struct {
			Title  string        `json:"title"`
			Tracks []model.Track `json:"tracks"`
		}
		var data json.RawMessage
		if kind == "album" {
			data, err = s.YT.Call(ctx, "album", map[string]string{"id": id})
		} else {
			if id == "LM" && cookies == "" {
				fail(w, 400, errors.New("to import your Liked Music, link your Google account in Settings first"))
				return
			}
			args := map[string]string{"id": id, "limit": "0"}
			if cookies != "" {
				args["cookies"] = cookies
			}
			data, err = s.YT.Call(ctx, "playlist", args)
		}
		if err != nil {
			msg := firstLine(err.Error())
			if cookies == "" && kind == "playlist" {
				msg += ". If it's a private playlist, link your Google account in Settings, or make the playlist Unlisted in YouTube Music"
			}
			fail(w, 502, errors.New(msg))
			return
		}
		json.Unmarshal(data, &got)
		var tracks []model.Track
		for _, t := range got.Tracks {
			if t.Valid() {
				tracks = append(tracks, t)
			}
		}
		if len(tracks) == 0 {
			fail(w, 400, errors.New("that playlist has no songs Gundil Music can play"))
			return
		}
		name := strings.TrimSpace(b.Name)
		if name == "" {
			name = got.Title
		}
		if id == "LM" && name == "" {
			name = "Liked Music (YouTube)"
		}
		if name == "" {
			name = "Imported playlist"
		}
		p, err := s.lib(r).CreatePlaylist(name, "Imported from YouTube Music", tracks)
		if err != nil {
			fail(w, 400, err)
			return
		}
		s.stateChanged(r)
		writeJSON(w, map[string]any{"id": p.ID, "name": p.Name, "count": len(p.Tracks), "state": json.RawMessage(s.lib(r).JSON())})
	})

	// A playlist from a file (CSV, TXT or M3U), read on the device: each song
	// is a YouTube video id or a name to look for on YouTube Music.
	mux.HandleFunc("POST /api/import/file", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Name  string     `json:"name"`
			File  string     `json:"file"`
			Items []fileSong `json:"items"`
		}
		if err := readBody(r, &b); err != nil {
			fail(w, 400, err)
			return
		}
		tracks, missing, notFound, err := s.findFileSongs(r.Context(), b.Items)
		if err != nil {
			fail(w, 502, err)
			return
		}
		if len(tracks) == 0 {
			fail(w, 400, errors.New("none of the songs in that file were found on YouTube Music"))
			return
		}
		name := strings.TrimSpace(b.Name)
		if name == "" {
			name = "Imported playlist"
		}
		about := "Imported from a file"
		if f := strings.TrimSpace(b.File); f != "" {
			about = "Imported from " + clipText(f, 80)
		}
		p, err := s.lib(r).CreatePlaylist(clipText(name, 150), about, tracks)
		if err != nil {
			fail(w, 400, err)
			return
		}
		s.stateChanged(r)
		writeJSON(w, map[string]any{"id": p.ID, "name": p.Name, "count": len(p.Tracks), "total": len(b.Items),
			"missing": missing, "notFound": notFound, "state": json.RawMessage(s.lib(r).JSON())})
	})
}

// fileSong is one song of a playlist file: a YouTube video id, or a name
// (title and artist) to look for, with its length when the file says.
type fileSong struct {
	VideoID  string  `json:"videoId,omitempty"`
	Q        string  `json:"q,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

const maxFileSongs = 2000

// findFileSongs looks a file's songs up on YouTube Music, in order, leaving
// out repeats. missing names (up to 50 of) the notFound songs nothing was
// found for.
func (s *Server) findFileSongs(ctx context.Context, songs []fileSong) (tracks []model.Track, missing []string, notFound int, err error) {
	if len(songs) == 0 {
		return nil, nil, 0, errors.New("no songs found in that file")
	}
	if len(songs) > maxFileSongs {
		return nil, nil, 0, fmt.Errorf("that file has %d songs; Gundil Music imports up to %d at once", len(songs), maxFileSongs)
	}
	labels := make([]string, len(songs)) // how the file names each song, for missing
	for i := range songs {
		songs[i].Q = clipText(strings.TrimSpace(songs[i].Q), 300)
		labels[i] = cmp.Or(songs[i].Q, clipText(songs[i].VideoID, 40))
		if !model.ValidID(songs[i].VideoID) {
			songs[i].VideoID = ""
		}
	}
	list, _ := json.Marshal(songs)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	data, err := s.YT.Call(ctx, "find", map[string]string{"items": string(list)})
	if err != nil {
		return nil, nil, 0, errors.New(firstLine(err.Error()))
	}
	var got struct {
		Tracks []*model.Track `json:"tracks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		return nil, nil, 0, err
	}
	seen := map[string]bool{}
	for i := range songs {
		var t *model.Track
		if i < len(got.Tracks) {
			t = got.Tracks[i]
		}
		if t == nil || !t.Valid() {
			notFound++
			if len(missing) < 50 {
				missing = append(missing, labels[i])
			}
			continue
		}
		if !seen[t.VideoID] {
			seen[t.VideoID] = true
			tracks = append(tracks, *t)
		}
	}
	return tracks, missing, notFound, nil
}

// clipText cuts s to at most n runes.
func clipText(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---------- Google cookies ----------

func (s *Server) cookiesPath(name string) string {
	return filepath.Join(s.userDir(name), "google-cookies.txt")
}

// accountCookies lists the linked Google accounts a stream may use when
// YouTube asks to sign in: the asker's own first, then the admins'.
func (s *Server) accountCookies(r *http.Request) []string {
	if u := s.user(r).Name; u != "" {
		return s.cookiesFor(u)
	}
	return s.playerCookies() // the server's own player
}

// playerCookies: the accounts the server's own player may play with.
func (s *Server) playerCookies() []string {
	if s.accountsOn() {
		return s.cookiesFor(s.playerAccount())
	}
	return s.cookiesFor("")
}

// cookiesFor lists name's linked account first, then the admins'.
func (s *Server) cookiesFor(first string) []string {
	var names []string
	if first != "" || !s.accountsOn() {
		names = append(names, first)
	}
	if s.accountsOn() {
		names = append(names, s.Users.Admins()...)
	}
	var out []string
	seen := map[string]bool{}
	for _, n := range names {
		p := s.cookiesPath(n)
		if !seen[p] && fileExists(p) {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

var googleCache sync.Map // cookies path -> googleCached

type googleCached struct {
	acc  map[string]any
	time time.Time
}

func (s *Server) googleAccount(ctx context.Context, path string, fresh bool) (map[string]any, error) {
	if c, ok := googleCache.Load(path); ok && !fresh && time.Since(c.(googleCached).time) < time.Hour {
		return c.(googleCached).acc, nil
	}
	data, err := s.YT.Call(ctx, "account", map[string]string{"cookies": path})
	if err != nil {
		return nil, err
	}
	var acc map[string]any
	json.Unmarshal(data, &acc)
	googleCache.Store(path, googleCached{acc, time.Now()})
	return acc, nil
}

// cleanCookies checks a cookies.txt export and keeps only YouTube and
// Google cookies, in the format yt-dlp and Python read.
func cleanCookies(text string) (string, error) {
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	n, signedIn := 0, false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		httpOnly := strings.HasPrefix(line, "#HttpOnly_")
		if line == "" || (strings.HasPrefix(line, "#") && !httpOnly) {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 7 {
			continue
		}
		domain := strings.TrimPrefix(f[0], "#HttpOnly_")
		d := strings.TrimPrefix(domain, ".")
		if d != "youtube.com" && !strings.HasSuffix(d, ".youtube.com") && d != "google.com" && !strings.HasSuffix(d, ".google.com") {
			continue
		}
		if f[5] == "__Secure-3PAPISID" && strings.HasSuffix(d, "youtube.com") {
			signedIn = true
		}
		b.WriteString(line + "\n")
		n++
	}
	if n == 0 {
		return "", errors.New("no YouTube cookies found. Export cookies.txt while music.youtube.com is open")
	}
	if !signedIn {
		return "", errors.New("these cookies aren't signed in to YouTube. Sign in at music.youtube.com, then export cookies.txt again")
	}
	return b.String(), nil
}

// ---------- links ----------

var (
	playlistID = regexp.MustCompile(`^(PL|OLAK5uy_|RDCLAK|VL|UU|LL|FL|LM$)[A-Za-z0-9_-]*$`)
	albumID    = regexp.MustCompile(`^MPREb_[A-Za-z0-9_-]+$`)
)

// ParseLink understands YouTube and YouTube Music links to playlists and
// albums, and bare playlist IDs. kind is "playlist" or "album".
func ParseLink(link string) (kind, id string, err error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", "", errors.New("paste a link to a YouTube Music playlist or album")
	}
	if playlistID.MatchString(link) {
		return "playlist", strings.TrimPrefix(link, "VL"), nil
	}
	if albumID.MatchString(link) {
		return "album", link, nil
	}
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	u, perr := url.Parse(link)
	if perr != nil || !(strings.HasSuffix(u.Hostname(), "youtube.com") || u.Hostname() == "youtu.be") {
		return "", "", errors.New("that isn't a YouTube or YouTube Music link")
	}
	if l := u.Query().Get("list"); l != "" {
		if strings.HasPrefix(l, "RD") && !strings.HasPrefix(l, "RDCLAK") {
			return "", "", errors.New("that's a mix made by YouTube for you, which can't be imported; open it and save the songs you like")
		}
		return "playlist", strings.TrimPrefix(l, "VL"), nil
	}
	if rest, ok := strings.CutPrefix(u.Path, "/browse/"); ok {
		switch {
		case albumID.MatchString(rest):
			return "album", rest, nil
		case strings.HasPrefix(rest, "VL"):
			return "playlist", strings.TrimPrefix(rest, "VL"), nil
		}
	}
	if u.Query().Get("v") != "" || u.Hostname() == "youtu.be" {
		return "", "", errors.New("that's a link to a song, not a playlist")
	}
	return "", "", errors.New("that link isn't to a playlist or an album")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimPrefix(strings.TrimPrefix(s, "RuntimeError: "), "Exception: ")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
