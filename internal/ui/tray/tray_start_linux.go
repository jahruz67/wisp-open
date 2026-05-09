//go:build linux

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
