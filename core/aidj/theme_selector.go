package aidj

import (
	"context"
	"math/rand"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
)

// ThemeSelector handles theme-based track selection
type ThemeSelector struct {
	ds model.DataStore
}

// NewThemeSelector creates a new theme selector
func NewThemeSelector(ds model.DataStore) *ThemeSelector {
	return &ThemeSelector{ds: ds}
}

// SelectTracksForTheme selects tracks matching a theme, excluding already-played tracks
// If preferences are provided, they are combined with theme criteria
func (s *ThemeSelector) SelectTracksForTheme(ctx context.Context, theme *Theme, excludeTrackIDs []string, preferences *model.UserPreferences) (*ThemeSet, error) {
	prefFilter := NewPreferenceFilter(preferences)

	// Minimum tracks needed for a viable set
	minTracks := conf.Server.AIDj.SetSizeMin
	if minTracks == 0 {
		minTracks = 3
	}

	// Try with full preferences first, then relax if insufficient tracks
	if prefFilter.HasFilters() {
		relaxations := prefFilter.RelaxedFilters()
		log.Debug(ctx, "Preference filter has filters", "numRelaxations", len(relaxations), "theme", theme.ID)
		for i, filters := range relaxations {
			pool, err := s.getTrackPoolWithFilters(ctx, theme, excludeTrackIDs, filters)
			if err != nil {
				log.Error(ctx, "Error getting track pool with filters", "relaxation", i, "numFilters", len(filters), err)
				return nil, err
			}
			log.Debug(ctx, "Track pool with filters", "relaxation", i, "numFilters", len(filters), "poolSize", len(pool), "minTracks", minTracks)
			// Use this filter level if we have enough tracks
			if len(pool) >= minTracks {
				return s.buildThemeSet(theme, pool)
			}
		}
	}

	// Fall back to no preference filters
	log.Debug(ctx, "Falling back to theme-only query", "theme", theme.ID)
	pool, err := s.getTrackPool(ctx, theme, excludeTrackIDs)
	if err != nil {
		return nil, err
	}

	log.Debug(ctx, "Theme-only pool", "theme", theme.ID, "poolSize", len(pool), "minTracks", minTracks)
	if len(pool) < minTracks {
		return nil, nil // Not enough tracks for a viable set
	}

	return s.buildThemeSet(theme, pool)
}

// buildThemeSet creates a ThemeSet from a track pool
func (s *ThemeSelector) buildThemeSet(theme *Theme, pool model.MediaFiles) (*ThemeSet, error) {
	// Select set size (3-5 tracks)
	setSize := s.determineSetSize(len(pool))
	log.Debug("buildThemeSet", "theme", theme.ID, "poolSize", len(pool), "setSize", setSize)

	// Shuffle and select tracks
	rand.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})

	selected := pool
	if len(pool) > setSize {
		selected = pool[:setSize]
	}

	// Extract track IDs and keep full track data
	trackIDs := make([]string, len(selected))
	for i, mf := range selected {
		trackIDs[i] = mf.ID
	}

	return &ThemeSet{
		Theme:    theme,
		TrackIDs: trackIDs,
		Tracks:   selected, // Include full track data for commentary
	}, nil
}

// getTrackPool queries tracks matching the theme criteria
func (s *ThemeSelector) getTrackPool(ctx context.Context, theme *Theme, excludeTrackIDs []string) (model.MediaFiles, error) {
	return s.getTrackPoolWithFilters(ctx, theme, excludeTrackIDs, nil)
}

// getTrackPoolWithFilters queries tracks matching theme criteria combined with additional filters
func (s *ThemeSelector) getTrackPoolWithFilters(ctx context.Context, theme *Theme, excludeTrackIDs []string, additionalFilters []criteria.Expression) (model.MediaFiles, error) {
	repo := s.ds.MediaFile(ctx)

	// Build combined expression: theme criteria AND all additional filters
	var expression criteria.Expression
	if len(additionalFilters) > 0 {
		// Combine theme expression with preference filters using All (AND)
		allExprs := make(criteria.All, 0, len(additionalFilters)+1)
		if theme.Criteria.Expression != nil {
			allExprs = append(allExprs, theme.Criteria.Expression)
		}
		allExprs = append(allExprs, additionalFilters...)
		expression = allExprs
	} else {
		expression = theme.Criteria.Expression
	}

	// Build query options from theme criteria
	// Use OrderBy() to get the properly resolved sort field (e.g., "-dateadded" -> "media_file.created_at desc")
	opts := model.QueryOptions{
		Filters: expression,
		Sort:    theme.Criteria.OrderBy(),
		Max:     theme.Criteria.Limit,
	}

	// Get matching tracks
	tracks, err := repo.GetAll(opts)
	if err != nil {
		return nil, err
	}

	// Filter out excluded tracks (already played in session)
	if len(excludeTrackIDs) > 0 {
		excludeSet := make(map[string]bool, len(excludeTrackIDs))
		for _, id := range excludeTrackIDs {
			excludeSet[id] = true
		}

		filtered := make(model.MediaFiles, 0, len(tracks))
		for _, track := range tracks {
			if !excludeSet[track.ID] {
				filtered = append(filtered, track)
			}
		}
		tracks = filtered
	}

	return tracks, nil
}

// determineSetSize returns the set size based on available tracks
func (s *ThemeSelector) determineSetSize(poolSize int) int {
	minSize := conf.Server.AIDj.SetSizeMin
	maxSize := conf.Server.AIDj.SetSizeMax

	// Use defaults if not configured
	if minSize == 0 {
		minSize = 3
	}
	if maxSize == 0 {
		maxSize = 5
	}

	if poolSize < minSize {
		return poolSize
	}

	// Random size between min and max
	if maxSize > minSize {
		return minSize + rand.Intn(maxSize-minSize+1)
	}
	return minSize
}

// CountTracksForTheme returns the number of tracks matching a theme
func (s *ThemeSelector) CountTracksForTheme(ctx context.Context, theme *Theme) (int64, error) {
	repo := s.ds.MediaFile(ctx)

	opts := model.QueryOptions{
		Filters: theme.Criteria.Expression,
	}

	return repo.CountAll(opts)
}
