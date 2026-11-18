package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
    if req["method"] == "relay":
        out = {"id": req["id"], "error": "no relay here"}
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
    c = (req.get("args") or {}).get("clients", "")
    out = {"id": req["id"], "result": {"url": "`+gv.URL+`/videoplayback?c=" + c, "mime": "video/mp4", "expires": 0}}
    if req["method"] == "relay":
        out = {"id": req["id"], "error": "no relay here"}
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
	if fmt.Sprint(asked) != "[  tv_samsung visionos tv]" {
		t.Errorf("asked as %q, want the usual player twice, then tv_samsung, visionos, tv", asked)
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

func TestClientForLinkAddress(t *testing.T) {
	cases := map[string]string{
		"https://rr1---sn-x.googlevideo.com/videoplayback?expire=1&ip=203.0.113.7&id=o-x":     "IPv4",
		"https://rr1---sn-x.googlevideo.com/videoplayback?ip=2001%3Adb8%3A%3A1&id=o-x":        "IPv6",
		"https://rr1---sn-x.googlevideo.com/videoplayback?ip=%3A%3Affff%3A203.0.113.7&itag=1": "IPv4",
		"https://example.com/a.wav": "",
	}
	for link, want := range cases {
		c, got := clientFor(link)
		if got != want {
			t.Errorf("%s: %q, want %q", link, got, want)
		}
		if (want == "IPv4") != (c == streamClient4) || (want == "IPv6") != (c == streamClient6) {
			t.Errorf("%s: wrong client", link)
		}
	}
}

// YouTube refuses the server's own connection but not yt-dlp's: the stream
// plays through the helper's relay (the real one, with yt-dlp's networking),
// and later pieces and streams go that way first.
func TestStreamThroughYtDlp(t *testing.T) {
	py := os.Getenv("GUNDIL_TEST_PYTHON") // a Python with yt-dlp installed
	if py == "" {
		py = "python3"
	}
	if exec.Command(py, "-c", "import yt_dlp").Run() != nil {
		t.Skip("yt-dlp not installed (set GUNDIL_TEST_PYTHON to a Python that has it)")
	}
	data := make([]byte, 12<<20+777)
	for i := range data {
		data[i] = byte(i * 13)
	}
	var direct, relayed int32
	gv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.UserAgent(), "Go-http-client") {
			atomic.AddInt32(&direct, 1)
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		atomic.AddInt32(&relayed, 1)
		start, end, ok := parseRange(r.Header.Get("Range"))
		if !ok || end-start+1 > streamChunk {
			http.Error(w, "bad range", http.StatusForbidden)
			return
		}
		end = min(end, int64(len(data))-1)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(data[start : end+1])
	}))
	defer gv.Close()
	real, _ := filepath.Abs("../yt")
	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.py")
	os.WriteFile(helper, []byte(`
import json, sys
sys.path.insert(0, `+fmt.Sprintf("%q", real)+`)
import helper
for line in sys.stdin:
    req = json.loads(line)
    if req["method"] == "relay":
        out = {"id": req["id"], "result": helper.m_relay({})}
    else:
        out = {"id": req["id"], "result": {"url": "`+gv.URL+`/videoplayback", "mime": "audio/webm", "expires": 0,
               "warnings": ["n challenge solving failed"]}}
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
		r := httptest.NewRequest("GET", "/api/stream/"+id, nil)
		r.SetPathValue("id", id)
		r.Header.Set("Range", "bytes=0-")
		w := httptest.NewRecorder()
		s.stream(w, r)
		return w
	}
	if w := play("aaaaaaaaaaa"); w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("status %d, %d of %d bytes", w.Code, w.Body.Len(), len(data))
	}
	if d := atomic.LoadInt32(&direct); d != 1 {
		t.Errorf("the server's own connection was tried %d times, want once", d)
	}
	if n := atomic.LoadInt32(&relayed); n != 2 {
		t.Errorf("%d pieces through yt-dlp, want 2", n)
	}
	if w := play("bbbbbbbbbbb"); w.Code != http.StatusPartialContent || atomic.LoadInt32(&direct) != 1 {
		t.Errorf("next stream: status %d, own connection tried %d times in all", w.Code, atomic.LoadInt32(&direct))
	}
}
