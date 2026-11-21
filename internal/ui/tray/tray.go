//go:build !darwin

// Package tray provides system tray functionality for the application.
// It displays an icon in the Windows notification area with a context menu.
package tray

import (
	"wis-free-v3/internal/config"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"

	"github.com/getlantern/systray"
)

// App defines the interface required by the tray package to interact with the main application.
type App interface {
	Quit()
	GetConfig() *config.Config
	ShowSettings()
	Version() string
}

// trayLabel is the short product name shown in the tray (includes version when set at link time).
var trayLabel = "wis-free-v3"

// Menu item references for dynamic updates
var startupMenuItem *systray.MenuItem

// onStartupChanged is called when the tray startup menu item is toggled.
// It receives the new enabled state. The callback is set by the app layer.
var onStartupChanged func(bool)

func appDisplayName(app App) string {
	v := app.Version()
	if v != "" && v != "dev" {
		return "wis-free-v3 v" + v
	}
	return "wis-free-v3"
}

// onReady is called when the system tray is ready to be configured.
func onReady(app App) {
	statusMu.Lock()
	defer statusMu.Unlock()
	trayLabel = appDisplayName(app)

	// Configure tray icon and tooltip
	systray.SetIcon(DefaultIconBytes())
	systray.SetTitle(trayLabel)
	systray.SetTooltip(trayLabel + " - " + lastStatus)

	// Build menu structure
	menuSettings := systray.AddMenuItem("Settings", "Open settings window")
	menuStartup := systray.AddMenuItemCheckbox(
		"Start with system",
		"Automatically start when computer boots",
		platform.IsInStartup(),
	)

	systray.AddSeparator()

	menuExit := systray.AddMenuItem("Exit", "Close the application")

	// Store reference for external updates
	startupMenuItem = menuStartup

	// Handle menu events in background
	go handleMenuEvents(app, menuSettings, menuStartup, menuExit)
}

// handleMenuEvents processes menu item click events.
func handleMenuEvents(app App, settings, startupItem, exit *systray.MenuItem) {
	for {
		select {
		case <-settings.ClickedCh:
			go app.ShowSettings()

		case <-startupItem.ClickedCh:
			go toggleStartup(startupItem)

		case <-exit.ClickedCh:
			go handleExit(app)
		}
	}
}

// toggleStartup handles the startup toggle menu item.
func toggleStartup(item *systray.MenuItem) {
	if item.Checked() {
		if err := platform.RemoveFromStartup(); err != nil {
			logger.Error("Failed to remove from startup: %v", err)
		} else {
			item.Uncheck()
			logger.Info("Removed from system startup")
			notifyStartupChanged(false)
		}
	} else {
		if err := platform.AddToStartup(); err != nil {
			logger.Error("Failed to add to startup: %v", err)
		} else {
			item.Check()
			logger.Info("Added to system startup")
			notifyStartupChanged(true)
		}
	}
}

// notifyStartupChanged calls the onStartupChanged callback if set.
func notifyStartupChanged(enabled bool) {
	if onStartupChanged != nil {
		onStartupChanged(enabled)
	}
}

// SetOnStartupChanged registers a callback that is invoked when the tray
// startup menu item is toggled. The callback receives the new enabled state.
func SetOnStartupChanged(fn func(bool)) {
	onStartupChanged = fn
}

// SetStartupChecked updates the tray startup menu item checkbox state.
// This is used to keep the tray in sync when the startup option is changed
// from the settings UI.
func SetStartupChecked(checked bool) {
	statusMu.Lock()
	defer statusMu.Unlock()
	if startupMenuItem == nil {
		return
	}
	if checked {
		startupMenuItem.Check()
	} else {
		startupMenuItem.Uncheck()
	}
}

// handleExit cleanly shuts down the application.
// Calls app.Quit() and lets wails run deferred shutdown handlers instead of
// os.Exit(0) which would skip instance-lock / socket cleanup in main().
func handleExit(app App) {
	logger.Info("User requested application exit")
	systray.Quit()
	app.Quit()
}

// buildTooltip creates the tray icon tooltip text.
func buildTooltip(app App) string {
	shortcut := config.DefaultShortcut
	if cfg := app.GetConfig(); cfg != nil && cfg.Shortcut != "" {
		shortcut = cfg.Shortcut
	}
	return trayLabel + " - " + shortcut + " to record"
}

// UpdateStatus updates the current status and tray icon tooltip.
func UpdateStatus(status string) {
	statusMu.Lock()
	defer statusMu.Unlock()
	lastStatus = status
	if startupMenuItem != nil {
		systray.SetTooltip(trayLabel + " - " + status)
	}
}

// onExit is called when the system tray is shutting down.
func onExit() {
	logger.Info("System tray terminated")
}

