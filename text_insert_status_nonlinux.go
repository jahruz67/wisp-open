//go:build !linux

// ============================================================
// WINDOWS-ONLY FILE — Stub for the Linux ydotool status check.
// On Windows, ydotool is not used, so this returns "not ready".
// The real implementation is in text_insert_linux.go
// ============================================================

package main

func linuxYdotoolStatus() map[string]interface{} {
	return map[string]interface{}{
		"ready":       false,
		"installed":   false,
		"socket":      false,
		"socket_path": "",
		"message":     "ydotool is only used on Linux.",
	}
}
