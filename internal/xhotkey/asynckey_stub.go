//go:build !windows

package hotkey

// AsyncKeyDown is a stub on non-Windows platforms.
func AsyncKeyDown(vk int) bool {
	return false
}
