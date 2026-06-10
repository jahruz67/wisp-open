//go:build !linux

// ============================================================
// WINDOWS-ONLY FILE — Stub for the Linux press daemon.
// On Windows, the press daemon is not used.
// The real implementation is in linux_press_daemon.go
// ============================================================

package main

func (a *App) startLinuxPressDaemon() {}

func stopLinuxPressDaemon() {}
