//go:build linux

package linux

/*
#cgo pkg-config: gtk-layer-shell-0 gtk+-3.0 x11 xext xrandr cairo-xlib
#cgo LDFLAGS: -lm
#include <stdlib.h>
void wis_linux_overlay_show(const char *message);
void wis_linux_overlay_hide(void);
void wis_linux_overlay_set_volume(double level);
void wis_linux_overlay_close(void);
*/
import "C"

import (
	"math"
	"sync"
	"unsafe"
)

// GTK operations are queued onto Wails' existing main loop.
type linuxOverlay struct {
	mu     sync.Mutex
	closed bool
}

func NewOverlay() *linuxOverlay {
	return &linuxOverlay{}
}

func (o *linuxOverlay) Show(message string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	value := C.CString(message)
	defer C.free(unsafe.Pointer(value))
	C.wis_linux_overlay_show(value)
}

func (o *linuxOverlay) Hide() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		C.wis_linux_overlay_hide()
	}
}

func (o *linuxOverlay) SetVolume(level float64) {
	if math.IsNaN(level) || math.IsInf(level, 0) {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		C.wis_linux_overlay_set_volume(C.double(math.Max(0, math.Min(1, level))))
	}
}

func (o *linuxOverlay) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		o.closed = true
		C.wis_linux_overlay_close()
	}
}
