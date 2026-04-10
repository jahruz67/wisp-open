//go:build linux

package hotkey

import "wis-free-v3/internal/xhotkey"

var ModMap = map[string]hotkey.Modifier{
	"ctrl":    hotkey.ModCtrl,
	"control": hotkey.ModCtrl,
	"shift":   hotkey.ModShift,
	"alt":     hotkey.Mod1,
	"win":     hotkey.Mod4,
	"windows": hotkey.Mod4,
	"meta":    hotkey.Mod4,
	"super":   hotkey.Mod4,
}
