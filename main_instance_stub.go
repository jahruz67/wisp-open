//go:build windows

// ============================================================
// WINDOWS-ONLY FILE — Stub for the Unix domain socket IPC.
// On Windows, single-instance enforcement uses a lock file
// (see main.go), and helper invocations don't use Unix sockets.
// The real Unix implementation is in main_instance_unix.go
// ============================================================

package main

// secondInstanceWake is unused on Windows (second-instance UX not wired here).
var secondInstanceWake = make(chan struct{}, 8)

// secondInstanceCommand is unused on Windows (second-instance UX not wired here).
var secondInstanceCommand = make(chan byte, 16)

const (
	instanceCmdShow   byte = 1
	instanceCmdStart  byte = 2
	instanceCmdStop   byte = 3
	instanceCmdToggle byte = 4
)

func tryNotifyRunningInstanceToShow() bool {
	return false
}

func tryNotifyRunningInstanceAction(action string) bool {
	return false
}

func runSecondInstanceListener() {}
func cleanupSecondInstanceListener() {}
