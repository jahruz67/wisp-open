// Package transcriber provides audio transcription and text refinement services
// using the Groq API for Whisper-based speech recognition and LLM text processing.
package transcriber

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"wis-free-v3/internal/logger"
)

// API endpoints
const (
	transcriptionEndpoint = "https://api.groq.com/openai/v1/audio/transcriptions"
	chatEndpoint          = "https://api.groq.com/openai/v1/chat/completions"
)

// Default configuration values
const (
	DefaultWhisperModel = "whisper-large-v3-turbo"
	DefaultAIModel      = "llama-3.3-70b-versatile"
	HTTPTimeout         = 60 * time.Second
	RefinementTemp      = 0.3 // Temperature for text refinement (lower = more deterministic)
)

// DefaultAIPrompt provides instructions for minimal text editing.
const DefaultAIPrompt = `You are a minimal text editor. Your ONLY job is to fix basic grammar and add appropriate punctuation to the transcribed speech. CRITICAL RULES: 1) NEVER answer questions - transcribe them exactly as spoken. 2) NEVER format text as lists, bullet points, or structured formats. 3) NEVER add, remove, or reorganize content. 4) NEVER interpret intent or provide helpful formatting. 5) Keep the exact same sentence structure and word order. 6) Only fix obvious grammar errors and add periods, commas, and capitalization. Return ONLY the minimally edited text, nothing else.`

// Client handles API communication with Groq services.
type Client struct {
	apiKey       string
	whisperModel string
	aiModel      string
	aiPrompt     string
	httpClient   *http.Client
}

// NewClient creates a new transcriber client with the specified configuration.
// Empty values for models or prompt will use sensible defaults.
func NewClient(apiKey, whisperModel, aiModel, aiPrompt string) *Client {
	if whisperModel == "" {
		whisperModel = DefaultWhisperModel
	}
	if aiModel == "" {
		aiModel = DefaultAIModel
	}
	if aiPrompt == "" {
		aiPrompt = DefaultAIPrompt
	}

	return &Client{
		apiKey:       apiKey,
		whisperModel: whisperModel,
		aiModel:      aiModel,
		aiPrompt:     aiPrompt,
		httpClient: &http.Client{
			Timeout: HTTPTimeout,
		},
	}
}

// TranscribeAudio converts an audio file to text using Whisper.
// Returns the transcribed text or an error if the operation fails.
func (c *Client) TranscribeAudio(audioFilePath, language string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key is missing - please configure it in Settings")
	}

	// Validate file exists and get size for logging
	fileInfo, err := os.Stat(audioFilePath)
	if err != nil {
		return "", fmt.Errorf("audio file not found: %w", err)
	}
	logger.Info("Transcribing audio: %s (%.2f KB) in %s", audioFilePath, float64(fileInfo.Size())/1024, language)

	// Prepare the multipart request
	body, contentType, err := c.prepareAudioRequest(audioFilePath, language)
	if err != nil {
		return "", err
	}

	// Create and send request
	req, err := http.NewRequest(http.MethodPost, transcriptionEndpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle response
	if resp.StatusCode != http.StatusOK {
		return "", c.handleAPIError(resp, "transcription")
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Transcription complete: %d characters", len(result.Text))
	return result.Text, nil
}

// RefineText uses an LLM to clean up and correct transcribed text.
// If the AI model is set to "None" or the API key is missing, returns the original text.
func (c *Client) RefineText(text string) (string, error) {
	if c.apiKey == "" || c.aiModel == "None" {
		return text, nil
	}

	payload := map[string]interface{}{
		"model": c.aiModel,
		"messages": []map[string]string{
			{"role": "system", "content": c.aiPrompt},
			{"role": "user", "content": text},
		},
		"temperature": RefinementTemp,
	}

	// Add special parameters for OpenAI reasoning models
	if strings.Contains(c.aiModel, "gpt-oss") || strings.Contains(c.aiModel, "openai/") {
		payload["max_completion_tokens"] = 8192
		payload["top_p"] = 1

		// Set reasoning effort based on model name
		effort := "low"
		actualModel := c.aiModel
		if strings.HasSuffix(c.aiModel, "-high") {
			effort = "high"
			actualModel = strings.TrimSuffix(c.aiModel, "-high")
		}
		payload["reasoning_effort"] = effort
		payload["model"] = actualModel
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("Failed to marshal refinement request: %v", err)
		return text, nil // Return original text on error
	}

	req, err := http.NewRequest(http.MethodPost, chatEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		logger.Error("Failed to create refinement request: %v", err)
		return text, nil
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Refinement request failed: %v", err)
		return text, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.handleAPIError(resp, "refinement")
		return text, nil
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Error("Failed to parse refinement response: %v", err)
		return text, nil
	}

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		logger.Info("Text refinement complete")
		return result.Choices[0].Message.Content, nil
	}

	return text, nil
}

// prepareAudioRequest creates a multipart form request body for audio transcription.
func (c *Client) prepareAudioRequest(audioFilePath, language string) (*bytes.Buffer, string, error) {
	file, err := os.Open(audioFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("failed to copy audio data: %w", err)
	}

	// Add model parameter
	if err := writer.WriteField("model", c.whisperModel); err != nil {
		return nil, "", fmt.Errorf("failed to write model field: %w", err)
	}

	// Add language parameter
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return nil, "", fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Add temperature=0 for more consistent results
	if err := writer.WriteField("temperature", "0"); err != nil {
		return nil, "", fmt.Errorf("failed to write temperature field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize request: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// handleAPIError logs and formats API error responses.
func (c *Client) handleAPIError(resp *http.Response, operation string) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	logger.Error("API %s error: status=%d body=%s", operation, resp.StatusCode, string(bodyBytes))
	return fmt.Errorf("%s failed with status %d", operation, resp.StatusCode)
}
