// Package users keeps the Gundil Music accounts on a server: who can sign
// in, their password hashes and their signed-in devices. Accounts are
// optional. Until the first one is made, the server has a single shared
// library and nobody signs in.
package users

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// User is an account. Name is how it signs in and is shown.
type User struct {
	Name    string `json:"name"`
	Admin   bool   `json:"admin"`
	Created int64  `json:"created"`
	Salt    string `json:"salt,omitempty"`
	Hash    string `json:"hash,omitempty"`
}

type session struct {
	User    string `json:"user"`
	Expires int64  `json:"expires"`
}

type file struct {
	Users    []*User            `json:"users"`
	Sessions map[string]session `json:"sessions"`
}

const (
	iterations  = 120000
	SessionLife = 365 * 24 * time.Hour
)

var (
	ErrExists    = errors.New("that name is taken")
	ErrNotFound  = errors.New("no such account")
	ErrLastAdmin = errors.New("keep at least one admin account")
	validName    = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N} ._-]{0,31}$`)
)

type Store struct {
	path string
	mu   sync.Mutex
	f    file
}

// Load reads the accounts from path. A missing file means no accounts.
func Load(path string) *Store {
	s := &Store{path: path}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &s.f)
	}
	if s.f.Sessions == nil {
		s.f.Sessions = map[string]session{}
	}
	return s
}

func (s *Store) saveLocked() error {
	now := time.Now().Unix()
	for k, v := range s.f.Sessions {
		if v.Expires < now {
			delete(s.f.Sessions, k)
		}
	}
	data, err := json.MarshalIndent(&s.f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Enabled says whether any account exists, so people must sign in.
func (s *Store) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.f.Users) > 0
}

func hash(password, salt string) string {
	key, _ := pbkdf2.Key(sha256.New, password, []byte(salt), iterations, 32)
	return hex.EncodeToString(key)
}

func token() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) findLocked(name string) *User {
	for _, u := range s.f.Users {
		if strings.EqualFold(u.Name, name) {
			return u
		}
	}
	return nil
}

// CheckPassword says what's wrong with a new password, if anything.
func CheckPassword(pw string) error {
	if len([]rune(pw)) < 4 {
		return errors.New("use a password of at least 4 characters")
	}
	if len(pw) > 200 {
		return errors.New("that password is too long")
	}
	return nil
}

// Create adds an account. The first account is always an admin.
func (s *Store) Create(name, password string, admin bool) (User, error) {
	name = strings.TrimSpace(name)
	if !validName.MatchString(name) {
		return User{}, errors.New("use a name of up to 32 letters, numbers, spaces, dots, dashes or underscores")
	}
	if err := CheckPassword(password); err != nil {
		return User{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.findLocked(name) != nil {
		return User{}, ErrExists
	}
	salt := token()
	u := &User{Name: name, Admin: admin || len(s.f.Users) == 0, Created: time.Now().Unix(), Salt: salt, Hash: hash(password, salt)}
	s.f.Users = append(s.f.Users, u)
	if err := s.saveLocked(); err != nil {
		s.f.Users = s.f.Users[:len(s.f.Users)-1]
		return User{}, err
	}
	return public(u), nil
}

func public(u *User) User { return User{Name: u.Name, Admin: u.Admin, Created: u.Created} }

// Login checks a name and password and starts a session.
func (s *Store) Login(name, password string) (User, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(strings.TrimSpace(name))
	if u == nil {
		hash(password, "timing") // don't reveal which names exist by timing
		return User{}, "", false
	}
	if subtle.ConstantTimeCompare([]byte(hash(password, u.Salt)), []byte(u.Hash)) != 1 {
		return User{}, "", false
	}
	t := token()
	s.f.Sessions[t] = session{User: u.Name, Expires: time.Now().Add(SessionLife).Unix()}
	s.saveLocked()
	return public(u), t, true
}

// Verify checks a password without starting a session.
func (s *Store) Verify(name, password string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(name)
	return u != nil && subtle.ConstantTimeCompare([]byte(hash(password, u.Salt)), []byte(u.Hash)) == 1
}

// Session returns the account signed in with this session token.
func (s *Store) Session(t string) (User, bool) {
	if t == "" {
		return User{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	se, ok := s.f.Sessions[t]
	if !ok || se.Expires < time.Now().Unix() {
		return User{}, false
	}
	u := s.findLocked(se.User)
	if u == nil {
		return User{}, false
	}
	return public(u), true
}

// Logout ends one session.
func (s *Store) Logout(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.f.Sessions[t]; ok {
		delete(s.f.Sessions, t)
		s.saveLocked()
	}
}

// Get returns an account by name.
func (s *Store) Get(name string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(name)
	if u == nil {
		return User{}, false
	}
	return public(u), true
}

// List returns every account, admins first.
func (s *Store) List() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]User, 0, len(s.f.Users))
	for _, u := range s.f.Users {
		out = append(out, public(u))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Admin != out[j].Admin {
			return out[i].Admin
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Admins lists the admin accounts' names.
func (s *Store) Admins() []string {
	var out []string
	for _, u := range s.List() {
		if u.Admin {
			out = append(out, u.Name)
		}
	}
	return out
}

// SetPassword changes a password and signs the account out everywhere
// except the session keep (pass "" to sign out everywhere).
func (s *Store) SetPassword(name, password, keep string) error {
	if err := CheckPassword(password); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(name)
	if u == nil {
		return ErrNotFound
	}
	u.Salt = token()
	u.Hash = hash(password, u.Salt)
	for k, v := range s.f.Sessions {
		if strings.EqualFold(v.User, u.Name) && k != keep {
			delete(s.f.Sessions, k)
		}
	}
	return s.saveLocked()
}

// SetAdmin gives or takes away admin rights.
func (s *Store) SetAdmin(name string, admin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(name)
	if u == nil {
		return ErrNotFound
	}
	if !admin && u.Admin && s.adminsLocked() == 1 {
		return ErrLastAdmin
	}
	u.Admin = admin
	return s.saveLocked()
}

func (s *Store) adminsLocked() int {
	n := 0
	for _, u := range s.f.Users {
		if u.Admin {
			n++
		}
	}
	return n
}

// Delete removes an account and signs it out.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.findLocked(name)
	if u == nil {
		return ErrNotFound
	}
	if u.Admin && s.adminsLocked() == 1 {
		return ErrLastAdmin
	}
	out := s.f.Users[:0]
	for _, x := range s.f.Users {
		if x != u {
			out = append(out, x)
		}
	}
	s.f.Users = out
	for k, v := range s.f.Sessions {
		if strings.EqualFold(v.User, u.Name) {
			delete(s.f.Sessions, k)
		}
	}
	return s.saveLocked()
}

// DirName is the folder name for an account's files. It is encoded so any
// name is safe as a folder on every system (Windows refuses "con" or "a.").
func DirName(name string) string { return "u-" + hex.EncodeToString([]byte(strings.ToLower(name))) }
