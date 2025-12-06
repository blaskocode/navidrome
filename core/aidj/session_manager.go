package aidj

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	SessionExpiry        = 60 * time.Minute
	CleanupInterval      = 5 * time.Minute
	QueueSizeInitial     = 30
	QueueRefillThreshold = 10
	PreloadSetCount      = 3 // Number of sets to keep pre-loaded
)

type SessionManager struct {
	sessions      sync.Map // map[sessionID]*model.DJSession
	selector      *TrackSelector
	themeSelector *ThemeSelector
	themeScorer   *ThemeScorer
}

func NewSessionManager(ds model.DataStore) *SessionManager {
	sm := &SessionManager{
		selector:      NewTrackSelector(ds),
		themeSelector: NewThemeSelector(ds),
		themeScorer:   NewThemeScorer(),
	}
	go sm.cleanupLoop()
	return sm
}

// buildQueuedSet creates a QueuedSet from a ThemeSet with generated commentary
func buildQueuedSet(set *ThemeSet, session *model.DJSession, isFirst bool) model.QueuedSet {
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsFirstSet = isFirst

	// Convert MediaFiles to model.MediaFile slice
	tracks := make([]model.MediaFile, len(set.Tracks))
	for i, t := range set.Tracks {
		tracks[i] = t
	}

	return model.QueuedSet{
		ThemeID:    set.Theme.ID,
		ThemeName:  set.Theme.Name,
		TrackIDs:   set.TrackIDs,
		Tracks:     tracks,
		Commentary: GenerateThemeCommentary(set.Theme, commentaryCtx),
		IsCurrent:  false,
	}
}

// getAllQueuedTrackIDs returns all track IDs across all queued sets and history
func (sm *SessionManager) getAllQueuedTrackIDs(session *model.DJSession) []string {
	ids := make([]string, 0, len(session.History))
	ids = append(ids, session.History...)

	for _, qs := range session.QueuedSets {
		ids = append(ids, qs.TrackIDs...)
	}
	return ids
}

// flattenQueuedSets converts queued sets into a flat track ID queue
func (sm *SessionManager) flattenQueuedSets(session *model.DJSession) []string {
	var queue []string
	for _, qs := range session.QueuedSets {
		queue = append(queue, qs.TrackIDs...)
	}
	return queue
}

// selectNextSetWithContinuity selects the next set using continuity logic
// 70% chance: preserve a random parameter from played tracks
// 30% chance: pick something completely different (wild card)
func (sm *SessionManager) selectNextSetWithContinuity(ctx context.Context, session *model.DJSession) (*Theme, *ThemeSet) {
	analyzer := NewContinuityAnalyzer()
	excludeIDs := sm.getAllQueuedTrackIDs(session)

	// Decide: continuity (70%) or wild card (30%)
	useContinuity := rand.Float32() < 0.7 //nolint:gosec // Non-cryptographic use: continuity ratio

	if useContinuity && len(session.History) > 0 {
		// Get recently played tracks (last set worth)
		recentCount := session.CurrentSetSize
		if recentCount == 0 {
			recentCount = 5
		}
		if recentCount > len(session.History) {
			recentCount = len(session.History)
		}

		// Load track data for analysis
		recentTracks := make([]model.MediaFile, 0, recentCount)
		for i := len(session.History) - recentCount; i < len(session.History); i++ {
			trackID := session.History[i]
			// Find track in queued sets
			if track := sm.findTrackInSession(session, trackID); track != nil {
				recentTracks = append(recentTracks, *track)
			}
		}

		if len(recentTracks) > 0 {
			// Analyze tracks for dominant parameters
			params := analyzer.AnalyzeTracks(recentTracks)

			// Pick a random parameter to preserve
			param, value := analyzer.SelectRandomParameter(params)

			if param != "" && value != "" {
				log.Debug(ctx, "Using continuity", "parameter", param, "value", value)

				// Find themes matching this parameter
				matchingThemes := analyzer.FindThemesMatchingParameter(param, value, session.ExcludedThemes)

				// Try matching themes in random order
				rand.Shuffle(len(matchingThemes), func(i, j int) {
					matchingThemes[i], matchingThemes[j] = matchingThemes[j], matchingThemes[i]
				})

				for _, theme := range matchingThemes {
					// Skip if recently used
					if sm.isRecentlyUsed(session, theme.ID) {
						continue
					}

					set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, session.UserPreferences)
					if err == nil && set != nil && len(set.TrackIDs) > 0 {
						log.Debug(ctx, "Continuity theme selected", "theme", theme.ID, "parameter", param, "value", value)
						return theme, set
					}
				}
			}
		}
	}

	// Wild card: pick best available theme (different from recent)
	log.Debug(ctx, "Using wild card selection")

	// Try all themes in score order until one works
	scores := sm.themeScorer.ScoreThemes(session)
	for _, scored := range scores {
		theme := scored.Theme

		// Skip if score is 0 (excluded themes)
		if scored.Score <= 0 {
			continue
		}

		set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, session.UserPreferences)
		if err != nil {
			log.Debug(ctx, "Theme selection failed", "theme", theme.ID, err)
			continue
		}
		if set != nil && len(set.TrackIDs) > 0 {
			log.Debug(ctx, "Wild card theme selected", "theme", theme.ID, "tracks", len(set.TrackIDs))
			return theme, set
		}
	}

	log.Debug(ctx, "No themes available with sufficient tracks")
	return nil, nil
}

