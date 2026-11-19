//go:build linux

package hotkey

import (
	"reflect"
	"testing"

	xhk "wis-free-v3/internal/xhotkey"
)

func TestLinuxControlCommandAliases(t *testing.T) {
	for _, control := range []string{"ctrl", "control", "Control"} {
		for _, command := range []string{"cmd", "command", "super", "meta", "win", "windows", "Command"} {
			shortcut := control + "+" + command
			_, mods, onlyModifiers, ok := ParseShortcut(shortcut)
			if !ok || !onlyModifiers || !reflect.DeepEqual(mods, []xhk.Modifier{xhk.ModCtrl, xhk.Mod4}) {
				t.Fatalf("ParseShortcut(%q) = %v, modifierOnly=%v, ok=%v", shortcut, mods, onlyModifiers, ok)
			}
		}
	}
}
