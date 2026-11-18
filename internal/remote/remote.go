// Package remote lets one Gundil Music program work either as the server
// (it fetches YouTube Music and keeps the library) or as a client of
// another Gundil Music server on the network. As a client it passes every
// request on to that server, so the browser on this computer shows the
// server's library and can play through this computer's speakers, the
// server's speakers or the server's Bluetooth speaker.
//
// The pages and API under /_gundil/ always belong to this program: they show
// and change the mode, and find servers on the local network.
package remote

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	Server = "server"
	Client = "client"

	DefaultPort = 8338

	// Every response carries this program's id, so it can tell when it is
	// asked to connect to itself.
	instanceHeader = "X-Gundil-Instance"
	// Responses passed on from another server carry this, so scans list
	// real servers rather than other clients.
	proxyHeader = "X-Gundil-Proxy"
)

// Config is what is saved in <data>/mode.json.
type Config struct {
	Mode   string `json:"mode"`             // "server" (default) or "client"
	Server string `json:"server,omitempty"` // the server used in client mode, like http://192.168.1.20:8338
}

// Found is a server seen on the network.
type Found struct {
	URL      string `json:"url"`
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
}

// Switch is the program's HTTP handler: the server's own handler in server
// mode, a pass-through to the chosen server in client mode.
type Switch struct {
	Local   http.Handler // the server, used in server mode
	Pages   fs.FS        // holds connect.html
	Version string
	Auth    string // "user:password": lets other devices change the mode when they know it
	Port    int
	// OnChange runs after the mode or the server changed.
	OnChange func(old, now Config)

	path     string
	instance string

	mu    sync.RWMutex
	cfg   Config
	proxy *httputil.ReverseProxy
}

// New loads the saved mode from path (server mode if there is none).
func New(path string) *Switch {
	b := make([]byte, 8)
	rand.Read(b)
	s := &Switch{path: path, instance: hex.EncodeToString(b), cfg: Config{Mode: Server}}
	if data, err := os.ReadFile(path); err == nil {
		var c Config
		if json.Unmarshal(data, &c) == nil && (c.Mode == Server || c.Mode == Client) {
			s.cfg = c
		}
	}
	if s.cfg.Mode == Client {
		if u, err := Normalize(s.cfg.Server); err == nil {
			s.cfg.Server = u
			s.proxy = newProxy(u)
		} else {
			s.cfg = Config{Mode: Server}
		}
	}
	return s
}

// Get returns the current mode.
func (s *Switch) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Set changes the mode and saves it. A client needs a server address; it is
// not checked here (see Check).
func (s *Switch) Set(c Config) (Config, error) {
	switch c.Mode {
	case Server:
		c.Server = strings.TrimSpace(c.Server)
		if c.Server != "" {
			if u, err := Normalize(c.Server); err == nil {
				c.Server = u // remembered for the next time client mode is chosen
			}
		}
	case Client:
		u, err := Normalize(c.Server)
		if err != nil {
			return s.Get(), err
		}
		c.Server = u
	default:
		return s.Get(), fmt.Errorf("unknown mode %q", c.Mode)
	}
	if c.Server == "" {
		c.Server = s.Get().Server
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return s.Get(), err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return s.Get(), err
	}
	s.mu.Lock()
	old := s.cfg
	s.cfg = c
	s.proxy = nil
	if c.Mode == Client {
		s.proxy = newProxy(c.Server)
	}
	s.mu.Unlock()
	if s.OnChange != nil && old != c {
		s.OnChange(old, c)
	}
	return c, nil
}

// Normalize turns what people type ("192.168.1.20", "pi.local:9000",
// "http://host:8338/") into a base URL like http://192.168.1.20:8338.
func Normalize(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("enter the server address, like 192.168.1.20")
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("%q is not a server address", addr)
	}
	if u.Port() == "" && u.Scheme == "http" {
		u.Host = net.JoinHostPort(u.Hostname(), fmt.Sprint(DefaultPort))
	}
	u.User = nil
	return u.Scheme + "://" + u.Host, nil
}

// Check asks base for its /api/info and says what it found.
func (s *Switch) Check(ctx context.Context, base string) (Found, error) {
	f, hdr, err := probe(ctx, base)
	if hdr != nil && hdr.Get(instanceHeader) == s.instance {
		return f, errors.New("that address is this computer; enter the address of the server")
	}
	if err != nil {
		return f, err
	}
	if hdr.Get(proxyHeader) != "" {
		return f, fmt.Errorf("%s is a client of another server, not a server itself", base)
	}
	return f, nil
}

var probeClient = &http.Client{Timeout: 25 * time.Second}

func probe(ctx context.Context, base string) (Found, http.Header, error) {
	f := Found{URL: base}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/api/info", nil)
	if err != nil {
		return f, nil, err
	}
	res, err := probeClient.Do(req)
	if err != nil {
		return f, nil, fmt.Errorf("no Gundil Music server answered at %s", base)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		// A server with a password still answers; the browser asks for it.
		f.Hostname = strings.TrimPrefix(strings.TrimPrefix(base, "http://"), "https://")
		return f, res.Header, nil
	}
	var info struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
		Version  string `json:"version"`
	}
	if res.StatusCode != 200 || json.NewDecoder(res.Body).Decode(&info) != nil || info.Name != "Gundil Music" {
		return f, res.Header, fmt.Errorf("%s is not a Gundil Music server", base)
	}
	f.Hostname, f.Version = info.Hostname, info.Version
	return f, res.Header, nil
}

