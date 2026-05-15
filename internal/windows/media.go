//go:build windows
package windows

import (
	_ "embed"
	"os"
	"os/exec"
	"syscall"
	"wis-free-v3/internal/logger"
)

var (
	user32Media = syscall.NewLazyDLL("user32.dll")
	keybd_event = user32Media.NewProc("keybd_event")
)

//go:embed check-media.ps1
var checkMediaScript []byte

const (
	KEYEVENTF_KEYUP     = 0x0002
	VK_MEDIA_PLAY_PAUSE = 0xB3
)

// sendKey sends a Windows virtual key code
func sendKey(key byte) {
	keybd_event.Call(
		uintptr(key),
		0,
		0,
		0,
	)
	keybd_event.Call(
		uintptr(key),
		0,
		uintptr(KEYEVENTF_KEYUP),
		0,
	)
}

// IsPlaying checks if media is currently playing using PowerShell SMTC query
func IsPlaying() bool {
	// Create a secure temp file for the script to prevent symlink attacks
	f, err := os.CreateTemp("", "wis_check_media_*.ps1")
	if err != nil {
		logger.Error("Failed to create secure media check script: %v", err)
		return false
	}
	scriptPath := f.Name()
	defer os.Remove(scriptPath) // Clean up immediately after execution

	if _, err := f.Write(checkMediaScript); err != nil {
		f.Close()
		logger.Error("Failed to write media check script: %v", err)
		return false
	}
	f.Close()

	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
	)

	// Hide window completely to prevent any flickering console
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | DETACHED_PROCESS
	}

	err = cmd.Run()

	// Exit code 0 = not playing, Exit code 1 = playing
	if err == nil {
		return false // Exit code 0
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() == 1
	}

	return false
}

// TogglePlayPause sends the play/pause toggle key
func TogglePlayPause() {
	sendKey(VK_MEDIA_PLAY_PAUSE)
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


