//go:build darwin

package main

import mac "wis-free-v3/internal/darwin"

func activeWindowTitle() string {
	return mac.ActiveApplicationName()
}
