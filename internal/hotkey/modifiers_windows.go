//go:build windows

package hotkey

import xhk "wis-free-v3/internal/xhotkey"

var ModMap = map[string]xhk.Modifier{
	"ctrl":    xhk.ModCtrl,
	"control": xhk.ModCtrl,
	"shift":   xhk.ModShift,
	"alt":     xhk.ModAlt,
	"win":     xhk.ModWin,
	"windows": xhk.ModWin,
	"meta":    xhk.ModWin,
	"super":   xhk.ModWin,
}
