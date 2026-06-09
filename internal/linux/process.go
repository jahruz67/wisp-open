//go:build linux

// ============================================================
// LINUX-ONLY FILE — Linux process management utilities.
// Uses syscall.Kill(pid, 0) to check if a process is running.
// The Windows equivalent is in internal/windows/process.go
// ============================================================

package linux

import "syscall"

// IsProcessRunning checks if a process with the given PID exists on Linux.
func IsProcessRunning(pid int) bool {
	// sending signal 0 checks if the process exists and we have permission
	err := syscall.Kill(pid, 0)
	return err == nil
}