// findTrackInSession looks for a track in queued sets
func (sm *SessionManager) findTrackInSession(session *model.DJSession, trackID string) *model.MediaFile {
	for _, qs := range session.QueuedSets {
		for i := range qs.Tracks {
			if qs.Tracks[i].ID == trackID {
				return &qs.Tracks[i]
			}
		}
	}
	return nil
}

// isRecentlyUsed checks if a theme was used in the last 2 sets
func (sm *SessionManager) isRecentlyUsed(session *model.DJSession, themeID string) bool {
	if len(session.ThemesUsed) == 0 {
		return false
	}

	checkCount := 2
	if checkCount > len(session.ThemesUsed) {
		checkCount = len(session.ThemesUsed)
	}

	for i := len(session.ThemesUsed) - checkCount; i < len(session.ThemesUsed); i++ {
		if session.ThemesUsed[i] == themeID {
			return true
		}
	}
	return false
}

// refillQueuedSets ensures we have PreloadSetCount sets pre-loaded
func (sm *SessionManager) refillQueuedSets(ctx context.Context, session *model.DJSession) error {
	// Keep adding sets until we have enough pre-loaded
	for len(session.QueuedSets) < PreloadSetCount {
		theme, set := sm.selectNextSetWithContinuity(ctx, session)
		if theme == nil || set == nil {
			// No more themes available - library may be exhausted
			log.Debug(ctx, "No more themes available for pre-loading", "currentSets", len(session.QueuedSets))
			break
		}

		queuedSet := buildQueuedSet(set, session, false)
		session.QueuedSets = append(session.QueuedSets, queuedSet)
		session.ThemesUsed = append(session.ThemesUsed, theme.ID)

		log.Debug(ctx, "Pre-loaded new set", "theme", theme.ID, "totalSets", len(session.QueuedSets))
	}

	return nil
}

func (sm *SessionManager) CreateSession(ctx context.Context, userID string, mode model.DJMode, seedTrackIDs, seedArtistIDs []string, seedPlaylistID *string) (*model.DJSession, error) {
	session := &model.DJSession{
		ID:             uuid.New().String(),
		UserID:         userID,
		Mode:           mode,
		CreatedAt:      time.Now(),
		LastActive:     time.Now(),
		Queue:          []string{},
		History:        []string{},
		SkippedArtists: make(map[string]int),
		SkippedGenres:  make(map[string]int),
		SeedTrackIDs:   seedTrackIDs,
		SeedArtistIDs:  seedArtistIDs,
		SeedPlaylistID: seedPlaylistID,
	}

	// Generate initial queue
	tracks, err := sm.selector.SelectTracks(ctx, session, QueueSizeInitial)
	if err != nil {
		return nil, err
	}
	session.Queue = tracks
	session.ChapterText = GenerateCommentary(session, nil)

	sm.sessions.Store(session.ID, session)
	return session, nil
}

