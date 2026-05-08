//go:build windows

package hotkey

import "wis-free-v3/internal/xhotkey/internal/win"

// AsyncKeyDown reports whether the virtual key is currently held (high bit of GetAsyncKeyState).
func AsyncKeyDown(vk int) bool {
	return win.GetAsyncKeyState(vk)&0x8000 != 0
}
