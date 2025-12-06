package aidj

import (
	"context"
	"fmt"
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
	lastFMRateLimit        = 5  // requests per second
	openAIRateLimit        = 10 // requests per second (adjust based on tier)
	enrichmentInitialDelay = 30 * time.Second

	// Concurrency settings for parallel enrichment
	openAIConcurrency = 30 // Number of concurrent OpenAI requests
	lastFMConcurrency = 5  // Number of concurrent Last.fm requests (matches their rate limit)
)

// lastFMClient interface for dependency injection in tests
type lastFMClient interface {
	GetTrackTags(ctx context.Context, artist, track string, mbid string) ([]lastfm.TrackTag, error)
}

// openAIEnricher interface for dependency injection in tests
type openAIEnricher interface {
	EnrichTrack(ctx context.Context, mf *model.MediaFile) (*OpenAIEnrichmentResult, error)
}

// EnrichmentService handles background enrichment of track metadata
type EnrichmentService struct {
	ds             model.DataStore
	lastFMClient   lastFMClient
	openAIEnricher openAIEnricher
	rateLimiter    *rate.Limiter
	openAILimiter  *rate.Limiter

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
	var openAI openAIEnricher

	hc := &http.Client{
		Timeout: consts.DefaultHttpClientTimeOut,
	}

	// Create Last.fm client if configured
	if conf.Server.LastFM.Enabled && conf.Server.LastFM.ApiKey != "" {
		cachedClient := cache.NewHTTPClient(hc, consts.DefaultHttpClientTimeOut)
		lfmClient = lastfm.NewClient(
			conf.Server.LastFM.ApiKey,
			conf.Server.LastFM.Secret,
			conf.Server.LastFM.Language,
			cachedClient,
		)
	}

	// Create OpenAI enricher if configured
	if conf.Server.AIDj.OpenAIEnabled && conf.Server.AIDj.OpenAIAPIKey != "" {
		openAI = NewOpenAIEnricher(
			conf.Server.AIDj.OpenAIAPIKey,
			conf.Server.AIDj.OpenAIModel,
			hc,
		)
	}

	return &EnrichmentService{
		ds:             ds,
		lastFMClient:   lfmClient,
		openAIEnricher: openAI,
		rateLimiter:    rate.NewLimiter(rate.Limit(lastFMRateLimit), 1),
		openAILimiter:  rate.NewLimiter(rate.Limit(openAIRateLimit), 1),
		shutdown:       make(chan struct{}),
		done:           make(chan struct{}),
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

	// Process in batches with parallel workers
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

		// Process batch in parallel
		s.processBatchParallel(ctx, batch)

		processed := s.enrichedTracks.Load()
		if processed%100 == 0 {
			pct := int(float64(processed) / float64(total) * 100)
			log.Info(ctx, "Enrichment progress", "processed", processed, "total", total, "percentage", pct)
		}
	}

	log.Info(ctx, "AI DJ metadata enrichment complete", "processed", s.enrichedTracks.Load())
	return nil
}

// enrichmentJob represents a track to be enriched
type enrichmentJob struct {
	track *model.MediaFile
	index int
}

// enrichmentResult holds the result of enriching a track
type enrichmentResult struct {
	track  *model.MediaFile
	moods  []string
	energy EnergyLevel
	vibes  []string
	source EnrichmentSource
	err    error
}

// processBatchParallel processes a batch of tracks using parallel workers
func (s *EnrichmentService) processBatchParallel(ctx context.Context, batch model.MediaFiles) {
	// Channel for jobs to process
	jobs := make(chan enrichmentJob, len(batch))

	// Channel for tracks that need Last.fm fallback
	lastFMFallback := make(chan enrichmentJob, len(batch))

	// Channel for final results to save
	results := make(chan enrichmentResult, len(batch))

	// WaitGroup for OpenAI workers
	var openAIWg sync.WaitGroup

	// WaitGroup for Last.fm workers
	var lastFMWg sync.WaitGroup

	// Start OpenAI workers
	for i := 0; i < openAIConcurrency; i++ {
		openAIWg.Add(1)
		go func() {
			defer openAIWg.Done()
			s.openAIWorker(ctx, jobs, lastFMFallback, results)
		}()
	}

	// Start Last.fm workers
	for i := 0; i < lastFMConcurrency; i++ {
		lastFMWg.Add(1)
		go func() {
			defer lastFMWg.Done()
			s.lastFMWorker(ctx, lastFMFallback, results)
		}()
	}

	// Send all tracks to the job queue
	go func() {
		for i := range batch {
			select {
			case <-ctx.Done():
				return
			case <-s.shutdown:
				return
			case jobs <- enrichmentJob{track: &batch[i], index: i}:
			}
		}
		close(jobs)
	}()

	// Wait for OpenAI workers to finish, then close Last.fm fallback channel
	go func() {
		openAIWg.Wait()
		close(lastFMFallback)
	}()

	// Wait for Last.fm workers to finish, then close results channel
	go func() {
		lastFMWg.Wait()
		close(results)
	}()

	// Collect and save results
	for result := range results {
		if result.err != nil {
			log.Warn(ctx, "Failed to enrich track", "track", result.track.Title, "artist", result.track.Artist, "error", result.err)
			continue
		}

		// Save enrichment data
		if err := s.saveEnrichment(ctx, result); err != nil {
			log.Warn(ctx, "Failed to save enrichment", "track", result.track.Title, "error", err)
			continue
		}

		s.enrichedTracks.Add(1)
	}
}

