package settings

import (
	"path/filepath"
	"testing"
)

func TestLoadUpdate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	st := Load(p, Settings{Region: "us", AutoUpdate: true})
	if g := st.Get(); g.Region != "US" || g.Language != "en" || g.AudioQuality != "normal" || !g.AutoUpdate {
		t.Fatalf("defaults not applied: %+v", g)
	}
	if _, _, err := st.Update(func(s *Settings) { s.AudioQuality = "best" }); err == nil {
		t.Fatal("bad quality accepted")
	}
	if _, _, err := st.Update(func(s *Settings) { s.Region = "x; rm" }); err == nil {
		t.Fatal("bad region accepted")
	}
	if _, now, err := st.Update(func(s *Settings) { s.Region = "IN"; s.AutoUpdate = false; s.AudioQuality = "low" }); err != nil || now.Region != "IN" {
		t.Fatalf("update failed: %v %+v", err, now)
	}
	// The saved file wins over the defaults on the next start.
	if g := Load(p, Settings{Region: "US", AutoUpdate: true}).Get(); g.Region != "IN" || g.AutoUpdate || g.AudioQuality != "low" {
		t.Fatalf("not saved: %+v", g)
	}
}
