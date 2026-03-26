// Package hotkey provides global keyboard shortcut detection and handling.
package hotkey

import "strings"

// Windows Virtual Key Codes
// See: https://docs.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes
const (
	VK_LCTRL  = 162 // Left Control
	VK_RCTRL  = 163 // Right Control
	VK_LSHIFT = 160 // Left Shift
	VK_RSHIFT = 161 // Right Shift
	VK_LALT   = 164 // Left Alt (Menu)
	VK_RALT   = 165 // Right Alt (Menu)
	VK_LWIN   = 91  // Left Windows key
	VK_RWIN   = 92  // Right Windows key
)

// keyMap maps human-readable key names to Windows Virtual Key Codes.
// Each key may have multiple valid codes (e.g., left and right variants).
var keyMap = map[string][]uint16{
	// Modifier keys
	"ctrl":    {VK_LCTRL, VK_RCTRL, 17},
	"control": {VK_LCTRL, VK_RCTRL, 17},
	"shift":   {VK_LSHIFT, VK_RSHIFT, 16},
	"alt":     {VK_LALT, VK_RALT, 18},
	"win":     {VK_LWIN, VK_RWIN, 91, 92},
	"windows": {VK_LWIN, VK_RWIN, 91, 92},
	"meta":    {VK_LWIN, VK_RWIN, 91, 92},
	"super":   {VK_LWIN, VK_RWIN, 91, 92},

	// Special keys
	"backspace": {8},
	"tab":       {9},
	"enter":     {13},
	"escape":    {27},
	"esc":       {27},
	"space":     {32},
	"left":      {37},
	"up":        {38},
	"right":     {39},
	"down":      {40},
	"delete":    {46},
	"del":       {46},

	// Number keys (top row)
	"0": {48}, "1": {49}, "2": {50}, "3": {51}, "4": {52},
	"5": {53}, "6": {54}, "7": {55}, "8": {56}, "9": {57},

	// Letter keys
	"a": {65}, "b": {66}, "c": {67}, "d": {68}, "e": {69},
	"f": {70}, "g": {71}, "h": {72}, "i": {73}, "j": {74},
	"k": {75}, "l": {76}, "m": {77}, "n": {78}, "o": {79},
	"p": {80}, "q": {81}, "r": {82}, "s": {83}, "t": {84},
	"u": {85}, "v": {86}, "w": {87}, "x": {88}, "y": {89},
	"z": {90},

	// Function keys
	"f1": {112}, "f2": {113}, "f3": {114}, "f4": {115},
	"f5": {116}, "f6": {117}, "f7": {118}, "f8": {119},
	"f9": {120}, "f10": {121}, "f11": {122}, "f12": {123},
}

// modifierKeys identifies which key names are modifiers.
var modifierKeys = map[string]bool{
	"ctrl": true, "control": true,
	"shift": true,
	"alt":   true,
	"win":   true, "windows": true, "meta": true, "super": true,
}

// ParseShortcut parses a shortcut string into a trigger key and modifier keys.
// The shortcut format is "modifier+modifier+key" (e.g., "ctrl+shift+k").
//
// Returns:
//   - trigger: the primary key code that activates the shortcut
//   - modifiers: slice of modifier key code variants that must be held
func ParseShortcut(shortcut string) (trigger []uint16, modifiers [][]uint16) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(shortcut)), "+")
	if len(parts) == 0 {
		return nil, nil
	}

	// Last part is always the trigger key
	triggerName := strings.TrimSpace(parts[len(parts)-1])
	if codes, ok := keyMap[triggerName]; ok && len(codes) > 0 {
		trigger = codes
	}

	// Preceding parts are modifiers
	for i := 0; i < len(parts)-1; i++ {
		modName := strings.TrimSpace(parts[i])
		if codes, ok := keyMap[modName]; ok {
			modifiers = append(modifiers, codes)
		}
	}

	return trigger, modifiers
}

// IsModifier returns true if the given key name is a modifier key.
func IsModifier(keyName string) bool {
	return modifierKeys[strings.ToLower(keyName)]
}

// GetKeyCode returns the virtual key code(s) for a given key name.
// Returns nil if the key name is not recognized.
func GetKeyCode(keyName string) []uint16 {
	return keyMap[strings.ToLower(keyName)]
}


