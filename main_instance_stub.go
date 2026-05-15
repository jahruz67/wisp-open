//go:build windows

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
