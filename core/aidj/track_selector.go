package aidj

import (
	"context"
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/random"
)

// Bucket weights per mode
var modeWeights = map[model.DJMode]map[string]int{
	model.DJModeDefault: {
		"favorites":        50,
		"recent_favorites": 30,
		"discovery":        20,
	},
	model.DJModeNostalgia: {
		"nostalgia": 60,
		"favorites": 30,
		"discovery": 10,
	},
	model.DJModeDiscovery: {
		"discovery": 60,
		"favorites": 30,
		"random":    10,
	},
}

// Configuration constants
const (
	FavoriteRatingThreshold = 4
	RecentFavoritesDays     = 30
	NostalgiaYearsAgo       = 7
	NostalgiaLastPlayedDays = 180
	DiscoveryMaxPlayCount   = 1
	DiscoveryLastPlayedDays = 365
	MaxArtistInRow          = 3
	RecentHistoryWindow     = 30
	SkipThreshold           = 3
)

type TrackSelector struct {
	ds model.DataStore
}

func NewTrackSelector(ds model.DataStore) *TrackSelector {
	return &TrackSelector{ds: ds}
}

func (s *TrackSelector) SelectTracks(ctx context.Context, session *model.DJSession, count int) ([]string, error) {
	// Build candidate buckets
	buckets, err := s.buildBuckets(ctx, session)
	if err != nil {
		return nil, err
	}

	// Create weighted chooser for buckets
	bucketChooser := random.NewWeightedChooser[string]()
	weights := modeWeights[session.Mode]
	for name, weight := range weights {
		if bucket, ok := buckets[name]; ok && len(bucket) > 0 {
			bucketChooser.Add(name, weight)
		}
	}

	// If no buckets have tracks, fall back to random selection
	if bucketChooser.Size() == 0 {
		return s.selectRandom(ctx, count)
	}

	result := make([]string, 0, count)
	recentHistory := s.getRecentHistory(session)
	lastArtistID := ""
	consecutiveArtistCount := 0

	for i := 0; i < count && bucketChooser.Size() > 0; i++ {
		// Pick removes the entry after selection, but we re-add it below
		// Error only occurs when chooser is empty, which is prevented by the loop condition
		bucketName, _ := bucketChooser.Pick()
		// Re-add bucket for next iteration (Pick removes the entry)
		if len(buckets[bucketName]) > 0 {
			bucketChooser.Add(bucketName, weights[bucketName])
		}

		track := s.pickFromBucket(buckets[bucketName], recentHistory, lastArtistID, consecutiveArtistCount, session)
		if track == nil {
			continue
		}

		result = append(result, track.ID)
		recentHistory[track.ID] = true

		// Track consecutive artist count
		if track.ArtistID == lastArtistID {
			consecutiveArtistCount++
		} else {
			lastArtistID = track.ArtistID
			consecutiveArtistCount = 1
		}
	}

	return result, nil
}

func (s *TrackSelector) buildBuckets(ctx context.Context, session *model.DJSession) (map[string][]model.MediaFile, error) {
	buckets := make(map[string][]model.MediaFile)

	repo := s.ds.MediaFile(ctx)
	now := time.Now()
	currentYear := now.Year()

	// Fetch all non-missing tracks
	allTracks, err := repo.GetAll(model.QueryOptions{})
	if err != nil {
		return nil, err
	}

	for _, track := range allTracks {
		// Skip missing tracks
		if track.Missing {
			continue
		}

		// Apply skip downweighting - skip heavily-skipped artists
		if session.SkippedArtists[track.ArtistID] >= SkipThreshold {
			continue
		}

		// FAVORITES: rating >= 4 OR starred
		isFavorite := track.Rating >= FavoriteRatingThreshold || track.Starred

		// RECENT_FAVORITES: favorites played in last 30 days
		isRecentFavorite := isFavorite && track.PlayDate != nil &&
			time.Since(*track.PlayDate) < time.Duration(RecentFavoritesDays)*24*time.Hour

		// NOSTALGIA: favorites that are old or haven't been played recently
		isNostalgia := isFavorite && (track.Year <= currentYear-NostalgiaYearsAgo ||
			(track.PlayDate != nil && time.Since(*track.PlayDate) > time.Duration(NostalgiaLastPlayedDays)*24*time.Hour))

		// DISCOVERY: rarely played tracks
		isDiscovery := track.PlayCount <= int64(DiscoveryMaxPlayCount) ||
			track.PlayDate == nil ||
			time.Since(*track.PlayDate) > time.Duration(DiscoveryLastPlayedDays)*24*time.Hour

		if isFavorite {
			buckets["favorites"] = append(buckets["favorites"], track)
		}
		if isRecentFavorite {
			buckets["recent_favorites"] = append(buckets["recent_favorites"], track)
		}
		if isNostalgia {
			buckets["nostalgia"] = append(buckets["nostalgia"], track)
		}
		if isDiscovery {
			buckets["discovery"] = append(buckets["discovery"], track)
		}

		// SIMILAR_TO_SEED: match seed artists
		if len(session.SeedArtistIDs) > 0 {
			for _, artistID := range session.SeedArtistIDs {
				if track.ArtistID == artistID {
					buckets["similar_to_seed"] = append(buckets["similar_to_seed"], track)
					break
				}
			}
		}

		// RANDOM bucket for discovery mode fallback
		buckets["random"] = append(buckets["random"], track)
	}

	return buckets, nil
}

func (s *TrackSelector) pickFromBucket(bucket []model.MediaFile, recentHistory map[string]bool, lastArtistID string, consecutiveCount int, session *model.DJSession) *model.MediaFile {
	if len(bucket) == 0 {
		return nil
	}

	// Create weighted chooser for tracks in bucket
	chooser := random.NewWeightedChooser[*model.MediaFile]()
	for i := range bucket {
		track := &bucket[i]
		// Skip if in recent history
		if recentHistory[track.ID] {
			continue
		}
		// Skip if too many from same artist in a row
		if track.ArtistID == lastArtistID && consecutiveCount >= MaxArtistInRow {
			continue
		}
		// Apply skip penalty
		skipPenalty := session.SkippedArtists[track.ArtistID] * 10
		weight := 100 - skipPenalty
		if weight < 10 {
			weight = 10
		}
		chooser.Add(track, weight)
	}

	if chooser.Size() == 0 {
		return nil
	}

	track, _ := chooser.Pick()
	return track
}

func (s *TrackSelector) getRecentHistory(session *model.DJSession) map[string]bool {
	recent := make(map[string]bool)
	start := len(session.History) - RecentHistoryWindow
	if start < 0 {
		start = 0
	}
	for _, id := range session.History[start:] {
		recent[id] = true
	}
	for _, id := range session.Queue {
		recent[id] = true
	}
	return recent
}

func (s *TrackSelector) selectRandom(ctx context.Context, count int) ([]string, error) {
	repo := s.ds.MediaFile(ctx)
	allTracks, err := repo.GetAll(model.QueryOptions{
		Sort: "random",
		Max:  count,
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, len(allTracks))
	for i, t := range allTracks {
		result[i] = t.ID
	}
	return result, nil
}
