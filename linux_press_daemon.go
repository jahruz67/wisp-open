//go:build linux

package main

import (
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const linuxPressAddr = "127.0.0.1:9876"

var (
	linuxPressMu            sync.Mutex
	linuxPressReleaseTimer  *time.Timer
	linuxPressDetectTimer   *time.Timer
	linuxPressRecording     bool
	linuxPressHoldMode      bool
	linuxPressDetectingHold bool
)

const (
	// GNOME custom shortcuts commonly auto-repeat while the key is held.
	// If we see a second ping quickly, treat the shortcut as push-to-talk.
	linuxPressHoldDetectWindow = 1200 * time.Millisecond

	// Once hold mode is confirmed, lack of fresh pings means the key was released.
	linuxPressReleaseGrace = 450 * time.Millisecond
)

func init() {
	if hasPressFlag(os.Args[1:]) {
		if err := sendLinuxPressPing(); err != nil {
			_ = exec.Command("notify-send", "Voice App", "Please open the main application first").Run()
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
		linuxPressMu.Lock()
		defer linuxPressMu.Unlock()

		if !linuxPressRecording {
			linuxPressRecording = true
			linuxPressHoldMode = false
			linuxPressDetectingHold = true
			go a.StartRecording()

			if linuxPressDetectTimer != nil {
				linuxPressDetectTimer.Stop()
			}
			linuxPressDetectTimer = time.AfterFunc(linuxPressHoldDetectWindow, func() {
				linuxPressMu.Lock()
				defer linuxPressMu.Unlock()
				linuxPressDetectingHold = false
			})

			w.WriteHeader(http.StatusNoContent)
			return
		}

		if linuxPressDetectingHold || linuxPressHoldMode {
			linuxPressHoldMode = true
			linuxPressDetectingHold = false
			if linuxPressDetectTimer != nil {
				linuxPressDetectTimer.Stop()
				linuxPressDetectTimer = nil
			}
			resetLinuxPressReleaseTimer(a)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		stopLinuxPressRecording(a)
		w.WriteHeader(http.StatusNoContent)
	})

	go func() {
		_ = http.ListenAndServe(linuxPressAddr, mux)
	}()
}

func resetLinuxPressReleaseTimer(a *App) {
	if linuxPressReleaseTimer != nil {
		linuxPressReleaseTimer.Stop()
	}

	linuxPressReleaseTimer = time.AfterFunc(linuxPressReleaseGrace, func() {
		linuxPressMu.Lock()
		defer linuxPressMu.Unlock()
		if linuxPressRecording && linuxPressHoldMode {
			stopLinuxPressRecording(a)
		}
	})
}

func stopLinuxPressRecording(a *App) {
	if linuxPressReleaseTimer != nil {
		linuxPressReleaseTimer.Stop()
		linuxPressReleaseTimer = nil
	}
	if linuxPressDetectTimer != nil {
		linuxPressDetectTimer.Stop()
		linuxPressDetectTimer = nil
	}
	if linuxPressRecording {
		linuxPressRecording = false
		linuxPressHoldMode = false
		linuxPressDetectingHold = false
		go a.StopRecording()
	}
}
