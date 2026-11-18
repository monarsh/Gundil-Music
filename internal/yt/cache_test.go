package yt

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A stand-in helper whose behaviour is steered by files in its folder:
// "fail" makes calls fail with the file's text; "fails" holds how many
// calls fail before they work. Each answer is a counter, so tests can tell
// a fresh fetch from a reused one.
const flakyHelper = `
import json, os, sys
d = os.path.dirname(os.path.abspath(__file__))
n = 0
for line in sys.stdin:
    req = json.loads(line)
    n += 1
    out = {"id": req["id"]}
    fails = os.path.join(d, "fails")
    msg = ""
    if os.path.exists(os.path.join(d, "fail")):
        msg = open(os.path.join(d, "fail")).read()
    elif os.path.exists(fails):
        left = int(open(fails).read() or "0")
        if left > 0:
            open(fails, "w").write(str(left - 1))
            msg = "HTTP Error 503: Service Unavailable"
    if msg:
        out["error"] = msg
    else:
        out["result"] = n
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()
`

func flaky(t *testing.T) (*Client, string) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.py")
	os.WriteFile(script, []byte(flakyHelper), 0o644)
	c, err := New(py, script, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c, dir
}

func home(t *testing.T, c *Client, ttl time.Duration) int {
	t.Helper()
	data, err := c.Cached(context.Background(), "home", nil, ttl)
	return num(t, data, err)
}

func num(t *testing.T, data json.RawMessage, err error) int {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	json.Unmarshal(data, &n)
	return n
}

func TestRetriesPassingErrors(t *testing.T) {
	c, dir := flaky(t)
	os.WriteFile(filepath.Join(dir, "fails"), []byte("2"), 0o644)
	start := time.Now()
	data, err := c.Cached(context.Background(), "home", nil, time.Minute)
	if n := num(t, data, err); n != 3 {
		t.Fatalf("answer %d, want the third call's", n)
	}
	if time.Since(start) < 3*time.Second {
		t.Errorf("retried too quickly: %v", time.Since(start))
	}
}

func TestNoRetryForRealErrors(t *testing.T) {
	c, dir := flaky(t)
	os.WriteFile(filepath.Join(dir, "fail"), []byte("Video unavailable"), 0o644)
	start := time.Now()
	if _, err := c.Cached(context.Background(), "album", map[string]string{"id": "x"}, time.Minute); err == nil {
		t.Fatal("no error")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("waited %v on an error that won't go away", time.Since(start))
	}
}

func TestStaleThenFresh(t *testing.T) {
	c, dir := flaky(t)
	ctx := context.Background()
	first := home(t, c, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	// Past its time to live: the old copy comes back at once, a new one is fetched.
	if n := home(t, c, time.Minute); n != first {
		t.Fatalf("got %d, want the old copy %d straight away", n, first)
	}
	time.Sleep(300 * time.Millisecond)
	if n := home(t, c, time.Minute); n == first {
		t.Fatal("the copy was never refreshed")
	}

	// YouTube down, and the copy is too old to show first: it still beats an error.
	c.cmu.Lock()
	for k, e := range c.cache {
		e.expires, e.fetched = time.Time{}, time.Now().Add(-2*staleFor)
		c.cache[k] = e
	}
	c.cmu.Unlock()
	os.WriteFile(filepath.Join(dir, "fail"), []byte("Video unavailable"), 0o644)
	if _, err := c.Cached(ctx, "home", nil, time.Minute); err != nil {
		t.Fatalf("an old copy was available, got %v", err)
	}
}

func TestCacheSurvivesRestartOfTheServer(t *testing.T) {
	c, dir := flaky(t)
	c.SetCacheFor("MY|en")
	want := home(t, c, time.Minute)
	if err := c.SaveCache(); err != nil {
		t.Fatal(err)
	}
	c2, err := New(c.python, c.script, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	c2.SetCacheFor("MY|en")
	c2.LoadCache()
	os.WriteFile(filepath.Join(dir, "fail"), []byte("no network"), 0o644)
	if n := home(t, c2, time.Minute); n != want {
		t.Fatalf("got %d after restart, want the saved %d", n, want)
	}
	// Another region or language: the saved pages don't apply.
	c3, _ := New(c.python, c.script, dir)
	defer c3.Close()
	c3.SetCacheFor("US|en")
	c3.LoadCache()
	if len(c3.cache) != 0 {
		t.Fatal("loaded pages saved for another region")
	}
}
