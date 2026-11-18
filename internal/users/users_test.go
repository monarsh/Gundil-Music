package users

import (
	"path/filepath"
	"testing"
)

func TestAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	s := Load(path)
	if s.Enabled() {
		t.Fatal("no accounts yet")
	}
	owner, err := s.Create("Monar", "secret1", false)
	if err != nil || !owner.Admin {
		t.Fatalf("first account must be admin: %+v %v", owner, err)
	}
	if _, err := s.Create("monar", "other1", false); err != ErrExists {
		t.Fatalf("names are case-insensitive: %v", err)
	}
	for _, bad := range []string{"", " ", "a/b", "../x", ".hidden"} {
		if _, err := s.Create(bad, "secret1", false); err == nil {
			t.Errorf("name %q should be refused", bad)
		}
	}
	if _, err := s.Create("Siti Aminah", "abc", false); err == nil {
		t.Fatal("short password should be refused")
	}
	if _, err := s.Create("Siti Aminah", "abcd", false); err != nil {
		t.Fatal(err)
	}

	if _, _, ok := s.Login("monar", "wrong"); ok {
		t.Fatal("wrong password accepted")
	}
	u, tok, ok := s.Login("MONAR", "secret1")
	if !ok || u.Name != "Monar" || tok == "" {
		t.Fatalf("login: %+v %q %v", u, tok, ok)
	}
	if got, ok := s.Session(tok); !ok || got.Name != "Monar" {
		t.Fatalf("session: %+v %v", got, ok)
	}

	// Remembered across restarts, without exposing hashes.
	again := Load(path)
	if got, ok := again.Session(tok); !ok || got.Name != "Monar" {
		t.Fatalf("session after reload: %+v %v", got, ok)
	}
	for _, x := range again.List() {
		if x.Hash != "" || x.Salt != "" {
			t.Fatal("List leaks password hashes")
		}
	}

	// The last admin can't be removed or demoted.
	if err := s.Delete("Monar"); err != ErrLastAdmin {
		t.Fatalf("delete last admin: %v", err)
	}
	if err := s.SetAdmin("Monar", false); err != ErrLastAdmin {
		t.Fatalf("demote last admin: %v", err)
	}

	// A new password signs out other sessions but can keep the current one.
	_, tok2, _ := s.Login("monar", "secret1")
	if err := s.SetPassword("monar", "newpass", tok); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session(tok); !ok {
		t.Fatal("kept session was signed out")
	}
	if _, ok := s.Session(tok2); ok {
		t.Fatal("other session survived the password change")
	}
	if !s.Verify("monar", "newpass") || s.Verify("monar", "secret1") {
		t.Fatal("password not changed")
	}

	_, tok3, _ := s.Login("siti aminah", "abcd")
	if err := s.Delete("Siti Aminah"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Session(tok3); ok {
		t.Fatal("deleted account still signed in")
	}
	if DirName("Con") == DirName("con.") || DirName("A") != DirName("a") {
		t.Fatal("DirName")
	}
}
