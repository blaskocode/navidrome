package aidj

import (
	"context"
	"fmt"
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

	// Try all themes until we find one with enough tracks
	var set *ThemeSet
	var theme *Theme

	// Keep trying themes until we find one or exhaust all options
	for {
		// Select next best theme (excludes already-tried themes via session.ExcludedThemes)
		theme = sm.themeScorer.SelectBestTheme(session)
		if theme == nil {
			break // No more themes available
		}

		log.Debug(ctx, "Trying theme", "theme", theme.ID)

		// Try to select tracks for this theme
		var err error
		set, err = sm.themeSelector.SelectTracksForTheme(ctx, theme, nil, nil)
		if err != nil {
			log.Error(ctx, "Error selecting tracks for theme", "theme", theme.ID, err)
		}

		if set != nil && len(set.TrackIDs) > 0 {
			break // Found a theme with enough tracks
		}

		// Mark theme as excluded and try another
		if session.ExcludedThemes == nil {
			session.ExcludedThemes = make(map[string]bool)
		}
		session.ExcludedThemes[theme.ID] = true
		log.Debug(ctx, "Theme has insufficient tracks, trying another", "theme", theme.ID)
	}

	if set == nil || len(set.TrackIDs) == 0 {
		return nil, fmt.Errorf("no tracks available for any theme")
	}

	// Clear excluded themes since we found a working one
	session.ExcludedThemes = nil
	session.CurrentThemeID = theme.ID
	session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	session.Queue = set.TrackIDs
	session.CurrentSetSize = len(set.TrackIDs)

	// Generate commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsFirstSet = true
	session.ChapterText = GenerateThemeCommentary(theme, commentaryCtx)

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

	// Try all themes until we find one with enough tracks
	var set *ThemeSet
	var theme *Theme

	// Keep trying themes until we find one or exhaust all options
	for {
		// Select next best theme (excludes already-tried themes via session.ExcludedThemes)
		theme = sm.themeScorer.SelectBestTheme(session)
		if theme == nil {
			break // No more themes available
		}

		log.Debug(ctx, "Trying theme", "theme", theme.ID)

		// Try to select tracks for this theme with user preferences
		var err error
		set, err = sm.themeSelector.SelectTracksForTheme(ctx, theme, nil, preferences)
		if err != nil {
			log.Error(ctx, "Error selecting tracks for theme", "theme", theme.ID, err)
		}

		if set != nil && len(set.TrackIDs) > 0 {
			break // Found a theme with enough tracks
		}

		// Mark theme as excluded and try another
		if session.ExcludedThemes == nil {
			session.ExcludedThemes = make(map[string]bool)
		}
		session.ExcludedThemes[theme.ID] = true
		log.Debug(ctx, "Theme has insufficient tracks, trying another", "theme", theme.ID)
	}

	if set == nil || len(set.TrackIDs) == 0 {
		return nil, fmt.Errorf("no tracks available for any theme with given preferences")
	}

	log.Debug(ctx, "CreatePreferencedSession found tracks",
		"theme", theme.ID,
		"numTracks", len(set.TrackIDs),
		"trackIDs", set.TrackIDs,
	)

	// Clear excluded themes since we found a working one
	session.ExcludedThemes = nil
	session.CurrentThemeID = theme.ID
	session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	session.Queue = set.TrackIDs
	session.CurrentSetSize = len(set.TrackIDs)

	// Generate commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsFirstSet = true
	session.ChapterText = GenerateThemeCommentary(theme, commentaryCtx)

	log.Debug(ctx, "CreatePreferencedSession returning",
		"sessionID", session.ID,
		"queueLen", len(session.Queue),
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

	// Check for set completion in autonomous mode
	if session.AutonomousMode && session.CurrentThemeID != "" {
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

	// Exclude already played/queued tracks
	exclude := append(session.History, session.Queue...)

	// Select tracks for new theme
	set, err := sm.themeSelector.SelectTracksForTheme(ctx, theme, exclude, session.UserPreferences)
	if err != nil {
		return err
	}

	if set == nil || len(set.TrackIDs) == 0 {
		return fmt.Errorf("no tracks for theme: %s", theme.ID)
	}

	// Update session state
	session.CurrentThemeID = theme.ID
	session.ThemesUsed = append(session.ThemesUsed, theme.ID)
	session.Queue = set.TrackIDs
	session.CurrentSetSize = len(set.TrackIDs)
	session.CurrentSetSkips = 0
	session.CurrentSetPlayed = 0
	session.SetStartTime = time.Now()

	// Generate pivot commentary with track context
	commentaryCtx := NewCommentaryContext(set, session)
	commentaryCtx.IsPivot = true
	session.ChapterText = GeneratePivotCommentary(theme, commentaryCtx)

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
