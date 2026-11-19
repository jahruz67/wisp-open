// Package hotkey provides global keyboard shortcut detection and handling.
package hotkey

import (
	"runtime"
	"sync"

	"wis-free-v3/internal/logger"
	xhk "wis-free-v3/internal/xhotkey"
)

// Listener handles global hotkey events and triggers callbacks when the
// configured shortcut is pressed and released.
type Listener struct {
	startCallback             func()
	stopCallback              func()
	registrationErrorCallback func(error)
	registrationReadyCallback func()
	isListening               bool
	shortcut                  string
	hk                        *xhk.Hotkey
	stopModPoll               chan struct{}
	modPollDone               chan struct{}
	mu                        sync.RWMutex
	eventLoopDone             chan struct{}
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

// SetRegistrationErrorCallback sets a callback to be invoked if hotkey registration fails.
func (l *Listener) SetRegistrationErrorCallback(cb func(error)) {
	l.mu.Lock()
	l.registrationErrorCallback = cb
	l.mu.Unlock()
}

// SetRegistrationReadyCallback sets a callback invoked when Linux input polling
// is active and the configured shortcut can be monitored.
func (l *Listener) SetRegistrationReadyCallback(cb func()) {
	l.mu.Lock()
	l.registrationReadyCallback = cb
	l.mu.Unlock()
}

// UpdateShortcut changes the shortcut without stopping the listener.
// This allows hot-swapping the shortcut while the application is running.
func (l *Listener) UpdateShortcut(shortcut string) {
	l.mu.Lock()
	l.shortcut = shortcut
	wasListening := l.isListening
	doneCh := l.eventLoopDone
	pollDone := l.modPollDone
	l.eventLoopDone = nil // prevent stopListeningLocked from waiting on it
	l.modPollDone = nil
	l.stopListeningLocked()
	l.mu.Unlock()

	// Wait for the old event loop to fully exit, but WITHOUT holding the
	// mutex (otherwise the loop can't acquire the lock to signal done).
	if doneCh != nil {
		<-doneCh
	}
	if pollDone != nil {
		<-pollDone
	}

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

	if runtime.GOOS == "linux" {
		ch := make(chan struct{})
		done := make(chan struct{})
		l.stopModPoll = ch
		l.modPollDone = done
		l.isListening = true
		l.mu.Unlock()
		logger.Info("Hotkey listener started (Linux input poll), waiting for %s", l.shortcut)
		go func() {
			defer close(done)
			l.linuxInputPollLoop(key, mods, modOnly, ch)
		}()
		return
	}

	if modOnly {
		// Modifier-only Windows shortcuts use key-state polling. Linux is
		// handled by the direct input poll above.
		if runtime.GOOS != "windows" {
			logger.Error("Modifier-only shortcuts like %q are not supported on %s", l.shortcut, runtime.GOOS)
			l.mu.Unlock()
			return
		}
		ch := make(chan struct{})
		done := make(chan struct{})
		l.stopModPoll = ch
		l.modPollDone = done
		l.isListening = true
		l.mu.Unlock()
		logger.Info("Hotkey listener started (modifier poll), waiting for %s", l.shortcut)
		go func() {
			defer close(done)
			l.modifierPollLoop(mods, ch)
		}()
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
			cb := l.registrationErrorCallback
			if l.hk == hkToRegister {
				l.hk = nil
				l.isListening = false
			}
			l.mu.Unlock()
			if cb != nil {
				cb(err)
			}
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
	doneCh := l.eventLoopDone
	pollDone := l.modPollDone
	l.eventLoopDone = nil
	l.modPollDone = nil
	l.stopListeningLocked()
	l.mu.Unlock()

	// Wait for the old event loop to fully exit, but WITHOUT holding the
	// mutex (otherwise the loop can't acquire the lock to signal done).
	if doneCh != nil {
		<-doneCh
	}
	if pollDone != nil {
		<-pollDone
	}
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
	// Note: we no longer wait on eventLoopDone here — the caller does that
	// after releasing l.mu, to avoid a self-deadlock.
	l.isListening = false
}

// eventLoop runs the main keyboard event processing loop.
func (l *Listener) eventLoop(hk *xhk.Hotkey) {
	done := make(chan struct{})
	l.mu.Lock()
	l.eventLoopDone = done
	l.mu.Unlock()
	defer close(done)

	l.runEventLoop(hk.Keydown(), hk.Keyup())
}

func (l *Listener) runEventLoop(keydown, keyup <-chan xhk.Event) {
	var isRecording bool

	for {
		select {
		case _, ok := <-keydown:
			if !ok {
				return // Hotkey was unregistered
			}
			if !isRecording {
				logger.Info("Shortcut activated: starting recording")
				l.startCallback()
				isRecording = true
			} else {
				// Repeated activation while held never toggles a recording off.
				continue
			}

		case _, ok := <-keyup:
			if !ok {
				return // Hotkey was unregistered
			}
			if isRecording {
				logger.Info("Shortcut released: stopping recording")
				l.stopCallback()
				isRecording = false
			}
		}
	}
}
