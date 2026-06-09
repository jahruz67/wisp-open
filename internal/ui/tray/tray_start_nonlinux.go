//go:build !linux

// ============================================================
// WINDOWS-ONLY FILE — Tray startup for Windows (and macOS).
// Uses systray.Run which blocks and runs the Win32 message pump.
// The Linux equivalent is tray_start_linux.go which uses
// systray.Register to integrate with the existing GTK loop.
// ============================================================

package tray

import (
	"runtime"

	"github.com/getlantern/systray"
)

// Start initializes and runs the system tray.
// This function blocks until the tray is terminated.
func Start(app App) {
	// Pin this goroutine to one OS thread for the whole lifetime of systray.Run.
	// getlantern/systray creates the notify icon and runs the Win32 message pump on
	// whichever thread calls Run; if the goroutine migrates between threads, callbacks
	// and the tray context menu break until restart (often seen after sleep/resume).
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	systray.Run(
		func() { onReady(app) },
		onExit,
	)
}
