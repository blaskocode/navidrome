package aidj

import (
	"math/rand"
	"strings"
	"sync"

	"github.com/navidrome/navidrome/model"
)

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

// CommentaryContext provides track-aware context for commentary generation
type CommentaryContext struct {
	Theme  *Theme           // Current theme
	Tracks model.MediaFiles // Selected tracks for the set

	// Extracted common values (populated by Analyze)
	PrimaryArtist string // Most common or first artist
	FirstArtist   string // Artist of the first track (for "starting with" templates)
	PrimaryGenre  string // Most common genre
	PrimaryDecade string // Decade most tracks belong to (e.g., "90s")
	PrimaryMood   string // Most common mood tag
	PrimaryAlbum  string // Album name if most tracks share one
	TrackCount    int    // Number of tracks in set

	// Session context
	IsFirstSet bool // True if this is the first set of the session
	IsPivot    bool // True if this is a pivot due to skips
	SetNumber  int  // Which set this is in the session

	// User preferences (for preference-aware commentary)
	UserPreferences *model.UserPreferences
}

// NewCommentaryContext creates and analyzes a context from a theme set
func NewCommentaryContext(set *ThemeSet, session *model.DJSession) *CommentaryContext {
	ctx := &CommentaryContext{
		TrackCount: 0,
	}

	if set != nil {
		ctx.Theme = set.Theme
		ctx.Tracks = set.Tracks
		ctx.TrackCount = len(set.Tracks)
	}

	// Determine session context
	if session != nil {
		ctx.SetNumber = len(session.ThemesUsed)
		ctx.IsFirstSet = ctx.SetNumber <= 1
		ctx.UserPreferences = session.UserPreferences
	}

	// Analyze tracks to extract common values
	ctx.analyze()

	return ctx
}

// analyze extracts common metadata from the tracks
func (c *CommentaryContext) analyze() {
	if len(c.Tracks) == 0 {
		return
	}

	// Count occurrences
	artistCounts := make(map[string]int)
	genreCounts := make(map[string]int)
	decadeCounts := make(map[string]int)
	moodCounts := make(map[string]int)
	albumCounts := make(map[string]int)

	for _, track := range c.Tracks {
		// Artist
		if track.Artist != "" {
			artistCounts[track.Artist]++
		}

		// Genre
		if track.Genre != "" {
			genreCounts[track.Genre]++
		}

		// Decade (from year)
		if track.Year > 0 {
			decade := c.yearToDecade(track.Year)
			decadeCounts[decade]++
		}

		// Mood (from ai_mood tag)
		if track.Tags != nil {
			if moods, ok := track.Tags["ai_mood"]; ok && len(moods) > 0 {
				moodCounts[moods[0]]++
			}
		}

		// Album
		if track.Album != "" {
			albumCounts[track.Album]++
		}
	}

	// Find most common values
	c.PrimaryArtist = c.findMostCommon(artistCounts)
	c.PrimaryGenre = c.findMostCommon(genreCounts)
	c.PrimaryDecade = c.findMostCommon(decadeCounts)
	c.PrimaryMood = c.findMostCommon(moodCounts)

	// Only set album if majority of tracks share it
	if album := c.findMostCommon(albumCounts); album != "" {
		if albumCounts[album] > len(c.Tracks)/2 {
			c.PrimaryAlbum = album
		}
	}

	// Always set FirstArtist from the first track (for "starting with" templates)
	if len(c.Tracks) > 0 {
		c.FirstArtist = c.Tracks[0].Artist
	}

	// Fallback: use first track's values if no common value found
	if c.PrimaryArtist == "" && len(c.Tracks) > 0 {
		c.PrimaryArtist = c.Tracks[0].Artist
	}
	if c.PrimaryGenre == "" && len(c.Tracks) > 0 {
		c.PrimaryGenre = c.Tracks[0].Genre
	}
	if c.PrimaryDecade == "" && len(c.Tracks) > 0 && c.Tracks[0].Year > 0 {
		c.PrimaryDecade = c.yearToDecade(c.Tracks[0].Year)
	}
}

