package aidj

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
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

	return r
}

type startRequest struct {
	Mode           string  `json:"mode"`
	SeedTrackID    *string `json:"seedTrackId"`
	SeedArtistID   *string `json:"seedArtistId"`
	SeedPlaylistID *string `json:"seedPlaylistId"`
}

func (api *Router) startSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)

	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	session, err := api.manager.CreateSession(ctx, user.ID, mode, seedTrackIDs, seedArtistIDs, req.SeedPlaylistID)
	if err != nil {
		log.Error(ctx, "Error creating DJ session", err)
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

func (api *Router) buildState(ctx context.Context, session *model.DJSession) (*model.DJState, error) {
	state := &model.DJState{
		SessionID:   session.ID,
		Mode:        session.Mode,
		ChapterText: session.ChapterText,
		UpNext:      make([]model.MediaFile, 0, len(session.Queue)),
	}

	// Load full track data for queue
	for _, trackID := range session.Queue {
		track, err := api.ds.MediaFile(ctx).Get(trackID)
		if err != nil {
			continue // Skip tracks that no longer exist
		}
		state.UpNext = append(state.UpNext, *track)
	}

	return state, nil
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
