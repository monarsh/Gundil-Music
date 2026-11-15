// Package store persists playlists, liked songs and play history.
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
)

type Playlist struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Tracks  []string `json:"tracks"`
	Created int64    `json:"created"`
}

type Play struct {
	Count int   `json:"count"`
	Last  int64 `json:"last"`
}

type State struct {
	Playlists []*Playlist      `json:"playlists"`
	Liked     []string         `json:"liked"`
	Plays     map[string]*Play `json:"plays"`
}

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
	if s.st.Liked == nil {
		s.st.Liked = []string{}
	}
	if s.st.Plays == nil {
		s.st.Plays = map[string]*Play{}
	}
	return s
}

// JSON returns the whole state encoded while holding the lock.
func (s *Store) JSON() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(&s.st)
	return data
}

func (s *Store) saveLocked() {
	data, err := json.MarshalIndent(&s.st, "", " ")
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
	return hex.EncodeToString(b)
}

func (s *Store) CreatePlaylist(name string, tracks []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("playlist name is required")
	}
	if tracks == nil {
		tracks = []string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Playlists = append(s.st.Playlists, &Playlist{ID: newID(), Name: name, Tracks: tracks, Created: time.Now().Unix()})
	s.saveLocked()
	return nil
}

// UpdatePlaylist renames a playlist and/or replaces its tracks.
func (s *Store) UpdatePlaylist(id string, name *string, tracks *[]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.st.Playlists {
		if p.ID != id {
			continue
		}
		if name != nil && strings.TrimSpace(*name) != "" {
			p.Name = strings.TrimSpace(*name)
		}
		if tracks != nil {
			p.Tracks = *tracks
			if p.Tracks == nil {
				p.Tracks = []string{}
			}
		}
		s.saveLocked()
		return nil
	}
	return ErrNotFound
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

func (s *Store) SetLiked(id string, liked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.st.Liked[:0]
	for _, x := range s.st.Liked {
		if x != id {
			out = append(out, x)
		}
	}
	if liked {
		out = append([]string{id}, out...)
	}
	s.st.Liked = out
	s.saveLocked()
}

func (s *Store) RecordPlay(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.st.Plays[id]
	if p == nil {
		p = &Play{}
		s.st.Plays[id] = p
	}
	p.Count++
	p.Last = time.Now().Unix()
	s.saveLocked()
}
