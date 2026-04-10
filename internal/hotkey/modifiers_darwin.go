//go:build darwin

package hotkey

import "wis-free-v3/internal/xhotkey"

var ModMap = map[string]hotkey.Modifier{
	"ctrl":    hotkey.ModCtrl,
	"control": hotkey.ModCtrl,
	"shift":   hotkey.ModShift,
	"alt":     hotkey.ModOption,
	"win":     hotkey.ModCmd,
	"windows": hotkey.ModCmd,
	"meta":    hotkey.ModCmd,
	"super":   hotkey.ModCmd,
	"cmd":     hotkey.ModCmd,
}
