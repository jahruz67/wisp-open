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
	if err := wailsruntime.ClipboardSetText(a.ctx, text); err != nil {
		logger.Error("Failed to copy transcription to clipboard: %v", err)
	} else {
		waitForLinuxClipboardText(a.ctx, text)
		if typer, err := pasteLinuxClipboardWithoutXTest(); err == nil {
			logger.Info("Pasted transcription on Linux using %s (%d chars)", typer, utf8.RuneCountInString(text))
			return
		} else {
			logger.Error("Linux auto-paste unavailable without XTEST: %v", err)
		}
	}

	if typer, err := typeLinuxTextWithoutXTest(text); err == nil {
		logger.Info("Typed transcription on Linux using %s (%d chars)", typer, utf8.RuneCountInString(text))
		return
	} else {
		logger.Error("Linux auto-type unavailable without XTEST: %v", err)
	}

	logger.Info("Copied transcription to clipboard; install ydotool with ydotoold/uinput access for automatic paste on GNOME Wayland")
	if a.overlay != nil {
		a.overlay.Show("Copied transcript. Press Ctrl+V to paste.")
	}
}

func waitForLinuxClipboardText(ctx context.Context, text string) {
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		current, err := wailsruntime.ClipboardGetText(ctx)
		if err == nil && current == text {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func pasteLinuxClipboardWithoutXTest() (string, error) {
	time.Sleep(150 * time.Millisecond)

	var errs []string

	if path, err := exec.LookPath("ydotool"); err == nil {
		if err := runLinuxInputCommand(path, []string{"key", "-d", "20", "29:1", "47:1", "47:0", "29:0"}, "", 3*time.Second); err == nil {
			return "ydotool", nil
		} else {
			errs = append(errs, "ydotool: "+err.Error())
		}
	}

	if path, err := exec.LookPath("wtype"); err == nil {
		if err := runLinuxInputCommand(path, []string{"-M", "ctrl", "v", "-m", "ctrl"}, "", 3*time.Second); err == nil {
			return "wtype", nil
		} else {
			errs = append(errs, "wtype: "+err.Error())
		}
	}

	if len(errs) == 0 {
		return "", errors.New("install ydotool with ydotoold/uinput access for GNOME Wayland auto-paste")
	}
	return "", errors.New(strings.Join(errs, "; "))
}

func typeLinuxTextWithoutXTest(text string) (string, error) {
	var errs []string

	if path, err := exec.LookPath("ydotool"); err == nil {
		if err := runLinuxInputCommand(path, []string{"type", "--file", "-"}, text, linuxTextTyperTimeout(text)); err == nil {
			return "ydotool", nil
		} else {
			errs = append(errs, "ydotool: "+err.Error())
		}
	}

	if path, err := exec.LookPath("wtype"); err == nil {
		if err := runLinuxInputCommand(path, []string{"-"}, text, linuxTextTyperTimeout(text)); err == nil {
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

func runLinuxInputCommand(path string, args []string, stdin string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
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
