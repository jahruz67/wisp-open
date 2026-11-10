//go:build linux

package linux

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"wis-free-v3/internal/logger"
)

var sinkVolumePattern = regexp.MustCompile(`:\s+(\d+)\s*/`)

func pactl(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pactl", args...).Output()
}

// DuckMedia uses PulseAudio (also supported by PipeWire) to lower all output
// channels to 1% of their current levels without changing playback or mute state.
func DuckMedia() func() {
	out, err := pactl("get-default-sink")
	if err != nil {
		logger.Error("Failed to find audio output: %v", err)
		return nil
	}
	sink := strings.TrimSpace(string(out))
	out, err = pactl("get-sink-volume", sink)
	if err != nil {
		logger.Error("Failed to read output volume: %v", err)
		return nil
	}
	channels := sinkVolumePattern.FindAllStringSubmatch(string(out), -1)
	if len(channels) == 0 {
		logger.Error("No output channel volumes found")
		return nil
	}
	original := []string{"set-sink-volume", sink}
	quiet := []string{"set-sink-volume", sink}
	for _, channel := range channels {
		value, err := strconv.ParseUint(channel[1], 10, 32)
		if err != nil {
			logger.Error("Invalid output volume: %v", err)
			return nil
		}
		original = append(original, channel[1])
		quiet = append(quiet, strconv.FormatUint(value/100, 10))
	}
	if _, err := pactl(quiet...); err != nil {
		logger.Error("Failed to lower output volume: %v", err)
		if _, restoreErr := pactl(original...); restoreErr != nil {
			logger.Error("Failed to restore output volume: %v", restoreErr)
		}
		return nil
	}
	return func() {
		if _, err := pactl(original...); err != nil {
			logger.Error("Failed to restore output volume: %v", err)
		}
	}
}
