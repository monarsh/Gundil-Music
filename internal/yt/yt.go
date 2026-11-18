// Package yt runs the Python helper (helper.py) that talks to YouTube Music
// and caches what it returns.
package yt

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed helper.py
var helperScript []byte

// Stream is a resolved audio URL for one video.
type Stream struct {
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Mime     string            `json:"mime"`
	Expires  int64             `json:"expires"`
	Duration float64           `json:"duration"`
	Size     int64             `json:"size"`
}

type reply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

type cached struct {
	data    json.RawMessage
	expires time.Time // fresh until then; afterwards reused while a new copy is fetched
	fetched time.Time
}

const (
	// A page older than its time to live is still shown straight away
	// (and fetched again behind the scenes) for this long...
	staleFor = 24 * time.Hour
	// ...and when YouTube can't be reached, a copy up to this old beats an error.
	keepFor  = 7 * 24 * time.Hour
	maxCache = 1500
)

type flight struct {
	done chan struct{}
	data json.RawMessage
	err  error
}

type Client struct {
	python  string
	script  string
	dataDir string

	updating sync.Mutex // one YouTube library update at a time

	mu      sync.Mutex
	cur     *proc          // the helper new calls go to
	old     map[*proc]bool // helpers finishing their calls after a restart
	next    int
	lastErr string

	cmu      sync.Mutex
	cache    map[string]cached
	flights  map[string]*flight
	signin   signIn
	dirty    bool   // the cache changed since it was last saved
	cacheFor string // region and language of the cached pages
	streams  map[string]*Stream
	quality  string // default audio quality
}

// New prepares a client. python may be empty to search PATH. script may be
// empty to use the built-in helper, which is written into dataDir.
func New(python, script, dataDir string) (*Client, error) {
	if python == "" {
		python = bundledPython()
	}
	if python == "" {
		names := []string{"python3", "python"}
		if runtime.GOOS == "windows" {
			// "py" only exists with a real Python install; python3.exe is often
			// just the Microsoft Store shortcut.
			names = []string{"py", "python", "python3"}
		}
		for _, name := range names {
			if p, err := exec.LookPath(name); err == nil {
				python = p
				break
			}
		}
	}
	if script == "" {
		script = filepath.Join(dataDir, "ytm_helper.py")
		if old, err := os.ReadFile(script); err != nil || string(old) != string(helperScript) {
			if err := os.WriteFile(script, helperScript, 0o644); err != nil {
				return nil, err
			}
		}
	}
	return &Client{
		python: python, script: script, dataDir: dataDir,
		old:   map[*proc]bool{},
		cache: map[string]cached{}, flights: map[string]*flight{}, streams: map[string]*Stream{},
	}, nil
}

// bundleDir returns <folder of this program>/name if it exists. The Windows
// zip and the AppImage ship Python and Deno there, so nothing else needs
// installing.
func bundleDir(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dir := filepath.Join(filepath.Dir(exe), name)
	if st, err := os.Stat(dir); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

func bundledPython() string {
	dir := bundleDir("python")
	if dir == "" {
		return ""
	}
	for _, p := range []string{filepath.Join(dir, "python.exe"), filepath.Join(dir, "bin", "python3")} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// proc is one running helper process and the calls waiting on it.
type proc struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	pending map[int]chan reply // guarded by Client.mu
	done    chan struct{}      // closed when its output ends
}

// start runs a new helper and makes it current. c.mu must be held.
func (c *Client) start() error {
	if c.python == "" {
		return errors.New("Python 3 was not found on the server. Install it, then run: pip install -U ytmusicapi \"yt-dlp[default]\"")
	}
	cmd := exec.Command(c.python, "-u", c.script)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "GUNDIL_PYLIB="+c.pylib())
	if dir := bundleDir("runtime"); dir != "" {
		// A JavaScript runtime shipped next to the program (Windows zip, AppImage).
		cmd.Env = append(cmd.Env, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the YouTube helper: %w", err)
	}
	p := &proc{cmd: cmd, stdin: stdin, pending: map[int]chan reply{}, done: make(chan struct{})}
	c.cur = p
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			log.Printf("helper: %s", sc.Text())
		}
	}()
	go func() {
		c.readLoop(p, stdout)
		close(p.done)
	}()
	return nil
}

