package hotkey

import (
	"strings"

	"wis-free-v3/internal/xhotkey"
)

// KeyMap maps human-readable key names to golang.design/x/hotkey Key instances.
var KeyMap = map[string]hotkey.Key{
	"space":  hotkey.KeySpace,
	"0":      hotkey.Key0,
	"1":      hotkey.Key1,
	"2":      hotkey.Key2,
	"3":      hotkey.Key3,
	"4":      hotkey.Key4,
	"5":      hotkey.Key5,
	"6":      hotkey.Key6,
	"7":      hotkey.Key7,
	"8":      hotkey.Key8,
	"9":      hotkey.Key9,
	"a":      hotkey.KeyA,
	"b":      hotkey.KeyB,
	"c":      hotkey.KeyC,
	"d":      hotkey.KeyD,
	"e":      hotkey.KeyE,
	"f":      hotkey.KeyF,
	"g":      hotkey.KeyG,
	"h":      hotkey.KeyH,
	"i":      hotkey.KeyI,
	"j":      hotkey.KeyJ,
	"k":      hotkey.KeyK,
	"l":      hotkey.KeyL,
	"m":      hotkey.KeyM,
	"n":      hotkey.KeyN,
	"o":      hotkey.KeyO,
	"p":      hotkey.KeyP,
	"q":      hotkey.KeyQ,
	"r":      hotkey.KeyR,
	"s":      hotkey.KeyS,
	"t":      hotkey.KeyT,
	"u":      hotkey.KeyU,
	"v":      hotkey.KeyV,
	"w":      hotkey.KeyW,
	"x":      hotkey.KeyX,
	"y":      hotkey.KeyY,
	"z":      hotkey.KeyZ,
	"return": hotkey.KeyReturn,
	"enter":  hotkey.KeyReturn,
	"escape": hotkey.KeyEscape,
	"esc":    hotkey.KeyEscape,
	"delete": hotkey.KeyDelete,
	"del":    hotkey.KeyDelete,
	"tab":    hotkey.KeyTab,
	"left":   hotkey.KeyLeft,
	"right":  hotkey.KeyRight,
	"up":     hotkey.KeyUp,
	"down":   hotkey.KeyDown,
	"f1":     hotkey.KeyF1,
	"f2":     hotkey.KeyF2,
	"f3":     hotkey.KeyF3,
	"f4":     hotkey.KeyF4,
	"f5":     hotkey.KeyF5,
	"f6":     hotkey.KeyF6,
	"f7":     hotkey.KeyF7,
	"f8":     hotkey.KeyF8,
	"f9":     hotkey.KeyF9,
	"f10":    hotkey.KeyF10,
	"f11":    hotkey.KeyF11,
	"f12":    hotkey.KeyF12,
}

// ParseShortcut parses a shortcut string and returns the hotkey.Key and a list of modifiers.
// Returns false if parsing fails.
func ParseShortcut(shortcut string) (hotkey.Key, []hotkey.Modifier, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(shortcut)), "+")
	if len(parts) == 0 {
		return 0, nil, false
	}

	// The last part is the trigger key
	triggerName := strings.TrimSpace(parts[len(parts)-1])
	k, exists := KeyMap[triggerName]
	if !exists {
		return 0, nil, false
	}

	// The preceding parts are modifiers
	var mods []hotkey.Modifier
	for i := 0; i < len(parts)-1; i++ {
		modName := strings.TrimSpace(parts[i])
		if m, ok := ModMap[modName]; ok {
			mods = append(mods, m)
		}
	}

	return k, mods, true
}