func (sm *SessionManager) GetSession(sessionID, userID string) (*model.DJSession, error) {
	val, ok := sm.sessions.Load(sessionID)
	if !ok {
		return nil, model.ErrNotFound
	}
	session := val.(*model.DJSession)
	if session.UserID != userID {
		return nil, model.ErrNotAuthorized
	}
	if time.Since(session.LastActive) > SessionExpiry {
		sm.sessions.Delete(sessionID)
		return nil, model.ErrNotFound
	}
	return session, nil
}

// CreateThemedSession creates a new DJ session with a specific theme
func (sm *SessionManager) CreateThemedSession(ctx context.Context, userID string, themeID string) (*model.DJSession, error) {
	registry := GetThemeRegistry()
	theme := registry.GetTheme(themeID)
	if theme == nil {
		return nil, fmt.Errorf("theme not found: %s", themeID)
	}

	session := &model.DJSession{
		ID:             uuid.New().String(),
		UserID:         userID,
		Mode:           model.DJModeDefault, // Default mode for compatibility
		CreatedAt:      time.Now(),
		LastActive:     time.Now(),
		Queue:          []string{},
		History:        []string{},
		SkippedArtists: make(map[string]int),
		SkippedGenres:  make(map[string]int),
		CurrentThemeID: themeID,
		ThemesUsed:     []string{themeID},
	}

	// Select initial set
	set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("selecting tracks for theme: %w", err)
	}

	if set == nil || len(set.TrackIDs) == 0 {
		return nil, fmt.Errorf("no tracks available for theme: %s", themeID)
	}

	session.Queue = set.TrackIDs
	session.CurrentSetSize = len(set.TrackIDs)

	// Generate commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsFirstSet = true
	session.ChapterText = GenerateThemeCommentary(theme, commentaryCtx)

	sm.sessions.Store(session.ID, session)
	return session, nil
}

// CreateAutonomousSession creates a session where the DJ picks themes autonomously
func (sm *SessionManager) CreateAutonomousSession(ctx context.Context, userID string) (*model.DJSession, error) {
	session := &model.DJSession{
		ID:             uuid.New().String(),
		UserID:         userID,
		Mode:           model.DJModeDefault,
		CreatedAt:      time.Now(),
		LastActive:     time.Now(),
		Queue:          []string{},
		History:        []string{},
		SkippedArtists: make(map[string]int),
		SkippedGenres:  make(map[string]int),
		SkippedMoods:   make(map[string]int),
		SkippedThemes:  make(map[string]int),
		ThemesUsed:     []string{},
		AutonomousMode: true,
		SetStartTime:   time.Now(),
	}

	// Pre-load multiple sets
	session.QueuedSets = make([]model.QueuedSet, 0, PreloadSetCount)

	for i := 0; i < PreloadSetCount; i++ {
		var set *ThemeSet
		var theme *Theme

		// For first set, just pick best theme
		// For subsequent sets, use continuity logic
		if i == 0 {
			// Try themes until we find one with enough tracks
			for {
				theme = sm.themeScorer.SelectBestTheme(session)
				if theme == nil {
					break
				}

				log.Debug(ctx, "Trying theme for set", "setIndex", i, "theme", theme.ID)

				var err error
				excludeIDs := sm.getAllQueuedTrackIDs(session)
				set, err = sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, nil)
				if err != nil {
					log.Error(ctx, "Error selecting tracks for theme", "theme", theme.ID, err)
				}

				if set != nil && len(set.TrackIDs) > 0 {
					break
				}

				// Mark theme as temporarily excluded and try another
				if session.ExcludedThemes == nil {
					session.ExcludedThemes = make(map[string]bool)
				}
				session.ExcludedThemes[theme.ID] = true
			}
		} else {
			// Use continuity logic for subsequent sets
			theme, set = sm.selectNextSetWithContinuity(ctx, session)
		}

		if set == nil || len(set.TrackIDs) == 0 {
			if i == 0 {
				return nil, fmt.Errorf("no tracks available for any theme")
			}
			// Couldn't build more sets, that's okay
			break
		}

		queuedSet := buildQueuedSet(set, session, i == 0)
		if i == 0 {
			queuedSet.IsCurrent = true
			session.CurrentThemeID = theme.ID
			session.CurrentSetSize = len(set.TrackIDs)
		}
		session.QueuedSets = append(session.QueuedSets, queuedSet)
		session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	}

	// Clear temporary exclusions
	session.ExcludedThemes = nil

	// If we only have 1 set, try to get more tracks to ensure continuous playback
	if len(session.QueuedSets) == 1 {
		log.Debug(ctx, "Only 1 set created, attempting to add more tracks for continuous playback")
		firstTheme := session.QueuedSets[0].ThemeID
		registry := GetThemeRegistry()
		if theme := registry.GetTheme(firstTheme); theme != nil {
			excludeIDs := sm.getAllQueuedTrackIDs(session)
			set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, nil)
			if err == nil && set != nil && len(set.TrackIDs) > 0 {
				queuedSet := buildQueuedSet(set, session, false)
				session.QueuedSets = append(session.QueuedSets, queuedSet)
				session.ThemesUsed = append(session.ThemesUsed, theme.ID)
				log.Debug(ctx, "Added repeat set from same theme", "theme", theme.ID, "tracks", len(set.TrackIDs))
			}
		}
	}

	// Build flat queue from all queued sets
	session.Queue = sm.flattenQueuedSets(session)
	if len(session.QueuedSets) > 0 {
		session.ChapterText = session.QueuedSets[0].Commentary
	}

	sm.sessions.Store(session.ID, session)
	return session, nil
}

