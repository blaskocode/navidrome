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

// InterpretedPreferences extends UserPreferences with optional hints for future use
type InterpretedPreferences struct {
	model.UserPreferences
	GenreHints  []string `json:"genreHints,omitempty"`
	ArtistHints []string `json:"artistHints,omitempty"`
	MoodHints   []string `json:"moodHints,omitempty"`
}

// PromptInterpreter interprets natural language prompts into structured preferences
type PromptInterpreter struct {
	apiKey string
	model  string
	hc     httpDoer
}

// NewPromptInterpreter creates a new prompt interpreter
func NewPromptInterpreter(apiKey, model string, hc httpDoer) *PromptInterpreter {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &PromptInterpreter{
		apiKey: apiKey,
		model:  model,
		hc:     hc,
	}
}

// Interpret converts a natural language prompt into structured preferences
func (p *PromptInterpreter) Interpret(ctx context.Context, prompt string) (*InterpretedPreferences, error) {
	systemPrompt := `You are a music preference interpreter. Given a natural language description of what music someone wants to hear, extract structured preferences.

Output ONLY valid JSON with these fields:
- energy: ONE of "upbeat", "soft", "intense", or null (if not specified or "any")
- decade: A decade year (1960, 1970, 1980, 1990, 2000, 2010, 2020) or null
- contexts: Array of applicable contexts from: "forgotten" (haven't heard in a while), "favorites" (user's favorites), "new" (never played). Can be empty array.
- genreHints: Array of genre keywords mentioned or implied (for future use)
- artistHints: Array of artist names mentioned or similar artists (for future use)
- moodHints: Array of mood/vibe keywords (for future use)

Examples:
- "soft songs like Switchfoot" -> {"energy":"soft","decade":null,"contexts":[],"genreHints":["alternative rock","acoustic"],"artistHints":["Switchfoot"],"moodHints":["emotional"]}
- "upbeat favorites from the 90s" -> {"energy":"upbeat","decade":1990,"contexts":["favorites"],"genreHints":[],"artistHints":[],"moodHints":[]}
- "something fresh I haven't heard" -> {"energy":null,"decade":null,"contexts":["new"],"genreHints":[],"artistHints":[],"moodHints":[]}
- "chill acoustic vibes" -> {"energy":"soft","decade":null,"contexts":[],"genreHints":["acoustic"],"artistHints":[],"moodHints":["chill","relaxed"]}
- "high energy workout music" -> {"energy":"intense","decade":null,"contexts":[],"genreHints":[],"artistHints":[],"moodHints":["energetic","motivating"]}`

	reqBody := openAIChatRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   200,
		Temperature: 0.3,
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
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	log.Trace(ctx, "Sending OpenAI prompt interpretation request", "model", p.model, "prompt", prompt)

	resp, err := p.hc.Do(req)
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

	// Handle markdown code blocks
	if strings.Contains(content, "```") {
		content = extractJSONFromMarkdown(content)
	}

	var result InterpretedPreferences
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse interpretation result: %w (content: %s)", err, content)
	}

	// Validate and normalize
	result.normalize()

	log.Debug(ctx, "Interpreted prompt", "prompt", prompt, "energy", result.Energy, "decade", result.Decade, "contexts", result.Contexts)

	return &result, nil
}

// normalize validates and normalizes the interpreted preferences
func (p *InterpretedPreferences) normalize() {
	// Validate energy
	if p.Energy != nil {
		validEnergy := map[string]bool{"upbeat": true, "soft": true, "intense": true}
		e := strings.ToLower(*p.Energy)
		if !validEnergy[e] {
			p.Energy = nil
		} else {
			p.Energy = &e
		}
	}

	// Validate decade
	if p.Decade != nil {
		validDecades := map[int]bool{1960: true, 1970: true, 1980: true, 1990: true, 2000: true, 2010: true, 2020: true}
		if !validDecades[*p.Decade] {
			p.Decade = nil
		}
	}

	// Validate contexts
	validContexts := map[string]bool{"forgotten": true, "favorites": true, "new": true}
	var validatedContexts []string
	for _, c := range p.Contexts {
		c = strings.ToLower(c)
		if validContexts[c] {
			validatedContexts = append(validatedContexts, c)
		}
	}
	p.Contexts = validatedContexts
}

// DefaultPreferences returns fallback preferences when interpretation fails
func DefaultPreferences() *InterpretedPreferences {
	return &InterpretedPreferences{
		UserPreferences: model.UserPreferences{
			Energy:   nil,
			Decade:   nil,
			Contexts: []string{},
		},
	}
}
