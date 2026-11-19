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
	"fmt"
	"os"
	"strings"

	"wis-free-v3/internal/globalshortcuts"
	"wis-free-v3/internal/hotkey"
	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// Hold-to-record polls only modifier key state. The XDG GlobalShortcuts
// trigger syntax requires a non-modifier key, so it cannot represent Ctrl+Super.
// The optional desktop command above remains a separate toggle shortcut.
func (a *App) startLinuxPressDaemon() {
	if !a.config.LinuxShortcutEnabled {
		a.setLinuxShortcutStatus(globalshortcuts.Status{Message: "Ctrl+Super hold-to-record is disabled in this profile."})
		return
	}
	a.startLinuxModifierShortcut()
}

func (a *App) GetLinuxShortcutStatus() globalshortcuts.Status {
	a.linuxShortcutMu.RLock()
	status := a.linuxShortcut
	a.linuxShortcutMu.RUnlock()
	if status.Message == "" {
		return globalshortcuts.Status{Busy: true, Message: "Checking Ctrl+Super keyboard access…", Trigger: "Ctrl+Super"}
	}
	return status
}

func (a *App) SetupLinuxShortcut() globalshortcuts.Status {
	if a.hotkeyListener != nil {
		a.hotkeyListener.Stop()
	}
	a.startLinuxModifierShortcut()
	return a.GetLinuxShortcutStatus()
}

func (a *App) startLinuxModifierShortcut() {
	a.setLinuxShortcutStatus(globalshortcuts.Status{Busy: true, Message: "Checking Ctrl+Super keyboard access…", Trigger: "Ctrl+Super"})
	listener := hotkey.NewListener(a.config.Shortcut, a.StartRecording, a.StopRecording)
	listener.SetRegistrationReadyCallback(func() {
		a.setLinuxShortcutStatus(globalshortcuts.Status{Ready: true, Message: "Ctrl+Super is ready. Hold both keys to record; release to transcribe and type.", Trigger: "Ctrl+Super"})
	})
	listener.SetRegistrationErrorCallback(func(err error) {
		a.setLinuxShortcutStatus(globalshortcuts.Status{Message: fmt.Sprintf("Ctrl+Super is not active: %v", err), Trigger: "Ctrl+Super"})
		logger.Error("Linux hold-to-record shortcut unavailable: %v", err)
	})
	a.hotkeyListener = listener
	listener.Start()
}

func (a *App) setLinuxShortcutStatus(status globalshortcuts.Status) {
	a.linuxShortcutMu.Lock()
	a.linuxShortcut = status
	a.linuxShortcutMu.Unlock()
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "linux:shortcut-status", status)
	}
}

func (a *App) stopLinuxPressDaemon() {
	if a.shortcutPortal != nil {
		a.shortcutPortal.Close()
	}
}
