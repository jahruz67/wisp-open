//go:build darwin

package hotkey

import xhk "wis-free-v3/internal/xhotkey"

var ModMap = map[string]xhk.Modifier{
	"ctrl":    xhk.ModCtrl,
	"control": xhk.ModCtrl,
	"shift":   xhk.ModShift,
	"alt":     xhk.ModOption,
	"win":     xhk.ModCmd,
	"windows": xhk.ModCmd,
	"meta":    xhk.ModCmd,
	"super":   xhk.ModCmd,
	"cmd":     xhk.ModCmd,
}