func (c *Client) readLoop(p *proc, stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var r reply
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		c.mu.Lock()
		ch := p.pending[r.ID]
		delete(p.pending, r.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- r
		}
	}
	p.cmd.Wait()
	c.mu.Lock()
	if c.cur == p {
		c.cur = nil
		log.Printf("yt: helper exited")
	}
	delete(c.old, p)
	for id, ch := range p.pending {
		close(ch)
		delete(p.pending, id)
	}
	c.mu.Unlock()
}

// Call runs one helper method without caching.
func (c *Client) Call(ctx context.Context, method string, args map[string]string) (json.RawMessage, error) {
	c.mu.Lock()
	if c.cur == nil {
		if err := c.start(); err != nil {
			c.lastErr = err.Error()
			c.mu.Unlock()
			return nil, err
		}
	}
	p := c.cur
	c.next++
	id := c.next
	ch := make(chan reply, 1)
	p.pending[id] = ch
	msg, _ := json.Marshal(map[string]any{"id": id, "method": method, "args": args})
	_, err := p.stdin.Write(append(msg, '\n'))
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(callTimeout(method))
	defer timeout.Stop()
	select {
	case r, ok := <-ch:
		if !ok {
			return nil, errors.New("the YouTube helper stopped unexpectedly")
		}
		if r.Error != nil {
			c.mu.Lock()
			c.lastErr = *r.Error
			c.mu.Unlock()
			return nil, errors.New(*r.Error)
		}
		c.mu.Lock()
		c.lastErr = ""
		c.mu.Unlock()
		return r.Result, nil
	case <-ctx.Done():
		c.forget(p, id)
		return nil, ctx.Err()
	case <-timeout.C:
		c.forget(p, id)
		return nil, errors.New("YouTube Music took too long to answer")
	}
}

// callTimeout is how long one helper call may take. Pages answer in a few
// seconds; when one hangs it's quicker to ask again than to keep waiting.
func callTimeout(method string) time.Duration {
	switch method {
	case "playlist", "library_playlists": // importing thousands of songs
		return 5 * time.Minute
	case "stream", "ping", "options":
		return 60 * time.Second
	}
	return 30 * time.Second
}

