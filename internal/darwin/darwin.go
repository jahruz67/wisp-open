//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=13.0
#cgo LDFLAGS: -framework AppKit -framework ApplicationServices -framework AVFoundation -framework CoreAudio -framework CoreGraphics -framework ServiceManagement -framework IOKit
#include <stdbool.h>
#include <stdlib.h>

int wis_microphone_permission(void);
bool wis_accessibility_permission(void);
void wis_request_microphone_permission(void);
void wis_request_accessibility_permission(void);
void wis_open_permission_settings(const char *kind);
void wis_set_settings_visible(bool visible);
char *wis_active_application_name(void);
bool wis_paste(void);
bool wis_output_active(void);
void wis_toggle_media(void);
char *wis_add_to_startup(void);
char *wis_remove_from_startup(void);
bool wis_is_in_startup(void);
void wis_overlay_create(void);
void wis_overlay_show(const char *message);
void wis_overlay_hide(void);
void wis_overlay_set_volume(double level);
void wis_overlay_close(void);
*/
import "C"

import (
	"errors"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

func MicrophonePermission() string {
	switch int(C.wis_microphone_permission()) {
	case 0:
		return "not_determined"
	case 1:
		return "restricted"
	case 2:
		return "denied"
	case 3:
		return "granted"
	default:
		return "unavailable"
	}
}

func AccessibilityPermission() string {
	if bool(C.wis_accessibility_permission()) {
		return "granted"
	}
	return "denied"
}

func RequestPermission(kind string) error {
	switch kind {
	case "microphone":
		C.wis_request_microphone_permission()
	case "accessibility":
		C.wis_request_accessibility_permission()
	default:
		return fmt.Errorf("unknown macOS permission %q", kind)
	}
	return nil
}

func OpenPermissionSettings(kind string) error {
	if kind != "microphone" && kind != "accessibility" {
		return fmt.Errorf("unknown macOS permission %q", kind)
	}
	cKind := C.CString(kind)
	defer C.free(unsafe.Pointer(cKind))
	C.wis_open_permission_settings(cKind)
	return nil
}

func SetSettingsWindowVisible(visible bool) {
	C.wis_set_settings_visible(C.bool(visible))
}

func ActiveApplicationName() string {
	value := C.wis_active_application_name()
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func Paste() error {
	if !bool(C.wis_accessibility_permission()) {
		return errors.New("Accessibility permission is required to paste into other applications")
	}
	if !bool(C.wis_paste()) {
		return errors.New("macOS rejected the paste keyboard event")
	}
	return nil
}

func PauseMedia() bool {
	// Two samples avoid reacting to a single notification sound.
	if !bool(C.wis_output_active()) {
		return false
	}
	time.Sleep(80 * time.Millisecond)
	if !bool(C.wis_output_active()) {
		return false
	}
	C.wis_toggle_media()
	time.Sleep(350 * time.Millisecond)
	// Only claim ownership of the pause when playback actually stopped.
	if !bool(C.wis_output_active()) {
		return true
	}
	// The output stream did not stop, so undo the unconfirmed toggle rather
	// than risk leaving unrelated media paused without resuming it later.
	C.wis_toggle_media()
	return false
}

func ResumeMedia(wasPaused bool) {
	if wasPaused {
		C.wis_toggle_media()
	}
}

func resultError(value *C.char) error {
	if value == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(value))
	return errors.New(C.GoString(value))
}

func AddToStartup() error { return resultError(C.wis_add_to_startup()) }

func RemoveFromStartup() error { return resultError(C.wis_remove_from_startup()) }

func IsInStartup() bool { return bool(C.wis_is_in_startup()) }

func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

type Overlay struct{}

func NewOverlay() *Overlay {
	C.wis_overlay_create()
	return &Overlay{}
}

func (o *Overlay) Show(message string) {
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))
	C.wis_overlay_show(cMessage)
}

func (o *Overlay) Hide() { C.wis_overlay_hide() }

func (o *Overlay) SetVolume(level float64) { C.wis_overlay_set_volume(C.double(level)) }

func (o *Overlay) Close() { C.wis_overlay_close() }
