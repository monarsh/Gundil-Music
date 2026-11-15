// Package library indexes audio files on disk and reads their tags.
package library

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
)

// Track is one audio file in the library.
type Track struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	AlbumArtist string  `json:"albumArtist,omitempty"`
	Track       int     `json:"track,omitempty"`
	Disc        int     `json:"disc,omitempty"`
	Year        int     `json:"year,omitempty"`
	Genre       string  `json:"genre,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	HasArt      bool    `json:"hasArt"`
	Added       int64   `json:"added"`
}

type entry struct {
	Path    string `json:"path"`
	ModTime int64  `json:"modTime"`
	Size    int64  `json:"size"`
	Track   Track  `json:"track"`
}

var audioExt = map[string]bool{
	".mp3": true, ".m4a": true, ".m4b": true, ".aac": true, ".flac": true,
	".ogg": true, ".oga": true, ".opus": true, ".wav": true, ".wma": true,
	".aiff": true, ".aif": true, ".ape": true, ".wv": true, ".mka": true,
}

var coverNames = []string{"cover", "folder", "front", "album", "albumart"}
var coverExt = []string{".jpg", ".jpeg", ".png", ".webp"}

// Library is a thread-safe in-memory index backed by a JSON cache file.
type Library struct {
	roots     []string
	cachePath string

	mu       sync.RWMutex
	entries  map[string]*entry // by track ID
	scanning bool
	dirty    bool

	OnChange func() // called after a scan finishes
}

func New(roots []string, cachePath string) *Library {
	l := &Library{roots: roots, cachePath: cachePath, entries: map[string]*entry{}}
	if data, err := os.ReadFile(cachePath); err == nil {
		var list []*entry
		if json.Unmarshal(data, &list) == nil {
			for _, e := range list {
				l.entries[e.Track.ID] = e
			}
		}
	}
	return l
}

func (l *Library) Roots() []string { return l.roots }

func (l *Library) Scanning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.scanning
}

func (l *Library) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Tracks returns all tracks sorted by artist, album, disc, track, title.
func (l *Library) Tracks() []Track {
	l.mu.RLock()
	out := make([]Track, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.Track)
	}
	l.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if x, y := strings.ToLower(a.Artist), strings.ToLower(b.Artist); x != y {
			return x < y
		}
		if x, y := strings.ToLower(a.Album), strings.ToLower(b.Album); x != y {
			return x < y
		}
		if a.Disc != b.Disc {
			return a.Disc < b.Disc
		}
		if a.Track != b.Track {
			return a.Track < b.Track
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	})
	return out
}

// Path returns the file path for a track ID.
func (l *Library) Path(id string) (string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.entries[id]
	if !ok {
		return "", false
	}
	return e.Path, true
}

// SetDuration remembers a duration learned from a player.
func (l *Library) SetDuration(id string, seconds float64) {
	if seconds <= 0 || seconds > 48*3600 {
		return
	}
	l.mu.Lock()
	if e, ok := l.entries[id]; ok && e.Track.Duration == 0 {
		e.Track.Duration = seconds
		l.dirty = true
	}
	l.mu.Unlock()
}

// Flush writes the cache if anything changed since the last save.
func (l *Library) Flush() {
	l.mu.Lock()
	dirty := l.dirty
	l.dirty = false
	l.mu.Unlock()
	if dirty {
		l.save()
	}
}

func (l *Library) save() {
	l.mu.RLock()
	list := make([]*entry, 0, len(l.entries))
	for _, e := range l.entries {
		list = append(list, e)
	}
	data, err := json.Marshal(list)
	l.mu.RUnlock()
	if err != nil {
		return
	}
	tmp := l.cachePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, l.cachePath)
	}
}

// Scan walks the music folders. Unchanged files are reused from the cache.
// It returns false if a scan was already running.
func (l *Library) Scan() bool {
	l.mu.Lock()
	if l.scanning {
		l.mu.Unlock()
		return false
	}
	l.scanning = true
	old := make(map[string]*entry, len(l.entries))
	for _, e := range l.entries {
		old[e.Path] = e
	}
	l.mu.Unlock()

	start := time.Now()
	fresh := map[string]*entry{}
	covers := map[string]string{}
	for _, root := range l.roots {
		root, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !audioExt[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if e, ok := old[path]; ok && e.ModTime == info.ModTime().Unix() && e.Size == info.Size() {
				if !e.Track.HasArt {
					e.Track.HasArt = folderCover(filepath.Dir(path), covers) != ""
				}
				fresh[e.Track.ID] = e
				return nil
			}
			t := readTrack(path, info, filepath.Dir(path) == root)
			if !t.HasArt {
				t.HasArt = folderCover(filepath.Dir(path), covers) != ""
			}
			fresh[t.ID] = &entry{Path: path, ModTime: info.ModTime().Unix(), Size: info.Size(), Track: t}
			return nil
		})
	}

	l.mu.Lock()
	l.entries = fresh
	l.scanning = false
	l.mu.Unlock()
	l.save()
	log.Printf("library: %d tracks indexed in %s", len(fresh), time.Since(start).Round(time.Millisecond))
	if l.OnChange != nil {
		l.OnChange()
	}
	return true
}

func trackID(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:8])
}

func readTrack(path string, info fs.FileInfo, loose bool) Track {
	t := Track{ID: trackID(path), Added: info.ModTime().Unix()}
	if f, err := os.Open(path); err == nil {
		if m, err := tag.ReadFrom(f); err == nil {
			t.Title = strings.TrimSpace(m.Title())
			t.Artist = strings.TrimSpace(m.Artist())
			t.Album = strings.TrimSpace(m.Album())
			t.AlbumArtist = strings.TrimSpace(m.AlbumArtist())
			t.Genre = strings.TrimSpace(m.Genre())
			t.Year = m.Year()
			t.Track, _ = m.Track()
			t.Disc, _ = m.Disc()
			t.HasArt = m.Picture() != nil
		}
		f.Close()
	}
	// Fall back to "Artist - Title.ext" and the folder name.
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if t.Title == "" {
		if a, s, ok := strings.Cut(base, " - "); ok && t.Artist == "" {
			t.Artist, t.Title = strings.TrimSpace(a), strings.TrimSpace(s)
		} else {
			t.Title = base
		}
	}
	if t.Artist == "" {
		t.Artist = "Unknown artist"
	}
	if t.Album == "" {
		t.Album = "Singles"
		if !loose {
			t.Album = filepath.Base(filepath.Dir(path))
		}
	}
	if t.AlbumArtist == t.Artist {
		t.AlbumArtist = ""
	}
	return t
}

func folderCover(dir string, cache map[string]string) string {
	if cache != nil {
		if c, ok := cache[dir]; ok {
			return c
		}
	}
	found := ""
	if items, err := os.ReadDir(dir); err == nil {
	outer:
		for _, name := range coverNames {
			for _, it := range items {
				lower := strings.ToLower(it.Name())
				ext := filepath.Ext(lower)
				if strings.TrimSuffix(lower, ext) != name {
					continue
				}
				for _, ce := range coverExt {
					if ext == ce {
						found = filepath.Join(dir, it.Name())
						break outer
					}
				}
			}
		}
	}
	if cache != nil {
		cache[dir] = found
	}
	return found
}

// Art returns cover art for a track: the embedded picture if there is one,
// otherwise a cover image from the track's folder.
func (l *Library) Art(id string) (data []byte, mime string, ok bool) {
	path, found := l.Path(id)
	if !found {
		return nil, "", false
	}
	if f, err := os.Open(path); err == nil {
		m, err := tag.ReadFrom(f)
		f.Close()
		if err == nil {
			if p := m.Picture(); p != nil && len(p.Data) > 0 {
				return p.Data, p.MIMEType, true
			}
		}
	}
	if c := folderCover(filepath.Dir(path), nil); c != "" {
		if data, err := os.ReadFile(c); err == nil {
			return data, "", true
		}
	}
	return nil, "", false
}
