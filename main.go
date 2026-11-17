// Gundil Music is a YouTube Music style player you host yourself: one small
// server that streams the YouTube Music catalogue to a web app on every
// device, and can play through its own speakers or a Bluetooth speaker when
// it runs headless on a Raspberry Pi or Orange Pi.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/monarsh/gundil-music/internal/bluetooth"
	"github.com/monarsh/gundil-music/internal/model"
	"github.com/monarsh/gundil-music/internal/player"
	"github.com/monarsh/gundil-music/internal/server"
	"github.com/monarsh/gundil-music/internal/store"
	"github.com/monarsh/gundil-music/internal/yt"
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

	addr := flag.String("addr", env("GUNDIL_ADDR", ":8338"), "address to listen on")
	dataDir := flag.String("data", env("GUNDIL_DATA", filepath.Join(configDir, "gundil-music")),
		"folder for your library (likes, playlists, history)")
	auth := flag.String("auth", env("GUNDIL_AUTH", ""), "require a login, as user:password")
	python := flag.String("python", env("GUNDIL_PYTHON", ""), "Python 3 with ytmusicapi and yt-dlp (default: python3 in PATH)")
	helper := flag.String("helper", env("GUNDIL_HELPER", ""), "use this helper script instead of the built-in one")
	mpvPath := flag.String("mpv", env("GUNDIL_MPV", ""), "path to mpv (default: look in PATH)")
	cookies := flag.String("cookies", env("GUNDIL_COOKIES", ""), "cookies.txt for YouTube, if it asks this server to sign in")
	open := flag.Bool("open", runtime.GOOS == "windows", "open the app in the web browser once started")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("cannot create data folder: %v", err)
	}

	if *cookies == "" {
		if p := filepath.Join(*dataDir, "cookies.txt"); fileExists(p) {
			*cookies = p
		}
	}
	if *cookies != "" {
		abs, _ := filepath.Abs(*cookies)
		os.Setenv("GUNDIL_COOKIES", abs) // read by the helper
		log.Printf("using YouTube cookies from %s", abs)
	}
	ytc, err := yt.New(*python, *helper, *dataDir)
	if err != nil {
		log.Fatalf("cannot set up the YouTube helper: %v", err)
	}
	st := store.New(filepath.Join(*dataDir, "library.json"))
	if *mpvPath == "" {
		// The Windows zip and the AppImage may carry mpv next to the program.
		if exe, err := os.Executable(); err == nil {
			for _, name := range []string{"mpv.exe", "mpv"} {
				if p := filepath.Join(filepath.Dir(exe), name); fileExists(p) {
					*mpvPath = p
					break
				}
			}
		}
	}
	pl := player.New(*mpvPath)
	bt := bluetooth.New()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("cannot listen on %s: %v", *addr, err)
	}
	tcp := ln.Addr().(*net.TCPAddr)

	web, _ := fs.Sub(webFiles, "web")
	srv := &server.Server{YT: ytc, Store: st, Player: pl, BT: bt, Web: web, Version: version, Auth: *auth, Port: tcp.Port}

	pl.Resolve = srv.StreamURL
	pl.OnChange = srv.BroadcastPlayer
	pl.OnPlay = st.AddHistory
	pl.Prefetch = func(id string) { ytc.Stream(context.Background(), id, false) }
	pl.More = func(last model.Track) []model.Track {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		data, err := ytc.Call(ctx, "watch", map[string]string{"videoId": last.VideoID, "radio": "1"})
		if err != nil {
			log.Printf("autoplay: %v", err)
			return nil
		}
		var w struct {
			Tracks []model.Track `json:"tracks"`
		}
		json.Unmarshal(data, &w)
		out := w.Tracks[:0]
		for _, t := range w.Tracks {
			if !st.Disliked(t.VideoID) {
				out = append(out, t)
			}
		}
		return out
	}

	handler := srv.Handler()

	log.Printf("Gundil Music %s", version)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		status := ytc.Status(ctx)
		if e, ok := status["error"]; ok {
			log.Printf("YouTube Music is not ready: %v", e)
		} else if w, ok := status["warning"]; ok {
			log.Printf("warning: %v", w)
		} else {
			log.Printf("YouTube Music ready (ytmusicapi %v, yt-dlp %v, js %v)", status["ytmusicapi"], status["yt_dlp"], status["js"])
		}
	}()
	if !pl.Available() {
		log.Printf("mpv not found: playing through this machine's speakers is disabled (browser playback still works)")
	}
	for _, u := range urls(tcp) {
		log.Printf("open %s", u)
	}
	if *open {
		go openBrowser(fmt.Sprintf("http://localhost:%d", tcp.Port))
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		pl.Close()
		ytc.Close()
		os.Exit(0)
	}()

	log.Fatal(http.Serve(ln, handler))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// openBrowser shows the app in the default browser (desktop builds).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open a browser: %v", err)
		return
	}
	go cmd.Wait()
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
