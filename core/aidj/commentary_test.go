package aidj

import (
	"strings"

	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CommentaryContext", func() {
	Describe("analyze", func() {
		It("extracts primary artist from tracks", func() {
			tracks := model.MediaFiles{
				{ID: "1", Artist: "The Beatles", Year: 1965},
				{ID: "2", Artist: "The Beatles", Year: 1966},
				{ID: "3", Artist: "The Rolling Stones", Year: 1965},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryArtist).To(Equal("The Beatles"))
		})

		It("extracts decade from years", func() {
			tracks := model.MediaFiles{
				{ID: "1", Year: 1992},
				{ID: "2", Year: 1995},
				{ID: "3", Year: 1998},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryDecade).To(Equal("90s"))
		})

		It("extracts mood from ai_mood tags", func() {
			tracks := model.MediaFiles{
				{ID: "1", Tags: model.Tags{"ai_mood": []string{"chill"}}},
				{ID: "2", Tags: model.Tags{"ai_mood": []string{"chill"}}},
				{ID: "3", Tags: model.Tags{"ai_mood": []string{"upbeat"}}},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryMood).To(Equal("chill"))
		})

		It("extracts primary genre from tracks", func() {
			tracks := model.MediaFiles{
				{ID: "1", Genre: "Rock"},
				{ID: "2", Genre: "Rock"},
				{ID: "3", Genre: "Pop"},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryGenre).To(Equal("Rock"))
		})

		It("extracts album when majority share it", func() {
			tracks := model.MediaFiles{
				{ID: "1", Album: "Abbey Road"},
				{ID: "2", Album: "Abbey Road"},
				{ID: "3", Album: "Let It Be"},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryAlbum).To(Equal("Abbey Road"))
		})

		It("does not set album when no majority", func() {
			tracks := model.MediaFiles{
				{ID: "1", Album: "Album A"},
				{ID: "2", Album: "Album B"},
				{ID: "3", Album: "Album C"},
				{ID: "4", Album: "Album D"},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryAlbum).To(Equal(""))
		})

		It("falls back to first track when no common value", func() {
			tracks := model.MediaFiles{
				{ID: "1", Artist: "Artist A", Year: 1980},
				{ID: "2", Artist: "Artist B", Year: 1990},
				{ID: "3", Artist: "Artist C", Year: 2000},
			}
			set := &ThemeSet{Tracks: tracks}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryArtist).To(Equal("Artist A"))
		})

		It("handles empty track list", func() {
			set := &ThemeSet{Tracks: model.MediaFiles{}}
			ctx := NewCommentaryContext(set, nil)

			Expect(ctx.PrimaryArtist).To(Equal(""))
			Expect(ctx.PrimaryGenre).To(Equal(""))
			Expect(ctx.TrackCount).To(Equal(0))
		})

		It("handles nil set", func() {
			ctx := NewCommentaryContext(nil, nil)

			Expect(ctx.PrimaryArtist).To(Equal(""))
			Expect(ctx.TrackCount).To(Equal(0))
		})

		It("sets IsFirstSet correctly from session", func() {
			set := &ThemeSet{Tracks: model.MediaFiles{{ID: "1"}}}
			session := &model.DJSession{
				ThemesUsed: []string{"theme1"},
			}
			ctx := NewCommentaryContext(set, session)

			Expect(ctx.IsFirstSet).To(BeTrue())
			Expect(ctx.SetNumber).To(Equal(1))
		})

		It("sets IsFirstSet false for subsequent sets", func() {
			set := &ThemeSet{Tracks: model.MediaFiles{{ID: "1"}}}
			session := &model.DJSession{
				ThemesUsed: []string{"theme1", "theme2", "theme3"},
			}
			ctx := NewCommentaryContext(set, session)

			Expect(ctx.IsFirstSet).To(BeFalse())
			Expect(ctx.SetNumber).To(Equal(3))
		})
	})

	Describe("yearToDecade", func() {
		It("converts years to decade strings", func() {
			ctx := &CommentaryContext{}

			Expect(ctx.yearToDecade(1965)).To(Equal("60s"))
			Expect(ctx.yearToDecade(1975)).To(Equal("70s"))
			Expect(ctx.yearToDecade(1985)).To(Equal("80s"))
			Expect(ctx.yearToDecade(1995)).To(Equal("90s"))
			Expect(ctx.yearToDecade(2005)).To(Equal("2000s"))
			Expect(ctx.yearToDecade(2015)).To(Equal("2010s"))
			Expect(ctx.yearToDecade(2023)).To(Equal("2020s"))
		})

		It("handles classic era", func() {
			ctx := &CommentaryContext{}

			Expect(ctx.yearToDecade(1955)).To(Equal("classic era"))
			Expect(ctx.yearToDecade(1940)).To(Equal("classic era"))
		})
	})

	Describe("humanizeMood", func() {
		It("returns human-friendly mood labels", func() {
			ctx := &CommentaryContext{PrimaryMood: "soft"}
			Expect(ctx.humanizeMood()).To(Equal("mellow"))

			ctx.PrimaryMood = "melancholy"
			Expect(ctx.humanizeMood()).To(Equal("reflective"))

			ctx.PrimaryMood = "chill"
			Expect(ctx.humanizeMood()).To(Equal("chill"))
		})

		It("returns original mood if not mapped", func() {
			ctx := &CommentaryContext{PrimaryMood: "custom_mood"}
			Expect(ctx.humanizeMood()).To(Equal("custom_mood"))
		})

		It("returns 'great' for empty mood", func() {
			ctx := &CommentaryContext{}
			Expect(ctx.humanizeMood()).To(Equal("great"))
		})
	})
})

