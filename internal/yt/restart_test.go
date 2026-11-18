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

// A stand-in helper that answers "pid" with its process id, after an
// optional delay, using the real helper's threaded loop: it reads requests
// until stdin closes, then exits once every answer is sent.
const fakeHelper = `
import json, os, sys, time, threading
from concurrent.futures import ThreadPoolExecutor
lock = threading.Lock()
def handle(req):
    time.sleep(float(req["args"].get("sleep", "0")))
    with lock:
        sys.stdout.write(json.dumps({"id": req["id"], "result": os.getpid()}) + "\n")
        sys.stdout.flush()
pool = ThreadPoolExecutor(max_workers=4)
for line in sys.stdin:
    pool.submit(handle, json.loads(line))
`

func TestRestartKeepsCallsGoing(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.py")
	os.WriteFile(script, []byte(fakeHelper), 0o644)
	c, err := New(py, script, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	pid := func(sleep string) (int, error) {
		data, err := c.Call(ctx, "pid", map[string]string{"sleep": sleep})
		var n int
		json.Unmarshal(data, &n)
		return n, err
	}
	first, err := pid("0")
	if err != nil {
		t.Fatal(err)
	}

	// A slow call is under way when the restart happens.
	slow := make(chan error, 1)
	var slowPid int
	go func() {
		n, err := pid("1.5")
		slowPid = n
		slow <- err
	}()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	c.Restart(true)
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("Restart blocked for %v", d)
	}

	// New calls go to the new helper while the old one finishes.
	second, err := pid("0")
	if err != nil {
		t.Fatalf("call right after restart: %v", err)
	}
	if second == first {
		t.Fatal("the call after the restart went to the old helper")
	}
	if err := <-slow; err != nil {
		t.Fatalf("the call in progress failed: %v", err)
	}
	if slowPid != first {
		t.Fatalf("the slow call was answered by %d, want the old helper %d", slowPid, first)
	}

	// The old helper then exits by itself.
	deadline := time.Now().Add(5 * time.Second)
	for {
		c.mu.Lock()
		n := len(c.old)
		c.mu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the old helper didn't exit")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
