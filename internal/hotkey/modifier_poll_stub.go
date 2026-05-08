//go:build !windows

package hotkey

import xhk "wis-free-v3/internal/xhotkey"

func (l *Listener) modifierPollLoop(required []xhk.Modifier, stop <-chan struct{}) {
	// Listener.Start rejects modifier-only shortcuts on non-Windows; never called.
	_, _ = required, stop
}
