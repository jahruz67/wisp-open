//go:build linux

package hotkey

import "errors"

func (hk *Hotkey) register() error {
	return errors.New("Linux in-app global hotkeys are disabled; use the desktop custom-shortcut command shown in Settings")
}

func (hk *Hotkey) unregister() error {
	hk.mu.Lock()
	defer hk.mu.Unlock()
	if !hk.registered {
		return errors.New("hotkey is not registered.")
	}
	hk.registered = false
	return nil
}
