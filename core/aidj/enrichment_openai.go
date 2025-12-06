package aidj

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	openAIAPIURL = "https://api.openai.com/v1/chat/completions"
)

// OpenAIEnricher enriches tracks using OpenAI's API
type OpenAIEnricher struct {
	apiKey string
	model  string
	hc     httpDoer
}

// httpDoer interface for HTTP client (allows mocking in tests)
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewOpenAIEnricher creates a new OpenAI enricher
func NewOpenAIEnricher(apiKey, model string, hc httpDoer) *OpenAIEnricher {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIEnricher{
		apiKey: apiKey,
		model:  model,
		hc:     hc,
	}
}

// OpenAIEnrichmentResult represents the JSON response from OpenAI
type OpenAIEnrichmentResult struct {
	Mood   string   `json:"mood"`
	Energy string   `json:"energy"`
	Vibes  []string `json:"vibes"`
}

// EnrichTrack uses OpenAI to analyze track metadata and return enrichment data
func (e *OpenAIEnricher) EnrichTrack(ctx context.Context, mf *model.MediaFile) (*OpenAIEnrichmentResult, error) {
	prompt := buildEnrichmentPrompt(mf)

	result, err := e.callOpenAI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildEnrichmentPrompt(mf *model.MediaFile) string {
	var sb strings.Builder
	sb.WriteString("Given this song metadata:\n")
	sb.WriteString(fmt.Sprintf("- Title: %s\n", mf.Title))
	sb.WriteString(fmt.Sprintf("- Artist: %s\n", mf.Artist))
	if mf.Album != "" {
		sb.WriteString(fmt.Sprintf("- Album: %s\n", mf.Album))
	}
	if mf.Genre != "" {
		sb.WriteString(fmt.Sprintf("- Genre: %s\n", mf.Genre))
	}
	if mf.Year > 0 {
		sb.WriteString(fmt.Sprintf("- Year: %d\n", mf.Year))
	}
	if mf.Duration > 0 {
		durationSec := int(mf.Duration)
		minutes := durationSec / 60
		seconds := durationSec % 60
		sb.WriteString(fmt.Sprintf("- Duration: %d:%02d\n", minutes, seconds))
	}
	if mf.BPM > 0 {
		sb.WriteString(fmt.Sprintf("- BPM: %d\n", mf.BPM))
	}

	sb.WriteString(`
Based on your knowledge of this song (or similar songs if you don't know it specifically), classify:

1. Mood: Choose ONE from: upbeat, soft, intense, chill, calm, energetic, melancholy
2. Energy: Choose ONE from: low, medium, high
3. Vibes: Choose 1-3 from: coffeeshop, workout, party, focus, late-night, romantic, road-trip, summer

Important notes:
- "Live" versions, "Acoustic" versions, "Piano" versions are typically softer than originals
- BPM below 80 usually indicates low energy/soft mood
- BPM above 120 usually indicates high energy/upbeat mood
- Consider the artist's typical style if you know them

Return ONLY valid JSON with no additional text:
{"mood": "...", "energy": "...", "vibes": ["..."]}`)

	return sb.String()
}

// OpenAI API request/response types
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (e *OpenAIEnricher) callOpenAI(ctx context.Context, prompt string) (*OpenAIEnrichmentResult, error) {
	reqBody := openAIChatRequest{
		Model: e.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: "You are a music expert that classifies songs by mood, energy, and vibe. You respond only with valid JSON.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   100,
		Temperature: 0.3, // Low temperature for consistent classification
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	log.Trace(ctx, "Sending OpenAI enrichment request", "model", e.model)

	resp, err := e.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiResp openAIChatResponse
		if json.Unmarshal(body, &apiResp) == nil && apiResp.Error != nil {
			return nil, fmt.Errorf("OpenAI API error: %s", apiResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	var apiResp openAIChatResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)

	// Parse the JSON response
	var result OpenAIEnrichmentResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// Try to extract JSON from markdown code block if present
		if strings.Contains(content, "```") {
			content = extractJSONFromMarkdown(content)
			if err := json.Unmarshal([]byte(content), &result); err != nil {
				return nil, fmt.Errorf("failed to parse enrichment result: %w (content: %s)", err, content)
			}
		} else {
			return nil, fmt.Errorf("failed to parse enrichment result: %w (content: %s)", err, content)
		}
	}

	// Validate the result
	if err := validateEnrichmentResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(content string) string {
	// Remove ```json and ``` markers
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")
	return strings.TrimSpace(content)
}

// validateEnrichmentResult ensures the result contains valid values
func validateEnrichmentResult(result *OpenAIEnrichmentResult) error {
	validMoods := map[string]bool{
		"upbeat": true, "soft": true, "intense": true, "chill": true,
		"calm": true, "energetic": true, "melancholy": true,
	}
	validEnergy := map[string]bool{
		"low": true, "medium": true, "high": true,
	}
	validVibes := map[string]bool{
		"coffeeshop": true, "workout": true, "party": true, "focus": true,
		"late-night": true, "romantic": true, "road-trip": true, "summer": true,
	}

	result.Mood = strings.ToLower(result.Mood)
	result.Energy = strings.ToLower(result.Energy)

	if !validMoods[result.Mood] {
		return fmt.Errorf("invalid mood: %s", result.Mood)
	}
	if !validEnergy[result.Energy] {
		return fmt.Errorf("invalid energy: %s", result.Energy)
	}

	// Filter vibes to only valid ones
	var validatedVibes []string
	for _, v := range result.Vibes {
		v = strings.ToLower(v)
		if validVibes[v] {
			validatedVibes = append(validatedVibes, v)
		}
	}
	result.Vibes = validatedVibes

	return nil
}
