//go:build linux

// ============================================================
// LINUX-ONLY FILE — This file compiles ONLY on Linux.
// Any changes here will NOT affect the Windows build.
// For the Windows equivalent, see text_insert_nonlinux.go
// ============================================================

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

	if err := typeLinuxTextWithYdotool(text); err == nil {
		logger.Info("Typed transcription on Linux using ydotool (%d chars)", utf8.RuneCountInString(text))
		return
	} else {
		logger.Error("Linux direct typing unavailable via ydotool: %v", err)
	}

	logger.Info("Transcription was not inserted; install ydotool with ydotoold/uinput access for direct Linux typing")
	if a.overlay != nil {
		a.overlay.Show("Direct typing unavailable. Check ydotool setup.")
	}
}

func (a *App) releaseLinuxInputFocus() {
	if a.ctx == nil {
		time.Sleep(30 * time.Millisecond)
		return
	}
	wailsruntime.WindowHide(a.ctx)
	time.Sleep(50 * time.Millisecond)
}

func typeLinuxTextWithYdotool(text string) error {
	path, socketPath, err := getYdotoolCommand()
	if err != nil {
		return err
	}

	timeout := linuxTextTyperTimeout(text)
	// ydotool has changed option parsing across releases. Use stdin to avoid
	// shell quoting and argument-size issues, but keep a direct-argument
	// fallback for older packages whose `type --file` support is incomplete.
	attempts := [][]string{
		{"type", "-d", "1", "--file", "-"},
		{"type", "-f", "-"},
		{"type", "--file", "-"},
	}
	var lastErr error
	for _, args := range attempts {
		if err := runLinuxInputCommand(path, args, text, timeout, socketPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Do not turn a leading dash into an option. Transcriptions are normally
	// small enough for argv and exec.Command passes this value without a shell.
	if text != "" && !strings.HasPrefix(text, "-") && !strings.ContainsRune(text, '\x00') {
		if err := runLinuxInputCommand(path, []string{"type", text}, "", timeout, socketPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return lastErr
}

func getYdotoolCommand() (string, string, error) {
	path, err := exec.LookPath("ydotool")
	if err != nil {
		for _, candidate := range []string{"/usr/local/bin/ydotool", "/usr/bin/ydotool"} {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() && info.Mode()&0111 != 0 {
				path = candidate
				err = nil
				break
			}
		}
		if err != nil {
			return "", "", errors.New("ydotool not found; install ydotool and start its user service")
		}
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
			"# Install ydotool with your package manager, for example:",
			"sudo apt install ydotool    # Debian/Ubuntu",
			"sudo dnf install ydotool    # Fedora",
			"sudo pacman -S ydotool      # Arch",
			"# Distribution packages normally provide the uinput rule and service:",
			"systemctl --user enable --now ydotool.service",
			"# If the socket is still missing, log out and back in, then run:",
			"systemctl --user restart ydotool.service",
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
	status["message"] = "ydotool is ready for direct typing."
	return status
}

func getYdotoolSocketPath() (string, error) {
	if socketPath := strings.TrimSpace(os.Getenv("YDOTOOL_SOCKET")); socketPath != "" {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}
		return "", fmt.Errorf("YDOTOOL_SOCKET is set but not accessible: %s", socketPath)
	}

	candidates := make([]string, 0, 6)
	if runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); runtimeDir != "" {
		candidates = append(candidates, filepath.Join(runtimeDir, ".ydotool_socket"))
	}
	candidates = append(candidates,
		filepath.Join("/run/user", fmt.Sprintf("%d", os.Getuid()), ".ydotool_socket"),
		"/tmp/.ydotool_socket",
		"/run/ydotoold/socket",
		"/run/ydotoold/.ydotool_socket",
	)
	for _, socketPath := range candidates {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}
	}

	return "", fmt.Errorf("ydotoold socket not found; run `systemctl --user enable --now ydotool.service` and log out/in only if the service still cannot access /dev/uinput")
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
