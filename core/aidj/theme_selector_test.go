package aidj

import (
	"github.com/navidrome/navidrome/model/criteria"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ThemeSelector", func() {
	Describe("determineSetSize", func() {
		var selector *ThemeSelector

		BeforeEach(func() {
			selector = &ThemeSelector{}
		})

		It("returns pool size when smaller than min", func() {
			size := selector.determineSetSize(2)
			Expect(size).To(Equal(2))
		})

		It("returns value in range for large pools", func() {
			size := selector.determineSetSize(100)
			Expect(size).To(BeNumerically(">=", 3))
			Expect(size).To(BeNumerically("<=", 5))
		})
	})

	Describe("Theme criteria generation", func() {
		BeforeEach(func() {
			// Register AI tags for testing
			criteria.AddTagNames([]string{"ai_mood", "ai_vibe", "ai_energy"})
		})

		It("generates valid criteria for era theme", func() {
			theme := &Theme{
				ID:       "80s-test",
				Category: ThemeCategoryEra,
				Criteria: criteria.Criteria{
					Expression: criteria.All{
						criteria.InTheRange{"year": []int{1980, 1989}},
					},
				},
			}

			sql, args, err := theme.Criteria.Expression.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("year"))
			Expect(args).To(HaveLen(2))
		})

		It("generates valid criteria for mood theme", func() {
			theme := &Theme{
				ID:       "chill-test",
				Category: ThemeCategoryMood,
				Criteria: criteria.Criteria{
					Expression: criteria.All{
						criteria.Is{"ai_mood": "chill"},
					},
				},
			}

			sql, _, err := theme.Criteria.Expression.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("ai_mood"))
		})

		It("generates valid criteria for energy theme", func() {
			theme := &Theme{
				ID:       "high-energy-test",
				Category: ThemeCategoryEnergy,
				Criteria: criteria.Criteria{
					Expression: criteria.All{
						criteria.Is{"ai_energy": "high"},
					},
				},
			}

			sql, _, err := theme.Criteria.Expression.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("ai_energy"))
		})

		It("generates valid criteria for familiarity theme with rating", func() {
			theme := &Theme{
				ID:       "favorites-test",
				Category: ThemeCategoryFamiliarity,
				Criteria: criteria.Criteria{
					Expression: criteria.Any{
						criteria.Gt{"rating": 3},
						criteria.Is{"loved": true},
					},
				},
			}

			sql, args, err := theme.Criteria.Expression.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("rating"))
			Expect(args).To(HaveLen(2))
		})

		It("generates valid criteria for temporal theme", func() {
			theme := &Theme{
				ID:       "recently-added-test",
				Category: ThemeCategoryTemporal,
				Criteria: criteria.Criteria{
					Expression: criteria.All{
						criteria.InTheLast{"dateadded": 30},
					},
				},
			}

			sql, args, err := theme.Criteria.Expression.ToSql()
			Expect(err).ToNot(HaveOccurred())
			Expect(sql).To(ContainSubstring("created_at"))
			Expect(args).To(HaveLen(1))
		})
	})

	Describe("ThemeRegistry", func() {
		It("returns theme by ID", func() {
			registry := GetThemeRegistry()
			theme := registry.GetTheme("80s-classics")
			Expect(theme).ToNot(BeNil())
			Expect(theme.Name).To(Equal("80s Classics"))
		})

		It("returns themes by category", func() {
			registry := GetThemeRegistry()
			eraThemes := registry.GetThemesByCategory(ThemeCategoryEra)
			Expect(len(eraThemes)).To(BeNumerically(">=", 4))
		})

		It("has at least 18 built-in themes", func() {
			registry := GetThemeRegistry()
			allThemes := registry.AllThemes()
			Expect(len(allThemes)).To(BeNumerically(">=", 18))
		})

		It("returns nil for unknown theme ID", func() {
			registry := GetThemeRegistry()
			theme := registry.GetTheme("nonexistent-theme")
			Expect(theme).To(BeNil())
		})
	})
})
