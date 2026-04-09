// Package hotkey provides global keyboard shortcut detection and handling.
// It uses the gohook library to capture system-wide key events.
package hotkey

import (
	"sync"

	"wis-free-v3/internal/logger"

	hook "github.com/robotn/gohook"
)

// Listener handles global hotkey events and triggers callbacks when the
// configured shortcut is pressed and released.
type Listener struct {
	startCallback func()
	stopCallback  func()
	isListening   bool
	stopChan      chan struct{}
	triggerKeys   []uint16
	modifiers     [][]uint16
	mu            sync.RWMutex
}

// NewListener creates a new hotkey listener with the specified shortcut and callbacks.
// The shortcut should be in format like "ctrl+k" or "alt+shift+space".
// onStart is called when the shortcut is pressed, onStop when it's released.
func NewListener(shortcut string, onStart, onStop func()) *Listener {
	trigger, mods := ParseShortcut(shortcut)

	logger.Info("Hotkey listener created: shortcut=%s, trigger=%d, modifiers=%v",
		shortcut, trigger, mods)

	return &Listener{
		startCallback: onStart,
		stopCallback:  onStop,
		stopChan:      make(chan struct{}),
		triggerKeys:   trigger,
		modifiers:     mods,
	}
}

// UpdateShortcut changes the shortcut without stopping the listener.
// This allows hot-swapping the shortcut while the application is running.
func (l *Listener) UpdateShortcut(shortcut string) {
	trigger, mods := ParseShortcut(shortcut)

	l.mu.Lock()
	l.triggerKeys = trigger
	l.modifiers = mods
	l.mu.Unlock()

	logger.Info("Hotkey updated: shortcut=%s, trigger=%d", shortcut, trigger)
}

// Start begins listening for the configured shortcut in a background goroutine.
// It's safe to call Start multiple times; subsequent calls are ignored.
func (l *Listener) Start() {
	if l.isListening {
		return
	}
	l.isListening = true

	go l.eventLoop()
}

// Stop terminates the hotkey listener.
// It's safe to call Stop multiple times.
func (l *Listener) Stop() {
	if !l.isListening {
		return
	}
	l.isListening = false
	close(l.stopChan)
	logger.Info("Hotkey listener stopped")
}

// eventLoop runs the main keyboard event processing loop.
func (l *Listener) eventLoop() {
	logger.Info("Hotkey listener started")

	evChan := hook.Start()
	defer hook.End()

	var isRecording bool
	pressedKeys := make(map[uint16]bool)

	for {
		select {
		case <-l.stopChan:
			return

		case ev := <-evChan:
			l.handleKeyEvent(ev, pressedKeys, &isRecording)
		}
	}
}

// handleKeyEvent processes a single keyboard event.
func (l *Listener) handleKeyEvent(ev hook.Event, pressedKeys map[uint16]bool, isRecording *bool) {
	// Update pressed keys state
	switch ev.Kind {
	case hook.KeyDown, hook.KeyHold:
		pressedKeys[ev.Rawcode] = true
	case hook.KeyUp:
		delete(pressedKeys, ev.Rawcode)
	default:
		return
	}

	// Read current configuration
	l.mu.RLock()
	triggerVariants := l.triggerKeys
	modGroups := l.modifiers
	l.mu.RUnlock()

	if len(triggerVariants) == 0 {
		return
	}

	// 1. Check trigger key first (fast path)
	triggerPressed := false
	for _, t := range triggerVariants {
		if pressedKeys[t] {
			triggerPressed = true
			break
		}
	}

	// 2. State transition handling
	active := false
	if triggerPressed {
		active = true
		for _, group := range modGroups {
			groupPressed := false
			for _, m := range group {
				if pressedKeys[m] {
					groupPressed = true
					break
				}
			}
			if !groupPressed {
				active = false
				break
			}
		}
	}

	if active && !*isRecording {
		logger.Info("Shortcut activated: starting recording")
		go l.startCallback()
		*isRecording = true
	} else if !active && *isRecording {
		logger.Info("Shortcut released: stopping recording")
		go l.stopCallback()
		*isRecording = false
	}
}


