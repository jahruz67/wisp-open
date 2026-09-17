//go:build darwin

package main

import (
	"time"

	mac "wis-free-v3/internal/darwin"
	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	oldClip, clipErr := wailsruntime.ClipboardGetText(a.ctx)
	wailsruntime.ClipboardSetText(a.ctx, text)
	time.Sleep(50 * time.Millisecond)

	if err := mac.Paste(); err != nil {
		logger.Error("macOS direct paste failed: %v; transcription remains on clipboard", err)
		wailsruntime.EventsEmit(a.ctx, "platform:paste-error", err.Error())
		return
	}

	if clipErr == nil && oldClip != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			current, currentErr := wailsruntime.ClipboardGetText(a.ctx)
			if currentErr == nil && current == text {
				wailsruntime.ClipboardSetText(a.ctx, oldClip)
				logger.Info("Clipboard history restored")
			}
		}()
	}
}

func (a *App) pasteText() {
	_ = mac.Paste()
}
