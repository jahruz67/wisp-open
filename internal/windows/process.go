//go:build windows
package windows

import (
	"errors"
	"syscall"
)

// IsProcessRunning checks if a process with the given PID exists on Windows.
// Uses GetExitCodeProcess to distinguish "not running" from "running but
// access denied" (protected/system processes), avoiding false negatives
// that would allow duplicate instances to launch.
func IsProcessRunning(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// ACCESS_DENIED means the process is running but we can't query it.
		// Treat that as "running" to avoid clobbering the lock file.
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return true
		}
		return false
	}
	defer syscall.CloseHandle(handle)

	// Still confirm the process hasn't exited by checking its exit code.
	// STILL_ACTIVE (259) is the documented value for a live process.
	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return true // We got a handle, so the process exists.
	}
	return exitCode == 259
}