// CreatePreferencedSession creates a session where the DJ picks themes autonomously
// but applies user preference filters to track selection
func (sm *SessionManager) CreatePreferencedSession(ctx context.Context, userID string, preferences *model.UserPreferences) (*model.DJSession, error) {
	session := &model.DJSession{
		ID:              uuid.New().String(),
		UserID:          userID,
		Mode:            model.DJModeDefault,
		CreatedAt:       time.Now(),
		LastActive:      time.Now(),
		Queue:           []string{},
		History:         []string{},
		SkippedArtists:  make(map[string]int),
		SkippedGenres:   make(map[string]int),
		SkippedMoods:    make(map[string]int),
		SkippedThemes:   make(map[string]int),
		ThemesUsed:      []string{},
		AutonomousMode:  true,
		SetStartTime:    time.Now(),
		UserPreferences: preferences,
	}

	// Pre-load multiple sets
	session.QueuedSets = make([]model.QueuedSet, 0, PreloadSetCount)

	for i := 0; i < PreloadSetCount; i++ {
		var set *ThemeSet
		var theme *Theme

		// For first set, just pick best theme
		// For subsequent sets, use continuity logic
		if i == 0 {
			// Try themes until we find one with enough tracks
			for {
				theme = sm.themeScorer.SelectBestTheme(session)
				if theme == nil {
					break
				}

				log.Debug(ctx, "Trying theme for set", "setIndex", i, "theme", theme.ID)

				var err error
				excludeIDs := sm.getAllQueuedTrackIDs(session)
				set, err = sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, preferences)
				if err != nil {
					log.Error(ctx, "Error selecting tracks for theme", "theme", theme.ID, err)
				}

				if set != nil && len(set.TrackIDs) > 0 {
					break
				}

				// Mark theme as temporarily excluded and try another
				if session.ExcludedThemes == nil {
					session.ExcludedThemes = make(map[string]bool)
				}
				session.ExcludedThemes[theme.ID] = true
			}
		} else {
			// Use continuity logic for subsequent sets
			theme, set = sm.selectNextSetWithContinuity(ctx, session)
		}

		if set == nil || len(set.TrackIDs) == 0 {
			if i == 0 {
				return nil, fmt.Errorf("no tracks available for any theme with given preferences")
			}
			// Couldn't build more sets, that's okay
			break
		}

		queuedSet := buildQueuedSet(set, session, i == 0)
		if i == 0 {
			queuedSet.IsCurrent = true
			session.CurrentThemeID = theme.ID
			session.CurrentSetSize = len(set.TrackIDs)
		}
		session.QueuedSets = append(session.QueuedSets, queuedSet)
		session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	}

	// Clear temporary exclusions
	session.ExcludedThemes = nil

	// If we only have 1 set, try to get more tracks to ensure continuous playback
	// This handles small libraries or restrictive preferences
	if len(session.QueuedSets) == 1 {
		log.Debug(ctx, "Only 1 set created, attempting to add more tracks for continuous playback")
		// Try to repeat the same theme with remaining tracks
		firstTheme := session.QueuedSets[0].ThemeID
		registry := GetThemeRegistry()
		if theme := registry.GetTheme(firstTheme); theme != nil {
			excludeIDs := sm.getAllQueuedTrackIDs(session)
			set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, excludeIDs, preferences)
			if err == nil && set != nil && len(set.TrackIDs) > 0 {
				queuedSet := buildQueuedSet(set, session, false)
				session.QueuedSets = append(session.QueuedSets, queuedSet)
				session.ThemesUsed = append(session.ThemesUsed, theme.ID)
				log.Debug(ctx, "Added repeat set from same theme", "theme", theme.ID, "tracks", len(set.TrackIDs))
			}
		}
	}

	// Build flat queue from all queued sets
	session.Queue = sm.flattenQueuedSets(session)
	if len(session.QueuedSets) > 0 {
		session.ChapterText = session.QueuedSets[0].Commentary
	}

	log.Debug(ctx, "CreatePreferencedSession returning",
		"sessionID", session.ID,
		"queueLen", len(session.Queue),
		"numSets", len(session.QueuedSets),
	)

	sm.sessions.Store(session.ID, session)
	return session, nil
}

