//go:build linux

package hotkey

import (
	"context"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	linuxHKNone = iota
	linuxHKX11
	linuxHKPortal
)

type platformHotkey struct {
	mu         sync.Mutex
	registered bool
	backend    int

	// X11 (CGO)
	ctx      context.Context
	cancel   context.CancelFunc
	canceled chan struct{}

	// Wayland / XDG portal global shortcuts
	portalStop  chan struct{}
	portalDone  chan struct{}
	portalConn  *dbus.Conn
	sessionPath dbus.ObjectPath
}
