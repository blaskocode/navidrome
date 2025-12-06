package aidj

import (
	"fmt"
	"math/rand"

	"github.com/navidrome/navidrome/model"
)

// ContinuityParameter represents a parameter that can be preserved between sets
type ContinuityParameter string

const (
	ContinuityEnergy ContinuityParameter = "energy"
	ContinuityMood   ContinuityParameter = "mood"
	ContinuityDecade ContinuityParameter = "decade"
	ContinuityArtist ContinuityParameter = "artist"
	ContinuityGenre  ContinuityParameter = "genre"
)

// ContinuityAnalyzer analyzes played tracks to find dominant parameters
type ContinuityAnalyzer struct{}

// NewContinuityAnalyzer creates a new analyzer
func NewContinuityAnalyzer() *ContinuityAnalyzer {
	return &ContinuityAnalyzer{}
}

// AnalyzeTracks examines recently played tracks and returns the most common value for each parameter
func (ca *ContinuityAnalyzer) AnalyzeTracks(tracks []model.MediaFile) map[ContinuityParameter]string {
	result := make(map[ContinuityParameter]string)

	if len(tracks) == 0 {
		return result
	}

	// Count occurrences of each value per parameter
	energyCounts := make(map[string]int)
	moodCounts := make(map[string]int)
	decadeCounts := make(map[string]int)
	artistCounts := make(map[string]int)
	genreCounts := make(map[string]int)

	for _, track := range tracks {
		// Energy from ai_energy tag
		if track.Tags != nil {
			if energies, ok := track.Tags["ai_energy"]; ok && len(energies) > 0 {
				energyCounts[energies[0]]++
			}
			// Mood from ai_mood tag
			if moods, ok := track.Tags["ai_mood"]; ok && len(moods) > 0 {
				moodCounts[moods[0]]++
			}
		}

		// Decade from year
		if track.Year > 0 {
			decade := (track.Year / 10) * 10
			decadeCounts[fmt.Sprintf("%d", decade)]++
		}

		// Artist
		if track.ArtistID != "" {
			artistCounts[track.ArtistID]++
		}

		// Genre
		if track.Genre != "" {
			genreCounts[track.Genre]++
		}
	}

	// Find dominant value for each parameter
	result[ContinuityEnergy] = findDominant(energyCounts)
	result[ContinuityMood] = findDominant(moodCounts)
	result[ContinuityDecade] = findDominant(decadeCounts)
	result[ContinuityArtist] = findDominant(artistCounts)
	result[ContinuityGenre] = findDominant(genreCounts)

	return result
}

// findDominant returns the key with highest count, or empty string if none
func findDominant(counts map[string]int) string {
	var maxKey string
	var maxCount int
	for k, v := range counts {
		if v > maxCount {
			maxKey = k
			maxCount = v
		}
	}
	return maxKey
}

// SelectRandomParameter picks a random parameter that has a value
func (ca *ContinuityAnalyzer) SelectRandomParameter(params map[ContinuityParameter]string) (ContinuityParameter, string) {
	// Collect parameters that have values
	var available []ContinuityParameter
	for param, value := range params {
		if value != "" {
			available = append(available, param)
		}
	}

	if len(available) == 0 {
		return "", ""
	}

	// Pick random one
	chosen := available[rand.Intn(len(available))] //nolint:gosec // Non-cryptographic use: random parameter selection
	return chosen, params[chosen]
}

// FindThemesMatchingParameter returns themes that match the given parameter value
func (ca *ContinuityAnalyzer) FindThemesMatchingParameter(param ContinuityParameter, value string, excludeThemes map[string]bool) []*Theme {
	registry := GetThemeRegistry()
	allThemes := registry.AllThemes()

	var matches []*Theme
	for _, theme := range allThemes {
		if excludeThemes != nil && excludeThemes[theme.ID] {
			continue
		}

		if ca.themeMatchesParameter(theme, param, value) {
			matches = append(matches, theme)
		}
	}

	return matches
}

// themeMatchesParameter checks if a theme matches a specific parameter value
func (ca *ContinuityAnalyzer) themeMatchesParameter(theme *Theme, param ContinuityParameter, value string) bool {
	switch param {
	case ContinuityEnergy:
		// Match energy themes
		if theme.Category == ThemeCategoryEnergy {
			if value == "high" && theme.ID == "high-energy" {
				return true
			}
			if value == "low" && theme.ID == "low-energy" {
				return true
			}
		}
		// Also match mood themes with energy associations
		if theme.Category == ThemeCategoryMood {
			if value == "high" && (theme.ID == "upbeat-energy" || theme.ID == "intense-power") {
				return true
			}
			if value == "low" && (theme.ID == "chill-vibes" || theme.ID == "mellow-mood") {
				return true
			}
		}

	case ContinuityMood:
		if theme.Category == ThemeCategoryMood {
			// Map mood values to theme IDs
			moodToTheme := map[string][]string{
				"chill":     {"chill-vibes", "mellow-mood"},
				"soft":      {"chill-vibes", "mellow-mood"},
				"calm":      {"chill-vibes", "mellow-mood"},
				"upbeat":    {"upbeat-energy"},
				"energetic": {"upbeat-energy", "intense-power"},
				"intense":   {"intense-power"},
			}
			if themeIDs, ok := moodToTheme[value]; ok {
				for _, tid := range themeIDs {
					if theme.ID == tid {
						return true
					}
				}
			}
		}

	case ContinuityDecade:
		if theme.Category == ThemeCategoryEra {
			// Map decade strings to theme IDs
			decadeToTheme := map[string]string{
				"1980": "80s-classics",
				"1990": "90s-nostalgia",
				"2000": "2000s-hits",
				"2010": "2010s-vibes",
				"2020": "recent-releases",
			}
			if tid, ok := decadeToTheme[value]; ok && theme.ID == tid {
				return true
			}
		}

	case ContinuityGenre:
		// Genre matching would require analyzing theme criteria
		// For now, skip direct genre matching (themes don't have genre category yet)
		return false

	case ContinuityArtist:
		// Artist continuity doesn't map to existing themes
		// Could be used for "more like this artist" in future
		return false
	}

	return false
}