// RefillThemedQueue adds more themed tracks to the session queue
func (sm *SessionManager) RefillThemedQueue(ctx context.Context, session *model.DJSession) error {
	if session.CurrentThemeID == "" {
		// Fall back to legacy refill - not a themed session
		return nil
	}

	registry := GetThemeRegistry()
	theme := registry.GetTheme(session.CurrentThemeID)
	if theme == nil {
		return fmt.Errorf("theme not found: %s", session.CurrentThemeID)
	}

	// Exclude already played/queued tracks
	exclude := append(session.History, session.Queue...)

	set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, exclude, session.UserPreferences)
	if err != nil {
		return err
	}

	if set != nil && len(set.TrackIDs) > 0 {
		session.Queue = append(session.Queue, set.TrackIDs...)
		session.CurrentSetSize = len(set.TrackIDs)
	}

	return nil
}

func (sm *SessionManager) MarkPlayed(ctx context.Context, session *model.DJSession, trackID string) error {
	session.LastActive = time.Now()

	// Move track from queue to history
	newQueue := make([]string, 0, len(session.Queue))
	for _, id := range session.Queue {
		if id != trackID {
			newQueue = append(newQueue, id)
		}
	}
	session.Queue = newQueue
	session.History = append(session.History, trackID)

	// Track set progress for themed sessions
	if session.CurrentThemeID != "" {
		session.CurrentSetPlayed++
	}

	// Check for set completion in autonomous mode with queued sets
	if session.AutonomousMode && len(session.QueuedSets) > 0 {
		currentSet := &session.QueuedSets[0]

		// Update track count in current set
		tracksPlayedInSet := 0
		for _, tid := range currentSet.TrackIDs {
			found := false
			for _, qid := range session.Queue {
				if qid == tid {
					found = true
					break
				}
			}
			if !found {
				tracksPlayedInSet++
			}
		}

		// Check if current set is complete
		if tracksPlayedInSet >= len(currentSet.TrackIDs) {
			// Remove completed set
			session.QueuedSets = session.QueuedSets[1:]

			// Mark new current set if available
			if len(session.QueuedSets) > 0 {
				session.QueuedSets[0].IsCurrent = true
				session.CurrentThemeID = session.QueuedSets[0].ThemeID
				session.CurrentSetSize = len(session.QueuedSets[0].TrackIDs)
				session.CurrentSetPlayed = 0
				session.CurrentSetSkips = 0
				session.SetStartTime = time.Now()
				session.ChapterText = session.QueuedSets[0].Commentary
			}

			// Refill to maintain pre-loaded sets
			if err := sm.refillQueuedSets(ctx, session); err != nil {
				log.Error(ctx, "Error refilling queued sets", err)
			}

			// Rebuild flat queue from queued sets
			session.Queue = sm.flattenQueuedSets(session)
			return nil
		}
	} else if session.AutonomousMode && session.CurrentThemeID != "" {
		// Legacy path for sessions without queued sets
		if session.CurrentSetPlayed >= session.CurrentSetSize {
			// Set complete - select next theme
			if err := sm.selectNextSet(ctx, session); err != nil {
				// Fallback to continuing with current theme
				log.Error(ctx, "Error selecting next set", err)
				_ = sm.RefillThemedQueue(ctx, session)
			}
			return nil
		}
	}

	// Refill if needed (non-autonomous or mid-set)
	if len(session.Queue) < QueueRefillThreshold {
		if session.CurrentThemeID != "" {
			_ = sm.RefillThemedQueue(ctx, session)
		} else {
			tracks, err := sm.selector.SelectTracks(ctx, session, QueueSizeInitial-len(session.Queue))
			if err != nil {
				return err
			}
			session.Queue = append(session.Queue, tracks...)
		}
	}

	// Update commentary periodically for legacy sessions
	if session.CurrentThemeID == "" && len(session.History)%5 == 0 {
		session.ChapterText = GenerateCommentary(session, nil)
	}

	return nil
}

