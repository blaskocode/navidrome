package aidj

import (
	"sync"
	"time"

	"github.com/navidrome/navidrome/model/criteria"
)

// ThemeRegistry holds all available themes
type ThemeRegistry struct {
	themes     map[string]*Theme
	byCategory map[ThemeCategory][]*Theme
}

var (
	registryInstance *ThemeRegistry
	registryOnce     sync.Once
)

// GetThemeRegistry returns the singleton theme registry
func GetThemeRegistry() *ThemeRegistry {
	registryOnce.Do(func() {
		registryInstance = newThemeRegistry()
	})
	return registryInstance
}

// GetTheme returns a theme by ID
func (r *ThemeRegistry) GetTheme(id string) *Theme {
	return r.themes[id]
}

// GetThemesByCategory returns all themes in a category
func (r *ThemeRegistry) GetThemesByCategory(category ThemeCategory) []*Theme {
	return r.byCategory[category]
}

// AllThemes returns all registered themes
func (r *ThemeRegistry) AllThemes() []*Theme {
	result := make([]*Theme, 0, len(r.themes))
	for _, t := range r.themes {
		result = append(result, t)
	}
	return result
}

func newThemeRegistry() *ThemeRegistry {
	r := &ThemeRegistry{
		themes:     make(map[string]*Theme),
		byCategory: make(map[ThemeCategory][]*Theme),
	}

	// Register all built-in themes
	for _, theme := range builtinThemes() {
		r.themes[theme.ID] = theme
		r.byCategory[theme.Category] = append(r.byCategory[theme.Category], theme)
	}

	return r
}

