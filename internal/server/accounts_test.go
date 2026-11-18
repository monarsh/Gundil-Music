package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/monarsh/gundil-music/internal/model"
	"github.com/monarsh/gundil-music/internal/settings"
	"github.com/monarsh/gundil-music/internal/store"
	"github.com/monarsh/gundil-music/internal/users"
)

func TestParseLink(t *testing.T) {
	ok := map[string][2]string{
		"https://music.youtube.com/playlist?list=PLabc_123-x":    {"playlist", "PLabc_123-x"},
		"music.youtube.com/playlist?list=OLAK5uy_abc&si=xyz":     {"playlist", "OLAK5uy_abc"},
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLxyz": {"playlist", "PLxyz"},
		"https://music.youtube.com/browse/MPREb_Abc123":          {"album", "MPREb_Abc123"},
		"https://music.youtube.com/browse/VLPLqwe":               {"playlist", "PLqwe"},
		"https://youtube.com/playlist?list=RDCLAK5uy_abc":        {"playlist", "RDCLAK5uy_abc"},
		"PLhello":   {"playlist", "PLhello"},
		"LM":        {"playlist", "LM"},
		"MPREb_xyz": {"album", "MPREb_xyz"},
		"https://m.youtube.com/playlist?list=PL1": {"playlist", "PL1"},
	}
	for in, want := range ok {
		k, id, err := ParseLink(in)
		if err != nil || k != want[0] || id != want[1] {
			t.Errorf("ParseLink(%q) = %q %q %v, want %v", in, k, id, err, want)
		}
	}
	for _, bad := range []string{"", "https://example.com/playlist?list=PL1", "https://youtu.be/dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=abc", "https://www.youtube.com/watch?v=x&list=RDMMx"} {
		if _, _, err := ParseLink(bad); err == nil {
			t.Errorf("ParseLink(%q) should fail", bad)
		}
	}
}

func TestCleanCookies(t *testing.T) {
	in := "# Netscape HTTP Cookie File\r\n" +
		".youtube.com\tTRUE\t/\tTRUE\t1900000000\t__Secure-3PAPISID\tabc\r\n" +
		"#HttpOnly_.youtube.com\tTRUE\t/\tTRUE\t1900000000\tSID\txyz\r\n" +
		".example.com\tTRUE\t/\tFALSE\t0\tother\tnope\r\n"
	out, err := cleanCookies(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "example.com") || !strings.Contains(out, "#HttpOnly_.youtube.com") || !strings.HasPrefix(out, "# Netscape") {
		t.Fatalf("cleaned:\n%s", out)
	}
	if _, err := cleanCookies(".youtube.com\tTRUE\t/\tTRUE\t0\tPREF\tx\n"); err == nil {
		t.Fatal("cookies without a sign-in should be refused")
	}
	if _, err := cleanCookies("hello"); err == nil {
		t.Fatal("garbage should be refused")
	}
}

type client struct {
	t   *testing.T
	h   http.Handler
	jar []*http.Cookie
}

