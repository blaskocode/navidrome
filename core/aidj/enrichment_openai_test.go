package aidj

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockHTTPClient implements httpDoer for testing
type mockHTTPClient struct {
	response *http.Response
	err      error
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

var _ = Describe("OpenAI Enrichment", func() {
	Describe("buildEnrichmentPrompt", func() {
		It("includes all available metadata", func() {
			mf := &model.MediaFile{
				Title:    "Bohemian Rhapsody",
				Artist:   "Queen",
				Album:    "A Night at the Opera",
				Genre:    "Rock",
				Year:     1975,
				Duration: 354.5,
				BPM:      72,
			}

			prompt := buildEnrichmentPrompt(mf)

			Expect(prompt).To(ContainSubstring("Title: Bohemian Rhapsody"))
			Expect(prompt).To(ContainSubstring("Artist: Queen"))
			Expect(prompt).To(ContainSubstring("Album: A Night at the Opera"))
			Expect(prompt).To(ContainSubstring("Genre: Rock"))
			Expect(prompt).To(ContainSubstring("Year: 1975"))
			Expect(prompt).To(ContainSubstring("Duration: 5:54"))
			Expect(prompt).To(ContainSubstring("BPM: 72"))
		})

		It("omits missing metadata", func() {
			mf := &model.MediaFile{
				Title:  "Unknown Track",
				Artist: "Unknown Artist",
			}

			prompt := buildEnrichmentPrompt(mf)

			Expect(prompt).To(ContainSubstring("Title: Unknown Track"))
			Expect(prompt).To(ContainSubstring("Artist: Unknown Artist"))
			Expect(prompt).NotTo(ContainSubstring("Album:"))
			Expect(prompt).NotTo(ContainSubstring("Genre:"))
			Expect(prompt).NotTo(ContainSubstring("Year:"))
			Expect(prompt).NotTo(ContainSubstring("Duration:"))
			Expect(prompt).NotTo(ContainSubstring("BPM:"))
		})

		It("formats duration correctly", func() {
			mf := &model.MediaFile{
				Title:    "Short Song",
				Artist:   "Artist",
				Duration: 65, // 1:05
			}

			prompt := buildEnrichmentPrompt(mf)
			Expect(prompt).To(ContainSubstring("Duration: 1:05"))
		})
	})

	Describe("extractJSONFromMarkdown", func() {
		It("removes markdown code block markers", func() {
			content := "```json\n{\"mood\": \"upbeat\"}\n```"
			result := extractJSONFromMarkdown(content)
			Expect(result).To(Equal("{\"mood\": \"upbeat\"}"))
		})

		It("handles content without markers", func() {
			content := "{\"mood\": \"upbeat\"}"
			result := extractJSONFromMarkdown(content)
			Expect(result).To(Equal("{\"mood\": \"upbeat\"}"))
		})
	})

	Describe("validateEnrichmentResult", func() {
		It("accepts valid moods", func() {
			validMoods := []string{"upbeat", "soft", "intense", "chill", "calm", "energetic", "melancholy"}
			for _, mood := range validMoods {
				result := &OpenAIEnrichmentResult{Mood: mood, Energy: "medium"}
				err := validateEnrichmentResult(result)
				Expect(err).NotTo(HaveOccurred(), "mood: %s", mood)
			}
		})

		It("rejects invalid moods", func() {
			result := &OpenAIEnrichmentResult{Mood: "invalid", Energy: "medium"}
			err := validateEnrichmentResult(result)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid mood"))
		})

		It("accepts valid energy levels", func() {
			validEnergy := []string{"low", "medium", "high"}
			for _, energy := range validEnergy {
				result := &OpenAIEnrichmentResult{Mood: "upbeat", Energy: energy}
				err := validateEnrichmentResult(result)
				Expect(err).NotTo(HaveOccurred(), "energy: %s", energy)
			}
		})

		It("rejects invalid energy levels", func() {
			result := &OpenAIEnrichmentResult{Mood: "upbeat", Energy: "invalid"}
			err := validateEnrichmentResult(result)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid energy"))
		})

		It("filters invalid vibes", func() {
			result := &OpenAIEnrichmentResult{
				Mood:   "upbeat",
				Energy: "high",
				Vibes:  []string{"party", "invalid", "workout", "alsoinvalid"},
			}
			err := validateEnrichmentResult(result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Vibes).To(ConsistOf("party", "workout"))
		})

		It("normalizes case", func() {
			result := &OpenAIEnrichmentResult{
				Mood:   "UPBEAT",
				Energy: "HIGH",
				Vibes:  []string{"PARTY"},
			}
			err := validateEnrichmentResult(result)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Mood).To(Equal("upbeat"))
			Expect(result.Energy).To(Equal("high"))
			Expect(result.Vibes).To(ConsistOf("party"))
		})
	})

	Describe("EnrichTrack", func() {
		It("parses valid OpenAI response", func() {
			responseBody := `{
				"choices": [{
					"message": {
						"content": "{\"mood\": \"upbeat\", \"energy\": \"high\", \"vibes\": [\"party\", \"workout\"]}"
					}
				}]
			}`

			mockClient := &mockHTTPClient{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
				},
			}

			enricher := NewOpenAIEnricher("test-key", "gpt-4o-mini", mockClient)
			mf := &model.MediaFile{
				Title:  "Test Song",
				Artist: "Test Artist",
			}

			result, err := enricher.EnrichTrack(context.Background(), mf)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Mood).To(Equal("upbeat"))
			Expect(result.Energy).To(Equal("high"))
			Expect(result.Vibes).To(ConsistOf("party", "workout"))
		})

		It("handles markdown-wrapped JSON", func() {
			responseBody := `{
				"choices": [{
					"message": {
						"content": "` + "```json\\n{\\\"mood\\\": \\\"soft\\\", \\\"energy\\\": \\\"low\\\", \\\"vibes\\\": [\\\"coffeeshop\\\"]}\\n```" + `"
					}
				}]
			}`

			mockClient := &mockHTTPClient{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
				},
			}

			enricher := NewOpenAIEnricher("test-key", "gpt-4o-mini", mockClient)
			mf := &model.MediaFile{
				Title:  "Test Song",
				Artist: "Test Artist",
			}

			result, err := enricher.EnrichTrack(context.Background(), mf)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Mood).To(Equal("soft"))
			Expect(result.Energy).To(Equal("low"))
		})

		It("returns error for API errors", func() {
			responseBody := `{
				"error": {
					"message": "Invalid API key",
					"type": "invalid_request_error"
				}
			}`

			mockClient := &mockHTTPClient{
				response: &http.Response{
					StatusCode: 401,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
				},
			}

			enricher := NewOpenAIEnricher("invalid-key", "gpt-4o-mini", mockClient)
			mf := &model.MediaFile{
				Title:  "Test Song",
				Artist: "Test Artist",
			}

			_, err := enricher.EnrichTrack(context.Background(), mf)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Invalid API key"))
		})

		It("returns error for empty choices", func() {
			responseBody := `{"choices": []}`

			mockClient := &mockHTTPClient{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
				},
			}

			enricher := NewOpenAIEnricher("test-key", "gpt-4o-mini", mockClient)
			mf := &model.MediaFile{
				Title:  "Test Song",
				Artist: "Test Artist",
			}

			_, err := enricher.EnrichTrack(context.Background(), mf)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no response from OpenAI"))
		})
	})

	Describe("NewOpenAIEnricher", func() {
		It("uses default model when not specified", func() {
			enricher := NewOpenAIEnricher("key", "", nil)
			Expect(enricher.model).To(Equal("gpt-4o-mini"))
		})

		It("uses specified model", func() {
			enricher := NewOpenAIEnricher("key", "gpt-4", nil)
			Expect(enricher.model).To(Equal("gpt-4"))
		})
	})

	Describe("parseEnergyLevel", func() {
		It("parses low energy", func() {
			Expect(parseEnergyLevel("low")).To(Equal(EnergyLow))
			Expect(parseEnergyLevel("LOW")).To(Equal(EnergyLow))
		})

		It("parses medium energy", func() {
			Expect(parseEnergyLevel("medium")).To(Equal(EnergyMedium))
			Expect(parseEnergyLevel("MEDIUM")).To(Equal(EnergyMedium))
		})

		It("parses high energy", func() {
			Expect(parseEnergyLevel("high")).To(Equal(EnergyHigh))
			Expect(parseEnergyLevel("HIGH")).To(Equal(EnergyHigh))
		})

		It("returns empty for invalid values", func() {
			Expect(parseEnergyLevel("invalid")).To(Equal(EnergyLevel("")))
			Expect(parseEnergyLevel("")).To(Equal(EnergyLevel("")))
		})
	})
})
