//go:build !linux

package hotkey

import xhk "wis-free-v3/internal/xhotkey"

func (l *Listener) linuxInputPollLoop(key xhk.Key, required []xhk.Modifier, modifierOnly bool, stop <-chan struct{}) {
	_, _, _, _ = key, required, modifierOnly, stop
}
