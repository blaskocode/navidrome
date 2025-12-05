package aidj

import (
	"time"
)

// EnrichmentSource identifies how a track was enriched
type EnrichmentSource string

const (
	EnrichmentSourceInferred EnrichmentSource = "inferred"
	EnrichmentSourceLastFM   EnrichmentSource = "lastfm"
)

// EnrichmentData represents AI-generated metadata for a track
type EnrichmentData struct {
	Moods      []string         `json:"ai_mood,omitempty"`
	Energy     string           `json:"ai_energy,omitempty"`
	Vibes      []string         `json:"ai_vibe,omitempty"`
	Source     EnrichmentSource `json:"ai_enrichment_source"`
	EnrichedAt time.Time        `json:"ai_enriched_at"`
}

// EnrichmentStatus represents the current state of the enrichment job
type EnrichmentStatus struct {
	Total      int64 `json:"total"`
	Enriched   int64 `json:"enriched"`
	Percentage int   `json:"percentage"`
	InProgress bool  `json:"inProgress"`
}

// AI tag names for storage in MediaFile.Tags
const (
	TagAIMood       = "ai_mood"
	TagAIEnergy     = "ai_energy"
	TagAIVibe       = "ai_vibe"
	TagAISource     = "ai_enrichment_source"
	TagAIEnrichedAt = "ai_enriched_at"
)
