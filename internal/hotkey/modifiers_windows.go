//go:build windows

package hotkey

import "wis-free-v3/internal/xhotkey"

var ModMap = map[string]hotkey.Modifier{
	"ctrl":    hotkey.ModCtrl,
	"control": hotkey.ModCtrl,
	"shift":   hotkey.ModShift,
	"alt":     hotkey.ModAlt,
	"win":     hotkey.ModWin,
	"windows": hotkey.ModWin,
	"meta":    hotkey.ModWin,
	"super":   hotkey.ModWin,
}
