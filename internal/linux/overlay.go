//go:build linux
package linux

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"wis-free-v3/internal/logger"
)

// linuxOverlay shows recording/transcription status via libnotify when available
// (notify-send). There is no full-screen overlay on Linux to avoid a GTK/Cairo
// dependency beyond what Wails already pulls in.
type linuxOverlay struct {
	mu         sync.Mutex
	lastMsg    string
	lastUpdate time.Time
}

func NewOverlay() *linuxOverlay {
	return &linuxOverlay{}
}

const overlayNotifyID = "wisfree-overlay"

var (
	notifyOnce sync.Once
	haveNotify bool
)

func detectNotifySend() {
	_, err := exec.LookPath("notify-send")
	haveNotify = err == nil
	if !haveNotify {
		logger.Info("notify-send not found; install libnotify-bin for recording status toasts on Linux")
	}
}

func (o *linuxOverlay) Show(message string) {
	notifyOnce.Do(detectNotifySend)
	if !haveNotify {
		return
	}
	o.mu.Lock()
	o.lastMsg = message
	o.mu.Unlock()
	cmd := exec.Command("notify-send",
		"-a", "wis-free-v3",
		"-r", overlayNotifyID,
		"-u", "low",
		"-t", "0",
		message,
	)
	if err := cmd.Run(); err != nil {
		logger.Error("notify-send failed: %v", err)
	}
}

func (o *linuxOverlay) Hide() {
	notifyOnce.Do(detectNotifySend)
	if !haveNotify {
		return
	}
	o.mu.Lock()
	o.lastMsg = ""
	o.mu.Unlock()
	// Replacing the same ID with a 1ms toast clears the bubble on many DEs (GNOME, KDE).
	_ = exec.Command("notify-send", "-a", "wis-free-v3", "-r", overlayNotifyID, "-t", "1", " ").Run()
}

func (o *linuxOverlay) SetVolume(level float64) {
	notifyOnce.Do(detectNotifySend)
	if !haveNotify {
		return
	}
	o.mu.Lock()
	now := time.Now()
	// Throttle to at most once per 300ms to prevent spawning notify-send processes at ~100Hz (buffer rate)
	if now.Sub(o.lastUpdate) < 300*time.Millisecond {
		o.mu.Unlock()
		return
	}
	o.lastUpdate = now
	base := o.lastMsg
	o.mu.Unlock()
	if strings.TrimSpace(base) == "" {
		return
	}
	pct := int(level*100 + 0.5)
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	body := fmt.Sprintf("%s — mic %d%%", base, pct)
	cmd := exec.Command("notify-send",
		"-a", "wis-free-v3",
		"-r", overlayNotifyID,
		"-u", "low",
		"-t", "0",
		body,
	)
	if err := cmd.Run(); err != nil {
		logger.Error("notify-send failed: %v", err)
	}
}

func (o *linuxOverlay) Close() {
	o.Hide()
}
