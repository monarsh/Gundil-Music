package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLRC(t *testing.T) {
	got := ParseLRC("[ar:Someone]\n[00:12.50] First line\n[00:10.00][01:02.25]Chorus\n\nno time\n[00:20.000]\n")
	want := []lyricLine{{"Chorus", 10, 12.5}, {"First line", 12.5, 20}, {"", 20, 62.25}, {"Chorus", 62.25, 0}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestFromLRCLIB(t *testing.T) {
	var asked []string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path+"?"+r.URL.RawQuery)
		switch r.URL.Path {
		case "/api/get":
			http.NotFound(w, r) // no exact match
		case "/api/search":
			if r.URL.Query().Get("track_name") != "Harbor Lights" {
				w.Write([]byte("[]"))
				return
			}
			json.NewEncoder(w).Encode([]lrclibTrack{
				{TrackName: "Harbor Lights (Live)", Duration: 260, SyncedLyrics: "[00:01.00]wrong version"},
				{TrackName: "Harbor Lights", Duration: 201, SyncedLyrics: "[00:05.00]Under the harbor lights\n[00:09.00]We kept the radio on"},
			})
		}
	}))
	defer fake.Close()
	lrclibBase = fake.URL
	defer func() { lrclibBase = "https://lrclib.net" }()

	// A video title with "(Official Video)" still finds the song, and the
	// version of the right length wins over the live one.
	l, err := fromLRCLIB(context.Background(), "Harbor Lights (Official Video)", "Lunar Tide", "", 200)
	if err != nil || l == nil || len(l.Lines) != 2 || l.Lines[0].Text != "Under the harbor lights" {
		t.Fatalf("got %+v %v (asked %v)", l, err, asked)
	}
	// Nothing of a similar length: no lyrics rather than the wrong timings.
	l, err = fromLRCLIB(context.Background(), "Harbor Lights", "Lunar Tide", "", 120)
	if err != nil || l != nil {
		t.Fatalf("wrong-length match: %+v %v", l, err)
	}
}
