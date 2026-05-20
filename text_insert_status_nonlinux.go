//go:build !linux

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
