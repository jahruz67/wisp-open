//go:build windows

package hotkey

import (
	"time"

	"wis-free-v3/internal/logger"
	xhk "wis-free-v3/internal/xhotkey"
)

const (
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkLWin    = 0x5B
	vkRWin    = 0x5C
)

func containsMod(mods []xhk.Modifier, want xhk.Modifier) bool {
	for _, m := range mods {
		if m == want {
			return true
		}
	}
	return false
}

// modifiersExactMatch reports whether ctrl/shift/alt/win match the required set exactly
// (each required modifier is held, and no extra modifiers among those four are held).
func modifiersExactMatch(required []xhk.Modifier) bool {
	needCtrl := containsMod(required, xhk.ModCtrl)
	needShift := containsMod(required, xhk.ModShift)
	needAlt := containsMod(required, xhk.ModAlt)
	needWin := containsMod(required, xhk.ModWin)

	ctrlDown := xhk.AsyncKeyDown(vkControl)
	shiftDown := xhk.AsyncKeyDown(vkShift)
	altDown := xhk.AsyncKeyDown(vkMenu)
	winDown := xhk.AsyncKeyDown(vkLWin) || xhk.AsyncKeyDown(vkRWin)

	return needCtrl == ctrlDown &&
		needShift == shiftDown &&
		needAlt == altDown &&
		needWin == winDown
}

// modifierPollLoop implements modifier-only shortcuts (e.g. ctrl+win), which Windows
// RegisterHotKey cannot represent.
func (l *Listener) modifierPollLoop(required []xhk.Modifier, stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()

	chordHeld := false
	isRecording := false

	for {
		select {
		case <-stop:
			if isRecording {
				l.stopCallback()
			}
			logger.Info("Modifier hotkey poll stopped")
			return
		case <-ticker.C:
			on := modifiersExactMatch(required)
			switch {
			case on && !chordHeld:
				chordHeld = true
				if !isRecording {
					logger.Info("Modifier chord activated: starting recording")
					go l.startCallback()
					isRecording = true
				}
			case !on && chordHeld:
				chordHeld = false
				if isRecording {
					logger.Info("Modifier chord released: stopping recording")
					go l.stopCallback()
					isRecording = false
				}
			}
		}
	}
}
