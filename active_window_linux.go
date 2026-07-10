//go:build linux

package main

// activeWindowTitle intentionally avoids robotgo on Linux. robotgo reads X11
// state directly and can fail or emit noisy errors on Wayland. Refinement is
// optional, so an empty application-context string is safer than risking a
// failed transcription after recording has completed.
func activeWindowTitle() string {
	return ""
}
