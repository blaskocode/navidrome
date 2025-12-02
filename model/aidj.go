package model

import (
	"time"
)

type DJMode string

const (
	DJModeDefault   DJMode = "default"
	DJModeNostalgia DJMode = "nostalgia"
	DJModeDiscovery DJMode = "discovery"
)

type DJSession struct {
	ID             string
	UserID         string
	Mode           DJMode
	CreatedAt      time.Time
	LastActive     time.Time
	Queue          []string       // track IDs
	History        []string       // track IDs played this session
	SkippedArtists map[string]int // artist ID -> skip count (for downweighting)
	SkippedGenres  map[string]int // genre -> skip count
	SeedTrackIDs   []string
	SeedArtistIDs  []string
	SeedPlaylistID *string
	ChapterText    string
}

type DJState struct {
	SessionID   string      `json:"sessionId"`
	Mode        DJMode      `json:"mode"`
	NowPlaying  *MediaFile  `json:"nowPlaying,omitempty"`
	UpNext      []MediaFile `json:"upNext"`
	ChapterText string      `json:"chapterText,omitempty"`
}

func (m DJMode) IsValid() bool {
	switch m {
	case DJModeDefault, DJModeNostalgia, DJModeDiscovery:
		return true
	}
	return false
}
