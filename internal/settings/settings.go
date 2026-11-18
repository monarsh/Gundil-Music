// Package settings keeps the server-wide choices made in the app's Settings
// screen: the YouTube Music region and language, the audio quality the server
// plays at, and whether YouTube support updates itself daily.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

type Settings struct {
	Region       string `json:"region"`       // country code, "" for YouTube's choice
	Language     string `json:"language"`     // YouTube Music language, e.g. en
	AudioQuality string `json:"audioQuality"` // low | normal | high, for playing on the server
	AutoUpdate   bool   `json:"autoUpdate"`   // update yt-dlp and ytmusicapi daily

	// With accounts: may people create their own account from the sign-in
	// screen, and the code they must give (none: anyone on the network).
	SignUp     bool   `json:"signUp"`
	InviteCode string `json:"inviteCode"`
}

// Public is what everyone may see: the invite code is for admins only.
func (s Settings) Public() Settings {
	s.InviteCode = ""
	return s
}

// Store holds the settings and saves them to a JSON file.
type Store struct {
	path string
	mu   sync.Mutex
	s    Settings
}

// Load reads the settings file. Values missing from it (or the whole file,
// on first start) come from defaults, which hold the command-line options.
func Load(path string, defaults Settings) *Store {
	st := &Store{path: path, s: defaults}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &st.s)
	}
	st.s = clean(st.s)
	return st
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update applies a change and saves it. It returns the old and new settings.
func (st *Store) Update(change func(*Settings)) (old, now Settings, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	old = st.s
	next := st.s
	change(&next)
	next = clean(next)
	if err := Validate(next); err != nil {
		return old, old, err
	}
	data, _ := json.MarshalIndent(next, "", "  ")
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return old, old, err
	}
	if err := os.Rename(tmp, st.path); err != nil {
		return old, old, err
	}
	st.s = next
	return old, next, nil
}

func clean(s Settings) Settings {
	s.Region = strings.ToUpper(strings.TrimSpace(s.Region))
	s.Language = strings.TrimSpace(s.Language)
	if s.Language == "" {
		s.Language = "en"
	}
	if s.AudioQuality == "" {
		s.AudioQuality = "normal"
	}
	s.InviteCode = strings.TrimSpace(s.InviteCode)
	return s
}

// Validate checks values that end up on the YouTube helper's command line.
func Validate(s Settings) error {
	if len(s.Region) > 3 || strings.Trim(s.Region, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return errors.New("unknown region")
	}
	if len(s.Language) > 8 || strings.Trim(s.Language, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-") != "" {
		return errors.New("unknown language")
	}
	if len([]rune(s.InviteCode)) > 64 {
		return errors.New("use an invite code of up to 64 characters")
	}
	switch s.AudioQuality {
	case "low", "normal", "high":
	default:
		return errors.New("audio quality must be low, normal or high")
	}
	return nil
}