// transient tells whether an error is worth asking again for: the network
// or YouTube hiccuped, rather than the song or page being unavailable.
func transient(err error) bool {
	if err == nil || IsSignInCheck(err) {
		return false
	}
	m := strings.ToLower(err.Error())
	for _, s := range []string{
		"took too long", "timed out", "timeout", "temporar", "connection", "reset by peer",
		"remote end closed", "incompleteread", "network is unreachable", "name resolution",
		"http error 500", "http error 502", "http error 503", "http error 504", "server error",
		"stopped unexpectedly", "broken pipe", "eof",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// callRetrying runs a helper call, asking again up to twice when the
// failure looks like a passing network or YouTube problem.
func (c *Client) callRetrying(method string, args map[string]string) (json.RawMessage, error) {
	var data json.RawMessage
	var err error
	for attempt, wait := 0, time.Second; attempt < 3; attempt, wait = attempt+1, wait*3 {
		if attempt > 0 {
			time.Sleep(wait)
		}
		data, err = c.Call(context.Background(), method, args)
		if err == nil || !transient(err) {
			return data, err
		}
	}
	return data, err
}

func (c *Client) forget(p *proc, id int) {
	c.mu.Lock()
	delete(p.pending, id)
	c.mu.Unlock()
}

func cacheKey(method string, args map[string]string) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(method)
	for _, k := range keys {
		b.WriteString("\x00" + k + "=" + args[k])
	}
	return b.String()
}

// Cached runs a method, reusing a result younger than ttl and sharing one
// helper call between concurrent identical requests. A result past its ttl
// is still answered at once while a new copy is fetched, and when YouTube
// fails, an older copy is answered rather than an error.
func (c *Client) Cached(ctx context.Context, method string, args map[string]string, ttl time.Duration) (json.RawMessage, error) {
	key := cacheKey(method, args)
	now := time.Now()
	c.cmu.Lock()
	e, have := c.cache[key]
	if have && now.Before(e.expires) {
		c.cmu.Unlock()
		return e.data, nil
	}
	f, running := c.flights[key]
	if !running {
		f = &flight{done: make(chan struct{})}
		c.flights[key] = f
	}
	c.cmu.Unlock()

	if !running {
		go c.fetch(key, method, args, ttl, f)
	}
	if have && ttl > 0 && now.Sub(e.fetched) < staleFor {
		return e.data, nil // a little old: show it now, the new copy is on its way
	}
	select {
	case <-f.done:
		if f.err != nil && have && ttl > 0 && now.Sub(e.fetched) < keepFor {
			return e.data, nil // YouTube failed: an older copy beats an error
		}
		return f.data, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) fetch(key, method string, args map[string]string, ttl time.Duration, f *flight) {
	data, err := c.callRetrying(method, args)
	c.cmu.Lock()
	f.data, f.err = data, err
	delete(c.flights, key)
	if err == nil && ttl > 0 {
		now := time.Now()
		if len(c.cache) >= maxCache {
			c.trimCache(now)
		}
		c.cache[key] = cached{data, now.Add(ttl), now}
		c.dirty = true
	}
	c.cmu.Unlock()
	close(f.done)
}

// trimCache drops copies too old to use, then the oldest. Call with cmu held.
func (c *Client) trimCache(now time.Time) {
	type kt struct {
		k string
		t time.Time
	}
	var all []kt
	for k, e := range c.cache {
		if now.Sub(e.fetched) > keepFor {
			delete(c.cache, k)
		} else {
			all = append(all, kt{k, e.fetched})
		}
	}
	if len(all) < maxCache*9/10 {
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].t.Before(all[j].t) })
	for _, x := range all[:len(all)-maxCache*3/4] {
		delete(c.cache, x.k)
	}
}

// Format picks which stream of a video to fetch.
type Format struct {
	Kind    string // "audio" (default) or "video" (video only, no sound)
	Quality string // audio: low, normal, high
	Height  string // video: highest picture height, e.g. 720
}

func (f Format) args(videoID string) map[string]string {
	a := map[string]string{"videoId": videoID}
	if f.Kind == "video" {
		a["kind"] = "video"
		a["height"] = f.Height
	} else if f.Quality != "" && f.Quality != "normal" {
		a["quality"] = f.Quality
	}
	return a
}

// Stream resolves (and caches) the audio URL for a video, in the server's
// default audio quality. refresh forces a new lookup, for when YouTube
// rejected the cached URL.
func (c *Client) Stream(ctx context.Context, videoID string, refresh bool) (*Stream, error) {
	return c.StreamFormat(ctx, videoID, Format{Quality: c.AudioQuality()}, refresh)
}

// SetAudioQuality sets the default audio quality: low, normal or high.
func (c *Client) SetAudioQuality(q string) {
	c.cmu.Lock()
	c.quality = q
	c.cmu.Unlock()
}

func (c *Client) AudioQuality() string {
	c.cmu.Lock()
	defer c.cmu.Unlock()
	return c.quality
}