func (c *client) do(method, path, body string) (int, map[string]any, []byte) {
	c.t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.168.1.30:4000"
	for _, k := range c.jar {
		r.AddCookie(k)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	for _, k := range w.Result().Cookies() {
		if k.MaxAge < 0 {
			c.jar = nil
		} else {
			c.jar = []*http.Cookie{k}
		}
	}
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m, w.Body.Bytes()
}

func TestAccountsFlow(t *testing.T) {
	dir := t.TempDir()
	shared := store.New(filepath.Join(dir, "library.json"))
	shared.SetRating(model.Track{VideoID: "dQw4w9WgXcQ", Title: "Old like"}, "like")
	s := &Server{Store: shared, Users: users.Load(filepath.Join(dir, "users.json")), DataDir: dir,
		Web: fstest.MapFS{"index.html": {Data: []byte("app")}}}
	h := s.Handler()
	anon := &client{t: t, h: h}

	// No accounts: everything open, one shared library.
	if code, _, _ := anon.do("GET", "/api/state", ""); code != 200 {
		t.Fatalf("state without accounts: %d", code)
	}

	// Set up: the first account becomes admin and gets the shared library.
	owner := &client{t: t, h: h}
	code, m, body := owner.do("POST", "/api/accounts/setup", `{"name":"Monar","password":"pw1234"}`)
	if code != 200 || m["user"] == nil {
		t.Fatalf("setup: %d %s", code, body)
	}
	if code, _, _ := owner.do("POST", "/api/accounts/setup", `{"name":"x","password":"pw1234"}`); code != 409 {
		t.Fatalf("second setup: %d", code)
	}
	_, _, body = owner.do("GET", "/api/state", "")
	if !strings.Contains(string(body), "Old like") {
		t.Fatalf("owner didn't get the shared library: %s", body)
	}

	// Signed out: the API is closed, the app and sign-in are open.
	for _, p := range []string{"/api/state", "/api/settings", "/api/users"} {
		if code, m, _ := anon.do("GET", p, ""); code != 401 || m["signin"] != true {
			t.Fatalf("%s signed out: %d %v", p, code, m)
		}
	}
	if code, _, _ := anon.do("GET", "/", ""); code != 200 {
		t.Fatalf("app page signed out: %d", code)
	}
	if code, m, _ := anon.do("GET", "/api/session", ""); code != 200 || m["accounts"] != true || m["user"] != nil {
		t.Fatalf("session signed out: %d %v", code, m)
	}

	// The admin adds a person, who gets an empty library of their own.
	if code, _, body := owner.do("POST", "/api/users", `{"name":"Siti","password":"pw5678"}`); code != 200 {
		t.Fatalf("add user: %d %s", code, body)
	}
	siti := &client{t: t, h: h}
	if code, _, _ := siti.do("POST", "/api/login", `{"name":"siti","password":"nope"}`); code != 401 {
		t.Fatalf("wrong password: %d", code)
	}
	if code, _, body := siti.do("POST", "/api/login", `{"name":"siti","password":"pw5678"}`); code != 200 {
		t.Fatalf("login: %d %s", code, body)
	}
	_, _, body = siti.do("GET", "/api/state", "")
	if strings.Contains(string(body), "Old like") {
		t.Fatal("Siti sees the owner's library")
	}
	siti.do("POST", "/api/rating", `{"track":{"videoId":"abcdefghijk","title":"Siti's song"},"rating":"like"}`)
	_, _, body = owner.do("GET", "/api/state", "")
	if strings.Contains(string(body), "Siti's song") {
		t.Fatal("the owner sees Siti's likes")
	}
	if _, err := os.Stat(filepath.Join(dir, "users", users.DirName("Siti"), "library.json")); err != nil {
		t.Fatalf("Siti's library file: %v", err)
	}

	// Siti isn't an admin.
	if code, _, _ := siti.do("PUT", "/api/settings", `{"region":"US"}`); code != 403 {
		t.Fatalf("non-admin changed settings: %d", code)
	}
	if code, _, _ := siti.do("POST", "/api/users", `{"name":"x","password":"pw5678"}`); code != 403 {
		t.Fatalf("non-admin added a user: %d", code)
	}
	if code, _, _ := siti.do("PUT", "/api/users/Monar", `{"password":"hacked1"}`); code != 403 {
		t.Fatalf("non-admin changed another password: %d", code)
	}
	// She can change her own, with her current password.
	if code, _, _ := siti.do("PUT", "/api/users/Siti", `{"password":"newpw1","current":"wrong"}`); code != 400 {
		t.Fatalf("own password with wrong current: %d", code)
	}
	if code, _, body := siti.do("PUT", "/api/users/Siti", `{"password":"newpw1","current":"pw5678"}`); code != 200 {
		t.Fatalf("own password: %d %s", code, body)
	}
	if code, _, _ := siti.do("GET", "/api/state", ""); code != 200 {
		t.Fatal("changing her password signed this device out")
	}

	// Signing out.
	siti.do("POST", "/api/logout", "")
	if code, _, _ := siti.do("GET", "/api/state", ""); code != 401 {
		t.Fatalf("after logout: %d", code)
	}

	// Deleting removes the account and its files.
	if code, _, body := owner.do("DELETE", "/api/users/Siti", ""); code != 200 {
		t.Fatalf("delete: %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(dir, "users", users.DirName("Siti"))); !os.IsNotExist(err) {
		t.Fatal("Siti's files are still there")
	}
	if code, _, _ := owner.do("DELETE", "/api/users/Monar", ""); code != 400 {
		t.Fatalf("deleting the last admin: %d", code)
	}

	// Too many wrong passwords from one address are slowed down.
	for i := 0; i < 5; i++ {
		anon.do("POST", "/api/login", `{"name":"monar","password":"bad"}`)
	}
	if code, _, _ := anon.do("POST", "/api/login", `{"name":"monar","password":"pw1234"}`); code != http.StatusTooManyRequests {
		t.Fatalf("after 5 wrong passwords: %d", code)
	}
}

func TestSignUp(t *testing.T) {
	dir := t.TempDir()
	cfg := settings.Load(filepath.Join(dir, "settings.json"), settings.Settings{})
	s := &Server{Store: store.New(filepath.Join(dir, "library.json")), Users: users.Load(filepath.Join(dir, "users.json")),
		DataDir: dir, Settings: cfg, Web: fstest.MapFS{"index.html": {Data: []byte("app")}}}
	h := s.Handler()
	admin := &client{t: t, h: h}
	admin.do("POST", "/api/accounts/setup", `{"name":"Monar","password":"pw1234"}`)

	// Off by default.
	guest := &client{t: t, h: h}
	if _, m, _ := guest.do("GET", "/api/session", ""); m["signUp"] != false {
		t.Fatalf("sign-up should be off: %v", m)
	}
	if code, _, _ := guest.do("POST", "/api/signup", `{"name":"Ali","password":"pw1234"}`); code != 403 {
		t.Fatalf("sign-up while off: %d", code)
	}

	// Open: anyone can make an account, which isn't an admin, and is signed in.
	if code, _, body := admin.do("PUT", "/api/settings", `{"signUp":true}`); code != 200 {
		t.Fatalf("turn on: %d %s", code, body)
	}
	if _, m, _ := guest.do("GET", "/api/session", ""); m["signUp"] != true || m["needsCode"] != false {
		t.Fatalf("session: %v", m)
	}
	code, m, body := guest.do("POST", "/api/signup", `{"name":"Ali","password":"pw1234"}`)
	if code != 200 || m["admin"] != false {
		t.Fatalf("sign-up: %d %s", code, body)
	}
	if code, _, _ := guest.do("GET", "/api/state", ""); code != 200 {
		t.Fatal("not signed in after signing up")
	}
	if code, _, _ := guest.do("PUT", "/api/settings", `{"signUp":false}`); code != 403 {
		t.Fatalf("a signed-up account changed settings: %d", code)
	}
	if code, _, _ := (&client{t: t, h: h}).do("POST", "/api/signup", `{"name":"ali","password":"pw1234"}`); code != 400 {
		t.Fatalf("taken name: %d", code)
	}

	// With an invite code: needed, compared without case, hidden from non-admins.
	admin.do("PUT", "/api/settings", `{"inviteCode":" Mango7 "}`)
	g2 := &client{t: t, h: h}
	if _, m, _ := g2.do("GET", "/api/session", ""); m["needsCode"] != true {
		t.Fatalf("needsCode: %v", m)
	}
	if code, _, _ := g2.do("POST", "/api/signup", `{"name":"Siti","password":"pw1234","code":"apple"}`); code != 403 {
		t.Fatalf("wrong code: %d", code)
	}
	if code, _, body := g2.do("POST", "/api/signup", `{"name":"Siti","password":"pw1234","code":"mango7"}`); code != 200 {
		t.Fatalf("right code: %d %s", code, body)
	}
	_, _, body = g2.do("GET", "/api/settings", "")
	if strings.Contains(string(body), "Mango7") {
		t.Fatal("a non-admin can read the invite code")
	}
	_, _, body = admin.do("GET", "/api/settings", "")
	if !strings.Contains(string(body), "Mango7") {
		t.Fatalf("the admin can't read the invite code: %s", body)
	}
}
