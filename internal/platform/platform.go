// ============================================================
// CROSS-PLATFORM FILE — This defines the Overlay interface
// that is implemented differently on each platform:
//   - Windows:   internal/windows/overlay.go (Win32 overlay)
//   - Linux:     internal/linux/overlay.go (no-op)
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
