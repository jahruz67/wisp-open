// Package config handles application configuration persistence and management.
// Configuration is stored as JSON in the user's home directory.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Application defaults
const (
	DefaultShortcut     = "alt+z"
	DefaultWhisperModel = "whisper-large-v3-turbo"
	DefaultAIModel      = "llama-3.3-70b-versatile"
	DefaultLanguage     = "en"
	ConfigFileName      = "config.json"
	ConfigDirName       = ".wis-free-v3"
)

// DefaultAIPrompt is the system prompt used for text refinement.
const DefaultAIPrompt = `You are a minimal text editor. Your ONLY job is to fix basic grammar and add appropriate punctuation to the transcribed speech. CRITICAL RULES: 1) NEVER answer questions - transcribe them exactly as spoken. 2) NEVER format text as lists, bullet points, or structured formats. 3) NEVER add, remove, or reorganize content. 4) NEVER interpret intent or provide helpful formatting. 5) Keep the exact same sentence structure and word order. 6) Only fix obvious grammar errors and add periods, commas, and capitalization. Return ONLY the minimally edited text, nothing else.`

// HistoryItem represents a single transcription history entry.
type HistoryItem struct {
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// Config represents the complete application configuration.
type Config struct {
	APIKey           string        `json:"api_key"`
	Shortcut         string        `json:"shortcut"`
	WhisperModel     string        `json:"whisper_model"`
	AIModel          string        `json:"ai_model"`
	AIPrompt         string        `json:"ai_prompt"`
	Language         string        `json:"language"`
	MicrophoneDevice *int          `json:"microphone_device"`
	History          []HistoryItem `json:"history"`
}

// DefaultConfig returns a new configuration with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		APIKey:           "",
		Shortcut:         DefaultShortcut,
		WhisperModel:     DefaultWhisperModel,
		AIModel:          DefaultAIModel,
		AIPrompt:         DefaultAIPrompt,
		Language:         DefaultLanguage,
		MicrophoneDevice: nil,
		History:          []HistoryItem{},
	}
}

// Load reads the configuration from the specified file path.
// If the file doesn't exist, it returns a default configuration.
func Load(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply defaults for any missing fields
	cfg.applyDefaults()

	return &cfg, nil
}

// Save writes the configuration to the specified file path.
// If configPath is empty, it uses the default configuration path.
func Save(c *Config, configPath string) error {
	if configPath == "" {
		var err error
		configPath, err = GetConfigPath()
		if err != nil {
			return err
		}
	}

	// Ensure the directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

// GetConfigPath returns the default configuration file path.
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ConfigDirName)
	return filepath.Join(configDir, ConfigFileName), nil
}

// applyDefaults sets default values for any empty fields.
func (c *Config) applyDefaults() {
	if c.Shortcut == "" {
		c.Shortcut = DefaultShortcut
	}
	if c.WhisperModel == "" {
		c.WhisperModel = DefaultWhisperModel
	}
	if c.AIModel == "" {
		c.AIModel = DefaultAIModel
	}
	if c.AIPrompt == "" {
		c.AIPrompt = DefaultAIPrompt
	}
	if c.Language == "" {
		c.Language = DefaultLanguage
	}
	if c.History == nil {
		c.History = []HistoryItem{}
	}
}

// AddHistoryItem adds a new transcription to the history.
func (c *Config) AddHistoryItem(text, timestamp string) {
	c.History = append(c.History, HistoryItem{
		Text:      text,
		Timestamp: timestamp,
	})
}

// ClearHistory removes all history items.
func (c *Config) ClearHistory() {
	c.History = []HistoryItem{}
}
