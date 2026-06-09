//go:build linux

// ============================================================
// LINUX-ONLY FILE — Tray startup for Linux. On Linux, Wails'
// GTK main loop is already running, so we use systray.Register
// instead of systray.Run (which would start a second gtk_main).
// The Windows equivalent is tray_start_nonlinux.go
// ============================================================

package tray

import "github.com/getlantern/systray"

// On Linux, Wails already runs the GTK main loop on the main thread.
// Calling systray.Run() would start a second gtk_main() and may abort.
// Instead, we only register the tray and let the existing GTK loop drive it.
func Start(app App) {
	systray.Register(
		func() { onReady(app) },
		onExit,
	)
}
