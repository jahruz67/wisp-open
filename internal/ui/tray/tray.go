// Package tray provides system tray functionality for the application.
// It displays an icon in the Windows notification area with a context menu.
package tray

import (
	_ "embed"
	"os"
	"strings"

	"wis-free-v3/internal/config"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

//go:embed icon_recording.png
var iconRecordingData []byte

//go:embed icon_transcribing.png
var iconTranscribingData []byte

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
var statusMenuItem *systray.MenuItem

// Start initializes and runs the system tray.
// This function blocks until the tray is terminated.
func Start(app App) {
	systray.Run(
		func() { onReady(app) },
		onExit,
	)
}

func appDisplayName(app App) string {
	v := app.Version()
	if v != "" && v != "dev" {
		return "wis-free-v3 v" + v
	}
	return "wis-free-v3"
}

// onReady is called when the system tray is ready to be configured.
func onReady(app App) {
	trayLabel = appDisplayName(app)

	// Configure tray icon and tooltip
	systray.SetIcon(iconData)
	systray.SetTitle(trayLabel)
	systray.SetTooltip(buildTooltip(app))

	// Build menu structure
	statusMenuItem = systray.AddMenuItem("Status: Ready", "Current application status")
	statusMenuItem.Disable()

	systray.AddSeparator()

	menuSettings := systray.AddMenuItem("Settings", "Open settings window")
	menuStartup := systray.AddMenuItemCheckbox(
		"Start with system",
		"Automatically start when computer boots",
		platform.IsInStartup(),
	)

	systray.AddSeparator()

	menuExit := systray.AddMenuItem("Exit", "Close the application")

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
		}
	} else {
		if err := platform.AddToStartup(); err != nil {
			logger.Error("Failed to add to startup: %v", err)
		} else {
			item.Check()
			logger.Info("Added to system startup")
		}
	}
}

// handleExit cleanly shuts down the application.
func handleExit(app App) {
	logger.Info("User requested application exit")
	app.Quit()
	systray.Quit()
	os.Exit(0)
}

// buildTooltip creates the tray icon tooltip text.
func buildTooltip(app App) string {
	shortcut := "Ctrl+K"
	if cfg := app.GetConfig(); cfg != nil && cfg.Shortcut != "" {
		shortcut = cfg.Shortcut
	}
	return trayLabel + " - " + shortcut + " to record"
}

// UpdateStatus updates the status text displayed in the tray menu.
func UpdateStatus(status string) {
	if statusMenuItem != nil {
		statusMenuItem.SetTitle("Status: " + status)
		systray.SetTooltip(trayLabel + " - " + status)

		if strings.Contains(status, "Recording") {
			systray.SetIcon(iconRecordingData)
		} else if strings.Contains(status, "Transcribing") {
			systray.SetIcon(iconTranscribingData)
		} else {
			systray.SetIcon(iconData)
		}
	}
}

// onExit is called when the system tray is shutting down.
func onExit() {
	logger.Info("System tray terminated")
}


