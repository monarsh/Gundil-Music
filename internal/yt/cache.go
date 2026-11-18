package yt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The pages fetched from YouTube Music are kept on disk too, so after a
// restart Home and recent pages show at once (and are fetched again behind
// the scenes) instead of waiting for YouTube.

const (
	cacheFile  = "youtube-cache.json"
	savedPages = 400 // the most recently fetched pages that are saved
)

type savedEntry struct {
	Key     string          `json:"k"`
	Data    json.RawMessage `json:"d"`
	Expires time.Time       `json:"e"`
	Fetched time.Time       `json:"f"`
}

type savedFile struct {
	For   string       `json:"for"` // YouTube Music region and language the pages are in
	Pages []savedEntry `json:"pages"`
}

// SetCacheFor names the region and language results are in now. Saved
// pages for another region or language aren't loaded.
func (c *Client) SetCacheFor(tag string) {
	c.cmu.Lock()
	c.cacheFor = tag
	c.cmu.Unlock()
}

// LoadCache reads the pages saved by an earlier run.
func (c *Client) LoadCache() {
	b, err := os.ReadFile(filepath.Join(c.dataDir, cacheFile))
	if err != nil {
		return
	}
	var saved savedFile
	if json.Unmarshal(b, &saved) != nil {
		return
	}
	now := time.Now()
	c.cmu.Lock()
	defer c.cmu.Unlock()
	if saved.For != c.cacheFor {
		return
	}
	for _, e := range saved.Pages {
		if now.Sub(e.Fetched) < keepFor && len(e.Data) > 0 {
			c.cache[e.Key] = cached{e.Data, e.Expires, e.Fetched}
		}
	}
}

// SaveCache writes the most recent pages to disk, if anything changed.
func (c *Client) SaveCache() error {
	c.cmu.Lock()
	if !c.dirty {
		c.cmu.Unlock()
		return nil
	}
	c.dirty = false
	tag := c.cacheFor
	saved := make([]savedEntry, 0, len(c.cache))
	for k, e := range c.cache {
		if strings.Contains(k, "\x00cookies=") { // someone's own YouTube Music account
			continue
		}
		saved = append(saved, savedEntry{k, e.data, e.expires, e.fetched})
	}
	c.cmu.Unlock()
	sort.Slice(saved, func(i, j int) bool { return saved[i].Fetched.After(saved[j].Fetched) })
	if len(saved) > savedPages {
		saved = saved[:savedPages]
	}
	b, err := json.Marshal(savedFile{tag, saved})
	if err != nil {
		return err
	}
	path := filepath.Join(c.dataDir, cacheFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// KeepCacheSaved saves the cache every minute while it changes. It runs
// until stop is closed, then saves once more.
func (c *Client) KeepCacheSaved(stop <-chan struct{}) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.SaveCache()
		case <-stop:
			c.SaveCache()
			return
		}
	}
}
