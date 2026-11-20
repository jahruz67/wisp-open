//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework AppKit
#include <stdbool.h>
#include <stdlib.h>

void wis_tray_start(const char *label, const char *shortcut, bool startup, const void *icon, int iconLen);
void wis_tray_set_status(const char *status);
void wis_tray_set_startup(bool checked);
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"

	"wis-free-v3/internal/config"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"
)

// App defines the operations used by the native macOS menu-bar item.
type App interface {
	Quit()
	GetConfig() *config.Config
	ShowSettings()
	Version() string
}

var (
	darwinApp        App
	darwinAppMu      sync.RWMutex
	onStartupChanged func(bool)
)

func appDisplayName(app App) string {
	v := app.Version()
	if v != "" && v != "dev" {
		return "wis-free-v3 v" + v
	}
	return "wis-free-v3"
}

func Start(app App) {
	darwinAppMu.Lock()
	darwinApp = app
	darwinAppMu.Unlock()
	shortcut := config.DefaultShortcut
	if cfg := app.GetConfig(); cfg != nil && cfg.Shortcut != "" {
		shortcut = cfg.Shortcut
	}
	label := C.CString(appDisplayName(app))
	cShortcut := C.CString(shortcut)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(cShortcut))
	withIcon(DefaultIconBytes(), func(ptr unsafe.Pointer, length C.int) {
		C.wis_tray_start(label, cShortcut, C.bool(platform.IsInStartup()), ptr, length)
	})
}

func withIcon(data []byte, fn func(unsafe.Pointer, C.int)) {
	if len(data) == 0 {
		fn(nil, 0)
		return
	}
	fn(unsafe.Pointer(&data[0]), C.int(len(data)))
	runtime.KeepAlive(data)
}

func UpdateStatus(status string) {
	statusMu.Lock()
	lastStatus = status
	statusMu.Unlock()
	cStatus := C.CString(status)
	defer C.free(unsafe.Pointer(cStatus))
	C.wis_tray_set_status(cStatus)
}

func SetOnStartupChanged(fn func(bool)) { onStartupChanged = fn }

func SetStartupChecked(checked bool) { C.wis_tray_set_startup(C.bool(checked)) }

func currentApp() App {
	darwinAppMu.RLock()
	defer darwinAppMu.RUnlock()
	return darwinApp
}

//export goWISTraySettings
func goWISTraySettings() {
	if app := currentApp(); app != nil {
		app.ShowSettings()
	}
}

//export goWISTrayToggleStartup
func goWISTrayToggleStartup() {
	enabled := !platform.IsInStartup()
	var err error
	if enabled {
		err = platform.AddToStartup()
	} else {
		err = platform.RemoveFromStartup()
	}
	if err != nil {
		logger.Error("Failed to update macOS login item: %v", err)
		SetStartupChecked(!enabled)
		return
	}
	SetStartupChecked(enabled)
	if onStartupChanged != nil {
		onStartupChanged(enabled)
	}
}

//export goWISTrayQuit
func goWISTrayQuit() {
	if app := currentApp(); app != nil {
		app.Quit()
	}
}
