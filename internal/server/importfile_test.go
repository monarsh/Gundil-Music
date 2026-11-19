package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/monarsh/gundil-music/internal/yt"
)

// A playlist file's songs are looked up in one helper call: found songs keep
// the file's order, repeats are dropped, and the rest are named as missing.
func TestFindFileSongs(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.py")
	os.WriteFile(helper, []byte(`
import json, sys
def t(v, title): return {"type": "song", "videoId": v, "title": title, "artists": [{"name": "A"}], "duration": 200}
for line in sys.stdin:
    req = json.loads(line)
    out = []
    for e in json.loads(req["args"]["items"]):
        if e.get("videoId"): out.append(t(e["videoId"], "by id"))
        elif e.get("q") == "Faded Alan Walker": out.append(t("aaaaaaaaaaa", "Faded"))
        elif e.get("q") == "Faded (again)": out.append(t("aaaaaaaaaaa", "Faded"))
        else: out.append(None)
    sys.stdout.write(json.dumps({"id": req["id"], "result": {"tracks": out}}) + "\n")
    sys.stdout.flush()
`), 0o644)
	ytc, err := yt.New(py, helper, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ytc.Close()
	s := &Server{YT: ytc}
	tracks, missing, notFound, err := s.findFileSongs(context.Background(), []fileSong{
		{Q: "Faded Alan Walker", Duration: 212},
		{VideoID: "60ItHLz5WEA"},
		{Q: "Nothing by Nobody"},
		{Q: "Faded (again)"},
		{VideoID: "not an id!"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tr := range tracks {
		got = append(got, tr.VideoID)
	}
	if fmt.Sprint(got) != "[aaaaaaaaaaa 60ItHLz5WEA]" {
		t.Errorf("tracks %v, want Faded then the id, once each", got)
	}
	if fmt.Sprint(missing) != "[Nothing by Nobody not an id!]" || notFound != 2 {
		t.Errorf("missing %q", missing)
	}
	if _, _, _, err := s.findFileSongs(context.Background(), make([]fileSong, maxFileSongs+1)); err == nil {
		t.Error("a file over the limit was accepted")
	}
}
