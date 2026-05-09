//go:build linux

package hotkey

import xhk "wis-free-v3/internal/xhotkey"

var ModMap = map[string]xhk.Modifier{
	"ctrl":    xhk.ModCtrl,
	"control": xhk.ModCtrl,
	"shift":   xhk.ModShift,
	"alt":     xhk.Mod1,
	"option":  xhk.Mod1,
	"win":     xhk.Mod4,
	"windows": xhk.Mod4,
	"meta":    xhk.Mod4,
	"super":   xhk.Mod4,
}
