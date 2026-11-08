//go:build linux

// ============================================================
// LINUX-ONLY FILE — Tray startup for Linux. On Linux, Wails'
// GTK main loop is already running, so we use systray.Register
// instead of systray.Run (which would start a second gtk_main).
// The Windows equivalent is tray_start_nonlinux.go
// ============================================================

package tray

/*
#cgo pkg-config: glib-2.0
#include <glib.h>
extern void goWISRegisterTray(void);
static gboolean wis_register_tray(gpointer unused) {
    goWISRegisterTray();
    return G_SOURCE_REMOVE;
}
static void wis_queue_tray(void) {
    g_idle_add(wis_register_tray, NULL);
}
*/
import "C"

import (
	"sync"

	"github.com/getlantern/systray"
)

var trayStartOnce sync.Once
var trayAppMu sync.Mutex
var trayApp App

// On Linux, Wails already runs the GTK main loop on the main thread.
// Calling systray.Run() would start a second gtk_main() and may abort.
// Instead, we only register the tray and let the existing GTK loop drive it.
func Start(app App) {
	trayStartOnce.Do(func() {
		trayAppMu.Lock()
		trayApp = app
		trayAppMu.Unlock()
		// Wails invokes OnStartup on a Go worker. GTK/AppIndicator creation must
		// run on the GTK thread, including during a fully hidden login launch.
		C.wis_queue_tray()
	})
}

//export goWISRegisterTray
func goWISRegisterTray() {
	trayAppMu.Lock()
	app := trayApp
	trayAppMu.Unlock()
	systray.Register(
		func() { onReady(app) },
		onExit,
	)
}
