package aidj

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/agents/lastfm"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/cache"
	"golang.org/x/time/rate"
)

const (
	enrichmentBatchSize    = 100
	lastFMRateLimit        = 5 // requests per second
	enrichmentInitialDelay = 30 * time.Second
)

// lastFMClient interface for dependency injection in tests
type lastFMClient interface {
	GetTrackTags(ctx context.Context, artist, track string, mbid string) ([]lastfm.TrackTag, error)
}

// EnrichmentService handles background enrichment of track metadata
type EnrichmentService struct {
	ds           model.DataStore
	lastFMClient lastFMClient
	rateLimiter  *rate.Limiter

	// Progress tracking
	totalTracks    atomic.Int64
	enrichedTracks atomic.Int64
	inProgress     atomic.Bool

	// Shutdown coordination
	shutdown chan struct{}
	done     chan struct{}
}

// NewEnrichmentService creates a new enrichment service
func NewEnrichmentService(ds model.DataStore) *EnrichmentService {
	var lfmClient lastFMClient

	// Create Last.fm client if configured
	if conf.Server.LastFM.Enabled && conf.Server.LastFM.ApiKey != "" {
		hc := &http.Client{
			Timeout: consts.DefaultHttpClientTimeOut,
		}
		cachedClient := cache.NewHTTPClient(hc, consts.DefaultHttpClientTimeOut)
		lfmClient = lastfm.NewClient(
			conf.Server.LastFM.ApiKey,
			conf.Server.LastFM.Secret,
			conf.Server.LastFM.Language,
			cachedClient,
		)
	}

	return &EnrichmentService{
		ds:           ds,
		lastFMClient: lfmClient,
		rateLimiter:  rate.NewLimiter(rate.Limit(lastFMRateLimit), 1),
		shutdown:     make(chan struct{}),
		done:         make(chan struct{}),
	}
}

// Run starts the enrichment background job
func (s *EnrichmentService) Run(ctx context.Context) error {
	defer close(s.done)

	log.Info(ctx, "AI DJ enrichment service starting", "initialDelay", enrichmentInitialDelay)

	// Initial delay to let server start up
	select {
	case <-time.After(enrichmentInitialDelay):
	case <-ctx.Done():
		return nil
	case <-s.shutdown:
		return nil
	}

	if !conf.Server.AIDj.EnrichmentEnabled {
		log.Info(ctx, "AI DJ enrichment is disabled")
		return nil
	}

	log.Info(ctx, "Starting AI DJ metadata enrichment")
	s.inProgress.Store(true)
	defer s.inProgress.Store(false)

	// Count total tracks needing enrichment
	total, err := s.countUnenrichedTracks(ctx)
	if err != nil {
		log.Error(ctx, "Failed to count unenriched tracks", err)
		return err
	}
	s.totalTracks.Store(total)

	if total == 0 {
		log.Info(ctx, "No tracks need enrichment")
		return nil
	}

	log.Info(ctx, "Found tracks needing enrichment", "count", total)

	// Process in batches
	processed := int64(0)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.shutdown:
			return nil
		default:
		}

		batch, err := s.getUnenrichedBatch(ctx, enrichmentBatchSize)
		if err != nil {
			log.Error(ctx, "Failed to get unenriched batch", err)
			return err
		}

		if len(batch) == 0 {
			break // No more tracks to process
		}

		for i := range batch {
			select {
			case <-ctx.Done():
				return nil
			case <-s.shutdown:
				return nil
			default:
			}

			if err := s.enrichTrack(ctx, &batch[i]); err != nil {
				log.Warn(ctx, "Failed to enrich track", "track", batch[i].Title, "artist", batch[i].Artist, err)
				// Continue with next track
			}

			processed++
			s.enrichedTracks.Store(processed)

			if processed%100 == 0 {
				pct := int(float64(processed) / float64(total) * 100)
				log.Info(ctx, "Enrichment progress", "processed", processed, "total", total, "percentage", pct)
			}
		}
	}

	log.Info(ctx, "AI DJ metadata enrichment complete", "processed", processed)
	return nil
}

// Stop gracefully stops the enrichment service
func (s *EnrichmentService) Stop() {
	close(s.shutdown)
	<-s.done
}

