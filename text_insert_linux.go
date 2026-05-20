//go:build linux

package main

import (
	"unicode/utf8"

	"wis-free-v3/internal/logger"

	"github.com/go-vgo/robotgo"
)

func (a *App) insertTranscription(text string) {
	// Avoid synthetic Ctrl+V on Linux; desktop environments can interpret it
	// together with lingering shortcut modifiers and open global UI instead.
	logger.Info("Typing transcription directly on Linux (%d chars)", utf8.RuneCountInString(text))
	robotgo.TypeStr(text)
}
