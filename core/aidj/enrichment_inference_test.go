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

	Describe("inferFromTitle", func() {
		It("detects acoustic keyword and returns soft mood", func() {
			moods, cleared := inferFromTitle("Song Name (Acoustic)")
			Expect(moods).To(Equal([]string{"soft"}))
			Expect(cleared).To(BeFalse())
		})

		It("detects piano keyword and returns soft mood", func() {
			moods, cleared := inferFromTitle("Song - Piano Version")
			Expect(moods).To(Equal([]string{"soft"}))
			Expect(cleared).To(BeFalse())
		})

		It("detects live performance and clears genre default", func() {
			moods, cleared := inferFromTitle("Song (Live on SNL)")
			Expect(moods).To(BeNil())
			Expect(cleared).To(BeTrue())
		})

		It("detects remix and returns upbeat mood", func() {
			moods, cleared := inferFromTitle("Song (Club Remix)")
			Expect(moods).To(Equal([]string{"upbeat"}))
			Expect(cleared).To(BeFalse())
		})

		It("returns nil for normal titles", func() {
			moods, cleared := inferFromTitle("Regular Song Title")
			Expect(moods).To(BeNil())
			Expect(cleared).To(BeFalse())
		})
	})

	Describe("inferFromAlbum", func() {
		It("detects unplugged album and returns soft mood", func() {
			moods, cleared := inferFromAlbum("MTV Unplugged")
			Expect(moods).To(Equal([]string{"soft"}))
			Expect(cleared).To(BeFalse())
		})

		It("detects live album and clears genre default", func() {
			moods, cleared := inferFromAlbum("Live at Madison Square Garden")
			Expect(moods).To(BeNil())
			Expect(cleared).To(BeTrue())
		})

		It("returns nil for normal albums", func() {
			moods, cleared := inferFromAlbum("Regular Album Name")
			Expect(moods).To(BeNil())
			Expect(cleared).To(BeFalse())
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

		It("overrides genre mood with title keyword (acoustic)", func() {
			mf := &model.MediaFile{
				Title: "Shake It Off (Acoustic)",
				Genre: "pop", // Would normally be "upbeat"
				BPM:   100,
			}
			moods, energy, _ := InferEnrichment(mf)
			Expect(moods).To(Equal([]string{"soft"}))
			Expect(energy).To(Equal(EnergyMedium))
		})

		It("clears genre default for live performances and uses BPM", func() {
			mf := &model.MediaFile{
				Title: "Lover (Live on SNL)",
				Genre: "pop", // Would normally be "upbeat"
				BPM:   70,    // Very slow = soft
			}
			moods, energy, _ := InferEnrichment(mf)
			Expect(moods).To(Equal([]string{"soft"})) // BPM override kicks in
			Expect(energy).To(Equal(EnergyLow))
		})

		It("overrides upbeat genre with very slow BPM", func() {
			mf := &model.MediaFile{
				Title: "Ballad Song",
				Genre: "pop",
				BPM:   65, // Very slow
			}
			moods, energy, _ := InferEnrichment(mf)
			Expect(moods).To(Equal([]string{"soft"}))
			Expect(energy).To(Equal(EnergyLow))
		})

		It("does not override already-soft moods with BPM", func() {
			mf := &model.MediaFile{
				Genre: "folk", // Already "soft"
				BPM:   70,
			}
			moods, _, _ := InferEnrichment(mf)
			Expect(moods).To(ContainElement("soft"))
		})

		It("uses album hints when title has no keywords", func() {
			mf := &model.MediaFile{
				Title: "Some Song",
				Album: "MTV Unplugged",
				Genre: "rock", // Would normally be "intense"
				BPM:   100,
			}
			moods, _, _ := InferEnrichment(mf)
			Expect(moods).To(Equal([]string{"soft"}))
		})
	})
})
