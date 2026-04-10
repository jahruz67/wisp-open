//go:build linux
package linux

// linuxOverlay provides a simple dummy implementation for Linux.
// It relies on tray icon status updates for visual feedback instead of raw drawing.
type linuxOverlay struct{}

func NewOverlay() *linuxOverlay {
	return &linuxOverlay{}
}

func (o *linuxOverlay) Show(message string)   {}
func (o *linuxOverlay) Hide()                 {}
func (o *linuxOverlay) SetVolume(level float64) {}
func (o *linuxOverlay) Close()                {}
