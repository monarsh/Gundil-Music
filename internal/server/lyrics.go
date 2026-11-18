package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Lyrics as the app shows them: timed lines when known, otherwise text.
type lyrics struct {
	Text   string      `json:"text"`
	Lines  []lyricLine `json:"lines"`
	Source string      `json:"source"`
}

type lyricLine struct {
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end,omitempty"`
}

// LRCLIB (lrclib.net) is a free, open database of timed lyrics. It fills in
// when YouTube Music has no timed lyrics for a song.
var (
	lrclibBase   = "https://lrclib.net"
	lrclibClient = &http.Client{Timeout: 8 * time.Second}
	lyricsCache  sync.Map // key -> lyricsCached
)

func init() {
	// For testing against a stand-in LRCLIB.
	if v := os.Getenv("GUNDIL_LRCLIB"); v != "" {
		lrclibBase = strings.TrimRight(v, "/")
	}
}

type lyricsCached struct {
	l    *lyrics
	time time.Time
}

// lyricsFor answers GET /api/lyrics: YouTube Music's lyrics (id is its
// lyrics browse id, possibly empty), with timed ones from LRCLIB when
// YouTube Music's aren't timed.
func (s *Server) lyricsFor(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	title, artist, album := q.Get("title"), q.Get("artist"), q.Get("album")
	duration, _ := strconv.ParseFloat(q.Get("duration"), 64)
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	var yt lyrics
	if id != "" {
		if data, err := s.YT.Cached(ctx, "lyrics", map[string]string{"id": id}, 24*time.Hour); err == nil {
			json.Unmarshal(data, &yt)
		}
	}
	if len(yt.Lines) > 0 || title == "" || artist == "" {
		writeJSON(w, yt)
		return
	}
	key := strings.ToLower(fmt.Sprintf("%s\x00%s\x00%s\x00%.0f", title, artist, album, duration))
	var lr *lyrics
	if c, ok := lyricsCache.Load(key); ok && time.Since(c.(lyricsCached).time) < 12*time.Hour {
		lr = c.(lyricsCached).l
	} else {
		var err error
		lr, err = fromLRCLIB(ctx, title, artist, album, duration)
		if err == nil { // remember "not found" too, but not network errors
			lyricsCache.Store(key, lyricsCached{lr, time.Now()})
		}
	}
	switch {
	case lr != nil && len(lr.Lines) > 0:
		writeJSON(w, lr)
	case yt.Text != "":
		writeJSON(w, yt)
	case lr != nil:
		writeJSON(w, lr)
	default:
		writeJSON(w, yt)
	}
}

type lrclibTrack struct {
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func lrclibGet(ctx context.Context, path string, args url.Values, v any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", lrclibBase+path+"?"+args.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Gundil Music (self-hosted music player)")
	res, err := lrclibClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == 200 {
		return 200, json.NewDecoder(res.Body).Decode(v)
	}
	return res.StatusCode, nil
}

// noise in video titles that LRCLIB's song titles don't have
var titleNoise = regexp.MustCompile(`(?i)\s*[\(\[][^\)\]]*(official|video|audio|lyric|visualizer|live|remaster|mv|hd|4k)[^\)\]]*[\)\]]`)

// fromLRCLIB finds timed lyrics for a song. nil, nil means none were found.
// Only a song of about the same length is accepted, so a live or extended
// version's timings aren't used for the original.
func fromLRCLIB(ctx context.Context, title, artist, album string, duration float64) (*lyrics, error) {
	var t lrclibTrack
	if duration > 0 {
		args := url.Values{"track_name": {title}, "artist_name": {artist}, "duration": {strconv.Itoa(int(math.Round(duration)))}}
		if album != "" {
			args.Set("album_name", album)
		}
		code, err := lrclibGet(ctx, "/api/get", args, &t)
		if err != nil {
			return nil, err
		}
		if code == 200 && t.SyncedLyrics != "" {
			return toLyrics(t), nil
		}
	}
	var best *lrclibTrack
	var plain *lrclibTrack
	for _, name := range uniq(title, strings.TrimSpace(titleNoise.ReplaceAllString(title, ""))) {
		var list []lrclibTrack
		code, err := lrclibGet(ctx, "/api/search", url.Values{"track_name": {name}, "artist_name": {artist}}, &list)
		if err != nil {
			return nil, err
		}
		if code != 200 {
			continue
		}
		sort.SliceStable(list, func(i, j int) bool {
			return math.Abs(list[i].Duration-duration) < math.Abs(list[j].Duration-duration)
		})
		for i := range list {
			c := &list[i]
			if duration > 0 && math.Abs(c.Duration-duration) > 3 {
				continue
			}
			if c.SyncedLyrics != "" && best == nil {
				best = c
			}
			if (c.PlainLyrics != "" || c.Instrumental) && plain == nil {
				plain = c
			}
		}
		if best != nil {
			break
		}
	}
	if best != nil {
		return toLyrics(*best), nil
	}
	if plain != nil {
		return toLyrics(*plain), nil
	}
	return nil, nil
}

func uniq(xs ...string) []string {
	var out []string
	for _, x := range xs {
		if x == "" {
			continue
		}
		dup := false
		for _, y := range out {
			dup = dup || strings.EqualFold(x, y)
		}
		if !dup {
			out = append(out, x)
		}
	}
	return out
}

func toLyrics(t lrclibTrack) *lyrics {
	l := &lyrics{Source: "Timed lyrics from LRCLIB"}
	if t.Instrumental && t.SyncedLyrics == "" && t.PlainLyrics == "" {
		l.Text = "♪ Instrumental ♪"
		return l
	}
	l.Lines = ParseLRC(t.SyncedLyrics)
	if len(l.Lines) == 0 {
		l.Source = "Lyrics from LRCLIB"
		l.Text = t.PlainLyrics
		return l
	}
	var b strings.Builder
	for _, ln := range l.Lines {
		b.WriteString(ln.Text + "\n")
	}
	l.Text = strings.TrimSpace(b.String())
	return l
}

var lrcStamp = regexp.MustCompile(`\[(\d+):(\d+(?:[.:]\d+)?)\]`)

// ParseLRC reads "[mm:ss.xx] words" lines (a line may carry several
// timestamps), sorted by time. Each line ends where the next begins.
func ParseLRC(text string) []lyricLine {
	var out []lyricLine
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		stamps := lrcStamp.FindAllStringSubmatchIndex(raw, -1)
		if len(stamps) == 0 || stamps[0][0] != 0 {
			continue // [ar:...] tags and untimed lines
		}
		words := strings.TrimSpace(raw[stamps[len(stamps)-1][1]:])
		for _, m := range stamps {
			min, _ := strconv.Atoi(raw[m[2]:m[3]])
			sec, _ := strconv.ParseFloat(strings.Replace(raw[m[4]:m[5]], ":", ".", 1), 64)
			out = append(out, lyricLine{Text: words, Start: float64(min)*60 + sec})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	for i := range out {
		if i+1 < len(out) {
			out[i].End = out[i+1].Start
		}
	}
	return out
}
