// Package store keeps the user's library on disk: liked and disliked songs,
// playlists, saved albums, playlists and artists, history and searches.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/monarsh/gundil-music/internal/model"
)

type Playlist struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Tracks      []model.Track `json:"tracks"`
	Created     int64         `json:"created"`
	Updated     int64         `json:"updated"`
}

// Saved is an album, YouTube playlist or artist added to the library.
type Saved struct {
	Type     string `json:"type"` // album | playlist | artist
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Thumb    string `json:"thumb"`
	Added    int64  `json:"added"`
}

type State struct {
	Playlists []*Playlist   `json:"playlists"`
	Liked     []model.Track `json:"liked"` // newest first
	Disliked  []string      `json:"disliked"`
	Saved     []Saved       `json:"saved"`   // newest first
	History   []model.Track `json:"history"` // most recent first
	Searches  []string      `json:"searches"`
}

const (
	maxHistory  = 300
	maxSearches = 30
)

type Store struct {
	path string
	mu   sync.Mutex
	st   State
}

var ErrNotFound = errors.New("not found")

func New(path string) *Store {
	s := &Store{path: path}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &s.st)
	}
	if s.st.Playlists == nil {
		s.st.Playlists = []*Playlist{}
	}
	for _, p := range s.st.Playlists {
		if p.Tracks == nil {
			p.Tracks = []model.Track{}
		}
	}
	for _, l := range []*[]model.Track{&s.st.Liked, &s.st.History} {
		if *l == nil {
			*l = []model.Track{}
		}
	}
	if s.st.Disliked == nil {
		s.st.Disliked = []string{}
	}
	if s.st.Saved == nil {
		s.st.Saved = []Saved{}
	}
	if s.st.Searches == nil {
		s.st.Searches = []string{}
	}
	return s
}

// JSON returns the whole state.
func (s *Store) JSON() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(&s.st)
	return data
}

func (s *Store) saveLocked() {
	data, err := json.Marshal(&s.st)
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, s.path)
	}
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "GM" + hex.EncodeToString(b)
}

func without(list []model.Track, id string) []model.Track {
	out := list[:0]
	for _, t := range list {
		if t.VideoID != id {
			out = append(out, t)
		}
	}
	return out
}

func clean(tracks []model.Track) []model.Track {
	out := make([]model.Track, 0, len(tracks))
	for _, t := range tracks {
		if t.Valid() {
			out = append(out, t)
		}
	}
	return out
}

// SetRating likes ("like"), dislikes ("dislike") or clears ("none") a song.
func (s *Store) SetRating(t model.Track, rating string) error {
	if !t.Valid() {
		return errors.New("invalid track")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Liked = without(s.st.Liked, t.VideoID)
	dis := s.st.Disliked[:0]
	for _, id := range s.st.Disliked {
		if id != t.VideoID {
			dis = append(dis, id)
		}
	}
	s.st.Disliked = dis
	switch rating {
	case "like":
		s.st.Liked = append([]model.Track{t}, s.st.Liked...)
	case "dislike":
		s.st.Disliked = append(s.st.Disliked, t.VideoID)
	case "none":
	default:
		return errors.New("rating must be like, dislike or none")
	}
	s.saveLocked()
	return nil
}

func (s *Store) AddHistory(t model.Track) {
	if !t.Valid() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.History = append([]model.Track{t}, without(s.st.History, t.VideoID)...)
	if len(s.st.History) > maxHistory {
		s.st.History = s.st.History[:maxHistory]
	}
	s.saveLocked()
}

func (s *Store) ClearHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.History = []model.Track{}
	s.saveLocked()
}

func (s *Store) ClearSearches() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Searches = []string{}
	s.saveLocked()
}

func (s *Store) AddSearch(q string) {
	q = strings.TrimSpace(q)
	if q == "" || len(q) > 200 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []string{q}
	for _, x := range s.st.Searches {
		if !strings.EqualFold(x, q) {
			out = append(out, x)
		}
	}
	if len(out) > maxSearches {
		out = out[:maxSearches]
	}
	s.st.Searches = out
	s.saveLocked()
}

func (s *Store) RemoveSearch(q string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.st.Searches[:0]
	for _, x := range s.st.Searches {
		if q != "" && x != q {
			out = append(out, x)
		}
	}
	s.st.Searches = out
	s.saveLocked()
}

// SetSaved adds or removes an album, playlist or artist from the library.
func (s *Store) SetSaved(it Saved, saved bool) error {
	if it.ID == "" || (it.Type != "album" && it.Type != "playlist" && it.Type != "artist") {
		return errors.New("invalid item")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.st.Saved[:0]
	for _, x := range s.st.Saved {
		if !(x.ID == it.ID && x.Type == it.Type) {
			out = append(out, x)
		}
	}
	s.st.Saved = out
	if saved {
		it.Added = time.Now().Unix()
		s.st.Saved = append([]Saved{it}, s.st.Saved...)
	}
	s.saveLocked()
	return nil
}

func (s *Store) CreatePlaylist(name, description string, tracks []model.Track) (*Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("playlist name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	p := &Playlist{ID: newID(), Name: name, Description: strings.TrimSpace(description), Tracks: clean(tracks), Created: now, Updated: now}
	s.st.Playlists = append([]*Playlist{p}, s.st.Playlists...)
	s.saveLocked()
	return p, nil
}

func (s *Store) find(id string) *Playlist {
	for _, p := range s.st.Playlists {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// UpdatePlaylist changes the name, description and/or the full track list.
func (s *Store) UpdatePlaylist(id string, name, description *string, tracks *[]model.Track) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.find(id)
	if p == nil {
		return ErrNotFound
	}
	if name != nil && strings.TrimSpace(*name) != "" {
		p.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		p.Description = strings.TrimSpace(*description)
	}
	if tracks != nil {
		p.Tracks = clean(*tracks)
	}
	p.Updated = time.Now().Unix()
	s.saveLocked()
	return nil
}

// AddToPlaylist appends tracks that are not already in it and returns how
// many were added.
func (s *Store) AddToPlaylist(id string, tracks []model.Track) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.find(id)
	if p == nil {
		return 0, ErrNotFound
	}
	have := map[string]bool{}
	for _, t := range p.Tracks {
		have[t.VideoID] = true
	}
	added := 0
	for _, t := range clean(tracks) {
		if !have[t.VideoID] {
			have[t.VideoID] = true
			p.Tracks = append(p.Tracks, t)
			added++
		}
	}
	p.Updated = time.Now().Unix()
	s.saveLocked()
	return added, nil
}

func (s *Store) DeletePlaylist(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.st.Playlists {
		if p.ID == id {
			s.st.Playlists = append(s.st.Playlists[:i], s.st.Playlists[i+1:]...)
			s.saveLocked()
			return nil
		}
	}
	return ErrNotFound
}

// Disliked reports whether a video was given a thumbs down.
func (s *Store) Disliked(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.st.Disliked {
		if x == id {
			return true
		}
	}
	return false
}