var _ = Describe("substituteVariables", func() {
	It("replaces artist variable", func() {
		ctx := &CommentaryContext{
			PrimaryArtist: "The Beatles",
			Theme:         &Theme{Description: "classic rock"},
		}
		result := substituteVariables("Here's {artist} with some {description}.", ctx)

		Expect(result).To(Equal("Here's The Beatles with some classic rock."))
	})

	It("replaces genre variable", func() {
		ctx := &CommentaryContext{
			PrimaryGenre: "Jazz",
			Theme:        &Theme{Description: "smooth jazz"},
		}
		result := substituteVariables("Time for some {genre}.", ctx)

		Expect(result).To(Equal("Time for some Jazz."))
	})

	It("replaces decade variable", func() {
		ctx := &CommentaryContext{
			PrimaryDecade: "90s",
			Theme:         &Theme{Description: "90s hits"},
		}
		result := substituteVariables("Taking you back to the {decade}.", ctx)

		Expect(result).To(Equal("Taking you back to the 90s."))
	})

	It("replaces mood variable with humanized value", func() {
		ctx := &CommentaryContext{
			PrimaryMood: "melancholy",
			Theme:       &Theme{Description: "sad songs"},
		}
		result := substituteVariables("Here's some {mood} music.", ctx)

		Expect(result).To(Equal("Here's some reflective music."))
	})

	It("provides fallback for missing artist", func() {
		ctx := &CommentaryContext{
			Theme: &Theme{Description: "great music"},
		}
		result := substituteVariables("Featuring {artist}.", ctx)

		Expect(result).To(Equal("Featuring your favorites."))
	})

	It("provides fallback for missing genre", func() {
		ctx := &CommentaryContext{
			Theme: &Theme{Description: "great tracks"},
		}
		result := substituteVariables("Time for some {genre}.", ctx)

		Expect(result).To(Equal("Time for some music."))
	})

	It("provides fallback for missing decade", func() {
		ctx := &CommentaryContext{
			Theme: &Theme{Description: "good songs"},
		}
		result := substituteVariables("From the {decade}.", ctx)

		Expect(result).To(Equal("From the recent."))
	})

	It("handles nil context", func() {
		result := substituteVariables("Template with {artist}.", nil)
		Expect(result).To(Equal("Template with {artist}."))
	})

	It("handles nil theme", func() {
		ctx := &CommentaryContext{
			PrimaryArtist: "Queen",
		}
		result := substituteVariables("Here's {artist} with {description}.", ctx)

		Expect(result).To(Equal("Here's Queen with {description}."))
	})

	It("replaces theme name variable", func() {
		ctx := &CommentaryContext{
			Theme: &Theme{
				Name:        "80s Classics",
				Description: "classic hits from the 80s",
			},
		}
		result := substituteVariables("Playing {themeName}.", ctx)

		Expect(result).To(Equal("Playing 80s Classics."))
	})
})