func builtinThemes() []*Theme {
	now := time.Now()
	currentYear := now.Year()

	return []*Theme{
		// === ERA THEMES ===
		{
			ID:          "80s-classics",
			Category:    ThemeCategoryEra,
			Name:        "80s Classics",
			Description: "classic hits from the 1980s",
			Priority:    50,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheRange{"year": []int{1980, 1989}},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "90s-hits",
			Category:    ThemeCategoryEra,
			Name:        "90s Hits",
			Description: "favorites from the 1990s",
			Priority:    50,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheRange{"year": []int{1990, 1999}},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "2000s-throwbacks",
			Category:    ThemeCategoryEra,
			Name:        "2000s Throwbacks",
			Description: "throwbacks from the 2000s",
			Priority:    50,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheRange{"year": []int{2000, 2009}},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "2010s-favorites",
			Category:    ThemeCategoryEra,
			Name:        "2010s Favorites",
			Description: "tracks from the 2010s",
			Priority:    50,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheRange{"year": []int{2010, 2019}},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "recent-releases",
			Category:    ThemeCategoryEra,
			Name:        "Recent Releases",
			Description: "recently released music",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Gt{"year": currentYear - 3},
				},
				Sort:  "random",
				Limit: 50,
			},
		},

		// === MOOD THEMES ===
		// Note: Mood values from enrichment_inference.go:
		// chill (jazz, soul, r&b, reggae, low-energy fallback), soft (folk, acoustic, indie),
		// calm (classical, ambient), upbeat (electronic, dance, hip-hop, pop),
		// intense/energetic (rock, metal, punk), melancholy (blues)
		{
			ID:          "chill-vibes",
			Category:    ThemeCategoryMood,
			Name:        "Chill Vibes",
			Description: "relaxing tracks to unwind",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.Any{
					criteria.Is{"ai_mood": "chill"},
					criteria.Is{"ai_mood": "soft"},
					criteria.Is{"ai_mood": "calm"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "upbeat-energy",
			Category:    ThemeCategoryMood,
			Name:        "Upbeat Energy",
			Description: "energetic tracks to get moving",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.Any{
					criteria.Is{"ai_mood": "upbeat"},
					criteria.Is{"ai_mood": "energetic"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "intense-power",
			Category:    ThemeCategoryMood,
			Name:        "Intense Power",
			Description: "powerful and intense tracks",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.Any{
					criteria.Is{"ai_mood": "intense"},
					criteria.Is{"ai_mood": "energetic"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "mellow-mood",
			Category:    ThemeCategoryMood,
			Name:        "Mellow Mood",
			Description: "soft and mellow music",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.Any{
					criteria.Is{"ai_mood": "soft"},
					criteria.Is{"ai_mood": "calm"},
					criteria.Is{"ai_mood": "melancholy"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},

		// === FAMILIARITY THEMES ===
		{
			ID:          "your-favorites",
			Category:    ThemeCategoryFamiliarity,
			Name:        "Your Favorites",
			Description: "your most loved tracks",
			Priority:    80,
			Criteria: criteria.Criteria{
				Expression: criteria.Any{
					criteria.Gt{"rating": 3},
					criteria.Is{"loved": true},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "deep-cuts",
			Category:    ThemeCategoryFamiliarity,
			Name:        "Deep Cuts",
			Description: "tracks you haven't heard in a while",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Gt{"playcount": 0},
					criteria.NotInTheLast{"lastplayed": 90},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "never-played",
			Category:    ThemeCategoryFamiliarity,
			Name:        "Never Played",
			Description: "tracks you've never listened to",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Is{"playcount": 0},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "heavy-rotation",
			Category:    ThemeCategoryFamiliarity,
			Name:        "Heavy Rotation",
			Description: "your most played tracks",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Gt{"playcount": 10},
				},
				Sort:  "-playcount",
				Limit: 50,
			},
		},

		// === RATING THEMES ===
		{
			ID:          "five-star",
			Category:    ThemeCategoryRating,
			Name:        "Five Star Tracks",
			Description: "your highest rated music",
			Priority:    80,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Is{"rating": 5},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "highly-rated-discoveries",
			Category:    ThemeCategoryRating,
			Name:        "Highly Rated Discoveries",
			Description: "great tracks you rarely play",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Gt{"rating": 3},
					criteria.Lt{"playcount": 5},
				},
				Sort:  "random",
				Limit: 50,
			},
		},

		// === ENERGY THEMES ===
		{
			ID:          "high-energy",
			Category:    ThemeCategoryEnergy,
			Name:        "High Energy",
			Description: "high-tempo tracks to energize",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Is{"ai_energy": "high"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},
		{
			ID:          "low-energy",
			Category:    ThemeCategoryEnergy,
			Name:        "Low Energy",
			Description: "slower, relaxed tracks",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.Is{"ai_energy": "low"},
				},
				Sort:  "random",
				Limit: 50,
			},
		},

		// === TEMPORAL THEMES ===
		{
			ID:          "recently-added",
			Category:    ThemeCategoryTemporal,
			Name:        "Recently Added",
			Description: "new additions to your library",
			Priority:    70,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheLast{"dateadded": 30},
				},
				Sort:  "-dateadded",
				Limit: 50,
			},
		},
		{
			ID:          "recently-played",
			Category:    ThemeCategoryTemporal,
			Name:        "Recently Played",
			Description: "tracks you've enjoyed lately",
			Priority:    60,
			Criteria: criteria.Criteria{
				Expression: criteria.All{
					criteria.InTheLast{"lastplayed": 7},
				},
				Sort:  "-lastplayed",
				Limit: 50,
			},
		},

		// === FALLBACK THEME ===
		// This theme matches ALL tracks and serves as a fallback when other themes
		// don't have enough matching tracks
		{
			ID:          "mix-tape",
			Category:    ThemeCategoryMood,
			Name:        "Mix Tape",
			Description: "a random mix from your library",
			Priority:    10, // Low priority - only selected when others fail
			Criteria: criteria.Criteria{
				Expression: nil, // No criteria - matches all tracks
				Sort:       "random",
				Limit:      100, // Larger pool for variety
			},
		},
	}
}
