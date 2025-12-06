package aidj

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/agents/lastfm"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/cache"
)

// EnrichmentDebugResult holds debug info from all enrichment sources
type EnrichmentDebugResult struct {
	Track        string
	Artist       string
	Album        string
	Genre        string
	BPM          int
	LocalResult  *LocalEnrichmentDebug
	LastFMResult *LastFMEnrichmentDebug
	OpenAIResult *OpenAIEnrichmentDebug
}

type LocalEnrichmentDebug struct {
	Moods  []string
	Energy string
	Vibes  []string
}

type LastFMEnrichmentDebug struct {
	RawTags []string
	Moods   []string
	Vibes   []string
	Error   string
}

type OpenAIEnrichmentDebug struct {
	Mood   string
	Energy string
	Vibes  []string
	Error  string
}

// DebugEnrichTrack shows what each enrichment source would return for a track
func DebugEnrichTrack(ctx context.Context, mf *model.MediaFile) (*EnrichmentDebugResult, error) {
	result := &EnrichmentDebugResult{
		Track:  mf.Title,
		Artist: mf.Artist,
		Album:  mf.Album,
		Genre:  mf.Genre,
		BPM:    mf.BPM,
	}

	// 1. Local Inference
	moods, energy, vibes := InferEnrichment(mf)
	result.LocalResult = &LocalEnrichmentDebug{
		Moods:  moods,
		Energy: string(energy),
		Vibes:  vibes,
	}

	// 2. Last.fm
	result.LastFMResult = debugLastFM(ctx, mf)

	// 3. OpenAI
	result.OpenAIResult = debugOpenAI(ctx, mf)

	return result, nil
}

func debugLastFM(ctx context.Context, mf *model.MediaFile) *LastFMEnrichmentDebug {
	debug := &LastFMEnrichmentDebug{}

	if !conf.Server.LastFM.Enabled || conf.Server.LastFM.ApiKey == "" {
		debug.Error = "Last.fm not configured"
		return debug
	}

	if mf.Artist == "" || mf.Title == "" {
		debug.Error = "Missing artist or title"
		return debug
	}

	hc := &http.Client{
		Timeout: consts.DefaultHttpClientTimeOut,
	}
	cachedClient := cache.NewHTTPClient(hc, consts.DefaultHttpClientTimeOut)
	client := lastfm.NewClient(
		conf.Server.LastFM.ApiKey,
		conf.Server.LastFM.Secret,
		conf.Server.LastFM.Language,
		cachedClient,
	)

	tags, err := client.GetTrackTags(ctx, mf.Artist, mf.Title, mf.MbzRecordingID)
	if err != nil {
		debug.Error = err.Error()
		return debug
	}

	// Collect raw tag names
	for _, tag := range tags {
		debug.RawTags = append(debug.RawTags, fmt.Sprintf("%s (%d)", tag.Name, tag.Count))
	}

	// Parse into moods/vibes
	debug.Moods, debug.Vibes = parseLastFMTags(tags)

	return debug
}

func debugOpenAI(ctx context.Context, mf *model.MediaFile) *OpenAIEnrichmentDebug {
	debug := &OpenAIEnrichmentDebug{}

	if !conf.Server.AIDj.OpenAIEnabled || conf.Server.AIDj.OpenAIAPIKey == "" {
		debug.Error = "OpenAI not configured"
		return debug
	}

	if mf.Artist == "" || mf.Title == "" {
		debug.Error = "Missing artist or title"
		return debug
	}

	hc := &http.Client{
		Timeout: 30 * time.Second,
	}
	enricher := NewOpenAIEnricher(
		conf.Server.AIDj.OpenAIAPIKey,
		conf.Server.AIDj.OpenAIModel,
		hc,
	)

	result, err := enricher.EnrichTrack(ctx, mf)
	if err != nil {
		debug.Error = err.Error()
		return debug
	}

	debug.Mood = result.Mood
	debug.Energy = result.Energy
	debug.Vibes = result.Vibes

	return debug
}

// PrintDebugResult formats the debug result for display
func PrintDebugResult(r *EnrichmentDebugResult) string {
	var s string
	s += fmt.Sprintf("=== Track: %s by %s ===\n", r.Track, r.Artist)
	s += fmt.Sprintf("Album: %s | Genre: %s | BPM: %d\n\n", r.Album, r.Genre, r.BPM)

	s += "--- Local Inference ---\n"
	s += fmt.Sprintf("  Moods:  %v\n", r.LocalResult.Moods)
	s += fmt.Sprintf("  Energy: %s\n", r.LocalResult.Energy)
	s += fmt.Sprintf("  Vibes:  %v\n\n", r.LocalResult.Vibes)

	s += "--- Last.fm ---\n"
	if r.LastFMResult.Error != "" {
		s += fmt.Sprintf("  Error: %s\n\n", r.LastFMResult.Error)
	} else {
		s += fmt.Sprintf("  Raw Tags: %v\n", r.LastFMResult.RawTags)
		s += fmt.Sprintf("  Moods:    %v\n", r.LastFMResult.Moods)
		s += fmt.Sprintf("  Vibes:    %v\n\n", r.LastFMResult.Vibes)
	}

	s += "--- OpenAI ---\n"
	if r.OpenAIResult.Error != "" {
		s += fmt.Sprintf("  Error: %s\n", r.OpenAIResult.Error)
	} else {
		s += fmt.Sprintf("  Mood:   %s\n", r.OpenAIResult.Mood)
		s += fmt.Sprintf("  Energy: %s\n", r.OpenAIResult.Energy)
		s += fmt.Sprintf("  Vibes:  %v\n", r.OpenAIResult.Vibes)
	}

	return s
}
