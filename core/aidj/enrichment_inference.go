package aidj

import (
	"strings"

	"github.com/navidrome/navidrome/model"
)

// EnergyLevel represents track energy classification
type EnergyLevel string

const (
	EnergyLow    EnergyLevel = "low"
	EnergyMedium EnergyLevel = "medium"
	EnergyHigh   EnergyLevel = "high"
)

// GenreMoodMapping maps genres to default moods and vibes
var GenreMoodMapping = map[string]struct {
	Moods []string
	Vibes []string
}{
	"jazz":              {[]string{"chill"}, []string{"coffeeshop", "late-night"}},
	"bossa nova":        {[]string{"chill"}, []string{"coffeeshop"}},
	"lounge":            {[]string{"chill"}, []string{"coffeeshop"}},
	"electronic":        {[]string{"upbeat"}, []string{"workout", "party"}},
	"dance":             {[]string{"upbeat"}, []string{"workout", "party"}},
	"house":             {[]string{"upbeat"}, []string{"workout", "party"}},
	"techno":            {[]string{"upbeat", "intense"}, []string{"workout", "party"}},
	"classical":         {[]string{"calm"}, []string{"focus"}},
	"ambient":           {[]string{"calm"}, []string{"focus"}},
	"new age":           {[]string{"calm"}, []string{"focus"}},
	"rock":              {[]string{"intense", "energetic"}, []string{}},
	"metal":             {[]string{"intense"}, []string{}},
	"punk":              {[]string{"intense", "energetic"}, []string{}},
	"r&b":               {[]string{"chill"}, []string{"romantic"}},
	"soul":              {[]string{"chill"}, []string{"romantic"}},
	"neo-soul":          {[]string{"chill"}, []string{"romantic"}},
	"hip-hop":           {[]string{"upbeat"}, []string{}},
	"rap":               {[]string{"upbeat"}, []string{}},
	"folk":              {[]string{"soft"}, []string{"coffeeshop"}},
	"acoustic":          {[]string{"soft"}, []string{"coffeeshop"}},
	"singer-songwriter": {[]string{"soft"}, []string{"coffeeshop"}},
	"blues":             {[]string{"melancholy"}, []string{"late-night"}},
	"country":           {[]string{"soft"}, []string{}},
	"reggae":            {[]string{"chill"}, []string{}},
	"indie":             {[]string{"soft"}, []string{"coffeeshop"}},
	"pop":               {[]string{"upbeat"}, []string{}},
}

// titleMoodKeywords maps title keywords to mood overrides
// These take precedence over genre-based inference
var titleMoodKeywords = map[string][]string{
	// Soft/mellow indicators
	"acoustic":  {"soft"},
	"unplugged": {"soft"},
	"piano":     {"soft"},
	"ballad":    {"soft"},
	"slow":      {"soft"},
	"gentle":    {"soft"},
	"quiet":     {"soft"},
	"stripped":  {"soft"},
	"solo":      {"soft"},
	"lullaby":   {"soft"},

	// Live performances - clear genre default, rely on BPM
	"live at":   {},
	"live on":   {},
	"live in":   {},
	"concert":   {},
	"(live)":    {},
	"- live":    {},
	"live from": {},

	// Upbeat/energetic indicators
	"remix":      {"upbeat"},
	"club mix":   {"upbeat"},
	"dance mix":  {"upbeat"},
	"party":      {"upbeat"},
	"radio edit": {"upbeat"},

	// Intense indicators
	"metal":    {"intense"},
	"hardcore": {"intense"},
	"heavy":    {"intense"},
	"scream":   {"intense"},
}

// albumMoodKeywords maps album name keywords to mood hints
var albumMoodKeywords = map[string][]string{
	"acoustic":   {"soft"},
	"unplugged":  {"soft"},
	"sessions":   {"soft"},
	"stripped":   {"soft"},
	"piano":      {"soft"},
	"live at":    {}, // Defer to BPM
	"live in":    {},
	"live from":  {},
	"in concert": {},
}

