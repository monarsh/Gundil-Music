package yt

import "testing"

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2026.10.26", "2026.10.26", 0},
		{"2026.10.26", "2026.11.3", -1},
		{"2026.11.3", "2026.10.26", 1},
		{"1.12.3", "1.12.10", -1},
		{"1.12", "1.12.0", 0},
		{"1.9.0", "1.10", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNeedsUpdate(t *testing.T) {
	newest := map[string]string{"yt-dlp": "2026.11.10", "ytmusicapi": "1.12.3"}
	if got := needsUpdate(map[string]string{"yt-dlp": "2026.11.10", "ytmusicapi": "1.12.3"}, newest); len(got) != 0 {
		t.Errorf("up to date, but needsUpdate = %v", got)
	}
	if got := needsUpdate(map[string]string{"yt-dlp": "2026.10.26", "ytmusicapi": "1.12.3"}, newest); len(got) != 1 || got[0] != "yt-dlp" {
		t.Errorf("old yt-dlp, but needsUpdate = %v", got)
	}
	if got := needsUpdate(map[string]string{}, newest); len(got) != 2 {
		t.Errorf("nothing installed, but needsUpdate = %v", got)
	}
}