// selectNextSet picks a new theme and builds a new set after set completion
func (sm *SessionManager) selectNextSet(ctx context.Context, session *model.DJSession) error {
	// Select next theme
	theme := sm.themeScorer.SelectBestTheme(session)
	if theme == nil {
		// Library may be exhausted - try any available theme
		return sm.RefillThemedQueue(ctx, session)
	}

	// Exclude already played tracks
	exclude := session.History

	// Select tracks for new theme
	set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, exclude, session.UserPreferences)
	if err != nil {
		return err
	}

	if set == nil || len(set.TrackIDs) == 0 {
		// No tracks for this theme - library may be exhausted
		session.ChapterText = "That's all the music I have for now. Thanks for listening!"
		return nil
	}

	// Update session state
	session.CurrentThemeID = theme.ID
	session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	session.Queue = append(session.Queue, set.TrackIDs...)
	session.CurrentSetSize = len(set.TrackIDs)
	session.CurrentSetSkips = 0
	session.CurrentSetPlayed = 0
	session.SetStartTime = time.Now()

	// Generate commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	session.ChapterText = GenerateThemeCommentary(theme, commentaryCtx)

	return nil
}

func (sm *SessionManager) MarkSkipped(ctx context.Context, session *model.DJSession, trackID string, track *model.MediaFile) error {
	session.LastActive = time.Now()

	// Remove from queue
	newQueue := make([]string, 0, len(session.Queue))
	for _, id := range session.Queue {
		if id != trackID {
			newQueue = append(newQueue, id)
		}
	}
	session.Queue = newQueue
	session.History = append(session.History, trackID)

	// Record skip for downweighting
	if track != nil {
		session.SkippedArtists[track.ArtistID]++
		if track.Genre != "" {
			session.SkippedGenres[track.Genre]++
		}
		// Track mood skips for theme adaptation
		if mood := extractMoodFromTrack(track); mood != "" {
			if session.SkippedMoods == nil {
				session.SkippedMoods = make(map[string]int)
			}
			session.SkippedMoods[mood]++
		}
	}

	// Track theme skip
	if session.CurrentThemeID != "" {
		if session.SkippedThemes == nil {
			session.SkippedThemes = make(map[string]int)
		}
		session.SkippedThemes[session.CurrentThemeID]++
	}

	// Increment set-level skip counter
	session.CurrentSetSkips++

	// Check for pivot trigger (2 skips in set) in autonomous mode
	if session.AutonomousMode && session.CurrentSetSkips >= 2 {
		if err := sm.pivotToNewTheme(ctx, session); err != nil {
			// Log but don't fail - continue with current queue
			log.Error(ctx, "Error pivoting theme", err)
		}
	} else {
		// Normal refill if needed
		if len(session.Queue) < QueueRefillThreshold {
			if session.CurrentThemeID != "" {
				_ = sm.RefillThemedQueue(ctx, session)
			} else {
				tracks, _ := sm.selector.SelectTracks(ctx, session, 1)
				session.Queue = append(session.Queue, tracks...)
			}
		}
	}

	return nil
}

