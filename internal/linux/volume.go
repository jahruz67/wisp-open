//go:build linux

package linux

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"wis-free-v3/internal/logger"
)

func pactl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pactl", args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd.Output()
}

// MuteOutput mutes the output without changing its volume setting and returns a
// function that restores its previous mute state, including already-muted audio.
func MuteOutput() func() {
	out, err := pactl("get-default-sink")
	if err != nil {
		logger.Error("Failed to find audio output: %v", err)
		return nil
	}
	sink := strings.TrimSpace(string(out))
	out, err = pactl("get-sink-mute", sink)
	if err != nil {
		logger.Error("Failed to read output mute state: %v", err)
		return nil
	}
	var original string
	switch strings.TrimSpace(string(out)) {
	case "Mute: yes":
		original = "1"
	case "Mute: no":
		original = "0"
	default:
		logger.Error("Unrecognized output mute state: %q", strings.TrimSpace(string(out)))
		return nil
	}
	if _, err := pactl("set-sink-mute", sink, "1"); err != nil {
		logger.Error("Failed to mute output: %v", err)
		if _, restoreErr := pactl("set-sink-mute", sink, original); restoreErr != nil {
			logger.Error("Failed to restore output mute state: %v", restoreErr)
		}
		return nil
	}
	return func() {
		if _, err := pactl("set-sink-mute", sink, original); err != nil {
			logger.Error("Failed to restore output mute state: %v", err)
		}
	}
}
