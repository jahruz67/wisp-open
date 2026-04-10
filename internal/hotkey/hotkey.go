// Package hotkey provides global keyboard shortcut detection and handling.
package hotkey

import (
	"sync"
	"time"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/xhotkey"
)

// Listener handles global hotkey events and triggers callbacks when the
// configured shortcut is pressed and released.
type Listener struct {
	startCallback func()
	stopCallback  func()
	isListening   bool
	shortcut      string
	hk            *hotkey.Hotkey
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

	if l.hk != nil {
		l.hk.Unregister()
		l.hk = nil
		l.isListening = false
	}
	l.mu.Unlock()

	logger.Info("Hotkey updated: shortcut=%s", shortcut)

	if wasListening {
		l.Start()
	}
}

// Start begins listening for the configured shortcut in a background goroutine.
func (l *Listener) Start() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.hk != nil {
		return
	}

	key, mods, ok := ParseShortcut(l.shortcut)
	if !ok {
		logger.Error("Failed to parse shortcut: %s", l.shortcut)
		return
	}

	l.hk = hotkey.New(mods, key)
	if err := l.hk.Register(); err != nil {
		logger.Error("Failed to register hotkey %s: %v", l.shortcut, err)
		l.hk = nil
		return
	}

	l.isListening = true
	logger.Info("Hotkey listener started, waiting for %s", l.shortcut)

	go l.eventLoop(l.hk)
}

// Stop terminates the hotkey listener.
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.hk == nil {
		return
	}

	err := l.hk.Unregister()
	if err != nil {
		logger.Error("Failed to unregister hotkey: %v", err)
	}

	l.hk = nil
	l.isListening = false
	logger.Info("Hotkey listener stopped")
}

// eventLoop runs the main keyboard event processing loop.
func (l *Listener) eventLoop(hk *hotkey.Hotkey) {
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
