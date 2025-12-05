package aidj

import (
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
)

// PreferenceFilter translates UserPreferences into SQL filter criteria
type PreferenceFilter struct {
	preferences *model.UserPreferences
}

// NewPreferenceFilter creates a new preference filter from user preferences
func NewPreferenceFilter(prefs *model.UserPreferences) *PreferenceFilter {
	return &PreferenceFilter{preferences: prefs}
}

// HasFilters returns true if any preferences are set
func (pf *PreferenceFilter) HasFilters() bool {
	if pf.preferences == nil {
		return false
	}
	return pf.preferences.Energy != nil ||
		pf.preferences.Decade != nil ||
		len(pf.preferences.Contexts) > 0
}

// ToExpressions returns criteria expressions for the user preferences
// These should be combined with theme criteria using All (AND)
func (pf *PreferenceFilter) ToExpressions() []criteria.Expression {
	if pf.preferences == nil {
		return nil
	}

	var expressions []criteria.Expression

	// Add energy filter
	if energyExpr := pf.energyExpression(); energyExpr != nil {
		expressions = append(expressions, energyExpr)
	}

	// Add decade filter
	if decadeExpr := pf.decadeExpression(); decadeExpr != nil {
		expressions = append(expressions, decadeExpr)
	}

	// Add context filters
	for _, ctx := range pf.preferences.Contexts {
		if ctxExpr := pf.contextExpression(ctx); ctxExpr != nil {
			expressions = append(expressions, ctxExpr)
		}
	}

	return expressions
}

// energyExpression returns the criteria expression for energy preference
// Energy Mapping:
// "upbeat" -> ai_mood in (upbeat, energetic, happy) OR ai_energy = high
// "soft"   -> ai_mood in (chill, soft, calm, mellow) OR ai_energy = low
// "intense"-> ai_mood in (intense, powerful, aggressive) OR ai_energy = high
func (pf *PreferenceFilter) energyExpression() criteria.Expression {
	if pf.preferences.Energy == nil {
		return nil
	}

	switch *pf.preferences.Energy {
	case "upbeat":
		return criteria.Any{
			criteria.Is{"ai_mood": "upbeat"},
			criteria.Is{"ai_mood": "energetic"},
			criteria.Is{"ai_mood": "happy"},
			criteria.Is{"ai_energy": "high"},
		}
	case "soft":
		return criteria.Any{
			criteria.Is{"ai_mood": "chill"},
			criteria.Is{"ai_mood": "soft"},
			criteria.Is{"ai_mood": "calm"},
			criteria.Is{"ai_mood": "mellow"},
			criteria.Is{"ai_energy": "low"},
		}
	case "intense":
		return criteria.Any{
			criteria.Is{"ai_mood": "intense"},
			criteria.Is{"ai_mood": "powerful"},
			criteria.Is{"ai_mood": "aggressive"},
			criteria.Is{"ai_mood": "energetic"},
			criteria.Is{"ai_energy": "high"},
		}
	default:
		return nil
	}
}

// decadeExpression returns the criteria expression for decade preference
// Maps decade to year range filter
func (pf *PreferenceFilter) decadeExpression() criteria.Expression {
	if pf.preferences.Decade == nil {
		return nil
	}

	decade := *pf.preferences.Decade
	// Valid decades: 1960, 1970, 1980, 1990, 2000, 2010, 2020
	if decade < 1960 || decade > 2020 || decade%10 != 0 {
		return nil
	}

	return criteria.InTheRange{"year": []int{decade, decade + 9}}
}

// contextExpression returns the criteria expression for a specific context
// Context Mapping:
// "forgotten" -> play_count > 0 AND play_date < (NOW - 90 days)
// "favorites" -> starred = true OR rating >= 4
// "new"       -> play_count = 0
func (pf *PreferenceFilter) contextExpression(ctx string) criteria.Expression {
	switch ctx {
	case "forgotten":
		return criteria.All{
			criteria.Gt{"playcount": 0},
			criteria.NotInTheLast{"lastplayed": 90},
		}
	case "favorites":
		return criteria.Any{
			criteria.Is{"loved": true},
			criteria.Gt{"rating": 3}, // rating >= 4
		}
	case "new":
		return criteria.Is{"playcount": 0}
	default:
		return nil
	}
}

// RelaxedFilters returns progressively relaxed filter expressions
// Relaxation order: drop decade first, then contexts, then energy
// Returns a slice of expression slices, from most restrictive to least
func (pf *PreferenceFilter) RelaxedFilters() [][]criteria.Expression {
	if pf.preferences == nil {
		return nil
	}

	var result [][]criteria.Expression

	// Start with full filters
	full := pf.ToExpressions()
	if len(full) > 0 {
		result = append(result, full)
	}

	// Relaxation 1: Drop decade filter
	if pf.preferences.Decade != nil {
		withoutDecade := pf.filtersWithoutDecade()
		if len(withoutDecade) > 0 && len(withoutDecade) < len(full) {
			result = append(result, withoutDecade)
		}
	}

	// Relaxation 2: Drop decade and contexts
	if len(pf.preferences.Contexts) > 0 || pf.preferences.Decade != nil {
		withoutDecadeAndContexts := pf.filtersWithoutDecadeAndContexts()
		if len(withoutDecadeAndContexts) > 0 {
			alreadyAdded := false
			for _, existing := range result {
				if len(existing) == len(withoutDecadeAndContexts) {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				result = append(result, withoutDecadeAndContexts)
			}
		}
	}

	// Relaxation 3: No filters (empty slice means no preference restrictions)
	result = append(result, nil)

	return result
}

// filtersWithoutDecade returns filters with decade filter removed
func (pf *PreferenceFilter) filtersWithoutDecade() []criteria.Expression {
	var expressions []criteria.Expression

	if energyExpr := pf.energyExpression(); energyExpr != nil {
		expressions = append(expressions, energyExpr)
	}

	for _, ctx := range pf.preferences.Contexts {
		if ctxExpr := pf.contextExpression(ctx); ctxExpr != nil {
			expressions = append(expressions, ctxExpr)
		}
	}

	return expressions
}

// filtersWithoutDecadeAndContexts returns filters with decade and context filters removed
// (only energy remains)
func (pf *PreferenceFilter) filtersWithoutDecadeAndContexts() []criteria.Expression {
	var expressions []criteria.Expression

	if energyExpr := pf.energyExpression(); energyExpr != nil {
		expressions = append(expressions, energyExpr)
	}

	return expressions
}
