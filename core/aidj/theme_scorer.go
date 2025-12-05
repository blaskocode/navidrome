package aidj

import (
	"math"
	"sort"
	"time"

	"github.com/navidrome/navidrome/model"
)

// ThemeScore represents a theme with its calculated score
type ThemeScore struct {
	Theme *Theme
	Score float64
}

// ThemeScorer calculates theme scores based on context
type ThemeScorer struct{}

// NewThemeScorer creates a new theme scorer
func NewThemeScorer() *ThemeScorer {
	return &ThemeScorer{}
}

// ScoreThemes returns all themes sorted by score (descending)
func (s *ThemeScorer) ScoreThemes(session *model.DJSession) []ThemeScore {
	registry := GetThemeRegistry()
	themes := registry.AllThemes()
	scores := make([]ThemeScore, 0, len(themes))

	for _, theme := range themes {
		score := s.calculateScore(theme, session)
		if score > 0 {
			scores = append(scores, ThemeScore{Theme: theme, Score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	return scores
}

// SelectBestTheme returns the highest-scoring theme
func (s *ThemeScorer) SelectBestTheme(session *model.DJSession) *Theme {
	scores := s.ScoreThemes(session)
	if len(scores) == 0 {
		return nil
	}
	return scores[0].Theme
}

func (s *ThemeScorer) calculateScore(theme *Theme, session *model.DJSession) float64 {
	// Check if theme is completely excluded (pivot-triggered)
	if session != nil && session.ExcludedThemes != nil && session.ExcludedThemes[theme.ID] {
		return 0 // Completely excluded - not selectable
	}

	// Start with base priority (normalized to 0-1)
	score := float64(theme.Priority) / 100.0

	// Apply time-of-day bonus
	score += s.timeOfDayBonus(theme)

	// Apply penalties
	score -= s.recentlyUsedPenalty(theme, session)
	score -= s.skippedThemePenalty(theme, session)
	score -= s.skippedMoodPenalty(theme, session)

	// Ensure non-negative
	if score < 0 {
		score = 0
	}

	return score
}

// timeOfDayBonus gives bonus to themes appropriate for current time
func (s *ThemeScorer) timeOfDayBonus(theme *Theme) float64 {
	hour := time.Now().Hour()

	// Morning (6-11): favor upbeat, high-energy
	if hour >= 6 && hour < 12 {
		if theme.Category == ThemeCategoryEnergy && theme.ID == "high-energy" {
			return 0.15
		}
		if theme.Category == ThemeCategoryMood && theme.ID == "upbeat-energy" {
			return 0.15
		}
	}

	// Afternoon (12-17): balanced, favor familiarity
	if hour >= 12 && hour < 18 {
		if theme.Category == ThemeCategoryFamiliarity {
			return 0.1
		}
	}

	// Evening (18-22): chill vibes, mellow
	if hour >= 18 && hour < 22 {
		if theme.ID == "chill-vibes" || theme.ID == "mellow-mood" || theme.ID == "low-energy" {
			return 0.15
		}
	}

	// Late night (22-6): low energy, deep cuts
	if hour >= 22 || hour < 6 {
		if theme.ID == "low-energy" || theme.ID == "deep-cuts" {
			return 0.2
		}
	}

	return 0
}

// recentlyUsedPenalty penalizes themes used recently in the session
func (s *ThemeScorer) recentlyUsedPenalty(theme *Theme, session *model.DJSession) float64 {
	if session == nil || len(session.ThemesUsed) == 0 {
		return 0
	}

	// Check how recently the theme was used
	for i := len(session.ThemesUsed) - 1; i >= 0; i-- {
		if session.ThemesUsed[i] == theme.ID {
			// More recent = higher penalty
			recency := len(session.ThemesUsed) - i
			if recency <= 2 {
				return 0.5 // Heavy penalty for last 2 themes
			}
			if recency <= 4 {
				return 0.25 // Medium penalty for themes 3-4 ago
			}
			return 0.1 // Light penalty for older themes
		}
	}
	return 0
}

// skippedThemePenalty penalizes themes that have been skipped
func (s *ThemeScorer) skippedThemePenalty(theme *Theme, session *model.DJSession) float64 {
	if session == nil || session.SkippedThemes == nil {
		return 0
	}

	skipCount := session.SkippedThemes[theme.ID]
	if skipCount == 0 {
		return 0
	}

	// Apply decay based on session duration
	decay := s.calculateDecay(session)
	effectiveSkips := float64(skipCount) * decay

	// 3+ skips = significant penalty
	if effectiveSkips >= 3 {
		return 0.4
	}
	if effectiveSkips >= 2 {
		return 0.25
	}
	return 0.1
}

// skippedMoodPenalty penalizes themes matching frequently-skipped moods
func (s *ThemeScorer) skippedMoodPenalty(theme *Theme, session *model.DJSession) float64 {
	if session == nil || session.SkippedMoods == nil {
		return 0
	}

	// Map theme to moods it represents
	moods := s.themeMoods(theme)
	if len(moods) == 0 {
		return 0
	}

	decay := s.calculateDecay(session)
	totalPenalty := 0.0

	for _, mood := range moods {
		skipCount := session.SkippedMoods[mood]
		effectiveSkips := float64(skipCount) * decay
		if effectiveSkips >= 3 {
			totalPenalty += 0.3
		} else if effectiveSkips >= 2 {
			totalPenalty += 0.15
		}
	}

	// Cap total penalty
	if totalPenalty > 0.5 {
		totalPenalty = 0.5
	}

	return totalPenalty
}

// themeMoods returns the moods associated with a theme
func (s *ThemeScorer) themeMoods(theme *Theme) []string {
	switch theme.ID {
	case "chill-vibes":
		return []string{"chill"}
	case "upbeat-energy":
		return []string{"upbeat"}
	case "intense-power":
		return []string{"intense"}
	case "mellow-mood":
		return []string{"soft"}
	case "high-energy":
		return []string{"high"}
	case "low-energy":
		return []string{"low"}
	default:
		return nil
	}
}

// calculateDecay returns a decay factor based on session duration
// Decay reduces by 50% every 30 minutes
func (s *ThemeScorer) calculateDecay(session *model.DJSession) float64 {
	if session == nil || session.CreatedAt.IsZero() {
		return 1.0
	}

	elapsed := time.Since(session.CreatedAt)
	halfLives := elapsed.Minutes() / 30.0

	return math.Pow(0.5, halfLives)
}
