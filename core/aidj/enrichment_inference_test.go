package aidj

import (
	"testing"

	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAIDJ(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AIDJ Suite")
}

var _ = Describe("EnrichmentInference", func() {
	Describe("inferEnergyFromBPM", func() {
		It("returns low energy for BPM < 90", func() {
			Expect(inferEnergyFromBPM(80)).To(Equal(EnergyLow))
			Expect(inferEnergyFromBPM(89)).To(Equal(EnergyLow))
		})

		It("returns medium energy for BPM between 90 and 120", func() {
			Expect(inferEnergyFromBPM(90)).To(Equal(EnergyMedium))
			Expect(inferEnergyFromBPM(100)).To(Equal(EnergyMedium))
			Expect(inferEnergyFromBPM(120)).To(Equal(EnergyMedium))
		})

		It("returns high energy for BPM > 120", func() {
			Expect(inferEnergyFromBPM(121)).To(Equal(EnergyHigh))
			Expect(inferEnergyFromBPM(150)).To(Equal(EnergyHigh))
		})

		It("returns medium energy for unknown BPM (0)", func() {
			Expect(inferEnergyFromBPM(0)).To(Equal(EnergyMedium))
		})
	})

	Describe("inferFromGenre", func() {
		It("returns chill mood and coffeeshop vibe for jazz", func() {
			moods, vibes := inferFromGenre("jazz")
			Expect(moods).To(ContainElement("chill"))
			Expect(vibes).To(ContainElements("coffeeshop", "late-night"))
		})

		It("returns upbeat mood and workout vibe for electronic", func() {
			moods, vibes := inferFromGenre("electronic")
			Expect(moods).To(ContainElement("upbeat"))
			Expect(vibes).To(ContainElements("workout", "party"))
		})

		It("handles case-insensitive genre matching", func() {
			moods, vibes := inferFromGenre("JAZZ")
			Expect(moods).To(ContainElement("chill"))
			Expect(vibes).To(ContainElements("coffeeshop", "late-night"))
		})

		It("handles partial genre matching", func() {
			moods, _ := inferFromGenre("Progressive Rock")
			Expect(moods).To(ContainElements("intense", "energetic"))
		})

		It("returns nil for unknown genre", func() {
			moods, vibes := inferFromGenre("unknowngenre12345")
			Expect(moods).To(BeNil())
			Expect(vibes).To(BeNil())
		})

		It("returns nil for empty genre", func() {
			moods, vibes := inferFromGenre("")
			Expect(moods).To(BeNil())
			Expect(vibes).To(BeNil())
		})
	})

	Describe("InferEnrichment", func() {
		It("infers mood and energy from jazz track with low BPM", func() {
			mf := &model.MediaFile{
				Genre: "jazz",
				BPM:   80,
			}
			moods, energy, vibes := InferEnrichment(mf)
			Expect(moods).To(ContainElement("chill"))
			Expect(energy).To(Equal(EnergyLow))
			Expect(vibes).To(ContainElements("coffeeshop", "late-night"))
		})

		It("infers mood and energy from electronic track with high BPM", func() {
			mf := &model.MediaFile{
				Genre: "techno",
				BPM:   140,
			}
			moods, energy, vibes := InferEnrichment(mf)
			Expect(moods).To(ContainElements("upbeat", "intense"))
			Expect(energy).To(Equal(EnergyHigh))
			Expect(vibes).To(ContainElements("workout", "party"))
		})

		It("falls back to energy-based mood when genre unknown", func() {
			mf := &model.MediaFile{
				Genre: "",
				BPM:   130,
			}
			moods, energy, vibes := InferEnrichment(mf)
			Expect(moods).To(ContainElement("energetic"))
			Expect(energy).To(Equal(EnergyHigh))
			Expect(vibes).To(BeNil())
		})

		It("handles track with no BPM and no genre", func() {
			mf := &model.MediaFile{
				Genre: "",
				BPM:   0,
			}
			moods, energy, vibes := InferEnrichment(mf)
			Expect(moods).To(BeEmpty())
			Expect(energy).To(Equal(EnergyMedium))
			Expect(vibes).To(BeNil())
		})
	})
})
