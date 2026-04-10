package platform

// Overlay defines the cross-platform interface for screen overlays
type Overlay interface {
	Show(message string)
	Hide()
	SetVolume(level float64)
	Close()
}