// StreamFormat resolves (and caches) the URL of one stream of a video.
func (c *Client) StreamFormat(ctx context.Context, videoID string, f Format, refresh bool) (*Stream, error) {
	args := f.args(videoID)
	key := cacheKey("stream", args)
	c.cmu.Lock()
	s := c.streams[key]
	if refresh {
		delete(c.streams, key)
		delete(c.cache, key)
		s = nil
	}
	c.cmu.Unlock()
	if s != nil && (s.Expires == 0 || time.Now().Unix() < s.Expires-300) {
		return s, nil
	}
	files := cookiesFrom(ctx)
	var data json.RawMessage
	var err error
	signedIn := false
	if c.checkActive() && len(files) > 0 {
		// YouTube has been asking this server to sign in: go straight to a
		// linked account instead of failing the usual way first.
		var asked bool
		data, err, asked = c.streamSignedIn(ctx, args, files)
		if err == nil {
			signedIn = true
			c.noteSignIn(true, true, false, false)
		} else if asked {
			c.noteSignIn(true, false, true, false) // the account's sign-in has expired
		}
	}
	if !signedIn {
		data, err = c.Cached(ctx, "stream", args, 0)
		if err == nil {
			c.noteSignIn(false, false, false, false)
		} else if IsSignInCheck(err) {
			if len(files) == 0 {
				c.noteSignIn(true, false, false, true)
			} else {
				d, e, asked := c.streamSignedIn(ctx, args, files)
				if e == nil {
					data, err = d, nil
					c.noteSignIn(true, true, false, false)
				} else {
					c.noteSignIn(true, false, asked, false)
					if !asked {
						err = e
					}
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	s = &Stream{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Expires == 0 {
		s.Expires = time.Now().Add(time.Hour).Unix()
	}
	c.cmu.Lock()
	if len(c.streams) > 300 {
		now := time.Now().Unix()
		for k, v := range c.streams {
			if now > v.Expires {
				delete(c.streams, k)
			}
		}
	}
	c.streams[key] = s
	c.cmu.Unlock()
	return s, nil
}

type cookiesKey struct{}

// WithCookies lists Google sign-in cookie files a stream may fall back to
// when YouTube asks to confirm the viewer isn't a bot.
func WithCookies(ctx context.Context, files []string) context.Context {
	if len(files) == 0 {
		return ctx
	}
	return context.WithValue(ctx, cookiesKey{}, files)
}

func cookiesFrom(ctx context.Context) []string {
	f, _ := ctx.Value(cookiesKey{}).([]string)
	return f
}

// IsSignInCheck says whether YouTube refused with "Sign in to confirm
// you're not a bot".
func IsSignInCheck(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "confirm you") && strings.Contains(m, "bot") || strings.Contains(m, "sign in to confirm")
}

// Status checks the helper and its Python packages.
func (c *Client) Status(ctx context.Context) map[string]any {
	data, err := c.Call(ctx, "ping", nil)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	var st map[string]any
	json.Unmarshal(data, &st)
	if st == nil {
		st = map[string]any{}
	}
	if st["ok"] != true {
		st["error"] = "Python packages are missing on the server. Run: pip install -U ytmusicapi \"yt-dlp[default]\""
	} else if js, _ := st["js"].([]any); len(js) == 0 {
		st["warning"] = "No usable JavaScript runtime was found, so most songs will not play. Install Deno (2.3+) or Node.js (22+) on the server."
	}
	return st
}

// Restart switches to a fresh helper, which picks up newly installed Python
// packages and changed settings. New calls go to the new helper at once;
// the old one finishes the calls it was answering, then exits.
//
// With keepCache, pages already fetched stay and are shown while new copies
// are fetched (after a YouTube support update). Without it they're dropped
// (after a change of region or language).
func (c *Client) Restart(keepCache bool) {
	c.mu.Lock()
	old := c.cur
	c.cur = nil
	if old != nil {
		// Start the new helper straight away, so nobody waits for it, and
		// let the old one finish the calls it's working on.
		c.old[old] = true
		old.stdin.Close() // the helper stops reading and exits once its calls are answered
		if err := c.start(); err != nil {
			c.lastErr = err.Error()
		}
	}
	c.mu.Unlock()
	if old != nil {
		go func() {
			select {
			case <-old.done:
			case <-time.After(2 * time.Minute):
				old.cmd.Process.Kill()
			}
		}()
	}
	c.cmu.Lock()
	if keepCache {
		for k, e := range c.cache {
			e.expires = time.Time{} // fetch again, show this meanwhile
			c.cache[k] = e
		}
	} else {
		c.cache = map[string]cached{}
	}
	c.dirty = true
	c.streams = map[string]*Stream{}
	c.cmu.Unlock()
}

// Close stops the helper.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	all := []*proc{}
	if c.cur != nil {
		all = append(all, c.cur)
	}
	for p := range c.old {
		all = append(all, p)
	}
	for _, p := range all {
		p.stdin.Close()
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
	}
}
