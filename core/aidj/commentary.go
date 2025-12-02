package aidj

import "github.com/navidrome/navidrome/model"

var commentaryTemplates = map[model.DJMode][]string{
	model.DJModeDefault: {
		"Mixing your top tracks with songs you haven't heard in a while.",
		"Playing a blend of your favorites and some fresh picks.",
		"Here's a mix tailored to your listening history.",
	},
	model.DJModeNostalgia: {
		"Diving into older favorites you haven't played recently.",
		"Taking a trip down memory lane with some classics from your library.",
		"Bringing back tracks that defined your earlier playlists.",
	},
	model.DJModeDiscovery: {
		"Exploring tracks you rarely play, with a few familiar anchors.",
		"Surfacing hidden gems from your library.",
		"Time to discover songs you might have forgotten about.",
	},
}

var skipCommentary = []string{
	"Got it, steering away from that sound for a bit.",
	"Noted, adjusting the mix.",
	"Changing direction based on your feedback.",
}

// CommentaryContext provides additional context for commentary generation
type CommentaryContext struct {
	RecentTracks []model.MediaFile
	SkipCount    int
}

// GenerateCommentary creates a commentary string based on the session state
func GenerateCommentary(session *model.DJSession, _ *CommentaryContext) string {
	templates := commentaryTemplates[session.Mode]
	if len(templates) == 0 {
		return ""
	}
	// Simple rotation based on history length
	idx := len(session.History) % len(templates)
	return templates[idx]
}

// GenerateSkipCommentary returns a commentary string for skip events
func GenerateSkipCommentary() string {
	return skipCommentary[0] // Simplified; could randomize
}
