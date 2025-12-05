package aidj

import (
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
)

// ThemeCategory represents the type of theme
type ThemeCategory string

const (
	ThemeCategoryEra         ThemeCategory = "era"
	ThemeCategoryMood        ThemeCategory = "mood"
	ThemeCategoryFamiliarity ThemeCategory = "familiarity"
	ThemeCategoryArtist      ThemeCategory = "artist"
	ThemeCategoryGenre       ThemeCategory = "genre"
	ThemeCategoryRating      ThemeCategory = "rating"
	ThemeCategoryTemporal    ThemeCategory = "temporal"
	ThemeCategoryEnergy      ThemeCategory = "energy"
)

// Theme defines a themed selection of tracks
type Theme struct {
	ID          string            // Unique identifier (e.g., "80s-classics")
	Category    ThemeCategory     // Category for grouping
	Name        string            // Display name (e.g., "80s Classics")
	Description string            // For commentary (e.g., "classic hits from the 1980s")
	Priority    int               // Base priority for autonomous selection (1-100)
	Criteria    criteria.Criteria // Filter criteria for track selection
}

// ThemeSet represents a set of tracks selected for a theme
type ThemeSet struct {
	Theme    *Theme           // The theme used for selection
	TrackIDs []string         // Selected track IDs (3-5 tracks)
	Tracks   model.MediaFiles // Full track data for commentary
}
