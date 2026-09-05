package settings

import (
	"fmt"
	"wis-free-v3/internal/config"
)

// ShowSettings displays current settings
func ShowSettings(cfg *config.Config) {
	fmt.Println("\n=== wis-free-v3 Settings ===")
	fmt.Printf("Groq API Key: %s\n", maskedKey(cfg.GroqAPIKey))
	fmt.Printf("Mistral API Key: %s\n", maskedKey(cfg.MistralAPIKey))
	fmt.Printf("Whisper Model: %s\n", cfg.WhisperModel)
	fmt.Printf("AI Model: %s\n", cfg.AIModel)
	fmt.Printf("Shortcut: %s\n", cfg.Shortcut)
	fmt.Println("===========================")
}

func maskedKey(key string) string {
	if key == "" {
		return "not set"
	}
	if len(key) <= 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}

// EditSettings provides a simple way to modify settings
func EditSettings(cfg *config.Config) error {
	// For now, just return the config
	// In a full implementation, this would open a dialog or prompt
	fmt.Println("To edit settings, modify: ~/.wis-free-v3/config.json")
	return nil
}
