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
	linuxPressMu        sync.Mutex
	linuxPressTimer     *time.Timer
	linuxPressRecording bool
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
			go a.StartRecording()
		}

		if linuxPressTimer != nil {
			linuxPressTimer.Stop()
		}

		linuxPressTimer = time.AfterFunc(300*time.Millisecond, func() {
			linuxPressMu.Lock()
			defer linuxPressMu.Unlock()
			if linuxPressRecording {
				linuxPressRecording = false
				go a.StopRecording()
			}
		})

		w.WriteHeader(http.StatusNoContent)
	})

	go func() {
		_ = http.ListenAndServe(linuxPressAddr, mux)
	}()
}
