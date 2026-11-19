//go:build linux

package hotkey

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"wis-free-v3/internal/logger"
	xhk "wis-free-v3/internal/xhotkey"
)

const linuxKeyBitmapBytes = 96 // KEY_MAX is 0x2ff.

const (
	keyLeftCtrl   = 29
	keyRightCtrl  = 97
	keyLeftAlt    = 56
	keyRightAlt   = 100
	keyLeftShift  = 42
	keyRightShift = 54
	keyLeftMeta   = 125
	keyRightMeta  = 126
)

type linuxInputDevice struct {
	file *os.File
	keys map[int]bool
}

func linuxIOCRead(size, number uintptr) uintptr {
	return (uintptr(2) << 30) | (size << 16) | (uintptr('E') << 8) | number
}

func linuxIOCBytes(fd uintptr, request uintptr, data []byte) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(unsafe.Pointer(&data[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func linuxBitSet(bits []byte, code int) bool {
	return code >= 0 && code/8 < len(bits) && bits[code/8]&(1<<uint(code%8)) != 0
}

func linuxKeyCodesForShortcut(key xhk.Key, modifierOnly bool) (map[int]bool, int, error) {
	interest := map[int]bool{
		keyLeftCtrl: true, keyRightCtrl: true,
		keyLeftAlt: true, keyRightAlt: true,
		keyLeftShift: true, keyRightShift: true,
		keyLeftMeta: true, keyRightMeta: true,
	}
	if modifierOnly {
		return interest, 0, nil
	}
	keyCode, ok := linuxKeyCodeByKeysym[key]
	if !ok {
		return nil, 0, fmt.Errorf("Linux input polling does not support key code %d", key)
	}
	interest[keyCode] = true
	return interest, keyCode, nil
}

var linuxKeyCodeByKeysym = map[xhk.Key]int{
	xhk.KeyA: 30, xhk.KeyB: 48, xhk.KeyC: 46, xhk.KeyD: 32, xhk.KeyE: 18,
	xhk.KeyF: 33, xhk.KeyG: 34, xhk.KeyH: 35, xhk.KeyI: 23, xhk.KeyJ: 36,
	xhk.KeyK: 37, xhk.KeyL: 38, xhk.KeyM: 50, xhk.KeyN: 49, xhk.KeyO: 24,
	xhk.KeyP: 25, xhk.KeyQ: 16, xhk.KeyR: 19, xhk.KeyS: 31, xhk.KeyT: 20,
	xhk.KeyU: 22, xhk.KeyV: 47, xhk.KeyW: 17, xhk.KeyX: 45, xhk.KeyY: 21,
	xhk.KeyZ: 44,
	xhk.Key0: 11, xhk.Key1: 2, xhk.Key2: 3, xhk.Key3: 4, xhk.Key4: 5,
	xhk.Key5: 6, xhk.Key6: 7, xhk.Key7: 8, xhk.Key8: 9, xhk.Key9: 10,
	xhk.KeySpace: 57, xhk.KeyReturn: 28, xhk.KeyEscape: 1, xhk.KeyDelete: 111,
	xhk.KeyTab: 15, xhk.KeyLeft: 105, xhk.KeyRight: 106, xhk.KeyUp: 103,
	xhk.KeyDown: 108, xhk.KeyF1: 59, xhk.KeyF2: 60, xhk.KeyF3: 61,
	xhk.KeyF4: 62, xhk.KeyF5: 63, xhk.KeyF6: 64, xhk.KeyF7: 65,
	xhk.KeyF8: 66, xhk.KeyF9: 67, xhk.KeyF10: 68, xhk.KeyF11: 87,
	xhk.KeyF12: 88,
}

func openLinuxInputDevices(interest map[int]bool) ([]linuxInputDevice, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, fmt.Errorf("find keyboard input devices: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no Linux keyboard input devices were found")
	}

	var devices []linuxInputDevice
	var permissionErr error
	for _, path := range paths {
		file, openErr := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if openErr != nil {
			if os.IsPermission(openErr) {
				permissionErr = openErr
			}
			continue
		}

		var eventTypes [8]byte
		if err := linuxIOCBytes(file.Fd(), linuxIOCRead(uintptr(len(eventTypes)), 0x20), eventTypes[:]); err != nil || !linuxBitSet(eventTypes[:], 0x01) {
			file.Close()
			continue
		}
		var keyCapabilities [linuxKeyBitmapBytes]byte
		if err := linuxIOCBytes(file.Fd(), linuxIOCRead(uintptr(len(keyCapabilities)), 0x21), keyCapabilities[:]); err != nil {
			file.Close()
			continue
		}

		keys := make(map[int]bool)
		for code := range interest {
			keys[code] = linuxBitSet(keyCapabilities[:], code)
		}
		relevant := false
		for _, supported := range keys {
			relevant = relevant || supported
		}
		if relevant {
			devices = append(devices, linuxInputDevice{file: file, keys: keys})
		} else {
			file.Close()
		}
	}

	if len(devices) > 0 {
		return devices, nil
	}
	if permissionErr != nil {
		return nil, fmt.Errorf("cannot read /dev/input/event* (%v); grant this user read access to Linux input devices, then restart the app", permissionErr)
	}
	return nil, fmt.Errorf("no readable keyboard supports the configured shortcut")
}

func linuxKeyDown(state map[int]bool, left, right int) bool {
	return state[left] || state[right]
}

func linuxShortcutActive(required []xhk.Modifier, triggerCode int, devices []linuxInputDevice) (bool, error) {
	state := make(map[int]bool)
	for _, device := range devices {
		var keysDown [linuxKeyBitmapBytes]byte
		if err := linuxIOCBytes(device.file.Fd(), linuxIOCRead(uintptr(len(keysDown)), 0x18), keysDown[:]); err != nil {
			return false, err
		}
		for code, supported := range device.keys {
			if supported && linuxBitSet(keysDown[:], code) {
				state[code] = true
			}
		}
	}

	ctrl := linuxKeyDown(state, keyLeftCtrl, keyRightCtrl)
	shift := linuxKeyDown(state, keyLeftShift, keyRightShift)
	alt := linuxKeyDown(state, keyLeftAlt, keyRightAlt)
	meta := linuxKeyDown(state, keyLeftMeta, keyRightMeta)

	needCtrl, needShift, needAlt, needMeta := false, false, false, false
	for _, mod := range required {
		switch mod {
		case xhk.ModCtrl:
			needCtrl = true
		case xhk.ModShift:
			needShift = true
		case xhk.Mod1:
			needAlt = true
		case xhk.Mod4:
			needMeta = true
		default:
			return false, fmt.Errorf("Linux input polling does not support modifier mask %d", mod)
		}
	}
	modsMatch := needCtrl == ctrl && needShift == shift && needAlt == alt && needMeta == meta
	if triggerCode == 0 {
		return modsMatch, nil
	}
	return modsMatch && state[triggerCode], nil
}

func (l *Listener) linuxInputPollLoop(key xhk.Key, required []xhk.Modifier, modifierOnly bool, stop <-chan struct{}) {
	interest, triggerCode, err := linuxKeyCodesForShortcut(key, modifierOnly)
	if err != nil {
		l.failLinuxInputPoll(err)
		return
	}
	devices, err := openLinuxInputDevices(interest)
	if err != nil {
		l.failLinuxInputPoll(err)
		return
	}
	defer func() {
		for _, device := range devices {
			device.file.Close()
		}
	}()

	l.mu.RLock()
	ready := l.registrationReadyCallback
	l.mu.RUnlock()
	if ready != nil {
		ready()
	}
	logger.Info("Linux keyboard shortcut polling started")

	// Run start/release callbacks in order while the poller continues sampling
	// key state. This preserves a quick press-and-release during microphone start.
	actions := make(chan bool, 8)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		isRecording := false
		for pressed := range actions {
			if pressed && !isRecording {
				l.startCallback()
				isRecording = true
			} else if !pressed && isRecording {
				l.stopCallback()
				isRecording = false
			}
		}
		if isRecording {
			l.stopCallback()
		}
	}()

	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()
	shortcutHeld := false
	for {
		select {
		case <-stop:
			close(actions)
			<-workerDone
			logger.Info("Linux keyboard shortcut polling stopped")
			return
		case <-ticker.C:
			held, stateErr := linuxShortcutActive(required, triggerCode, devices)
			if stateErr != nil {
				if shortcutHeld {
					actions <- false
				}
				close(actions)
				<-workerDone
				l.failLinuxInputPoll(stateErr)
				return
			}
			if held != shortcutHeld {
				actions <- held
				shortcutHeld = held
			}
		}
	}
}

func (l *Listener) failLinuxInputPoll(err error) {
	l.mu.Lock()
	cb := l.registrationErrorCallback
	l.isListening = false
	l.stopModPoll = nil
	l.mu.Unlock()
	if cb != nil {
		cb(err)
	}
}

// modifierPollLoop is only called for modifier-only Windows shortcuts; Linux
// uses linuxInputPollLoop for both modifier-only chords and key combinations.
func (l *Listener) modifierPollLoop(required []xhk.Modifier, stop <-chan struct{}) {
	_, _ = required, stop
}