// InferEnrichment derives mood, energy, and vibe from BPM, title, album, and genre
func InferEnrichment(mf *model.MediaFile) (moods []string, energy EnergyLevel, vibes []string) {
	// Infer energy from BPM first
	energy = inferEnergyFromBPM(mf.BPM)

	// Check title for mood overrides BEFORE genre (highest priority)
	titleMoods, titleCleared := inferFromTitle(mf.Title)
	if len(titleMoods) > 0 {
		moods = titleMoods
	} else if !titleCleared {
		// Check album name for hints (second priority)
		albumMoods, albumCleared := inferFromAlbum(mf.Album)
		if len(albumMoods) > 0 {
			moods = albumMoods
		} else if !albumCleared {
			// Fall back to genre-based inference (lowest priority)
			moods, vibes = inferFromGenre(mf.Genre)
		}
	}

	// BPM override: Very slow songs should be "soft" regardless of genre
	// This catches cases like slow pop ballads tagged as "upbeat" due to genre
	if mf.BPM > 0 && mf.BPM < 80 {
		if !containsMood(moods, "soft", "chill", "calm", "melancholy") {
			moods = []string{"soft"}
		}
	}

	// Adjust moods based on energy if still empty
	if len(moods) == 0 {
		moods = inferMoodsFromEnergy(energy)
	}

	return moods, energy, vibes
}

// inferFromTitle checks the title for mood-indicating keywords
// Returns moods and a "cleared" flag (true if we matched a "live" keyword that clears genre defaults)
func inferFromTitle(title string) (moods []string, cleared bool) {
	if title == "" {
		return nil, false
	}

	normalized := strings.ToLower(title)

	for keyword, keywordMoods := range titleMoodKeywords {
		if strings.Contains(normalized, keyword) {
			if len(keywordMoods) == 0 {
				// Empty moods means "clear genre default, use BPM"
				return nil, true
			}
			return keywordMoods, false
		}
	}

	return nil, false
}

// inferFromAlbum checks the album name for mood-indicating keywords
func inferFromAlbum(album string) (moods []string, cleared bool) {
	if album == "" {
		return nil, false
	}

	normalized := strings.ToLower(album)

	for keyword, keywordMoods := range albumMoodKeywords {
		if strings.Contains(normalized, keyword) {
			if len(keywordMoods) == 0 {
				return nil, true
			}
			return keywordMoods, false
		}
	}

	return nil, false
}

// containsMood checks if any of the target moods are in the moods slice
func containsMood(moods []string, targets ...string) bool {
	for _, mood := range moods {
		for _, target := range targets {
			if mood == target {
				return true
			}
		}
	}
	return false
}

func inferEnergyFromBPM(bpm int) EnergyLevel {
	switch {
	case bpm == 0:
		return EnergyMedium // Unknown BPM defaults to medium
	case bpm < 90:
		return EnergyLow
	case bpm > 120:
		return EnergyHigh
	default:
		return EnergyMedium
	}
}

func inferFromGenre(genre string) (moods []string, vibes []string) {
	if genre == "" {
		return nil, nil
	}

	// Normalize genre for lookup
	normalized := strings.ToLower(strings.TrimSpace(genre))

	// Direct match
	if mapping, ok := GenreMoodMapping[normalized]; ok {
		return mapping.Moods, mapping.Vibes
	}

	// Partial match (e.g., "Progressive Rock" contains "rock")
	for key, mapping := range GenreMoodMapping {
		if strings.Contains(normalized, key) {
			return mapping.Moods, mapping.Vibes
		}
	}

	return nil, nil
}

func inferMoodsFromEnergy(energy EnergyLevel) []string {
	switch energy {
	case EnergyLow:
		return []string{"chill"}
	case EnergyHigh:
		return []string{"energetic"}
	default:
		return []string{}
	}
}
