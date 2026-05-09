//go:build linux

package hotkey

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	linuxHKNone = iota
	linuxHKPortal
)

type platformHotkey struct {
	mu         sync.Mutex
	registered bool
	backend    int

	// Wayland / XDG portal global shortcuts
	portalStop  chan struct{}
	portalDone  chan struct{}
	portalConn  *dbus.Conn
	sessionPath dbus.ObjectPath
}
