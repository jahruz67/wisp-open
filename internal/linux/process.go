//go:build linux
package linux

import "syscall"

// IsProcessRunning checks if a process with the given PID exists on Linux.
func IsProcessRunning(pid int) bool {
	// sending signal 0 checks if the process exists and we have permission
	err := syscall.Kill(pid, 0)
	return err == nil
}
