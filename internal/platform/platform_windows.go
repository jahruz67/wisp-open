//go:build windows
package platform

import "wis-free-v3/internal/windows"

func NewOverlay() Overlay {
	return windows.NewOverlay()
}

func PauseMedia() bool {
	return windows.PauseMedia()
}

func ResumeMedia(wasPaused bool) {
	windows.ResumeMedia(wasPaused)
}

func AddToStartup() error {
	return windows.AddToStartup()
}

func RemoveFromStartup() error {
	return windows.RemoveFromStartup()
}

func IsInStartup() bool {
	return windows.IsInStartup()
}

func IsProcessRunning(pid int) bool {
	return windows.IsProcessRunning(pid)
}
