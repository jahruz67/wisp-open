//go:build !linux

// ============================================================
// WINDOWS-ONLY FILE — This file compiles on Windows (and macOS)
// but NOT on Linux. It uses robotgo for clipboard/paste which
// is Windows-specific in this app. The Linux equivalent is
// text_insert_linux.go which uses ydotool instead.
// ============================================================

package main

import (
	"time"

	"wis-free-v3/internal/logger"

	"github.com/go-vgo/robotgo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	// Save old clipboard
	oldClip, clipErr := wailsruntime.ClipboardGetText(a.ctx)

	// Copy to clipboard
	wailsruntime.ClipboardSetText(a.ctx, text)

	// Paste
	a.pasteText()

	// Restore old clipboard after a delay, but ONLY if the clipboard still
	// contains our transcribed text (i.e. user hasn't copied something else).
	if clipErr == nil && oldClip != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			current, currentErr := wailsruntime.ClipboardGetText(a.ctx)
			// Only restore if no error reading AND clipboard still holds our text
			// AND it hasn't been modified by another goroutine
			if currentErr == nil && current == text {
				wailsruntime.ClipboardSetText(a.ctx, oldClip)
				logger.Info("Clipboard history restored")
			}
		}()
	}
}

// pasteText simulates Ctrl+V to paste from clipboard
func (a *App) pasteText() {
	time.Sleep(50 * time.Millisecond)
	robotgo.KeyTap("v", "ctrl")
}
