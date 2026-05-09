// Package hotkey provides global keyboard shortcut detection and handling.
package hotkey

import (
	"runtime"
	"sync"
	"time"

	"wis-free-v3/internal/logger"
	xhk "wis-free-v3/internal/xhotkey"
)

// Listener handles global hotkey events and triggers callbacks when the
// configured shortcut is pressed and released.
type Listener struct {
	startCallback func()
	stopCallback  func()
	isListening   bool
	shortcut      string
	hk            *xhk.Hotkey
	stopModPoll   chan struct{}
	mu            sync.RWMutex
}

// NewListener creates a new hotkey listener with the specified shortcut and callbacks.
// The shortcut should be in format like "ctrl+k" or "alt+shift+space".
func NewListener(shortcut string, onStart, onStop func()) *Listener {
	logger.Info("Hotkey listener created: shortcut=%s", shortcut)

	return &Listener{
		startCallback: onStart,
		stopCallback:  onStop,
		shortcut:      shortcut,
	}
}

// UpdateShortcut changes the shortcut without stopping the listener.
// This allows hot-swapping the shortcut while the application is running.
func (l *Listener) UpdateShortcut(shortcut string) {
	l.mu.Lock()
	l.shortcut = shortcut
	wasListening := l.isListening
	l.stopListeningLocked()
	l.mu.Unlock()

	logger.Info("Hotkey updated: shortcut=%s", shortcut)

	if wasListening {
		l.Start()
	}
}

// Start begins listening for the configured shortcut in a background goroutine.
func (l *Listener) Start() {
	l.mu.Lock()
	if l.isListening {
		l.mu.Unlock()
		return
	}

	key, mods, modOnly, ok := ParseShortcut(l.shortcut)
	if !ok {
		logger.Error("Failed to parse shortcut: %s", l.shortcut)
		l.mu.Unlock()
		return
	}

	if modOnly {
		if runtime.GOOS != "windows" {
			logger.Error("Modifier-only shortcuts like %q are only supported on Windows", l.shortcut)
			l.mu.Unlock()
			return
		}
		ch := make(chan struct{})
		l.stopModPoll = ch
		l.isListening = true
		l.mu.Unlock()
		logger.Info("Hotkey listener started (modifier poll), waiting for %s", l.shortcut)
		go l.modifierPollLoop(mods, ch)
		return
	}

	l.hk = xhk.New(mods, key)
	l.isListening = true
	hkToRegister := l.hk
	shortcutToRegister := l.shortcut
	l.mu.Unlock()

	go func() {
		if err := hkToRegister.Register(); err != nil {
			logger.Error("Failed to register hotkey %s: %v", shortcutToRegister, err)
			l.mu.Lock()
			if l.hk == hkToRegister {
				l.hk = nil
				l.isListening = false
			}
			l.mu.Unlock()
			return
		}

		l.mu.Lock()
		if l.hk != hkToRegister {
			// Was stopped or updated while we were registering
			l.mu.Unlock()
			hkToRegister.Unregister()
			return
		}
		l.mu.Unlock()

		logger.Info("Hotkey listener started, waiting for %s", shortcutToRegister)
		go l.eventLoop(hkToRegister)
	}()
}

// Stop terminates the hotkey listener.
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stopListeningLocked()
	logger.Info("Hotkey listener stopped")
}

func (l *Listener) stopListeningLocked() {
	if l.stopModPoll != nil {
		close(l.stopModPoll)
		l.stopModPoll = nil
	}
	if l.hk != nil {
		if err := l.hk.Unregister(); err != nil {
			logger.Error("Failed to unregister hotkey: %v", err)
		}
		l.hk = nil
	}
	l.isListening = false
}

// eventLoop runs the main keyboard event processing loop.
func (l *Listener) eventLoop(hk *xhk.Hotkey) {
	var isRecording bool

	for {
		select {
		case _, ok := <-hk.Keydown():
			if !ok {
				return // Hotkey was unregistered
			}
			if !isRecording {
				logger.Info("Shortcut activated: starting recording")
				go l.startCallback()
				isRecording = true
			} else {
				// We received a second Keydown while already recording.
				// If Wayland doesn't send Deactivated (Keyup), this acts as a Toggle fallback.
				select {
				case <-hk.Keyup():
					// Out-of-order X11 auto-repeat. Ignore both.
				case <-time.After(20 * time.Millisecond):
					// Genuine second press. Toggle off.
					logger.Info("Shortcut activated again: stopping recording (Wayland toggle fallback)")
					go l.stopCallback()
					isRecording = false
				}
			}

		case _, ok := <-hk.Keyup():
			if !ok {
				return // Hotkey was unregistered
			}
			if isRecording {
				// X11 AutoRepeat Debounce logic
				// Wait a tiny fraction of a second. If we get a Keydown during this window,
				// it's an auto-repeat from holding the key, so we ignore both the Keyup and Keydown.
				select {
				case _, downOk := <-hk.Keydown():
					if !downOk {
						return
					}
					// AutoRepeat detected, skip this release!
				case <-time.After(50 * time.Millisecond):
					// Key was genuinely physically released
					logger.Info("Shortcut released: stopping recording")
					go l.stopCallback()
					isRecording = false
				}
			}
		}
	}
}