var _ = Describe("GenerateThemeCommentary", func() {
	It("generates non-empty commentary", func() {
		theme := &Theme{
			ID:          "test-theme",
			Category:    ThemeCategoryEra,
			Description: "classic hits",
		}
		tracks := model.MediaFiles{
			{ID: "1", Artist: "Queen", Year: 1985},
		}
		set := &ThemeSet{Theme: theme, Tracks: tracks}
		ctx := NewCommentaryContext(set, nil)

		result := GenerateThemeCommentary(theme, ctx)

		Expect(result).NotTo(BeEmpty())
	})

	It("returns empty string for nil theme", func() {
		result := GenerateThemeCommentary(nil, nil)
		Expect(result).To(Equal(""))
	})

	It("includes description in output", func() {
		theme := &Theme{
			ID:          "80s-classics",
			Category:    ThemeCategoryEra,
			Description: "80s classics",
		}
		tracks := model.MediaFiles{
			{ID: "1", Artist: "Queen", Year: 1985},
		}
		set := &ThemeSet{Theme: theme, Tracks: tracks}
		ctx := NewCommentaryContext(set, nil)

		// Try multiple times since template selection is random
		found := false
		for i := 0; i < 20; i++ {
			result := GenerateThemeCommentary(theme, ctx)
			if strings.Contains(result, "80s classics") || strings.Contains(result, "Queen") || strings.Contains(result, "80s") {
				found = true
				break
			}
		}

		Expect(found).To(BeTrue())
	})

	It("uses first-set templates when IsFirstSet is true", func() {
		theme := &Theme{
			ID:          "test-theme",
			Category:    ThemeCategoryMood,
			Description: "relaxing tracks",
		}
		tracks := model.MediaFiles{
			{ID: "1", Artist: "Norah Jones"},
		}
		set := &ThemeSet{Theme: theme, Tracks: tracks}
		ctx := NewCommentaryContext(set, nil)
		ctx.IsFirstSet = true

		// First-set templates contain "Welcome" or "start" or "begin" or "kicking"
		foundFirstSetTemplate := false
		for i := 0; i < 20; i++ {
			result := GenerateThemeCommentary(theme, ctx)
			if strings.Contains(result, "Welcome") ||
				strings.Contains(result, "start") ||
				strings.Contains(result, "begin") ||
				strings.Contains(result, "kicking") {
				foundFirstSetTemplate = true
				break
			}
		}

		Expect(foundFirstSetTemplate).To(BeTrue())
	})

	It("works without context (backwards compatibility)", func() {
		theme := &Theme{
			ID:          "test-theme",
			Category:    ThemeCategoryGenre,
			Description: "jazz standards",
		}

		result := GenerateThemeCommentary(theme, nil)

		Expect(result).NotTo(BeEmpty())
	})
})

var _ = Describe("GeneratePivotCommentary", func() {
	It("generates pivot-specific commentary", func() {
		theme := &Theme{
			ID:          "chill-vibes",
			Category:    ThemeCategoryMood,
			Description: "relaxing tracks",
		}
		tracks := model.MediaFiles{
			{ID: "1", Artist: "Norah Jones"},
		}
		set := &ThemeSet{Theme: theme, Tracks: tracks}
		ctx := NewCommentaryContext(set, nil)

		result := GeneratePivotCommentary(theme, ctx)

		Expect(result).NotTo(BeEmpty())
		// Pivot templates contain words like "switching", "different", "direction", "pivoting", "how about"
		Expect(result).To(SatisfyAny(
			ContainSubstring("witch"),
			ContainSubstring("different"),
			ContainSubstring("direction"),
			ContainSubstring("Pivoting"),
			ContainSubstring("How about"),
		))
	})

	It("returns default message for nil theme", func() {
		result := GeneratePivotCommentary(nil, nil)
		Expect(result).To(Equal("Switching things up."))
	})

	It("sets IsPivot flag on context", func() {
		theme := &Theme{
			ID:          "test",
			Category:    ThemeCategoryMood,
			Description: "test",
		}
		ctx := &CommentaryContext{}

		GeneratePivotCommentary(theme, ctx)

		Expect(ctx.IsPivot).To(BeTrue())
	})
})

var _ = Describe("selectTemplate", func() {
	It("returns empty string for empty templates", func() {
		result := selectTemplate([]string{}, "test")
		Expect(result).To(Equal(""))
	})

	It("returns a template from the list", func() {
		templates := []string{"Template A", "Template B", "Template C"}
		result := selectTemplate(templates, "test-category")

		Expect(templates).To(ContainElement(result))
	})

	It("avoids recently used templates", func() {
		templates := []string{"A", "B", "C", "D", "E"}

		// Call multiple times and track results
		results := make(map[string]int)
		for i := 0; i < 50; i++ {
			result := selectTemplate(templates, "repetition-test")
			results[result]++
		}

		// With 5 templates and avoiding last 3, we should see variety
		Expect(len(results)).To(BeNumerically(">=", 2))
	})
})