// yearToDecade converts a year to decade string (e.g., 1995 -> "90s")
func (c *CommentaryContext) yearToDecade(year int) string {
	decade := (year / 10) * 10
	switch {
	case decade >= 2020:
		return "2020s"
	case decade >= 2010:
		return "2010s"
	case decade >= 2000:
		return "2000s"
	case decade >= 1990:
		return "90s"
	case decade >= 1980:
		return "80s"
	case decade >= 1970:
		return "70s"
	case decade >= 1960:
		return "60s"
	default:
		return "classic era"
	}
}

// findMostCommon returns the most common value from a count map
func (c *CommentaryContext) findMostCommon(counts map[string]int) string {
	var maxKey string
	var maxCount int

	for key, count := range counts {
		if count > maxCount {
			maxCount = count
			maxKey = key
		}
	}

	return maxKey
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

// Template variables that can be substituted
const (
	VarArtist      = "{artist}"
	VarGenre       = "{genre}"
	VarDecade      = "{decade}"
	VarMood        = "{mood}"
	VarAlbum       = "{album}"
	VarDescription = "{description}"
	VarThemeName   = "{themeName}"
)

// themeTemplates maps theme categories to template variations with variable placeholders
var themeTemplates = map[ThemeCategory][]string{
	ThemeCategoryEra: {
		"Let's take it back to the {decade} with some {description}.",
		"Here's some {decade} music, starting with {artist}.",
		"Time for a {decade} throwback featuring {artist}.",
		"Taking you back to the {decade} - enjoy these classics.",
		"The {decade} are calling - here's {artist} and more.",
	},
	ThemeCategoryMood: {
		"Time for some {description}, with {artist} setting the tone.",
		"Let's set the mood with {description}.",
		"Here's some {mood} music to match the vibe.",
		"Shifting to {description} - {artist} starts us off.",
		"Getting into {mood} territory with tracks like {artist}.",
	},
	ThemeCategoryFamiliarity: {
		"Here are some {description}, including {artist}.",
		"I found some {description} for you.",
		"Let's revisit {description} - starting with {artist}.",
		"Time to explore {description}.",
		"Digging into {description}, featuring {artist}.",
	},
	ThemeCategoryRating: {
		"Here's some of {description}, featuring {artist}.",
		"Playing {description} from your collection.",
		"Your {description} coming up, including {artist}.",
		"Time for {description}.",
		"Let's enjoy some {description}, starting with {artist}.",
	},
	ThemeCategoryEnergy: {
		"Let's {mood} with some {description}.",
		"Here's some {description} to change the pace.",
		"Shifting energy with {description}, featuring {artist}.",
		"Time for {description}.",
		"{description} coming up, with {artist} leading the way.",
	},
	ThemeCategoryTemporal: {
		"Here's some {description} from your library.",
		"Checking out {description}, including {artist}.",
		"Let's explore {description}.",
		"{description} up next, featuring {artist}.",
		"Time for {description} - {artist} and more.",
	},
	ThemeCategoryGenre: {
		"Time for some {genre} - {artist} kicks things off.",
		"Let's explore {genre} from your library.",
		"Here's a {genre} set, starting with {artist}.",
		"Diving into {genre} with {artist} and more.",
		"{genre} time - enjoy these tracks.",
	},
	ThemeCategoryArtist: {
		"Here's more from {artist}.",
		"Let's dive into {artist}'s catalog.",
		"Enjoying {artist}? Here's more.",
		"More {artist} coming your way.",
		"Continuing with {artist} and similar sounds.",
	},
}

// pivotTemplates for when switching themes due to skips
var pivotTemplates = []string{
	"Switching things up - how about some {description} with {artist}?",
	"Let's try something different - {description}.",
	"I can tell you're not feeling that vibe. How about {artist} and some {description}?",
	"Changing direction with {description}, featuring {artist}.",
	"New direction - here's some {description}.",
	"Pivoting to {description} - {artist} leads the way.",
}

// firstSetTemplates for the opening set of a session (generic, no preferences)
var firstSetTemplates = []string{
	"Welcome! Let's start with some {description}, featuring {artist}.",
	"Here we go - kicking off with {description}.",
	"Starting your session with {description} and {artist}.",
	"Let's begin with some {description}.",
	"Welcome back! Here's some {description} to get started.",
}

// preferenceFirstSetTemplates for sessions with user preferences
var preferenceFirstSetTemplates = map[string][]string{
	// Energy-based
	"upbeat": {
		"You asked for upbeat - here's some energetic music featuring {artist}.",
		"Let's get moving with some upbeat tracks, starting with {artist}.",
		"Upbeat vibes coming your way, featuring {artist}.",
		"Here's that upbeat energy you wanted, with {artist} and more.",
	},
	"soft": {
		"You wanted soft music - here's something mellow featuring {artist}.",
		"Let's keep it soft and gentle, starting with {artist}.",
		"Soft vibes as requested, featuring {artist}.",
		"Here's that mellow sound you asked for, with {artist}.",
	},
	"intense": {
		"You asked for intense - here's some powerful music featuring {artist}.",
		"Let's turn up the intensity with {artist}.",
		"Intense vibes coming your way, featuring {artist}.",
		"Here's the intensity you wanted, starting with {artist}.",
	},
	// Context-based
	"favorites": {
		"Playing your favorites, starting with {artist}.",
		"Here are some of your top-rated tracks, featuring {artist}.",
		"Your favorites as requested - {artist} and more.",
		"Let's enjoy your favorites, starting with {artist}.",
	},
	"forgotten": {
		"Here are some tracks you haven't heard in a while, featuring {artist}.",
		"Dusting off some forgotten gems, starting with {artist}.",
		"Remember these? Here's {artist} and other tracks from the archives.",
		"Rediscovering some music you haven't played recently, with {artist}.",
	},
	"new": {
		"Here's some music you haven't explored yet, featuring {artist}.",
		"Time to discover something new - starting with {artist}.",
		"Fresh picks from your library, featuring {artist}.",
		"Let's explore tracks you haven't heard before, with {artist}.",
	},
}

// recentTemplates tracks recently used templates to avoid repetition
var recentTemplates = struct {
	sync.RWMutex
	used map[string][]int
}{
	used: make(map[string][]int),
}

// selectTemplate picks a template, avoiding recently used ones
func selectTemplate(templates []string, category string) string {
	if len(templates) == 0 {
		return ""
	}

	recentTemplates.Lock()
	defer recentTemplates.Unlock()

	recent := recentTemplates.used[category]

	// Find a template not recently used
	var selectedIdx int
	for attempts := 0; attempts < len(templates); attempts++ {
		selectedIdx = rand.Intn(len(templates)) //nolint:gosec // Non-cryptographic use: template selection

		// Check if recently used
		isRecent := false
		for _, idx := range recent {
			if idx == selectedIdx {
				isRecent = true
				break
			}
		}

		if !isRecent {
			break
		}
	}

	// Track this selection (keep last 3)
	recent = append(recent, selectedIdx)
	if len(recent) > 3 {
		recent = recent[1:]
	}
	recentTemplates.used[category] = recent

	return templates[selectedIdx]
}

// substituteVariables replaces template variables with context values
func substituteVariables(template string, ctx *CommentaryContext) string {
	if ctx == nil {
		return template
	}

	result := template

	// Artist - use first artist for "starting with" templates, since that's what plays first
	artist := ctx.FirstArtist
	if artist == "" {
		artist = ctx.PrimaryArtist // Fall back to most common if no first artist
	}
	if artist == "" {
		artist = "your favorites"
	}
	result = strings.ReplaceAll(result, VarArtist, artist)

	// Genre
	genre := ctx.PrimaryGenre
	if genre == "" {
		genre = "music"
	}
	result = strings.ReplaceAll(result, VarGenre, genre)

	// Decade
	decade := ctx.PrimaryDecade
	if decade == "" {
		decade = "recent"
	}
	result = strings.ReplaceAll(result, VarDecade, decade)

	// Mood - human-friendly labels
	mood := ctx.humanizeMood()
	result = strings.ReplaceAll(result, VarMood, mood)

	// Album
	album := ctx.PrimaryAlbum
	if album == "" {
		album = "various tracks"
	}
	result = strings.ReplaceAll(result, VarAlbum, album)

	// Theme description
	if ctx.Theme != nil {
		result = strings.ReplaceAll(result, VarDescription, ctx.Theme.Description)
		result = strings.ReplaceAll(result, VarThemeName, ctx.Theme.Name)
	}

	return result
}

// humanizeMood converts mood tag values to human-friendly labels
func (c *CommentaryContext) humanizeMood() string {
	switch c.PrimaryMood {
	case "chill":
		return "chill"
	case "upbeat":
		return "upbeat"
	case "intense":
		return "intense"
	case "soft":
		return "mellow"
	case "calm":
		return "calm"
	case "energetic":
		return "energetic"
	case "melancholy":
		return "reflective"
	default:
		if c.PrimaryMood != "" {
			return c.PrimaryMood
		}
		return "great"
	}
}

// GenerateThemeCommentary generates commentary for a themed set
// If ctx is nil, falls back to simple description-based commentary
func GenerateThemeCommentary(theme *Theme, ctx *CommentaryContext) string {
	if theme == nil {
		return ""
	}

	// Build context if not provided (backwards compatibility)
	if ctx == nil {
		ctx = &CommentaryContext{Theme: theme}
	} else {
		ctx.Theme = theme
	}

	var template string

	// Use first-set templates for opening
	if ctx.IsFirstSet {
		// Check if we have user preferences - use preference-aware templates
		if ctx.UserPreferences != nil {
			template = selectPreferenceTemplate(ctx.UserPreferences)
		}
		// Fall back to generic first-set templates if no preference template found
		if template == "" && len(firstSetTemplates) > 0 {
			template = selectTemplate(firstSetTemplates, "first")
		}
	} else {
		// Get templates for this category
		templates := themeTemplates[theme.Category]
		if len(templates) == 0 {
			// Fallback to description
			return theme.Description
		}
		template = selectTemplate(templates, string(theme.Category))
	}

	// Substitute variables
	return substituteVariables(template, ctx)
}

// selectPreferenceTemplate picks a template based on user preferences
func selectPreferenceTemplate(prefs *model.UserPreferences) string {
	if prefs == nil {
		return ""
	}

	// Try energy first (most prominent user choice)
	if prefs.Energy != nil && *prefs.Energy != "" {
		if templates, ok := preferenceFirstSetTemplates[*prefs.Energy]; ok && len(templates) > 0 {
			return selectTemplate(templates, "pref-"+*prefs.Energy)
		}
	}

	// Try contexts
	for _, ctx := range prefs.Contexts {
		if templates, ok := preferenceFirstSetTemplates[ctx]; ok && len(templates) > 0 {
			return selectTemplate(templates, "pref-"+ctx)
		}
	}

	// No matching preference template found
	return ""
}

// GeneratePivotCommentary generates commentary when pivoting themes due to skips
func GeneratePivotCommentary(theme *Theme, ctx *CommentaryContext) string {
	if theme == nil {
		return "Switching things up."
	}

	// Build context if not provided
	if ctx == nil {
		ctx = &CommentaryContext{Theme: theme}
	} else {
		ctx.Theme = theme
	}
	ctx.IsPivot = true

	template := selectTemplate(pivotTemplates, "pivot")
	return substituteVariables(template, ctx)
}
