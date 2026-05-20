//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	if typer, err := typeLinuxTextWithoutXTest(text); err == nil {
		logger.Info("Typed transcription on Linux using %s (%d chars)", typer, utf8.RuneCountInString(text))
		return
	} else {
		logger.Error("Linux auto-type unavailable without XTEST: %v", err)
	}

	if err := wailsruntime.ClipboardSetText(a.ctx, text); err != nil {
		logger.Error("Failed to copy transcription to clipboard: %v", err)
		return
	}

	logger.Info("Copied transcription to clipboard; GNOME Wayland blocks automatic typing without a remote-desktop permission prompt")
	if a.overlay != nil {
		a.overlay.Show("Copied transcript. Press Ctrl+V to paste.")
	}
}

func typeLinuxTextWithoutXTest(text string) (string, error) {
	var errs []string

	if path, err := exec.LookPath("ydotool"); err == nil {
		if err := runLinuxTextTyper(path, []string{"type", "--file", "-"}, text); err == nil {
			return "ydotool", nil
		} else {
			errs = append(errs, "ydotool: "+err.Error())
		}
	}

	if path, err := exec.LookPath("wtype"); err == nil {
		if err := runLinuxTextTyper(path, []string{"-"}, text); err == nil {
			return "wtype", nil
		} else {
			errs = append(errs, "wtype: "+err.Error())
		}
	}

	if len(errs) == 0 {
		return "", errors.New("install ydotool with ydotoold/uinput access for GNOME Wayland auto-typing")
	}
	return "", errors.New(strings.Join(errs, "; "))
}

func runLinuxTextTyper(path string, args []string, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), linuxTextTyperTimeout(text))
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func linuxTextTyperTimeout(text string) time.Duration {
	timeout := 5*time.Second + time.Duration(utf8.RuneCountInString(text))*30*time.Millisecond
	if timeout > 2*time.Minute {
		return 2 * time.Minute
	}
	return timeout
}
