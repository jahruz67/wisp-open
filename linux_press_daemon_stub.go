//go:build !linux

// ============================================================
// WINDOWS-ONLY FILE — Stub for the Linux press daemon.
// On Windows, the press daemon is not used.
// The real implementation is in linux_press_daemon.go
// ============================================================

package main

import "wis-free-v3/internal/globalshortcuts"

func (a *App) startLinuxPressDaemon() {}

func (a *App) stopLinuxPressDaemon() {}

func (a *App) GetLinuxShortcutStatus() globalshortcuts.Status { return globalshortcuts.Status{} }
func (a *App) SetupLinuxShortcut() globalshortcuts.Status     { return globalshortcuts.Status{} }

func linuxShortcutCommand(executable string) string {
	return executable + " --action=toggle"
}
