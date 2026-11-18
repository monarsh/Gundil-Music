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
	"strings"
	"syscall"
	"time"

	"github.com/monarsh/gundil-music/internal/bluetooth"
	"github.com/monarsh/gundil-music/internal/model"
	"github.com/monarsh/gundil-music/internal/player"
	"github.com/monarsh/gundil-music/internal/remote"
	"github.com/monarsh/gundil-music/internal/server"
	"github.com/monarsh/gundil-music/internal/settings"
	"github.com/monarsh/gundil-music/internal/store"
	"github.com/monarsh/gundil-music/internal/users"
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
	updateNow := flag.Bool("update-youtube", false, "update yt-dlp and ytmusicapi now, then exit")
	autoUpdate := flag.Bool("auto-update", env("GUNDIL_AUTO_UPDATE", "true") != "false", "update yt-dlp and ytmusicapi daily when newer versions exist")
	showVersion := flag.Bool("version", false, "print the version and exit")
	resetPassword := flag.String("reset-password", "", "set a Gundil Music account's password, as name:newpassword, then exit")
	modeFlag := flag.String("mode", env("GUNDIL_MODE", ""), "server, or client of the server given with -connect (remembered; also changeable in the app)")
	connect := flag.String("connect", env("GUNDIL_CONNECT", ""), "be a client of the Gundil Music server at this address, like 192.168.1.20 (remembered)")
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
	accts := users.Load(filepath.Join(*dataDir, "users.json"))
	if *resetPassword != "" {
		name, pw, _ := strings.Cut(*resetPassword, ":")
		if err := accts.SetPassword(name, pw, ""); err != nil {
			fmt.Fprintln(os.Stderr, "cannot set the password:", err)
			os.Exit(1)
		}
		fmt.Printf("Password for %s changed; it is signed out on every device.\n", name)
		return
	}
	// Settings chosen in the app; the command-line options are the defaults.
	cfg := settings.Load(filepath.Join(*dataDir, "settings.json"), settings.Settings{
		Region: os.Getenv("GUNDIL_REGION"), Language: os.Getenv("GUNDIL_LANG"), AutoUpdate: *autoUpdate,
	})
	applyEnv := func(c settings.Settings) {
		os.Setenv("GUNDIL_REGION", c.Region) // read by the helper
		os.Setenv("GUNDIL_LANG", c.Language)
	}
	applyEnv(cfg.Get())
	ytc, err := yt.New(*python, *helper, *dataDir)
	if err != nil {
		log.Fatalf("cannot set up the YouTube helper: %v", err)
	}
	if *updateNow {
		fmt.Println("Updating YouTube support (yt-dlp, ytmusicapi)...")
		r, err := ytc.Update(context.Background(), true)
		ytc.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%s: yt-dlp %s, ytmusicapi %s\n", r.Message, r.Versions["yt-dlp"], r.Versions["ytmusicapi"])
		fmt.Println("Restart Gundil Music if it is running, so it uses the new versions.")
		return
	}
	ytc.SetAudioQuality(cfg.Get().AudioQuality)
	ytc.SetCacheFor(cfg.Get().Region + "|" + cfg.Get().Language)
	ytc.LoadCache() // pages from the last run show at once while fresh ones load
	stopSaving := make(chan struct{})
	savedCache := make(chan struct{})
	go func() { ytc.KeepCacheSaved(stopSaving); close(savedCache) }()
	saveCache := func() {
		close(stopSaving)
		select {
		case <-savedCache:
		case <-time.After(5 * time.Second):
		}
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
		// Started again while already running (closing the browser window
		// leaves the server on): show the running one instead of failing.
		if u := runningAt(*addr); u != "" {
			log.Printf("Gundil Music is already running at %s", u)
			if *open {
				openBrowser(u)
				time.Sleep(time.Second) // let the browser start before exiting
			}
			return
		}
		log.Fatalf("cannot listen on %s: %v", *addr, err)
	}
	tcp := ln.Addr().(*net.TCPAddr)

	web, _ := fs.Sub(webFiles, "web")
	srv := &server.Server{YT: ytc, Store: st, Player: pl, BT: bt, Web: web, Version: version, Auth: *auth, Port: tcp.Port, Settings: cfg,
		Users: accts, DataDir: *dataDir}
	srv.OnSettings = func(old, now settings.Settings) {
		ytc.SetAudioQuality(now.AudioQuality)
		if old.Region != now.Region || old.Language != now.Language {
			applyEnv(now)
			ytc.SetCacheFor(now.Region + "|" + now.Language)
			go ytc.Restart(false) // results in the new region and language
		}
	}

	pl.Resolve = srv.StreamURL
	pl.OnChange = srv.BroadcastPlayer
	pl.OnPlay = func(t model.Track) { srv.PlayerLibrary().AddHistory(t) }
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
			if !srv.PlayerLibrary().Disliked(t.VideoID) {
				out = append(out, t)
			}
		}
		return out
	}

	sw := remote.New(remote.DefaultPath(*dataDir))
	sw.Local, sw.Version, sw.Auth, sw.Port = srv.Handler(), version, *auth, tcp.Port
	sw.Pages, _ = fs.Sub(webFiles, "web")
	if *connect != "" || *modeFlag != "" {
		c := sw.Get()
		if *connect != "" {
			c = remote.Config{Mode: remote.Client, Server: *connect}
		}
		if *modeFlag != "" {
			c.Mode = *modeFlag
		}
		if _, err := sw.Set(c); err != nil {
			log.Fatalf("cannot set the mode: %v", err)
		}
	}
	sw.OnQuit = func() {
		log.Printf("quitting (asked from the app)")
		pl.Close()
		saveCache()
		ytc.Close()
		os.Exit(0)
	}
	sw.OnChange = func(old, now remote.Config) {
		if now.Mode == remote.Client {
			pl.Stop() // this computer's speakers now belong to the other server
			log.Printf("now a client of the server at %s", now.Server)
		} else {
			log.Printf("now the server")
			go logYouTubeStatus(ytc)
		}
	}
	serving := func() bool { return sw.Get().Mode == remote.Server }

	log.Printf("Gundil Music %s", version)
	go func() {
		// YouTube changes often; keep yt-dlp current. First check shortly
		// after start, then once a day, unless turned off in Settings.
		time.Sleep(3 * time.Minute)
		for {
			if cfg.Get().AutoUpdate && serving() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
				r, err := ytc.Update(ctx, false)
				cancel()
				if err != nil {
					log.Printf("YouTube auto-update: %v", err)
				} else {
					log.Printf("YouTube auto-update: %s", r.Message)
					if r.Updated {
						srv.Broadcast("info", map[string]bool{"youtubeUpdated": true})
					}
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}()
	if serving() {
		go func() {
			logYouTubeStatus(ytc)
			srv.Warm() // Home and Explore ready before anyone opens them
		}()
	} else {
		log.Printf("client mode: using the server at %s (change it at http://localhost:%d/_gundil/ or with -mode server)", sw.Get().Server, tcp.Port)
	}
	if serving() && !pl.Available() {
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
		saveCache()
		ytc.Close()
		os.Exit(0)
	}()

	log.Fatal(http.Serve(ln, sw))
}

func logYouTubeStatus(ytc *yt.Client) {
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
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// runningAt returns the address of a Gundil Music already listening on
// addr's port on this computer, or "" if something else holds it.
func runningAt(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	u := "http://localhost:" + port
	c := http.Client{Timeout: 5 * time.Second}
	res, err := c.Get("http://127.0.0.1:" + port + "/_gundil/mode")
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.Header.Get("X-Gundil-Instance") == "" {
		return ""
	}
	return u
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
