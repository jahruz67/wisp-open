//go:build linux

package hotkey

import "errors"

func (hk *Hotkey) register() error {
	return hk.registerPortal()
}

func (hk *Hotkey) unregister() error {
	hk.mu.Lock()
	if !hk.registered {
		hk.mu.Unlock()
		return errors.New("hotkey is not registered.")
	}
	hk.registered = false
	hk.mu.Unlock()
	return hk.cleanupPortal()
}
