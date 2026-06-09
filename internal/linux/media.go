//go:build linux

// ============================================================
// LINUX-ONLY FILE — Linux media playback management.
// Uses playerctl to pause/resume media during recording.
// The Windows equivalent is in internal/windows/
// ============================================================

package linux

import (
	"os/exec"
	"strings"

	"wis-free-v3/internal/logger"
)

// IsPlaying checks if media is currently playing using playerctl
func IsPlaying() bool {
	cmd := exec.Command("playerctl", "status")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "playing")
}

// TogglePlayPause sends the play-pause command via playerctl
func TogglePlayPause() {
	cmd := exec.Command("playerctl", "play-pause")
	err := cmd.Run()
	if err != nil {
		logger.Error("Failed to toggle media on Linux: %v", err)
	}
}

// PauseMedia checks if playing, pauses if so, returns whether we paused
func PauseMedia() bool {
	wasPlaying := IsPlaying()
	if wasPlaying {
		TogglePlayPause()
	}
	return wasPlaying
}

// ResumeMedia resumes only if we paused it
func ResumeMedia(wasPaused bool) {
	if wasPaused {
		TogglePlayPause()
	}
}
