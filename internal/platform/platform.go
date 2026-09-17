// ============================================================
// CROSS-PLATFORM FILE — This defines the Overlay interface
// that is implemented differently on each platform:
//   - Windows:   internal/windows/overlay.go (Win32 overlay)
//   - Linux:     internal/linux/overlay.go (no-op)
//   - macOS:     internal/darwin/native_darwin.m (AppKit overlay)
// This file itself is compiled on ALL platforms.
// ============================================================

package platform

// Overlay defines the cross-platform interface for screen overlays
type Overlay interface {
	Show(message string)
	Hide()
	SetVolume(level float64)
	Close()
}

// PermissionState is a stable value exposed to the settings frontend.
type PermissionState string

const (
	PermissionNotDetermined PermissionState = "not_determined"
	PermissionGranted       PermissionState = "granted"
	PermissionDenied        PermissionState = "denied"
	PermissionRestricted    PermissionState = "restricted"
	PermissionUnavailable   PermissionState = "unavailable"
)

// Status describes platform-specific setup requirements.
type Status struct {
	OS            string          `json:"os"`
	Architecture  string          `json:"architecture"`
	SetupRequired bool            `json:"setup_required"`
	Microphone    PermissionState `json:"microphone"`
	Accessibility PermissionState `json:"accessibility"`
}
