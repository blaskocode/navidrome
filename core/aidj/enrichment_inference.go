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

// InferEnrichment derives mood, energy, and vibe from BPM and genre
func InferEnrichment(mf *model.MediaFile) (moods []string, energy EnergyLevel, vibes []string) {
	// Infer energy from BPM
	energy = inferEnergyFromBPM(mf.BPM)

	// Get genre-based defaults
	moods, vibes = inferFromGenre(mf.Genre)

	// Adjust moods based on energy if genre didn't provide strong signal
	if len(moods) == 0 {
		moods = inferMoodsFromEnergy(energy)
	}

	return moods, energy, vibes
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
