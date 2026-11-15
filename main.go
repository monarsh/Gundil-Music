// Gundil Music is a self-hosted music streaming app: one small server that
// indexes your music folders, serves a web app to any device, and can play
// through its own speakers or a Bluetooth speaker when it runs headless.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/monarsh/gundil-music/internal/bluetooth"
	"github.com/monarsh/gundil-music/internal/library"
	"github.com/monarsh/gundil-music/internal/player"
	"github.com/monarsh/gundil-music/internal/server"
	"github.com/monarsh/gundil-music/internal/store"
)

//go:embed web
var webFiles embed.FS

var version = "dev"

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	home, _ := os.UserHomeDir()
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = home
	}

	music := flag.String("music", env("GUNDIL_MUSIC", filepath.Join(home, "Music")),
		"music folder(s), separated by "+string(os.PathListSeparator))
	addr := flag.String("addr", env("GUNDIL_ADDR", ":8338"), "address to listen on")
	dataDir := flag.String("data", env("GUNDIL_DATA", filepath.Join(configDir, "gundil-music")),
		"folder for playlists and the library cache")
	auth := flag.String("auth", env("GUNDIL_AUTH", ""), "require a login, as user:password")
	mpvPath := flag.String("mpv", env("GUNDIL_MPV", ""), "path to mpv (default: look in PATH)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data folder: %v", err)
	}

	var roots []string
	for _, r := range filepath.SplitList(*music) {
		if r = strings.TrimSpace(r); r != "" {
			roots = append(roots, r)
		}
	}

	lib := library.New(roots, filepath.Join(*dataDir, "library.json"))
	st := store.New(filepath.Join(*dataDir, "state.json"))
	pl := player.New(*mpvPath)
	bt := bluetooth.New()

	web, _ := fs.Sub(webFiles, "web")
	srv := &server.Server{Lib: lib, Store: st, Player: pl, BT: bt, Web: web, Version: version, Auth: *auth}

	pl.Resolve = lib.Path
	pl.OnChange = srv.BroadcastPlayer
	pl.OnPlay = st.RecordPlay
	pl.OnDuration = lib.SetDuration
	lib.OnChange = func() { srv.Broadcast("library", map[string]int{"tracks": lib.Count()}) }

	handler := srv.Handler()
	go lib.Scan()
	go func() {
		for range time.Tick(time.Minute) {
			lib.Flush()
		}
	}()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("cannot listen on %s: %v", *addr, err)
	}
	log.Printf("Gundil Music %s", version)
	log.Printf("music folders: %s", strings.Join(roots, ", "))
	if !pl.Available() {
		log.Printf("mpv not found: playing through this machine's speakers is disabled (browser playback still works)")
	}
	for _, u := range urls(ln.Addr().(*net.TCPAddr)) {
		log.Printf("open %s", u)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		pl.Close()
		lib.Flush()
		os.Exit(0)
	}()

	log.Fatal(http.Serve(ln, handler))
}

// urls lists the addresses the app can be reached at.
func urls(a *net.TCPAddr) []string {
	if a.IP != nil && !a.IP.IsUnspecified() {
		return []string{fmt.Sprintf("http://%s", a)}
	}
	out := []string{fmt.Sprintf("http://localhost:%d", a.Port)}
	addrs, _ := net.InterfaceAddrs()
	for _, ia := range addrs {
		if n, ok := ia.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
			out = append(out, fmt.Sprintf("http://%s:%d", n.IP, a.Port))
		}
	}
	return out
}
