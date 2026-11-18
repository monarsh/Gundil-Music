package yt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A helper whose streams need a sign-in while the file "check" exists, and
// refuse a cookies file whose name contains "expired". Every stream call is
// logged, so tests can see how a song was looked up.
const checkHelper = `
import json, os, sys
d = os.path.dirname(os.path.abspath(__file__))
for line in sys.stdin:
    req = json.loads(line)
    a = req["args"]
    out = {"id": req["id"]}
    if req["method"] == "stream":
        how = "cookies" if a.get("cookies") else "plain"
        open(os.path.join(d, "log"), "a").write(how + "\n")
        checked = os.path.exists(os.path.join(d, "check"))
        if checked and (not a.get("cookies") or "expired" in a["cookies"]):
            out["error"] = "ERROR: [youtube] x: Sign in to confirm you're not a bot"
        else:
            out["result"] = {"url": "https://example.com/" + a["videoId"] + "?" + how, "expires": 0}
    else:
        out["result"] = {}
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()
`

func TestSignInCheckGoesStraightToTheAccount(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "helper.py")
	os.WriteFile(script, []byte(checkHelper), 0o644)
	c, err := New(py, script, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	changes := make(chan SignInCheck, 10)
	c.OnSignInCheck(func(s SignInCheck) { changes <- s })
	log := func() string {
		b, _ := os.ReadFile(filepath.Join(dir, "log"))
		os.Remove(filepath.Join(dir, "log"))
		return strings.TrimSpace(strings.ReplaceAll(string(b), "\n", " "))
	}
	good := filepath.Join(dir, "google-cookies.txt")
	ctx := WithCookies(context.Background(), []string{good})
	play := func(id string) string {
		t.Helper()
		s, err := c.StreamFormat(ctx, id, Format{}, false)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return s.URL[strings.Index(s.URL, "?")+1:]
	}

	if got := play("aaaaaaaaaaa"); got != "plain" || log() != "plain" {
		t.Fatalf("no check: played %s", got)
	}
	if st := c.SignInCheckState(); st.Active {
		t.Fatal("check reported without one")
	}

	os.WriteFile(filepath.Join(dir, "check"), nil, 0o644)
	if got, l := play("bbbbbbbbbbb"), log(); got != "cookies" || l != "plain cookies" {
		t.Fatalf("first song with the check: played %s, calls %q", got, l)
	}
	if st := <-changes; !st.Active || !st.AccountWorks {
		t.Fatalf("state %+v", st)
	}
	// The next song skips the failing plain lookup.
	if got, l := play("ccccccccccc"), log(); got != "cookies" || l != "cookies" {
		t.Fatalf("second song: played %s, calls %q (want straight to the account)", got, l)
	}

	// The account's sign-in expired: reported, so someone links it again.
	expired := filepath.Join(dir, "expired-cookies.txt")
	if _, err := c.StreamFormat(WithCookies(context.Background(), []string{expired}), "ddddddddddd", Format{}, false); err == nil {
		t.Fatal("played with an expired account")
	}
	log()
	if st := <-changes; !st.AccountFailed {
		t.Fatalf("expired account not reported: %+v", st)
	}

	// No account at all: reported too.
	if _, err := c.StreamFormat(context.Background(), "eeeeeeeeeee", Format{}, false); err == nil || !IsSignInCheck(err) {
		t.Fatalf("want the sign-in error, got %v", err)
	}
	log()
	if st := <-changes; !st.NoAccount {
		t.Fatalf("missing account not reported: %+v", st)
	}

	// The check ends: once the hour is up, the usual way is tried and works again.
	os.Remove(filepath.Join(dir, "check"))
	c.signin.mu.Lock()
	c.signin.until = c.signin.until.Add(-checkFor)
	c.signin.mu.Unlock()
	if got := play("fffffffffff"); got != "plain" {
		t.Fatalf("after the check: played %s", got)
	}
	if st := <-changes; st.Active {
		t.Fatalf("check still reported: %+v", st)
	}
}
