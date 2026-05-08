package hotkey

import (
	"strings"

	xhk "wis-free-v3/internal/xhotkey"
)

// KeyMap maps human-readable key names to golang.design/x/hotkey Key instances.
var KeyMap = map[string]xhk.Key{
	"space":  xhk.KeySpace,
	"0":      xhk.Key0,
	"1":      xhk.Key1,
	"2":      xhk.Key2,
	"3":      xhk.Key3,
	"4":      xhk.Key4,
	"5":      xhk.Key5,
	"6":      xhk.Key6,
	"7":      xhk.Key7,
	"8":      xhk.Key8,
	"9":      xhk.Key9,
	"a":      xhk.KeyA,
	"b":      xhk.KeyB,
	"c":      xhk.KeyC,
	"d":      xhk.KeyD,
	"e":      xhk.KeyE,
	"f":      xhk.KeyF,
	"g":      xhk.KeyG,
	"h":      xhk.KeyH,
	"i":      xhk.KeyI,
	"j":      xhk.KeyJ,
	"k":      xhk.KeyK,
	"l":      xhk.KeyL,
	"m":      xhk.KeyM,
	"n":      xhk.KeyN,
	"o":      xhk.KeyO,
	"p":      xhk.KeyP,
	"q":      xhk.KeyQ,
	"r":      xhk.KeyR,
	"s":      xhk.KeyS,
	"t":      xhk.KeyT,
	"u":      xhk.KeyU,
	"v":      xhk.KeyV,
	"w":      xhk.KeyW,
	"x":      xhk.KeyX,
	"y":      xhk.KeyY,
	"z":      xhk.KeyZ,
	"return": xhk.KeyReturn,
	"enter":  xhk.KeyReturn,
	"escape": xhk.KeyEscape,
	"esc":    xhk.KeyEscape,
	"delete": xhk.KeyDelete,
	"del":    xhk.KeyDelete,
	"tab":    xhk.KeyTab,
	"left":   xhk.KeyLeft,
	"right":  xhk.KeyRight,
	"up":     xhk.KeyUp,
	"down":   xhk.KeyDown,
	"f1":     xhk.KeyF1,
	"f2":     xhk.KeyF2,
	"f3":     xhk.KeyF3,
	"f4":     xhk.KeyF4,
	"f5":     xhk.KeyF5,
	"f6":     xhk.KeyF6,
	"f7":     xhk.KeyF7,
	"f8":     xhk.KeyF8,
	"f9":     xhk.KeyF9,
	"f10":    xhk.KeyF10,
	"f11":    xhk.KeyF11,
	"f12":    xhk.KeyF12,
}

// ParseShortcut parses a shortcut string and returns the trigger key, modifiers, whether the
// shortcut is modifier-only (no letter/key, e.g. "ctrl+win"), and whether parsing succeeded.
//
// For shortcuts that include a non-modifier key, the trigger key is the rightmost segment in
// KeyMap; every other segment must be in ModMap (so "win+ctrl+k" and "ctrl+win+k" both work).
//
// If no segment matches KeyMap but every segment is a known modifier, modifierOnly is true
// (supported on Windows via key-state polling; RegisterHotKey cannot represent these chords).
func ParseShortcut(shortcut string) (key xhk.Key, mods []xhk.Modifier, modifierOnly bool, ok bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(shortcut)), "+")
	if len(parts) == 0 {
		return 0, nil, false, false
	}

	keyIdx := -1
	var triggerKey xhk.Key
	for i := len(parts) - 1; i >= 0; i-- {
		name := strings.TrimSpace(parts[i])
		if name == "" {
			continue
		}
		if k, exists := KeyMap[name]; exists {
			keyIdx = i
			triggerKey = k
			break
		}
	}

	if keyIdx >= 0 {
		mods = nil
		for i := 0; i < len(parts); i++ {
			if i == keyIdx {
				continue
			}
			modName := strings.TrimSpace(parts[i])
			if modName == "" {
				continue
			}
			m, valid := ModMap[modName]
			if !valid {
				return 0, nil, false, false
			}
			mods = append(mods, m)
		}
		return triggerKey, mods, false, true
	}

	mods = nil
	for i := 0; i < len(parts); i++ {
		modName := strings.TrimSpace(parts[i])
		if modName == "" {
			continue
		}
		m, valid := ModMap[modName]
		if !valid {
			return 0, nil, false, false
		}
		mods = append(mods, m)
	}
	if len(mods) < 2 {
		// Single-modifier "chords" would fire on every press of that key alone.
		return 0, nil, false, false
	}
	return 0, mods, true, true
}

