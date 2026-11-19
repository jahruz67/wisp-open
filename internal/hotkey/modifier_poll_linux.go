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

type linuxModifierDevice struct {
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

func openLinuxModifierDevices() ([]linuxModifierDevice, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, fmt.Errorf("find keyboard input devices: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no Linux keyboard input devices were found")
	}

	var devices []linuxModifierDevice
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
		for _, code := range []int{keyLeftCtrl, keyRightCtrl, keyLeftAlt, keyRightAlt, keyLeftShift, keyRightShift, keyLeftMeta, keyRightMeta} {
			keys[code] = linuxBitSet(keyCapabilities[:], code)
		}
		// A candidate keyboard must report at least one of the requested
		// modifier keys. Some keyboards expose the left and right keys on
		// separate event nodes, so polling combines state from every node.
		if keys[keyLeftCtrl] || keys[keyRightCtrl] || keys[keyLeftMeta] || keys[keyRightMeta] {
			devices = append(devices, linuxModifierDevice{file: file, keys: keys})
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
	return nil, fmt.Errorf("no readable keyboard exposes Ctrl and Meta/Super state")
}

func linuxKeyDown(bits []byte, keys map[int]bool, left, right int) bool {
	return (keys[left] && linuxBitSet(bits, left)) || (keys[right] && linuxBitSet(bits, right))
}

func linuxModifierState(required []xhk.Modifier, devices []linuxModifierDevice) (bool, error) {
	var ctrl, shift, alt, meta bool
	for _, device := range devices {
		var state [linuxKeyBitmapBytes]byte
		if err := linuxIOCBytes(device.file.Fd(), linuxIOCRead(uintptr(len(state)), 0x18), state[:]); err != nil {
			return false, err
		}
		ctrl = ctrl || linuxKeyDown(state[:], device.keys, keyLeftCtrl, keyRightCtrl)
		shift = shift || linuxKeyDown(state[:], device.keys, keyLeftShift, keyRightShift)
		alt = alt || linuxKeyDown(state[:], device.keys, keyLeftAlt, keyRightAlt)
		meta = meta || linuxKeyDown(state[:], device.keys, keyLeftMeta, keyRightMeta)
	}

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
			return false, fmt.Errorf("Linux modifier polling does not support modifier mask %d", mod)
		}
	}
	return needCtrl == ctrl && needShift == shift && needAlt == alt && needMeta == meta, nil
}

func (l *Listener) modifierPollLoop(required []xhk.Modifier, stop <-chan struct{}) {
	devices, err := openLinuxModifierDevices()
	if err != nil {
		l.mu.Lock()
		cb := l.registrationErrorCallback
		l.isListening = false
		l.stopModPoll = nil
		l.mu.Unlock()
		if cb != nil {
			cb(err)
		}
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
	logger.Info("Linux modifier hotkey polling started")
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()

	chordHeld, isRecording := false, false
	for {
		select {
		case <-stop:
			if isRecording {
				l.stopCallback()
			}
			logger.Info("Linux modifier hotkey poll stopped")
			return
		case <-ticker.C:
			on, stateErr := linuxModifierState(required, devices)
			if stateErr != nil {
				logger.Error("Could not read Linux keyboard modifier state: %v", stateErr)
				on = false
			}
			switch {
			case on && !chordHeld:
				chordHeld = true
				if !isRecording {
					logger.Info("Linux modifier chord activated: starting recording")
					go l.startCallback()
					isRecording = true
				}
			case !on && chordHeld:
				chordHeld = false
				if isRecording {
					logger.Info("Linux modifier chord released: stopping recording")
					go l.stopCallback()
					isRecording = false
				}
			}
		}
	}
}
