//go:build windows

package hotkey

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"wis-free-v3/internal/xhotkey/internal/win"
)

func TestWindowsHeldShortcutAndUnregister(t *testing.T) {
	// Post messages only to this test's thread. No global shortcut or physical
	// keyboard input is needed to simulate a key held across a slow startup.
	hk := New(nil, KeyF20)
	hk.funcs = make(chan func())
	hk.canceled = make(chan struct{})
	hk.done = make(chan struct{})
	hk.registered = true
	var held atomic.Bool
	held.Store(true)
	go hk.handle(func(int) bool { return held.Load() })
	t.Cleanup(func() {
		if hk.registered {
			close(hk.canceled)
			<-hk.done
		}
	})
	postMessage := syscall.NewLazyDLL("user32.dll").NewProc("PostThreadMessageW")
	threadID := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	activate := func() {
		t.Helper()
		posted := make(chan error, 1)
		hk.funcs <- func() {
			msg := win.MSG{}
			win.PeekMessage(&msg, 0, 0, 0) // create this thread's message queue
			id, _, _ := threadID.Call()
			ok, _, err := postMessage.Call(id, uintptr(wmHotkey), 0, 0)
			if ok == 0 {
				posted <- err
			} else {
				posted <- nil
			}
		}
		if err := <-posted; err != nil {
			t.Fatal(err)
		}
	}
	await := func(events <-chan Event) {
		t.Helper()
		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for shortcut event")
		}
	}
	activate()
	await(hk.Keydown())
	activate() // duplicate notification while the key remains held
	select {
	case <-hk.Keydown():
		t.Fatal("held shortcut produced a second activation")
	case <-hk.Keyup():
		t.Fatal("held shortcut produced a premature release")
	case <-time.After(100 * time.Millisecond):
	}
	held.Store(false)
	await(hk.Keyup())
	held.Store(true)
	activate()
	await(hk.Keydown())
	// Updating/quitting while held must still service the unregister request.
	unregistered := make(chan error, 1)
	go func() { unregistered <- hk.Unregister() }()
	select {
	case err := <-unregistered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("unregister blocked while shortcut was held")
	}
}
