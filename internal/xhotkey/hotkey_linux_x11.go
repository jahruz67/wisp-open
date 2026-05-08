//go:build linux && cgo

package hotkey

/*
#cgo LDFLAGS: -lX11

#include <stdint.h>

int displayTest();
int waitHotkey(uintptr_t hkhandle, unsigned int mod, int key);
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/cgo"
)

func (hk *Hotkey) registerX11() error {
	if C.displayTest() != 0 {
		return fmt.Errorf("X11 display not available (is DISPLAY set?)")
	}
	hk.mu.Lock()
	if hk.registered {
		hk.mu.Unlock()
		return errors.New("hotkey already registered.")
	}
	hk.backend = linuxHKX11
	hk.registered = true
	hk.ctx, hk.cancel = context.WithCancel(context.Background())
	hk.canceled = make(chan struct{})
	hk.mu.Unlock()

	go hk.handleX11()
	return nil
}

func (hk *Hotkey) handleX11() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var mod Modifier
	for _, m := range hk.mods {
		mod = mod | m
	}
	h := cgo.NewHandle(hk)
	defer h.Delete()

	for {
		_ = C.waitHotkey(C.uintptr_t(h), C.uint(mod), C.int(hk.key))
		close(hk.canceled)
		return
	}
}

//export checkCancel
func checkCancel(h uintptr) C.int {
	hk := cgo.Handle(h).Value().(*Hotkey)
	select {
	case <-hk.ctx.Done():
		return 1
	default:
		return 0
	}
}

//export hotkeyDown
func hotkeyDown(h uintptr) {
	hk := cgo.Handle(h).Value().(*Hotkey)
	hk.keydownIn <- Event{}
}

//export hotkeyUp
func hotkeyUp(h uintptr) {
	hk := cgo.Handle(h).Value().(*Hotkey)
	hk.keyupIn <- Event{}
}
