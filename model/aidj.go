package model

import (
	"time"
)

// UserPreferences captures user preferences for AI DJ sessions
type UserPreferences struct {
	Energy   *string  `json:"energy,omitempty"`   // "upbeat", "soft", "intense", nil = any
	Decade   *int     `json:"decade,omitempty"`   // 1960, 1970, 1980, 1990, 2000, 2010, 2020
	Contexts []string `json:"contexts,omitempty"` // "forgotten", "favorites", "new"
}

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

	// New fields for theme engine
	CurrentThemeID string   // Active theme ID
	CurrentSetSize int      // Tracks remaining in current set
	ThemesUsed     []string // Theme IDs used this session

	// Enhanced skip tracking for adaptation (Phase C)
	SkippedMoods     map[string]int  // mood -> skip count (for deprioritizing moods)
	SkippedThemes    map[string]int  // theme ID -> skip count
	ExcludedThemes   map[string]bool // themes excluded after pivot (2 skips = permanent exclusion)
	CurrentSetSkips  int             // skips in current set (reset on new set)
	CurrentSetPlayed int             // tracks played in current set
	SetStartTime     time.Time       // when current set started (for decay)

	// Pre-loaded sets for continuous playback
	QueuedSets []QueuedSet `json:"-"` // Not serialized directly, exposed via DJState

	// Autonomous mode flag
	AutonomousMode bool // true if DJ is selecting themes autonomously

	// User preferences for track filtering
	UserPreferences *UserPreferences `json:"userPreferences,omitempty"`
}

// QueuedSet represents a pre-built themed set ready for playback
type QueuedSet struct {
	ThemeID    string      `json:"themeId"`
	ThemeName  string      `json:"themeName"`
	TrackIDs   []string    `json:"trackIds"`
	Tracks     []MediaFile `json:"tracks"`
	Commentary string      `json:"commentary"`
	IsCurrent  bool        `json:"isCurrent"` // true if this is the actively playing set
}

type DJState struct {
	SessionID   string      `json:"sessionId"`
	Mode        DJMode      `json:"mode"`
	NowPlaying  *MediaFile  `json:"nowPlaying,omitempty"`
	UpNext      []MediaFile `json:"upNext"`
	ChapterText string      `json:"chapterText,omitempty"`

	// Theme fields (current set)
	ThemeID   string `json:"themeId,omitempty"`
	ThemeName string `json:"themeName,omitempty"`

	// Set progress info
	SetProgress  int  `json:"setProgress,omitempty"`  // tracks played in current set
	SetSize      int  `json:"setSize,omitempty"`      // total tracks in current set
	IsAutonomous bool `json:"isAutonomous,omitempty"` // true if autonomous mode
	IsExhausted  bool `json:"isExhausted,omitempty"`  // true if library is exhausted

	// Pre-loaded sets with metadata
	Sets []QueuedSet `json:"sets,omitempty"`
}

func (m DJMode) IsValid() bool {
	switch m {
	case DJModeDefault, DJModeNostalgia, DJModeDiscovery:
		return true
	}
	return false
}