// openAIWorker processes tracks using OpenAI, sending failures to Last.fm fallback
func (s *EnrichmentService) openAIWorker(ctx context.Context, jobs <-chan enrichmentJob, fallback chan<- enrichmentJob, results chan<- enrichmentResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		default:
		}

		mf := job.track
		result := enrichmentResult{track: mf, source: EnrichmentSourceInferred}

		// Try OpenAI if available
		if s.openAIEnricher != nil && mf.Artist != "" && mf.Title != "" {
			if err := s.openAILimiter.Wait(ctx); err != nil {
				result.err = err
				results <- result
				continue
			}

			openAIResult, err := s.openAIEnricher.EnrichTrack(ctx, mf)
			if err == nil && openAIResult != nil {
				result.moods = []string{openAIResult.Mood}
				result.energy = parseEnergyLevel(openAIResult.Energy)
				result.vibes = openAIResult.Vibes
				result.source = EnrichmentSourceOpenAI
				log.Debug(ctx, "Enriched track via OpenAI", "track", mf.Title, "artist", mf.Artist, "mood", openAIResult.Mood)
				results <- result
				continue
			}
			log.Debug(ctx, "OpenAI enrichment failed, falling back", "track", mf.Title, "error", err)
		}

		// Send to Last.fm fallback queue
		select {
		case fallback <- job:
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		}
	}
}

// lastFMWorker processes tracks using Last.fm, falling back to local inference
func (s *EnrichmentService) lastFMWorker(ctx context.Context, jobs <-chan enrichmentJob, results chan<- enrichmentResult) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		default:
		}

		mf := job.track
		result := enrichmentResult{track: mf, source: EnrichmentSourceInferred}

		// Try Last.fm if available
		if s.lastFMClient != nil && mf.Artist != "" && mf.Title != "" {
			if err := s.rateLimiter.Wait(ctx); err != nil {
				result.err = err
				results <- result
				continue
			}

			tags, err := s.lastFMClient.GetTrackTags(ctx, mf.Artist, mf.Title, mf.MbzRecordingID)
			if err == nil && len(tags) > 0 {
				lfmMoods, lfmVibes := parseLastFMTags(tags)
				if len(lfmMoods) > 0 {
					result.moods = lfmMoods
					result.source = EnrichmentSourceLastFM
				}
				if len(lfmVibes) > 0 {
					result.vibes = lfmVibes
					if result.source != EnrichmentSourceLastFM {
						result.source = EnrichmentSourceLastFM
					}
				}
				log.Debug(ctx, "Enriched track via Last.fm", "track", mf.Title, "artist", mf.Artist, "moods", lfmMoods)
			}
		}

		// Fall back to local inference if needed
		if result.source == EnrichmentSourceInferred || len(result.moods) == 0 {
			inferredMoods, inferredEnergy, inferredVibes := InferEnrichment(mf)
			if len(result.moods) == 0 {
				result.moods = inferredMoods
			}
			if result.energy == "" {
				result.energy = inferredEnergy
			}
			if len(result.vibes) == 0 {
				result.vibes = inferredVibes
			}
			if result.source == EnrichmentSourceInferred {
				log.Debug(ctx, "Enriched track via local inference", "track", mf.Title, "moods", result.moods)
			}
		}

		// Ensure energy is set
		if result.energy == "" {
			result.energy = inferEnergyFromBPM(mf.BPM)
		}

		results <- result
	}
}

// saveEnrichment saves the enrichment result to the database
func (s *EnrichmentService) saveEnrichment(ctx context.Context, result enrichmentResult) error {
	mf := result.track
	now := time.Now().UTC().Format(time.RFC3339)

	if mf.Tags == nil {
		mf.Tags = make(model.Tags)
	}

	if len(result.moods) > 0 {
		mf.Tags[TagAIMood] = result.moods
	}
	mf.Tags[TagAIEnergy] = []string{string(result.energy)}
	if len(result.vibes) > 0 {
		mf.Tags[TagAIVibe] = result.vibes
	}
	mf.Tags[TagAISource] = []string{string(result.source)}
	mf.Tags[TagAIEnrichedAt] = []string{now}

	return s.ds.MediaFile(ctx).Put(mf)
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

// ResetEnrichment clears enrichment data from all tracks, allowing re-enrichment
// Returns the number of tracks that were reset
func (s *EnrichmentService) ResetEnrichment(ctx context.Context) (int64, error) {
	if s.inProgress.Load() {
		return 0, fmt.Errorf("enrichment is currently in progress, please wait for it to complete")
	}

	count, err := s.ds.MediaFile(ctx).ResetEnrichment(TagAIEnrichedAt)
	if err != nil {
		return 0, err
	}

	// Reset progress counters so the next Run() starts fresh
	s.totalTracks.Store(0)
	s.enrichedTracks.Store(0)

	log.Info(ctx, "Reset enrichment data", "tracksReset", count)
	return count, nil
}

func (s *EnrichmentService) countUnenrichedTracks(ctx context.Context) (int64, error) {
	return s.ds.MediaFile(ctx).CountUnenriched(TagAIEnrichedAt)
}

func (s *EnrichmentService) getUnenrichedBatch(ctx context.Context, limit int) (model.MediaFiles, error) {
	return s.ds.MediaFile(ctx).GetUnenriched(TagAIEnrichedAt, limit)
}

// parseEnergyLevel converts string to EnergyLevel
func parseEnergyLevel(s string) EnergyLevel {
	switch strings.ToLower(s) {
	case "low":
		return EnergyLow
	case "medium":
		return EnergyMedium
	case "high":
		return EnergyHigh
	default:
		return ""
	}
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
