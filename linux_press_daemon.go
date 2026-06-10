//go:build linux

// ============================================================
// LINUX-ONLY FILE — Local HTTP daemon that receives hotkey
// press/release pings from the GNOME custom shortcut helper.
// This is the Linux equivalent of the Windows hotkey listener.
// Any changes here will NOT affect the Windows build.
// ============================================================

package main

import (
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"wis-free-v3/internal/logger"
)

const linuxPressAddr = "127.0.0.1:9876"

// linuxPressState holds the daemon's state, protected by a mutex.
// All field access must be done while holding the mutex.
type linuxPressState struct {
	mu            sync.Mutex
	releaseTimer  *time.Timer
	detectTimer   *time.Timer
	recording     bool
	holdMode      bool
	detectingHold bool
	cycle         uint64
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

// linuxPressServer holds the HTTP server reference for graceful shutdown.
var linuxPressServer *http.Server

func (a *App) startLinuxPressDaemon() {
	mux := http.NewServeMux()
	mux.HandleFunc("/press", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("Linux press handler recovered from panic: %v", recovered)
				http.Error(w, "press handler failed", http.StatusInternalServerError)
			}
		}()

		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		a.handleLinuxPressPing()
		w.WriteHeader(http.StatusNoContent)
	})

	go func() {
		server := &http.Server{
			Addr:              linuxPressAddr,
			Handler:           mux,
			ReadHeaderTimeout: 2 * time.Second,
			IdleTimeout:       5 * time.Second,
		}
		linuxPressServer = server
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Linux press daemon stopped: %v", err)
		}
	}()
}

func (a *App) handleLinuxPressPing() {
	pressState.mu.Lock()
	defer pressState.mu.Unlock()

	if !pressState.recording {
		// First ping: start recording with hold detection
		pressState.cycle++
		cycle := pressState.cycle
		pressState.recording = true
		pressState.holdMode = false
		pressState.detectingHold = true
		go a.startLinuxPressRecording(cycle)

		if pressState.detectTimer != nil {
			pressState.detectTimer.Stop()
		}
		pressState.detectTimer = time.AfterFunc(linuxPressHoldDetectWindow, func() {
			pressState.mu.Lock()
			defer pressState.mu.Unlock()
			if pressState.cycle == cycle {
				pressState.detectingHold = false
			}
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

	cycle := pressState.cycle
	pressState.releaseTimer = time.AfterFunc(linuxPressReleaseGrace, func() {
		pressState.mu.Lock()
		defer pressState.mu.Unlock()
		if pressState.cycle == cycle && pressState.recording && pressState.holdMode {
			a.stopLinuxPressRecordingLocked()
		}
	})
}

func (a *App) stopLinuxPressRecordingLocked() {
	// Caller must hold pressState.mu
	wasRecording := pressState.recording
	pressState.cycle++
	if pressState.releaseTimer != nil {
		pressState.releaseTimer.Stop()
		pressState.releaseTimer = nil
	}
	if pressState.detectTimer != nil {
		pressState.detectTimer.Stop()
		pressState.detectTimer = nil
	}
	pressState.recording = false
	pressState.holdMode = false
	pressState.detectingHold = false
	if wasRecording {
		go a.stopLinuxPressRecording()
	}
}

func (a *App) startLinuxPressRecording(cycle uint64) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error("Linux press start recovered from panic: %v", recovered)
			a.resetFailedLinuxPressCycle(cycle)
		}
	}()

	a.StartRecording()
	if atomic.LoadInt32(&a.recording) == 0 {
		a.resetFailedLinuxPressCycle(cycle)
	}
}

func (a *App) stopLinuxPressRecording() {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.Error("Linux press stop recovered from panic: %v", recovered)
		}
	}()

	a.StopRecording()
}

func (a *App) resetFailedLinuxPressCycle(cycle uint64) {
	pressState.mu.Lock()
	defer pressState.mu.Unlock()

	if pressState.cycle != cycle {
		return
	}
	pressState.cycle++
	if pressState.releaseTimer != nil {
		pressState.releaseTimer.Stop()
		pressState.releaseTimer = nil
	}
	if pressState.detectTimer != nil {
		pressState.detectTimer.Stop()
		pressState.detectTimer = nil
	}
	pressState.recording = false
	pressState.holdMode = false
	pressState.detectingHold = false
}
