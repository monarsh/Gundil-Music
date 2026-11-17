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
	expires time.Time
}

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
	stdin   io.WriteCloser
	cmd     *exec.Cmd
	done    chan struct{} // closed when the current helper's reader exits
	pending map[int]chan reply
	next    int
	lastErr string

	cmu     sync.Mutex
	cache   map[string]cached
	flights map[string]*flight
	streams map[string]*Stream
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
		pending: map[int]chan reply{},
		cache:   map[string]cached{}, flights: map[string]*flight{}, streams: map[string]*Stream{},
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
	c.cmd, c.stdin = cmd, stdin
	done := make(chan struct{})
	c.done = done
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			log.Printf("helper: %s", sc.Text())
		}
	}()
	go func() {
		c.readLoop(cmd, stdout)
		close(done)
	}()
	return nil
}

func (c *Client) readLoop(cmd *exec.Cmd, stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		var r reply
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[r.ID]
		delete(c.pending, r.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- r
		}
	}
	cmd.Wait()
	c.mu.Lock()
	if c.cmd == cmd {
		c.cmd, c.stdin = nil, nil
	}
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	log.Printf("yt: helper exited")
}

// Call runs one helper method without caching.
func (c *Client) Call(ctx context.Context, method string, args map[string]string) (json.RawMessage, error) {
	c.mu.Lock()
	if c.cmd == nil {
		if err := c.start(); err != nil {
			c.lastErr = err.Error()
			c.mu.Unlock()
			return nil, err
		}
	}
	c.next++
	id := c.next
	ch := make(chan reply, 1)
	c.pending[id] = ch
	msg, _ := json.Marshal(map[string]any{"id": id, "method": method, "args": args})
	_, err := c.stdin.Write(append(msg, '\n'))
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(90 * time.Second)
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
		c.forget(id)
		return nil, ctx.Err()
	case <-timeout.C:
		c.forget(id)
		return nil, errors.New("YouTube Music took too long to answer")
	}
}

func (c *Client) forget(id int) {
	c.mu.Lock()
	delete(c.pending, id)
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
// helper call between concurrent identical requests.
func (c *Client) Cached(ctx context.Context, method string, args map[string]string, ttl time.Duration) (json.RawMessage, error) {
	key := cacheKey(method, args)
	c.cmu.Lock()
	if e, ok := c.cache[key]; ok && time.Now().Before(e.expires) {
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
		go func() {
			data, err := c.Call(context.Background(), method, args)
			c.cmu.Lock()
			f.data, f.err = data, err
			delete(c.flights, key)
			if err == nil && ttl > 0 {
				if len(c.cache) > 400 {
					now := time.Now()
					for k, e := range c.cache {
						if now.After(e.expires) {
							delete(c.cache, k)
						}
					}
				}
				c.cache[key] = cached{data, time.Now().Add(ttl)}
			}
			c.cmu.Unlock()
			close(f.done)
		}()
	}
	select {
	case <-f.done:
		return f.data, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stream resolves (and caches) the audio URL for a video. refresh forces a
// new lookup, for when YouTube rejected the cached URL.
func (c *Client) Stream(ctx context.Context, videoID string, refresh bool) (*Stream, error) {
	c.cmu.Lock()
	s := c.streams[videoID]
	if refresh {
		delete(c.streams, videoID)
		delete(c.cache, cacheKey("stream", map[string]string{"videoId": videoID}))
		s = nil
	}
	c.cmu.Unlock()
	if s != nil && (s.Expires == 0 || time.Now().Unix() < s.Expires-300) {
		return s, nil
	}
	data, err := c.Cached(ctx, "stream", map[string]string{"videoId": videoID}, 0)
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
	c.streams[videoID] = s
	c.cmu.Unlock()
	return s, nil
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

// Restart stops the helper and waits for it to exit; the next call starts a
// fresh one, which picks up newly installed Python packages.
func (c *Client) Restart() {
	c.mu.Lock()
	done := c.done
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	c.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	}
	c.cmu.Lock()
	c.cache = map[string]cached{}
	c.streams = map[string]*Stream{}
	c.cmu.Unlock()
}

// Close stops the helper.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}
