package aidj

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/model"
)

const (
	SessionExpiry        = 60 * time.Minute
	CleanupInterval      = 5 * time.Minute
	QueueSizeInitial     = 30
	QueueRefillThreshold = 10
)

type SessionManager struct {
	sessions sync.Map // map[sessionID]*model.DJSession
	selector *TrackSelector
}

func NewSessionManager(ds model.DataStore) *SessionManager {
	sm := &SessionManager{
		selector: NewTrackSelector(ds),
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

	// Refill if needed
	if len(session.Queue) < QueueRefillThreshold {
		tracks, err := sm.selector.SelectTracks(ctx, session, QueueSizeInitial-len(session.Queue))
		if err != nil {
			return err
		}
		session.Queue = append(session.Queue, tracks...)
	}

	// Update commentary periodically (every 5 tracks)
	if len(session.History)%5 == 0 {
		session.ChapterText = GenerateCommentary(session, nil)
	}

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
	}

	// Refill if needed
	if len(session.Queue) < QueueRefillThreshold {
		tracks, err := sm.selector.SelectTracks(ctx, session, 1)
		if err != nil {
			return err
		}
		session.Queue = append(session.Queue, tracks...)
	}

	return nil
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
