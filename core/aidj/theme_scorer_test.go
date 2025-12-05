package aidj

import (
	"time"

	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ThemeScorer", func() {
	var scorer *ThemeScorer

	BeforeEach(func() {
		scorer = NewThemeScorer()
	})

	Describe("ScoreThemes", func() {
		It("returns themes with positive scores", func() {
			session := &model.DJSession{
				CreatedAt: time.Now(),
			}
			scores := scorer.ScoreThemes(session)
			Expect(scores).NotTo(BeEmpty())
			for _, s := range scores {
				Expect(s.Score).To(BeNumerically(">", 0))
			}
		})

		It("penalizes recently used themes", func() {
			session := &model.DJSession{
				CreatedAt:  time.Now(),
				ThemesUsed: []string{"chill-vibes"},
			}
			scores := scorer.ScoreThemes(session)

			var chillScore float64
			for _, s := range scores {
				if s.Theme.ID == "chill-vibes" {
					chillScore = s.Score
					break
				}
			}

			// chill-vibes should be penalized
			freshSession := &model.DJSession{CreatedAt: time.Now()}
			freshScores := scorer.ScoreThemes(freshSession)
			var freshChillScore float64
			for _, s := range freshScores {
				if s.Theme.ID == "chill-vibes" {
					freshChillScore = s.Score
					break
				}
			}

			Expect(chillScore).To(BeNumerically("<", freshChillScore))
		})
	})

	Describe("SelectBestTheme", func() {
		It("returns a theme", func() {
			session := &model.DJSession{CreatedAt: time.Now()}
			theme := scorer.SelectBestTheme(session)
			Expect(theme).NotTo(BeNil())
		})

		It("avoids recently used themes", func() {
			session := &model.DJSession{
				CreatedAt:  time.Now(),
				ThemesUsed: []string{"your-favorites", "five-star"}, // high priority themes
			}
			theme := scorer.SelectBestTheme(session)
			Expect(theme.ID).NotTo(Equal("your-favorites"))
			Expect(theme.ID).NotTo(Equal("five-star"))
		})

		It("completely excludes pivot-triggered themes", func() {
			session := &model.DJSession{
				CreatedAt:      time.Now(),
				ExcludedThemes: map[string]bool{"your-favorites": true, "five-star": true},
			}
			theme := scorer.SelectBestTheme(session)
			Expect(theme).NotTo(BeNil())
			Expect(theme.ID).NotTo(Equal("your-favorites"))
			Expect(theme.ID).NotTo(Equal("five-star"))
		})
	})

	Describe("calculateDecay", func() {
		It("returns 1.0 for new sessions", func() {
			session := &model.DJSession{
				CreatedAt: time.Now(),
			}
			decay := scorer.calculateDecay(session)
			Expect(decay).To(BeNumerically("~", 1.0, 0.01))
		})

		It("returns ~0.5 after 30 minutes", func() {
			session := &model.DJSession{
				CreatedAt: time.Now().Add(-30 * time.Minute),
			}
			decay := scorer.calculateDecay(session)
			Expect(decay).To(BeNumerically("~", 0.5, 0.1))
		})

		It("returns ~0.25 after 60 minutes", func() {
			session := &model.DJSession{
				CreatedAt: time.Now().Add(-60 * time.Minute),
			}
			decay := scorer.calculateDecay(session)
			Expect(decay).To(BeNumerically("~", 0.25, 0.1))
		})
	})

	Describe("skippedThemePenalty", func() {
		It("returns 0 for themes with no skips", func() {
			session := &model.DJSession{
				CreatedAt:     time.Now(),
				SkippedThemes: map[string]int{},
			}
			registry := GetThemeRegistry()
			theme := registry.GetTheme("chill-vibes")
			penalty := scorer.skippedThemePenalty(theme, session)
			Expect(penalty).To(Equal(0.0))
		})

		It("applies penalty for skipped themes", func() {
			session := &model.DJSession{
				CreatedAt:     time.Now(),
				SkippedThemes: map[string]int{"chill-vibes": 2},
			}
			registry := GetThemeRegistry()
			theme := registry.GetTheme("chill-vibes")
			penalty := scorer.skippedThemePenalty(theme, session)
			Expect(penalty).To(BeNumerically(">", 0))
		})
	})

	Describe("recentlyUsedPenalty", func() {
		It("returns 0 for themes not used", func() {
			session := &model.DJSession{
				CreatedAt:  time.Now(),
				ThemesUsed: []string{"high-energy"},
			}
			registry := GetThemeRegistry()
			theme := registry.GetTheme("chill-vibes")
			penalty := scorer.recentlyUsedPenalty(theme, session)
			Expect(penalty).To(Equal(0.0))
		})

		It("applies heavy penalty for last used theme", func() {
			session := &model.DJSession{
				CreatedAt:  time.Now(),
				ThemesUsed: []string{"chill-vibes"},
			}
			registry := GetThemeRegistry()
			theme := registry.GetTheme("chill-vibes")
			penalty := scorer.recentlyUsedPenalty(theme, session)
			Expect(penalty).To(Equal(0.5))
		})

		It("applies lighter penalty for themes used longer ago", func() {
			session := &model.DJSession{
				CreatedAt:  time.Now(),
				ThemesUsed: []string{"chill-vibes", "high-energy", "low-energy", "deep-cuts", "your-favorites"},
			}
			registry := GetThemeRegistry()
			theme := registry.GetTheme("chill-vibes")
			penalty := scorer.recentlyUsedPenalty(theme, session)
			Expect(penalty).To(Equal(0.1)) // Light penalty for older themes
		})
	})
})
