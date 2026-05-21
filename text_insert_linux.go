//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	a.releaseLinuxInputFocus()

	if isASCII(text) {
		if err := typeLinuxTextWithYdotool(text); err == nil {
			logger.Info("Typed transcription on Linux using ydotool (%d chars)", utf8.RuneCountInString(text))
			return
		} else {
			logger.Error("Linux auto-type unavailable via ydotool: %v", err)
		}
	} else {
		logger.Info("Skipping ydotool direct typing fallback for non-ASCII transcript")
	}

	if err := wailsruntime.ClipboardSetText(a.ctx, text); err != nil {
		logger.Error("Failed to copy transcription to clipboard: %v", err)
	} else {
		waitForLinuxClipboardText(a.ctx, text)
		if err := pasteLinuxClipboardWithYdotool(); err == nil {
			logger.Info("Pasted transcription on Linux using clipboard + ydotool (%d chars)", utf8.RuneCountInString(text))
			return
		} else {
			logger.Error("Linux auto-paste unavailable via ydotool: %v", err)
		}
	}

	logger.Info("Copied transcription to clipboard; install ydotool with ydotoold/uinput access for automatic paste on GNOME Wayland")
	if a.overlay != nil {
		a.overlay.Show("Copied transcript. Press Ctrl+V to paste.")
	}
}

func (a *App) releaseLinuxInputFocus() {
	if a.ctx == nil {
		time.Sleep(100 * time.Millisecond)
		return
	}
	wailsruntime.WindowHide(a.ctx)
	time.Sleep(150 * time.Millisecond)
}

func isASCII(text string) bool {
	for _, r := range text {
		if r > 127 {
			return false
		}
	}
	return true
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

func pasteLinuxClipboardWithYdotool() error {
	time.Sleep(150 * time.Millisecond)

	path, socketPath, err := getYdotoolCommand()
	if err != nil {
		return err
	}

	return runLinuxInputCommand(path, []string{"key", "-d", "20", "29:1", "47:1", "47:0", "29:0"}, "", 3*time.Second, socketPath)
}

func typeLinuxTextWithYdotool(text string) error {
	path, socketPath, err := getYdotoolCommand()
	if err != nil {
		return err
	}

	fastArgs := []string{"type", "-d", "1", "--file", "-"}
	if err := runLinuxInputCommand(path, fastArgs, text, linuxTextTyperTimeout(text), socketPath); err == nil {
		return nil
	}

	return runLinuxInputCommand(path, []string{"type", "--file", "-"}, text, linuxTextTyperTimeout(text), socketPath)
}

func getYdotoolCommand() (string, string, error) {
	path, err := exec.LookPath("ydotool")
	if err != nil {
		return "", "", errors.New("ydotool not found; install ydotool and start the ydotool user service")
	}

	socketPath, err := getYdotoolSocketPath()
	if err != nil {
		return "", "", err
	}

	return path, socketPath, nil
}

func linuxYdotoolStatus() map[string]interface{} {
	status := map[string]interface{}{
		"ready":       false,
		"installed":   false,
		"socket":      false,
		"socket_path": "",
		"message":     "",
		"setup_commands": []string{
			"sudo dnf install ydotool",
			"echo 'KERNEL==\"uinput\", SUBSYSTEM==\"misc\", TAG+=\"uaccess\", OPTIONS+=\"static_node=uinput\"' | sudo tee /etc/udev/rules.d/80-uinput.rules",
			"sudo udevadm control --reload-rules && sudo udevadm trigger",
			"systemctl --user enable --now ydotool.service",
			"# Restart your computer, then open WIS Free V3 again.",
		},
	}

	path, err := exec.LookPath("ydotool")
	if err != nil {
		status["message"] = "ydotool is not installed."
		return status
	}
	status["installed"] = true

	socketPath, err := getYdotoolSocketPath()
	if err != nil {
		status["message"] = err.Error()
		return status
	}
	status["socket"] = true
	status["socket_path"] = socketPath

	if err := runLinuxInputCommand(path, []string{"key", "-d", "1", "0"}, "", 800*time.Millisecond, socketPath); err != nil {
		status["message"] = "ydotool is installed, but the daemon test failed: " + err.Error()
		return status
	}

	status["ready"] = true
	status["message"] = "ydotool is ready for automatic paste."
	return status
}

func getYdotoolSocketPath() (string, error) {
	if socketPath := strings.TrimSpace(os.Getenv("YDOTOOL_SOCKET")); socketPath != "" {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}
		return "", fmt.Errorf("YDOTOOL_SOCKET is set but not accessible: %s", socketPath)
	}

	socketPath := filepath.Join("/run/user", fmt.Sprintf("%d", os.Getuid()), ".ydotool_socket")
	if _, err := os.Stat(socketPath); err != nil {
		return "", fmt.Errorf("ydotoold socket not found at %s; run `systemctl --user start ydotool.service` after configuring /dev/uinput permissions", socketPath)
	}

	return socketPath, nil
}

func runLinuxInputCommand(path string, args []string, stdin string, timeout time.Duration, socketPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "YDOTOOL_SOCKET="+socketPath)
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
