//go:build darwin

package hotkey

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

import (
	"time"

	xhk "wis-free-v3/internal/xhotkey"
)

func (l *Listener) modifierPollLoop(required []xhk.Modifier, stop <-chan struct{}) {
	var wanted C.CGEventFlags
	for _, mod := range required {
		switch mod {
		case xhk.ModCtrl:
			wanted |= C.kCGEventFlagMaskControl
		case xhk.ModShift:
			wanted |= C.kCGEventFlagMaskShift
		case xhk.ModOption:
			wanted |= C.kCGEventFlagMaskAlternate
		case xhk.ModCmd:
			wanted |= C.kCGEventFlagMaskCommand
		}
	}
	const mask = C.kCGEventFlagMaskControl | C.kCGEventFlagMaskShift | C.kCGEventFlagMaskAlternate | C.kCGEventFlagMaskCommand
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()
	held := false
	defer func() {
		if held {
			l.stopCallback()
		}
	}()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			on := C.CGEventSourceFlagsState(C.kCGEventSourceStateCombinedSessionState)&mask == wanted
			if on == held {
				continue
			}
			held = on
			// Keep callbacks ordered when microphone initialization is slow.
			if held {
				l.startCallback()
			} else {
				l.stopCallback()
			}
		}
	}
}
