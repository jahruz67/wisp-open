//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework AppKit
#include <stdbool.h>
#include <stdlib.h>

void wis_tray_start(const char *label, const char *shortcut, bool startup, const void *icon, int iconLen);
void wis_tray_set_status(const char *status, const void *icon, int iconLen, bool templateIcon);
void wis_tray_set_trigger_count(int count);
void wis_tray_set_startup(bool checked);
*/
import "C"

import (
	_ "embed"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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

//go:embed icon.png
var iconData []byte

//go:embed icon_recording.png
var iconRecordingData []byte

//go:embed icon_transcribing.png
var iconTranscribingData []byte

var (
	darwinApp        App
	darwinAppMu      sync.RWMutex
	triggerCount     int32
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
	shortcut := "alt+z"
	if cfg := app.GetConfig(); cfg != nil && cfg.Shortcut != "" {
		shortcut = cfg.Shortcut
	}
	label := C.CString(appDisplayName(app))
	cShortcut := C.CString(shortcut)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(cShortcut))
	withIcon(iconData, func(ptr unsafe.Pointer, length C.int) {
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
	data := iconData
	templateIcon := true
	if strings.Contains(status, "Recording") {
		data = iconRecordingData
		templateIcon = false
	} else if strings.Contains(status, "Processing") || strings.Contains(status, "Transcribing") || strings.Contains(status, "Typing") {
		data = iconTranscribingData
		templateIcon = false
	}
	cStatus := C.CString(status)
	defer C.free(unsafe.Pointer(cStatus))
	withIcon(data, func(ptr unsafe.Pointer, length C.int) {
		C.wis_tray_set_status(cStatus, ptr, length, C.bool(templateIcon))
	})
}

func isProcessingStatus(status string) bool {
	return strings.Contains(status, "Processing") ||
		strings.Contains(status, "Transcribing") ||
		strings.Contains(status, "Typing")
}

func IncrementTriggerCount() {
	count := atomic.AddInt32(&triggerCount, 1)
	C.wis_tray_set_trigger_count(C.int(count))
}

func DefaultIconBytes() []byte { return iconData }

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
