package aidj

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/aidj"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server"
)

type Router struct {
	http.Handler
	ds      model.DataStore
	manager *aidj.SessionManager
}

func New(ds model.DataStore, manager *aidj.SessionManager) *Router {
	r := &Router{ds: ds, manager: manager}
	r.Handler = r.routes()
	return r
}

func (api *Router) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(server.Authenticator(api.ds))
	r.Use(server.JWTRefresher)
	r.Use(server.UpdateLastAccessMiddleware(api.ds))

	r.Post("/start", api.startSession)
	r.Get("/state", api.getState)
	r.Post("/next", api.next)
	r.Post("/skip", api.skip)
	r.Post("/end", api.endSession)
	r.Get("/enrichment/status", api.getEnrichmentStatus)
	r.Post("/enrichment/reset", api.resetEnrichment)

	// Theme endpoints
	r.Get("/themes", api.listThemes)
	r.Post("/start-themed", api.startThemedSession)

	// TTS endpoint
	r.Get("/commentary-audio", api.getCommentaryAudio)

	return r
}

type startRequest struct {
	Mode           string                 `json:"mode,omitempty"`
	SeedTrackID    *string                `json:"seedTrackId,omitempty"`
	SeedArtistID   *string                `json:"seedArtistId,omitempty"`
	SeedPlaylistID *string                `json:"seedPlaylistId,omitempty"`
	ThemeID        *string                `json:"themeId,omitempty"`
	Autonomous     bool                   `json:"autonomous,omitempty"`  // explicit autonomous flag
	Preferences    *model.UserPreferences `json:"preferences,omitempty"` // user preference filters
}