// Scan looks for servers on this computer's local networks (each /24).
func (s *Switch) Scan(ctx context.Context) []Found {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var bases []string
	seen := map[string]bool{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.IsLoopback() || n.IP.To4() == nil || !n.IP.IsPrivate() {
			continue
		}
		ip := n.IP.To4()
		for i := 1; i < 255; i++ {
			b := fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], i)
			if !seen[b] {
				seen[b] = true
				bases = append(bases, b)
			}
		}
	}
	var mu sync.Mutex
	var out []Found
	sem := make(chan struct{}, 64)
	var wg sync.WaitGroup
	for _, host := range bases {
		wg.Add(1)
		sem <- struct{}{}
		go func(host string) {
			defer wg.Done()
			defer func() { <-sem }()
			addr := net.JoinHostPort(host, fmt.Sprint(DefaultPort))
			d := net.Dialer{Timeout: 700 * time.Millisecond}
			c, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				return
			}
			c.Close()
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			defer pcancel()
			f, hdr, err := probe(pctx, "http://"+addr)
			if err != nil || hdr == nil || hdr.Get(instanceHeader) == s.instance || hdr.Get(proxyHeader) != "" {
				return
			}
			mu.Lock()
			out = append(out, f)
			mu.Unlock()
		}(host)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

func newProxy(base string) *httputil.ReverseProxy {
	target, _ := url.Parse(base)
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
		},
		FlushInterval: -1, // live updates (/api/events) and audio go out as they arrive
		ModifyResponse: func(res *http.Response) error {
			res.Header.Del(instanceHeader) // this program's own id is set already
			res.Header.Set(proxyHeader, base)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if errors.Is(r.Context().Err(), context.Canceled) {
				return // the browser went away
			}
			log.Printf("server %s: %v", base, err)
			msg := fmt.Sprintf("Can't reach the server at %s. Check that it is running and on the same network.", base)
			if r.Method == "GET" && strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/_gundil/?error="+url.QueryEscape(msg), http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": msg})
		},
	}
}

// mayChange says whether this request may change the mode: from this
// computer itself, or with the server's password when one is set.
func (s *Switch) mayChange(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	if s.Auth == "" {
		return false
	}
	user, pass, _ := strings.Cut(s.Auth, ":")
	u, p, ok := r.BasicAuth()
	return ok && subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
}

func (s *Switch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(instanceHeader, s.instance)
	if r.URL.Path == "/_gundil" || strings.HasPrefix(r.URL.Path, "/_gundil/") {
		s.own(w, r)
		return
	}
	s.mu.RLock()
	p := s.proxy
	s.mu.RUnlock()
	if p != nil {
		p.ServeHTTP(w, r)
		return
	}
	s.Local.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *Switch) own(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/_gundil"), "/")
	// Changes need a header a web page on another site can't add without
	// asking first, so such a page can't switch this program's mode.
	changing := r.Method != "GET" && r.Method != "HEAD"
	if changing && (r.Header.Get("X-Gundil") == "" || !s.mayChange(r)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "the mode can only be changed on the computer running Gundil Music"})
		return
	}
	switch {
	case path == "" || path == "connect.html":
		data, err := fs.ReadFile(s.Pages, "connect.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	case path == "icon.svg":
		data, _ := fs.ReadFile(s.Pages, "icon.svg")
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	case path == "mode" && r.Method == "GET":
		c := s.Get()
		host, _ := os.Hostname()
		writeJSON(w, 200, map[string]any{
			"mode": c.Mode, "server": c.Server, "canChange": s.mayChange(r),
			"version": s.Version, "hostname": host, "addresses": Addresses(s.Port),
		})
	case path == "mode" && r.Method == "PUT":
		var c Config
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&c); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if c.Mode == Client {
			u, err := Normalize(c.Server)
			if err == nil {
				c.Server = u
				_, err = s.Check(r.Context(), u)
			}
			if err != nil {
				writeJSON(w, 400, map[string]string{"error": err.Error()})
				return
			}
		}
		now, err := s.Set(c)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, now)
	case path == "scan" && r.Method == "GET":
		list := s.Scan(r.Context())
		if list == nil {
			list = []Found{}
		}
		writeJSON(w, 200, list)
	default:
		http.NotFound(w, r)
	}
}

// Addresses lists the addresses other devices can reach this computer at.
func Addresses(port int) []string {
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, ia := range addrs {
		if n, ok := ia.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			out = append(out, fmt.Sprintf("http://%s:%d", n.IP, port))
		}
	}
	return out
}

// DefaultPath is where the mode is saved inside the data folder.
func DefaultPath(dataDir string) string { return filepath.Join(dataDir, "mode.json") }
