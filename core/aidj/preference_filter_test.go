package aidj

import (
	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PreferenceFilter", func() {
	Describe("HasFilters", func() {
		It("returns false for nil preferences", func() {
			pf := NewPreferenceFilter(nil)
			Expect(pf.HasFilters()).To(BeFalse())
		})

		It("returns false for empty preferences", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			Expect(pf.HasFilters()).To(BeFalse())
		})

		It("returns true when energy is set", func() {
			energy := "upbeat"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			Expect(pf.HasFilters()).To(BeTrue())
		})

		It("returns true when decade is set", func() {
			decade := 1990
			pf := NewPreferenceFilter(&model.UserPreferences{Decade: &decade})
			Expect(pf.HasFilters()).To(BeTrue())
		})

		It("returns true when contexts is set", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{Contexts: []string{"favorites"}})
			Expect(pf.HasFilters()).To(BeTrue())
		})
	})

	Describe("ToExpressions", func() {
		It("returns nil for nil preferences", func() {
			pf := NewPreferenceFilter(nil)
			Expect(pf.ToExpressions()).To(BeNil())
		})

		It("returns expressions for energy preference", func() {
			energy := "upbeat"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			exprs := pf.ToExpressions()
			Expect(exprs).To(HaveLen(1))
		})

		It("returns expressions for decade preference", func() {
			decade := 1990
			pf := NewPreferenceFilter(&model.UserPreferences{Decade: &decade})
			exprs := pf.ToExpressions()
			Expect(exprs).To(HaveLen(1))
		})

		It("returns expressions for context preferences", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{Contexts: []string{"favorites", "forgotten"}})
			exprs := pf.ToExpressions()
			Expect(exprs).To(HaveLen(2))
		})

		It("returns combined expressions for all preferences", func() {
			energy := "soft"
			decade := 1980
			pf := NewPreferenceFilter(&model.UserPreferences{
				Energy:   &energy,
				Decade:   &decade,
				Contexts: []string{"new"},
			})
			exprs := pf.ToExpressions()
			Expect(exprs).To(HaveLen(3))
		})
	})

	Describe("energyExpression", func() {
		It("returns nil for nil energy", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.energyExpression()
			Expect(expr).To(BeNil())
		})

		It("returns expression for upbeat energy", func() {
			energy := "upbeat"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			expr := pf.energyExpression()
			Expect(expr).NotTo(BeNil())
		})

		It("returns expression for soft energy", func() {
			energy := "soft"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			expr := pf.energyExpression()
			Expect(expr).NotTo(BeNil())
		})

		It("returns expression for intense energy", func() {
			energy := "intense"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			expr := pf.energyExpression()
			Expect(expr).NotTo(BeNil())
		})

		It("returns nil for unknown energy", func() {
			energy := "unknown"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			expr := pf.energyExpression()
			Expect(expr).To(BeNil())
		})
	})

	Describe("decadeExpression", func() {
		It("returns nil for nil decade", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.decadeExpression()
			Expect(expr).To(BeNil())
		})

		It("returns expression for valid decades", func() {
			for _, decade := range []int{1960, 1970, 1980, 1990, 2000, 2010, 2020} {
				d := decade
				pf := NewPreferenceFilter(&model.UserPreferences{Decade: &d})
				expr := pf.decadeExpression()
				Expect(expr).NotTo(BeNil(), "Expected expression for decade %d", decade)
			}
		})

		It("returns nil for invalid decade", func() {
			decade := 1955 // not on decade boundary
			pf := NewPreferenceFilter(&model.UserPreferences{Decade: &decade})
			expr := pf.decadeExpression()
			Expect(expr).To(BeNil())
		})

		It("returns nil for decade before 1960", func() {
			decade := 1950
			pf := NewPreferenceFilter(&model.UserPreferences{Decade: &decade})
			expr := pf.decadeExpression()
			Expect(expr).To(BeNil())
		})

		It("returns nil for decade after 2020", func() {
			decade := 2030
			pf := NewPreferenceFilter(&model.UserPreferences{Decade: &decade})
			expr := pf.decadeExpression()
			Expect(expr).To(BeNil())
		})
	})

	Describe("contextExpression", func() {
		It("returns expression for forgotten context", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.contextExpression("forgotten")
			Expect(expr).NotTo(BeNil())
		})

		It("returns expression for favorites context", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.contextExpression("favorites")
			Expect(expr).NotTo(BeNil())
		})

		It("returns expression for new context", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.contextExpression("new")
			Expect(expr).NotTo(BeNil())
		})

		It("returns nil for unknown context", func() {
			pf := NewPreferenceFilter(&model.UserPreferences{})
			expr := pf.contextExpression("unknown")
			Expect(expr).To(BeNil())
		})
	})

	Describe("RelaxedFilters", func() {
		It("returns nil for nil preferences", func() {
			pf := NewPreferenceFilter(nil)
			Expect(pf.RelaxedFilters()).To(BeNil())
		})

		It("returns single relaxation level for only energy", func() {
			energy := "upbeat"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			relaxations := pf.RelaxedFilters()
			// Should have: full filters, no filters
			Expect(len(relaxations)).To(BeNumerically(">=", 2))
		})

		It("returns multiple relaxation levels for all preferences", func() {
			energy := "soft"
			decade := 1990
			pf := NewPreferenceFilter(&model.UserPreferences{
				Energy:   &energy,
				Decade:   &decade,
				Contexts: []string{"favorites"},
			})
			relaxations := pf.RelaxedFilters()
			// Should have: full, without decade, without decade+contexts, no filters
			Expect(len(relaxations)).To(BeNumerically(">=", 3))
		})

		It("always ends with no filters", func() {
			energy := "intense"
			pf := NewPreferenceFilter(&model.UserPreferences{Energy: &energy})
			relaxations := pf.RelaxedFilters()
			// Last element should be nil (no filters)
			Expect(relaxations[len(relaxations)-1]).To(BeNil())
		})
	})
})
