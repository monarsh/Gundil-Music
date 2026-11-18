package remote

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"192.168.1.20":               "http://192.168.1.20:8338",
		" pi.local:9000 ":            "http://pi.local:9000",
		"http://host:8338/some/path": "http://host:8338",
		"https://music.example.com":  "https://music.example.com",
		"http://user:pw@10.0.0.2":    "http://10.0.0.2:8338",
	} {
		if got, err := Normalize(in); err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x", "http://"} {
		if _, err := Normalize(bad); err == nil {
			t.Errorf("Normalize(%q) should fail", bad)
		}
	}
}

func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/info":
			w.Write([]byte(`{"name":"Gundil Music","hostname":"pi","version":"v1"}`))
		default:
			w.Write([]byte("remote " + r.URL.Path))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, h http.Handler, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5000"
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSwitchModes(t *testing.T) {
	remote := fakeServer(t)
	path := filepath.Join(t.TempDir(), "mode.json")
	s := New(path)
	s.Local = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "local "+r.URL.Path) })
	s.Pages = fstest.MapFS{"connect.html": {Data: []byte("connect page")}}
	var changes []Config
	s.OnChange = func(old, now Config) { changes = append(changes, now) }

	if s.Get().Mode != Server {
		t.Fatalf("default mode %q", s.Get().Mode)
	}
	if w := get(t, s, "GET", "/api/x", "", nil); w.Body.String() != "local /api/x" {
		t.Fatalf("server mode served %q", w.Body.String())
	}

	// No X-Gundil header: refused, so other web sites can't switch it.
	if w := get(t, s, "PUT", "/_gundil/mode", `{"mode":"client","server":"`+remote.URL+`"}`, nil); w.Code != 403 {
		t.Fatalf("change without header: %d", w.Code)
	}
	h := map[string]string{"X-Gundil": "1"}
	if w := get(t, s, "PUT", "/_gundil/mode", `{"mode":"client","server":"`+remote.URL+`"}`, h); w.Code != 200 {
		t.Fatalf("connect: %d %s", w.Code, w.Body)
	}
	w := get(t, s, "GET", "/api/x", "", nil)
	if w.Body.String() != "remote /api/x" || w.Header().Get(proxyHeader) == "" || len(w.Header().Values(instanceHeader)) != 1 {
		t.Fatalf("client mode served %q, headers %v", w.Body.String(), w.Header())
	}
	if w := get(t, s, "GET", "/_gundil/", "", nil); w.Body.String() != "connect page" {
		t.Fatalf("own page: %q", w.Body.String())
	}

	// The mode is remembered.
	if again := New(path); again.Get() != s.Get() {
		t.Fatalf("reloaded %+v, want %+v", again.Get(), s.Get())
	}

	// Another device can't change it without the password.
	r := httptest.NewRequest("PUT", "/_gundil/mode", strings.NewReader(`{"mode":"server"}`))
	r.RemoteAddr = "192.168.1.50:5000"
	r.Header.Set("X-Gundil", "1")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatalf("change from another device: %d", rec.Code)
	}
	s.Auth = "me:secret"
	r = httptest.NewRequest("PUT", "/_gundil/mode", strings.NewReader(`{"mode":"server"}`))
	r.RemoteAddr = "192.168.1.50:5000"
	r.Header.Set("X-Gundil", "1")
	r.SetBasicAuth("me", "secret")
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, r)
	if rec.Code != 200 || s.Get().Mode != Server || s.Get().Server == "" {
		t.Fatalf("change with password: %d %+v", rec.Code, s.Get())
	}
	if len(changes) != 2 {
		t.Fatalf("OnChange ran %d times", len(changes))
	}

	// Connecting to itself is refused.
	self := httptest.NewServer(s)
	defer self.Close()
	if w := get(t, s, "PUT", "/_gundil/mode", `{"mode":"client","server":"`+self.URL+`"}`, h); w.Code != 400 || !strings.Contains(w.Body.String(), "this computer") {
		t.Fatalf("connect to itself: %d %s", w.Code, w.Body)
	}
}

func TestClientServerDown(t *testing.T) {
	remote := fakeServer(t)
	s := New(filepath.Join(t.TempDir(), "mode.json"))
	if _, err := s.Set(Config{Mode: Client, Server: remote.URL}); err != nil {
		t.Fatal(err)
	}
	remote.Close()
	w := get(t, s, "GET", "/", "", map[string]string{"Accept": "text/html"})
	if w.Code != http.StatusSeeOther || !strings.HasPrefix(w.Header().Get("Location"), "/_gundil/?error=") {
		t.Fatalf("page with server down: %d %v", w.Code, w.Header())
	}
	if w := get(t, s, "GET", "/api/state", "", nil); w.Code != http.StatusBadGateway {
		t.Fatalf("api with server down: %d", w.Code)
	}
}
