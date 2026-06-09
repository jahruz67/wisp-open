//go:build windows

// ============================================================
// WINDOWS-ONLY FILE — Delegates all platform operations to the
// internal/windows package (overlay, media, startup, process).
// The Linux equivalent is platform_linux.go
// ============================================================

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

func EnsureDesktopFile(iconBytes []byte) error {
	// Not applicable on Windows
	return nil
}
