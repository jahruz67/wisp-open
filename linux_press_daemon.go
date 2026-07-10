//go:build linux

// Linux shortcut compatibility helpers.
//
// Older releases used a loopback HTTP server for the `--press` command. That
// made a normal desktop shortcut unreliable: a second key press inside the
// hold-detection window could be mistaken for key auto-repeat, and another
// local process could occupy the fixed TCP port. Linux already has a
// per-user Unix socket for single-instance IPC, so use that authenticated
// per-user path instead.
package main

import (
	"os"
	"strings"
)

func init() {
	// Keep `--press` working for existing custom shortcuts. A desktop shortcut
	// invokes a command once, so its correct, predictable behaviour is toggle
	// (start on the first press and stop on the next), not push-to-talk.
	if hasPressFlag(os.Args[1:]) {
		if tryNotifyRunningInstanceAction("toggle") {
			os.Exit(0)
		}
		// Do not launch a second GUI process from this legacy helper. The current
		// settings UI emits --action=toggle, which can start the app if needed.
		os.Exit(1)
	}
}

func hasPressFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--press" {
			return true
		}
	}
	return false
}

// linuxShortcutCommand returns a shell-safe command for GNOME/KDE custom
// shortcuts. Quote the executable because user home directories can contain
// spaces or shell metacharacters.
func linuxShortcutCommand(executable string) string {
	return linuxShellQuote(executable) + " --action=toggle"
}

func linuxShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// The old daemon API remains a no-op so the common lifecycle code and
// non-Linux stub stay simple. Shortcut delivery now uses the instance socket.
func (a *App) startLinuxPressDaemon() {}

func stopLinuxPressDaemon() {}