func (api *Router) startSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var session *model.DJSession
	var err error

	// Use autonomous mode if explicitly requested OR if no mode/seeds specified
	isAutonomous := req.Autonomous || (req.Mode == "" && req.SeedTrackID == nil && req.SeedArtistID == nil && req.SeedPlaylistID == nil && req.ThemeID == nil)

	if isAutonomous && req.Preferences != nil {
		// Autonomous mode with user preferences
		session, err = api.manager.CreatePreferencedSession(ctx, user.ID, req.Preferences)
	} else if isAutonomous {
		session, err = api.manager.CreateAutonomousSession(ctx, user.ID)
	} else if req.ThemeID != nil {
		// Theme-based session
		session, err = api.manager.CreateThemedSession(ctx, user.ID, *req.ThemeID)
	} else {
		// Legacy mode-based session
		mode := model.DJMode(req.Mode)
		if mode == "" {
			mode = model.DJModeDefault
		}
		if !mode.IsValid() {
			http.Error(w, "invalid mode", http.StatusBadRequest)
			return
		}

		var seedTrackIDs, seedArtistIDs []string
		if req.SeedTrackID != nil {
			seedTrackIDs = []string{*req.SeedTrackID}
		}
		if req.SeedArtistID != nil {
			seedArtistIDs = []string{*req.SeedArtistID}
		}

		session, err = api.manager.CreateSession(ctx, user.ID, mode, seedTrackIDs, seedArtistIDs, req.SeedPlaylistID)
	}

	if err != nil {
		log.Error(ctx, "Error creating DJ session", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Debug(ctx, "Session created successfully",
		"sessionID", session.ID,
		"queueLen", len(session.Queue),
		"hasPreferences", req.Preferences != nil,
	)

	state, err := api.buildState(ctx, session)
	if err != nil {
		log.Error(ctx, "Error building DJ state", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Debug(ctx, "Responding with state",
		"sessionID", state.SessionID,
		"upNextLen", len(state.UpNext),
	)

	respondJSON(w, http.StatusOK, state)
}

func (api *Router) getState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)
	sessionID := r.URL.Query().Get("sessionId")

	session, err := api.manager.GetSession(sessionID, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "session not found or expired", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := api.buildState(ctx, session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, state)
}

type nextRequest struct {
	SessionID     string `json:"sessionId"`
	PlayedTrackID string `json:"playedTrackId"`
}

func (api *Router) next(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req nextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := api.manager.GetSession(req.SessionID, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "session not found or expired", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := api.manager.MarkPlayed(ctx, session, req.PlayedTrackID); err != nil {
		log.Error(ctx, "Error marking track played", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := api.buildState(ctx, session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, state)
}

type skipRequest struct {
	SessionID      string `json:"sessionId"`
	SkippedTrackID string `json:"skippedTrackId"`
}

func (api *Router) skip(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req skipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session, err := api.manager.GetSession(req.SessionID, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "session not found or expired", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get track info for downweighting
	var track *model.MediaFile
	if t, err := api.ds.MediaFile(ctx).Get(req.SkippedTrackID); err == nil {
		track = t
	}

	if err := api.manager.MarkSkipped(ctx, session, req.SkippedTrackID, track); err != nil {
		log.Error(ctx, "Error marking track skipped", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update commentary for skip
	session.ChapterText = aidj.GenerateSkipCommentary()

	state, err := api.buildState(ctx, session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, state)
}

type endRequest struct {
	SessionID string `json:"sessionId"`
}

func (api *Router) endSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req endRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := api.manager.EndSession(req.SessionID, user.ID); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		log.Error(ctx, "Error ending DJ session", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *Router) getEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	es := aidj.GetEnrichmentService(api.ds)
	if es == nil {
		// Service not initialized yet
		respondJSON(w, http.StatusOK, aidj.EnrichmentStatus{
			Total:      0,
			Enriched:   0,
			Percentage: 0,
			InProgress: false,
		})
		return
	}
	status := es.Status()
	respondJSON(w, http.StatusOK, status)
}

func (api *Router) resetEnrichment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	es := aidj.GetEnrichmentService(api.ds)
	if es == nil {
		http.Error(w, "enrichment service not initialized", http.StatusServiceUnavailable)
		return
	}

	count, err := es.ResetEnrichment(ctx)
	if err != nil {
		log.Error(ctx, "Failed to reset enrichment", err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Enrichment data reset successfully. Re-enrichment will begin automatically.",
		"tracksReset": count,
	})
}

func (api *Router) buildState(ctx context.Context, session *model.DJSession) (*model.DJState, error) {
	log.Debug(ctx, "buildState called", "sessionID", session.ID, "queueLen", len(session.Queue), "numSets", len(session.QueuedSets))

	state := &model.DJState{
		SessionID:    session.ID,
		Mode:         session.Mode,
		ChapterText:  session.ChapterText,
		UpNext:       make([]model.MediaFile, 0, len(session.Queue)),
		IsAutonomous: session.AutonomousMode,
	}

	// Add theme info if present
	if session.CurrentThemeID != "" {
		state.ThemeID = session.CurrentThemeID
		registry := aidj.GetThemeRegistry()
		if theme := registry.GetTheme(session.CurrentThemeID); theme != nil {
			state.ThemeName = theme.Name
		}
		state.SetProgress = session.CurrentSetPlayed
		state.SetSize = session.CurrentSetSize
	}

	// Check for exhaustion
	if len(session.Queue) == 0 {
		state.IsExhausted = true
		if session.ChapterText == "" {
			session.ChapterText = "That's all the music I have for now. Thanks for listening!"
			state.ChapterText = session.ChapterText
		}
	}

	// Include queued sets with full metadata
	if len(session.QueuedSets) > 0 {
		state.Sets = make([]model.QueuedSet, len(session.QueuedSets))
		for i, qs := range session.QueuedSets {
			// Copy set info and load fresh track data
			state.Sets[i] = model.QueuedSet{
				ThemeID:    qs.ThemeID,
				ThemeName:  qs.ThemeName,
				TrackIDs:   qs.TrackIDs,
				Tracks:     make([]model.MediaFile, 0, len(qs.TrackIDs)),
				Commentary: qs.Commentary,
				IsCurrent:  qs.IsCurrent,
			}

			// Load track data for each set
			for _, trackID := range qs.TrackIDs {
				track, err := api.ds.MediaFile(ctx).Get(trackID)
				if err != nil {
					log.Warn(ctx, "Could not load track for set", "trackID", trackID, err)
					continue
				}
				state.Sets[i].Tracks = append(state.Sets[i].Tracks, *track)
			}
		}

		// Build UpNext from sets if we have them
		for _, set := range state.Sets {
			state.UpNext = append(state.UpNext, set.Tracks...)
		}
	} else {
		// Load full track data for queue (legacy path)
		for _, trackID := range session.Queue {
			track, err := api.ds.MediaFile(ctx).Get(trackID)
			if err != nil {
				log.Warn(ctx, "Could not load track", "trackID", trackID, err)
				continue // Skip tracks that no longer exist
			}
			state.UpNext = append(state.UpNext, *track)
		}
	}

	log.Debug(ctx, "buildState complete", "upNextLen", len(state.UpNext), "numSets", len(state.Sets))
	return state, nil
}

// listThemes returns all available themes
func (api *Router) listThemes(w http.ResponseWriter, r *http.Request) {
	registry := aidj.GetThemeRegistry()
	themes := registry.AllThemes()

	// Build response
	type themeResponse struct {
		ID          string `json:"id"`
		Category    string `json:"category"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	response := make([]themeResponse, len(themes))
	for i, t := range themes {
		response[i] = themeResponse{
			ID:          t.ID,
			Category:    string(t.Category),
			Name:        t.Name,
			Description: t.Description,
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// startThemedSession starts a new DJ session with a specific theme
func (api *Router) startThemedSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req struct {
		ThemeID string `json:"themeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ThemeID == "" {
		http.Error(w, "themeId is required", http.StatusBadRequest)
		return
	}

	session, err := api.manager.CreateThemedSession(ctx, user.ID, req.ThemeID)
	if err != nil {
		log.Error(ctx, "Error creating themed DJ session", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := api.buildState(ctx, session)
	if err != nil {
		log.Error(ctx, "Error building DJ state", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, state)
}

// getCommentaryAudio generates TTS audio for the provided commentary text
func (api *Router) getCommentaryAudio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if OpenAI is enabled
	if !conf.Server.AIDj.OpenAIEnabled || conf.Server.AIDj.OpenAIAPIKey == "" {
		http.Error(w, "TTS not available", http.StatusServiceUnavailable)
		return
	}

	// Get the commentary text from query parameter
	commentary := r.URL.Query().Get("text")
	if commentary == "" {
		http.Error(w, "text parameter required", http.StatusBadRequest)
		return
	}

	// Generate TTS audio
	ttsClient := aidj.NewTTSClient(conf.Server.AIDj.OpenAIAPIKey, http.DefaultClient)
	audio, err := ttsClient.GenerateSpeech(ctx, commentary)
	if err != nil {
		log.Error(ctx, "Failed to generate TTS audio", "error", err)
		http.Error(w, "TTS generation failed", http.StatusInternalServerError)
		return
	}

	// Return MP3 audio
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audio)))
	if _, err := w.Write(audio); err != nil {
		log.Error(ctx, "Failed to write TTS audio response", "error", err)
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
