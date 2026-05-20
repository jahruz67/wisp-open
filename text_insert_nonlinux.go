//go:build !linux

package main

import (
	"time"

	"wis-free-v3/internal/logger"

	"github.com/go-vgo/robotgo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	if len(text) < 50 {
		// Type short text directly to avoid clipboard interference
		robotgo.TypeStr(text)
		return
	}

	// Save old clipboard
	oldClip, clipErr := wailsruntime.ClipboardGetText(a.ctx)

	// Copy to clipboard
	wailsruntime.ClipboardSetText(a.ctx, text)

	// Paste
	a.pasteText()

	// Restore old clipboard after a short delay, but ONLY if the user
	// hasn't manually copied something else or another burst hasn't finished.
	if clipErr == nil && oldClip != "" {
		go func() {
			time.Sleep(1000 * time.Millisecond)
			current, _ := wailsruntime.ClipboardGetText(a.ctx)
			if current == text {
				wailsruntime.ClipboardSetText(a.ctx, oldClip)
				logger.Info("Clipboard history restored")
			}
		}()
	}
}

// pasteText simulates Ctrl+V to paste from clipboard
func (a *App) pasteText() {
	time.Sleep(200 * time.Millisecond)
	robotgo.KeyTap("v", "ctrl")
}
