//go:build linux && cgo

package hotkey_test

import (
	"strings"
	"testing"

	"wis-free-v3/internal/xhotkey"
)

func TestHotkeyLinuxUsesDesktopShortcutFallback(t *testing.T) {
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl, hotkey.Mod2, hotkey.Mod4}, hotkey.KeyA)
	err := hk.Register()
	if err == nil {
		t.Fatal("expected Linux in-app hotkey registration to be disabled")
	}
	if !strings.Contains(err.Error(), "desktop custom-shortcut command") {
		t.Fatalf("expected desktop shortcut fallback error, got %v", err)
	}
}
