//go:build darwin

package platform

import (
	"runtime"

	mac "wis-free-v3/internal/darwin"
)

func NewOverlay() Overlay { return mac.NewOverlay() }

func PauseMedia() bool { return mac.PauseMedia() }

func ResumeMedia(wasPaused bool) { mac.ResumeMedia(wasPaused) }

func AddToStartup() error { return mac.AddToStartup() }

func RemoveFromStartup() error { return mac.RemoveFromStartup() }

func IsInStartup() bool { return mac.IsInStartup() }

func IsProcessRunning(pid int) bool { return mac.IsProcessRunning(pid) }

func EnsureDesktopFile(iconBytes []byte) error { return nil }

func GetStatus() Status {
	mic := PermissionState(mac.MicrophonePermission())
	accessibility := PermissionState(mac.AccessibilityPermission())
	return Status{
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		SetupRequired: mic != PermissionGranted || accessibility != PermissionGranted,
		Microphone:    mic,
		Accessibility: accessibility,
	}
}

func RequestPermission(kind string) error { return mac.RequestPermission(kind) }

func OpenPermissionSettings(kind string) error { return mac.OpenPermissionSettings(kind) }

func SetSettingsWindowVisible(visible bool) { mac.SetSettingsWindowVisible(visible) }
