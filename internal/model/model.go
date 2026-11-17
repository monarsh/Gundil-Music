// Package model holds the track type shared by the player, store and API.
package model

// Ref names an artist or album, with its YouTube Music browse ID if known.
type Ref struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

// Track is a playable YouTube Music song or video.
type Track struct {
	Type     string  `json:"type,omitempty"` // song | video
	VideoID  string  `json:"videoId"`
	Title    string  `json:"title"`
	Artists  []Ref   `json:"artists"`
	Album    *Ref    `json:"album,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Thumb    string  `json:"thumb,omitempty"`
	Explicit bool    `json:"explicit,omitempty"`
}

// Valid reports whether t looks like a real YouTube video ID.
func (t Track) Valid() bool { return ValidID(t.VideoID) }

// ValidID checks the 11-character YouTube video ID format.
func ValidID(id string) bool {
	if len(id) != 11 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
