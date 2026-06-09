//go:build linux

// ============================================================
// LINUX-ONLY FILE — Delegates all platform operations to the
// internal/linux package (overlay, media, startup, process).
// The Windows equivalent is platform_windows.go
// ============================================================

package platform

import "wis-free-v3/internal/linux"

func NewOverlay() Overlay {
	return linux.NewOverlay()
}

func PauseMedia() bool {
	return linux.PauseMedia()
}

func ResumeMedia(wasPaused bool) {
	linux.ResumeMedia(wasPaused)
}

func AddToStartup() error {
	return linux.AddToStartup()
}

func RemoveFromStartup() error {
	return linux.RemoveFromStartup()
}

func IsInStartup() bool {
	return linux.IsInStartup()
}

func IsProcessRunning(pid int) bool {
	return linux.IsProcessRunning(pid)
}

func EnsureDesktopFile(iconBytes []byte) error {
	return linux.EnsureDesktopFile(iconBytes)
}
