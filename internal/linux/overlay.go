//go:build linux

package linux

// linuxOverlay intentionally does not show system notifications. Tray status
// still updates, but dictation no longer spams desktop notification bubbles.
type linuxOverlay struct{}

func NewOverlay() *linuxOverlay {
	return &linuxOverlay{}
}

func (o *linuxOverlay) Show(message string) {}

func (o *linuxOverlay) Hide() {}

func (o *linuxOverlay) SetVolume(level float64) {}

func (o *linuxOverlay) Close() {}
