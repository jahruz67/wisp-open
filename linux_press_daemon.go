//go:build linux

package main

import (
	"net/http"
	"os"
	"sync"
	"time"
)

const linuxPressAddr = "127.0.0.1:9876"

// linuxPressState holds the daemon's state, protected by a mutex.
// All field access must be done while holding the mutex.
type linuxPressState struct {
	mu               sync.Mutex
	releaseTimer     *time.Timer
	detectTimer      *time.Timer
	recording        bool
	holdMode         bool
	detectingHold    bool
}

var pressState linuxPressState

const (
	// linuxPressHoldDetectWindow is the window in which a second ping indicates
	// the shortcut is being held (push-to-talk mode).
	linuxPressHoldDetectWindow = 1200 * time.Millisecond

	// linuxPressReleaseGrace is how long after the last ping to wait before
	// stopping recording in hold mode.
	linuxPressReleaseGrace = 450 * time.Millisecond
)

func init() {
	if hasPressFlag(os.Args[1:]) {
		if err := sendLinuxPressPing(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func hasPressFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--press" {
			return true
		}
	}
	return false
}

func sendLinuxPressPing() error {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get("http://" + linuxPressAddr + "/press")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return os.ErrNotExist
	}
	return nil
}

func (a *App) startLinuxPressDaemon() {
	mux := http.NewServeMux()
	mux.HandleFunc("/press", func(w http.ResponseWriter, r *http.Request) {
		a.handleLinuxPressPing()
		w.WriteHeader(http.StatusNoContent)
	})

	go func() {
		_ = http.ListenAndServe(linuxPressAddr, mux)
	}()
}

func (a *App) handleLinuxPressPing() {
	pressState.mu.Lock()
	defer pressState.mu.Unlock()

	if !pressState.recording {
		// First ping: start recording with hold detection
		pressState.recording = true
		pressState.holdMode = false
		pressState.detectingHold = true
		go a.StartRecording()

		if pressState.detectTimer != nil {
			pressState.detectTimer.Stop()
		}
		pressState.detectTimer = time.AfterFunc(linuxPressHoldDetectWindow, func() {
			pressState.mu.Lock()
			defer pressState.mu.Unlock()
			pressState.detectingHold = false
		})
		return
	}

	// Already recording. If we're still in the hold-detect window or confirmed hold mode,
	// this is a hold-mode keepalive ping (key auto-repeat).
	if pressState.detectingHold || pressState.holdMode {
		pressState.holdMode = true
		pressState.detectingHold = false
		if pressState.detectTimer != nil {
			pressState.detectTimer.Stop()
			pressState.detectTimer = nil
		}
		a.resetLinuxPressReleaseTimerLocked()
		return
	}

	// Already recording but not in hold mode: this is a toggle-off ping (second press).
	a.stopLinuxPressRecordingLocked()
}

func (a *App) resetLinuxPressReleaseTimerLocked() {
	if pressState.releaseTimer != nil {
		pressState.releaseTimer.Stop()
	}

	pressState.releaseTimer = time.AfterFunc(linuxPressReleaseGrace, func() {
		pressState.mu.Lock()
		defer pressState.mu.Unlock()
		if pressState.recording && pressState.holdMode {
			a.stopLinuxPressRecordingLocked()
		}
	})
}

func (a *App) stopLinuxPressRecordingLocked() {
	// Caller must hold pressState.mu
	if pressState.releaseTimer != nil {
		pressState.releaseTimer.Stop()
		pressState.releaseTimer = nil
	}
	if pressState.detectTimer != nil {
		pressState.detectTimer.Stop()
		pressState.detectTimer = nil
	}
	if pressState.recording {
		pressState.recording = false
		pressState.holdMode = false
		pressState.detectingHold = false
		go a.StopRecording()
	}
}