// pivotToNewTheme selects a new theme and rebuilds the queue after 2 skips
func (sm *SessionManager) pivotToNewTheme(ctx context.Context, session *model.DJSession) error {
	// Mark current theme as excluded (2 skips = permanent exclusion for session)
	if session.CurrentThemeID != "" {
		if session.ExcludedThemes == nil {
			session.ExcludedThemes = make(map[string]bool)
		}
		session.ExcludedThemes[session.CurrentThemeID] = true
	}

	// Select new theme (excluded themes will return score of 0)
	theme := sm.themeScorer.SelectBestTheme(session)
	if theme == nil {
		return fmt.Errorf("no alternative themes available")
	}

	// Exclude already played tracks only (we're replacing the queue)
	exclude := session.History

	// Select tracks for new theme
	set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, exclude, session.UserPreferences)
	if err != nil {
		return err
	}

	if set == nil || len(set.TrackIDs) == 0 {
		return fmt.Errorf("no tracks for theme: %s", theme.ID)
	}

	// Generate pivot commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsPivot = true

	// If using queued sets, rebuild all sets starting from the pivot theme
	if len(session.QueuedSets) > 0 || session.AutonomousMode {
		// Clear existing queued sets
		session.QueuedSets = make([]model.QueuedSet, 0, PreloadSetCount)

		// Add pivot set as first set
		pivotSet := model.QueuedSet{
			ThemeID:    theme.ID,
			ThemeName:  theme.Name,
			TrackIDs:   set.TrackIDs,
			Tracks:     set.Tracks,
			Commentary: GeneratePivotCommentary(theme, commentaryCtx),
			IsCurrent:  true,
		}
		session.QueuedSets = append(session.QueuedSets, pivotSet)

		// Update session state
		session.CurrentThemeID = theme.ID
		session.ThemesUsed = append(session.ThemesUsed, theme.ID)
		session.CurrentSetSize = len(set.TrackIDs)
		session.CurrentSetSkips = 0
		session.CurrentSetPlayed = 0
		session.SetStartTime = time.Now()
		session.ChapterText = pivotSet.Commentary

		// Refill to maintain pre-loaded sets
		if err := sm.refillQueuedSets(ctx, session); err != nil {
			log.Error(ctx, "Error refilling queued sets after pivot", err)
		}

		// Rebuild flat queue from queued sets
		session.Queue = sm.flattenQueuedSets(session)
	} else {
		// Legacy path for non-queued-set sessions
		session.CurrentThemeID = theme.ID
		session.ThemesUsed = append(session.ThemesUsed, theme.ID)
		session.Queue = set.TrackIDs
		session.CurrentSetSize = len(set.TrackIDs)
		session.CurrentSetSkips = 0
		session.CurrentSetPlayed = 0
		session.SetStartTime = time.Now()
		session.ChapterText = GeneratePivotCommentary(theme, commentaryCtx)
	}

	return nil
}

// extractMoodFromTrack gets the ai_mood tag from a track
func extractMoodFromTrack(track *model.MediaFile) string {
	if track.Tags == nil {
		return ""
	}
	if moods, ok := track.Tags["ai_mood"]; ok && len(moods) > 0 {
		return moods[0]
	}
	return ""
}

func (sm *SessionManager) EndSession(sessionID, userID string) error {
	session, err := sm.GetSession(sessionID, userID)
	if err != nil {
		return err
	}
	sm.sessions.Delete(session.ID)
	return nil
}

func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		sm.sessions.Range(func(key, value any) bool {
			session := value.(*model.DJSession)
			if time.Since(session.LastActive) > SessionExpiry {
				sm.sessions.Delete(key)
			}
			return true
		})
	}
}
