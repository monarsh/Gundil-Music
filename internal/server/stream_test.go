package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/monarsh/gundil-music/internal/yt"
)

// A stand-in for YouTube's video servers: it serves a 25 MB stream, but like
// YouTube refuses (403) any single request for more than streamChunk bytes.
func fakeGooglevideo(t *testing.T, data []byte) (*httptest.Server, *int32) {
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		start, end, ok := parseRange(r.Header.Get("Range"))
		if !ok || end < 0 || end-start+1 > streamChunk {
			http.Error(w, "too much at once", http.StatusForbidden)
			return
		}
		if start >= int64(len(data)) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		end = min(end, int64(len(data))-1)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestStreamFetchesInPieces(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	data := make([]byte, 25<<20+12345)
	for i := range data {
		data[i] = byte(i * 7)
	}
	gv, requests := fakeGooglevideo(t, data)
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.py")
	os.WriteFile(helper, []byte(`
import json, sys
for line in sys.stdin:
    req = json.loads(line)
    out = {"id": req["id"], "result": {"url": "`+gv.URL+`/videoplayback", "mime": "video/mp4", "expires": 0}}
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()
`), 0o644)
	ytc, err := yt.New(py, helper, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ytc.Close()
	s := &Server{YT: ytc}

	get := func(rangeHeader string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/stream/aaaaaaaaaaa?kind=video&h=1080", nil)
		r.SetPathValue("id", "aaaaaaaaaaa")
		if rangeHeader != "" {
			r.Header.Set("Range", rangeHeader)
		}
		w := httptest.NewRecorder()
		s.stream(w, r)
		return w
	}

	// The whole stream, as a browser asks for it first.
	w := get("bytes=0-")
	if w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("bytes=0-: status %d, %d of %d bytes right", w.Code, w.Body.Len(), len(data))
	}
	if cr := w.Header().Get("Content-Range"); cr != fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)) {
		t.Errorf("Content-Range %q", cr)
	}
	if n := atomic.LoadInt32(requests); n != 3 {
		t.Errorf("%d requests to YouTube, want 3 pieces", n)
	}

	// No Range at all (a download): a plain 200 with the full length.
	w = get("")
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Length") != fmt.Sprint(len(data)) {
		t.Fatalf("no range: status %d, %d bytes, length %q", w.Code, w.Body.Len(), w.Header().Get("Content-Length"))
	}

	// Seeking: a range in the middle, crossing a piece boundary.
	from, to := int64(9<<20), int64(21<<20)
	w = get(fmt.Sprintf("bytes=%d-%d", from, to))
	if w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), data[from:to+1]) {
		t.Fatalf("seek: status %d, %d bytes", w.Code, w.Body.Len())
	}
	if w.Header().Get("Content-Length") != fmt.Sprint(to-from+1) {
		t.Errorf("seek Content-Length %q", w.Header().Get("Content-Length"))
	}

	// Past the end.
	if w = get(fmt.Sprintf("bytes=%d-", len(data)+10)); w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("past the end: status %d", w.Code)
	}
}

// YouTube refuses the links of the usual players (as it does when they need
// proof only its own apps give): the server asks as other players, plays
// with the first that works, and goes straight to it for the next stream.
func TestStreamTriesOtherPlayers(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found")
	}
	data := []byte("some video bytes, enough to tell them apart")
	var asked []string
	gv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("c")
		asked = append(asked, c)
		if c != "tv" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data)
	}))
	defer gv.Close()
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.py")
	os.WriteFile(helper, []byte(`
import json, sys
for line in sys.stdin:
    req = json.loads(line)
    c = req["args"].get("clients", "")
    out = {"id": req["id"], "result": {"url": "`+gv.URL+`/videoplayback?c=" + c, "mime": "video/mp4", "expires": 0}}
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()
`), 0o644)
	ytc, err := yt.New(py, helper, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ytc.Close()
	s := &Server{YT: ytc}
	play := func(id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/stream/"+id+"?kind=video&h=720", nil)
		r.SetPathValue("id", id)
		r.Header.Set("Range", "bytes=0-")
		w := httptest.NewRecorder()
		s.stream(w, r)
		return w
	}
	if w := play("aaaaaaaaaaa"); w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("first video: status %d, body %q", w.Code, w.Body.String())
	}
	if fmt.Sprint(asked) != "[  android_vr tv]" {
		t.Errorf("asked as %q, want the usual player twice, then android_vr, then tv", asked)
	}
	asked = nil
	if w := play("bbbbbbbbbbb"); w.Code != http.StatusPartialContent {
		t.Fatalf("second video: status %d", w.Code)
	}
	if fmt.Sprint(asked) != "[tv]" {
		t.Errorf("second video asked as %q, want straight to tv", asked)
	}
	// Songs are tracked apart: their usual player is still tried first.
	asked = nil
	r := httptest.NewRequest("GET", "/api/stream/ccccccccccc", nil)
	r.SetPathValue("id", "ccccccccccc")
	s.stream(httptest.NewRecorder(), r)
	if len(asked) == 0 || asked[0] != "" {
		t.Errorf("song asked as %q first, want the usual player", asked)
	}
}
