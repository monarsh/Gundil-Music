package yt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The YouTube libraries are updated into <data>/pylib, which the helper puts
// first on its import path. This works everywhere, including the read-only
// AppImage, the Windows zip and the Pi's venv.

var packages = []struct{ name, pip, status string }{
	{"yt-dlp", "yt-dlp[default]", "yt_dlp"},
	{"ytmusicapi", "ytmusicapi", "ytmusicapi"},
}

// UpdateResult describes what an update did.
type UpdateResult struct {
	Updated  bool              `json:"updated"`
	Message  string            `json:"message"`
	Versions map[string]string `json:"versions"`
	Time     int64             `json:"time"`
}

func (c *Client) pylib() string { return filepath.Join(c.dataDir, "pylib") }

func (c *Client) recordPath() string { return filepath.Join(c.dataDir, "youtube-update.json") }

// LastUpdate returns the last successful update, if any.
func (c *Client) LastUpdate() *UpdateResult {
	data, err := os.ReadFile(c.recordPath())
	if err != nil {
		return nil
	}
	var r UpdateResult
	if json.Unmarshal(data, &r) != nil {
		return nil
	}
	return &r
}

// compareVersions compares dotted versions like 2026.10.26 or 1.12.3.
// Non-numeric parts (e.g. "dev0") compare as 0.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(strings.TrimLeft(pa[i], "v"))
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// latest asks PyPI for the newest release of each package.
func latest(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	client := &http.Client{Timeout: 20 * time.Second}
	for _, p := range packages {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://pypi.org/pypi/"+p.name+"/json", nil)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("could not reach PyPI: %w", err)
		}
		var body struct {
			Info struct {
				Version string `json:"version"`
			} `json:"info"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || body.Info.Version == "" {
			return nil, fmt.Errorf("unexpected answer from PyPI for %s (%s)", p.name, resp.Status)
		}
		out[p.name] = body.Info.Version
	}
	return out, nil
}

// needsUpdate reports which packages are older than the newest release.
func needsUpdate(installed, newest map[string]string) []string {
	var old []string
	for _, p := range packages {
		have, want := installed[p.name], newest[p.name]
		if have == "" || (want != "" && compareVersions(have, want) < 0) {
			old = append(old, p.name)
		}
	}
	return old
}

// Installed returns the versions the helper is running.
func (c *Client) Installed(ctx context.Context) map[string]string {
	st := c.Status(ctx)
	out := map[string]string{}
	for _, p := range packages {
		if v, _ := st[p.status].(string); v != "" {
			out[p.name] = v
		}
	}
	return out
}

// Update installs the newest yt-dlp and ytmusicapi into <data>/pylib and
// restarts the helper. Unless force is set, it does nothing when the running
// versions are already the newest.
func (c *Client) Update(ctx context.Context, force bool) (*UpdateResult, error) {
	if !c.updating.TryLock() {
		return nil, errors.New("an update is already running")
	}
	defer c.updating.Unlock()
	if c.python == "" {
		return nil, errors.New("Python 3 was not found on the server")
	}

	installed := c.Installed(ctx)
	if !force {
		newest, err := latest(ctx)
		if err != nil {
			return nil, err
		}
		if len(needsUpdate(installed, newest)) == 0 {
			return &UpdateResult{Message: "YouTube support is up to date", Versions: installed, Time: time.Now().Unix()}, nil
		}
	}

	// Install into a fresh folder, then swap it in, so a failed or interrupted
	// update never leaves a half-updated library behind.
	next := c.pylib() + ".new"
	os.RemoveAll(next)
	args := []string{"-m", "pip", "install", "--upgrade", "--no-cache-dir", "--disable-pip-version-check",
		"--no-warn-script-location", "--target", next}
	for _, p := range packages {
		args = append(args, p.pip)
	}
	ictx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ictx, c.python, args...)
	cmd.Env = append(os.Environ(), "PIP_NO_INPUT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(next)
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 6 {
			lines = lines[len(lines)-6:]
		}
		return nil, fmt.Errorf("updating failed: %v\n%s", err, strings.Join(lines, "\n"))
	}
	old := c.pylib() + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(c.pylib()); err == nil {
		if err := os.Rename(c.pylib(), old); err != nil {
			os.RemoveAll(next)
			return nil, fmt.Errorf("could not replace the old libraries: %w", err)
		}
	}
	if err := os.Rename(next, c.pylib()); err != nil {
		os.Rename(old, c.pylib())
		return nil, fmt.Errorf("could not install the new libraries: %w", err)
	}
	os.RemoveAll(old)

	c.Restart()
	versions := c.Installed(ctx)
	r := &UpdateResult{Updated: true, Message: "YouTube support updated", Versions: versions, Time: time.Now().Unix()}
	for _, p := range packages {
		if before := installed[p.name]; before != "" && before != versions[p.name] {
			r.Message += fmt.Sprintf(" (%s %s → %s)", p.name, before, versions[p.name])
		}
	}
	if data, err := json.Marshal(r); err == nil {
		os.WriteFile(c.recordPath(), data, 0o644)
	}
	return r, nil
}
