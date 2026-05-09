//go:build windows

package main

// secondInstanceWake is unused on Windows (second-instance UX not wired here).
var secondInstanceWake = make(chan struct{}, 8)

func tryNotifyRunningInstanceToShow() {}

func runSecondInstanceListener() {}