// Status returns the current enrichment status
func (s *EnrichmentService) Status() EnrichmentStatus {
	total := s.totalTracks.Load()
	enriched := s.enrichedTracks.Load()

	pct := 0
	if total > 0 {
		pct = int(float64(enriched) / float64(total) * 100)
	}

	return EnrichmentStatus{
		Total:      total,
		Enriched:   enriched,
		Percentage: pct,
		InProgress: s.inProgress.Load(),
	}
}

func (s *EnrichmentService) countUnenrichedTracks(ctx context.Context) (int64, error) {
	return s.ds.MediaFile(ctx).CountUnenriched(TagAIEnrichedAt)
}

func (s *EnrichmentService) getUnenrichedBatch(ctx context.Context, limit int) (model.MediaFiles, error) {
	return s.ds.MediaFile(ctx).GetUnenriched(TagAIEnrichedAt, limit)
}

func (s *EnrichmentService) enrichTrack(ctx context.Context, mf *model.MediaFile) error {
	// Start with local inference
	moods, energy, vibes := InferEnrichment(mf)
	source := EnrichmentSourceInferred

	// Try Last.fm for additional tags if client is available
	if s.lastFMClient != nil && mf.Artist != "" && mf.Title != "" {
		// Wait for rate limiter
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return err
		}

		tags, err := s.lastFMClient.GetTrackTags(ctx, mf.Artist, mf.Title, mf.MbzRecordingID)
		if err == nil && len(tags) > 0 {
			// Merge Last.fm tags with inferred data
			lfmMoods, lfmVibes := parseLastFMTags(tags)
			if len(lfmMoods) > 0 {
				moods = mergeTags(moods, lfmMoods)
				source = EnrichmentSourceLastFM
			}
			if len(lfmVibes) > 0 {
				vibes = mergeTags(vibes, lfmVibes)
				source = EnrichmentSourceLastFM
			}
		}
	}

	// Store enrichment data in tags
	now := time.Now().UTC().Format(time.RFC3339)

	if mf.Tags == nil {
		mf.Tags = make(model.Tags)
	}

	if len(moods) > 0 {
		mf.Tags[TagAIMood] = moods
	}
	mf.Tags[TagAIEnergy] = []string{string(energy)}
	if len(vibes) > 0 {
		mf.Tags[TagAIVibe] = vibes
	}
	mf.Tags[TagAISource] = []string{string(source)}
	mf.Tags[TagAIEnrichedAt] = []string{now}

	// Save to database
	return s.ds.MediaFile(ctx).Put(mf)
}

// parseLastFMTags categorizes Last.fm tags into moods and vibes
func parseLastFMTags(tags []lastfm.TrackTag) (moods []string, vibes []string) {
	moodKeywords := map[string]bool{
		"chill": true, "chillout": true, "relaxing": true, "calm": true,
		"upbeat": true, "energetic": true, "happy": true,
		"melancholy": true, "sad": true, "dark": true,
		"intense": true, "aggressive": true, "powerful": true,
		"romantic": true, "love": true,
		"soft": true, "mellow": true,
	}

	vibeKeywords := map[string]bool{
		"coffeeshop": true, "cafe": true, "coffee": true,
		"workout": true, "gym": true, "running": true,
		"party": true, "club": true, "dance": true,
		"late-night": true, "night": true, "midnight": true,
		"focus": true, "study": true, "concentration": true,
		"road trip": true, "driving": true,
		"summer": true, "beach": true,
	}

	for _, tag := range tags {
		normalized := strings.ToLower(tag.Name)
		if moodKeywords[normalized] {
			moods = append(moods, normalized)
		}
		if vibeKeywords[normalized] {
			vibes = append(vibes, normalized)
		}
	}

	return moods, vibes
}

func mergeTags(existing, new []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(existing)+len(new))

	for _, t := range existing {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	for _, t := range new {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}

	return result
}

// Singleton pattern for status access from API
var (
	enrichmentInstance *EnrichmentService
	enrichmentOnce     sync.Once
	enrichmentMu       sync.Mutex
)

// GetEnrichmentService returns the singleton enrichment service
// If ds is nil, returns the existing instance (may be nil if not initialized)
func GetEnrichmentService(ds model.DataStore) *EnrichmentService {
	if ds != nil {
		enrichmentOnce.Do(func() {
			enrichmentInstance = NewEnrichmentService(ds)
		})
	}
	return enrichmentInstance
}

// SetEnrichmentService allows setting the singleton instance (for testing)
func SetEnrichmentService(es *EnrichmentService) {
	enrichmentMu.Lock()
	defer enrichmentMu.Unlock()
	enrichmentInstance = es
}
