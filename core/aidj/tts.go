package aidj

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/navidrome/navidrome/log"
)

const (
	openAITTSURL = "https://api.openai.com/v1/audio/speech"
	ttsModel     = "tts-1"
	ttsVoice     = "onyx" // Deep, authoritative - radio broadcaster style
)

// TTSClient generates speech audio from text using OpenAI's TTS API
type TTSClient struct {
	apiKey string
	hc     httpDoer
}

// NewTTSClient creates a new TTS client
func NewTTSClient(apiKey string, hc httpDoer) *TTSClient {
	return &TTSClient{
		apiKey: apiKey,
		hc:     hc,
	}
}

// ttsRequest represents the OpenAI TTS API request body
type ttsRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}

// GenerateSpeech converts text to speech and returns MP3 audio data
func (c *TTSClient) GenerateSpeech(ctx context.Context, text string) ([]byte, error) {
	reqBody := ttsRequest{
		Model:          ttsModel,
		Input:          text,
		Voice:          ttsVoice,
		ResponseFormat: "mp3",
		Speed:          1.0,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAITTSURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	log.Debug(ctx, "Generating TTS audio", "textLength", len(text))

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API returned status %d: %s", resp.StatusCode, string(body))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TTS response: %w", err)
	}

	log.Debug(ctx, "TTS audio generated", "audioSize", len(audio))
	return audio, nil
}
