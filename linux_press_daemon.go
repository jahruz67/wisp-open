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

	"wis-free-v3/internal/config"
	"wis-free-v3/internal/globalshortcuts"
	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// XDG shortcut syntax for Ctrl+Super.
const linuxDefaultShortcutTrigger = "CTRL+Super_L"

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

// Hold-to-record uses real Activated/Deactivated events from the desktop. The
// command above remains an explicitly labelled optional toggle shortcut.
func (a *App) startLinuxPressDaemon() {
	a.shortcutPortal = globalshortcuts.New(a.StartRecording, a.StopRecording, func(status globalshortcuts.Status) {
		if status.Ready && !a.config.LinuxShortcutEnabled {
			a.config.LinuxShortcutEnabled = true
			if err := config.Save(a.config, ""); err != nil {
				logger.Error("Could not save Linux shortcut preference: %v", err)
			}
		}
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "linux:shortcut-status", status)
		}
	})
	if a.config.LinuxShortcutEnabled {
		go a.shortcutPortal.Start(linuxDefaultShortcutTrigger)
	}
}

func (a *App) GetLinuxShortcutStatus() globalshortcuts.Status {
	if a.shortcutPortal == nil {
		return globalshortcuts.Status{Message: "Shortcut service is starting…"}
	}
	return a.shortcutPortal.Status()
}

func (a *App) SetupLinuxShortcut() globalshortcuts.Status {
	if a.shortcutPortal == nil {
		return a.GetLinuxShortcutStatus()
	}
	return a.shortcutPortal.Configure(linuxDefaultShortcutTrigger)
}

func (a *App) stopLinuxPressDaemon() {
	if a.shortcutPortal != nil {
		a.shortcutPortal.Close()
	}
}
