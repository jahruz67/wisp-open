//go:build linux && cgo

package hotkey

import (
	"errors"
	"log"
)

func (hk *Hotkey) register() error {
	if usePortalBackend() {
		if err := hk.registerPortal(); err == nil {
			return nil
		} else {
			log.Printf("wis-free-v3: Wayland global-shortcuts portal unavailable (%v); trying X11", err)
		}
	}
	return hk.registerX11()
}

func (hk *Hotkey) unregister() error {
	hk.mu.Lock()
	if !hk.registered {
		hk.mu.Unlock()
		return errors.New("hotkey is not registered.")
	}
	switch hk.backend {
	case linuxHKPortal:
		hk.registered = false
		hk.mu.Unlock()
		return hk.cleanupPortal()
	case linuxHKX11:
		hk.cancel()
		hk.registered = false
		hk.mu.Unlock()
		<-hk.canceled
		return nil
	default:
		hk.mu.Unlock()
		return errors.New("hotkey: invalid backend state")
	}
}
