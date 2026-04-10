//go:build !windows

package whisper

import "os/exec"

func hideWindowContext(cmd *exec.Cmd) {
	// Not needed on non-Windows platforms
}
