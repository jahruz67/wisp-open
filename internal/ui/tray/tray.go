// Package tray provides system tray functionality for the application.
// It displays an icon in the Windows notification area with a context menu.
package tray

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"wis-free-v3/internal/config"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"

	"github.com/getlantern/systray"
)

//go:embed icon.ico
var iconData []byte

//go:embed icon.png
var iconPNGData []byte

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
var triggerCountItem *systray.MenuItem
var triggerCount int32
var iconsInitOnce sync.Once
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
	initDynamicIcons()
	trayLabel = appDisplayName(app)

	// Configure tray icon and tooltip
	systray.SetIcon(getDefaultIcon())
	systray.SetTitle(trayLabel)
	systray.SetTooltip(buildTooltip(app))

	// Build menu structure
	statusMenuItem = systray.AddMenuItem("Status: Ready", "Current application status")
	statusMenuItem.Disable()

	if runtime.GOOS != "windows" {
		triggerCountItem = systray.AddMenuItem("Shortcut detected: 0 times", "Troubleshooting counter")
		triggerCountItem.Disable()
	}

	systray.AddSeparator()

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
	shortcut := "Ctrl+K"
	if cfg := app.GetConfig(); cfg != nil && cfg.Shortcut != "" {
		shortcut = cfg.Shortcut
	}
	return trayLabel + " - " + shortcut + " to record"
}

func getDefaultIcon() []byte {
	initDynamicIcons()
	if runtime.GOOS == "linux" {
		return iconPNGData
	}
	return iconData
}

// DefaultIconBytes returns the raw default icon bytes for the current platform.
func DefaultIconBytes() []byte {
	return getDefaultIcon()
}

func icoToPNG(data []byte) ([]byte, error) {
	// Try to find a PNG image embedded inside the ICO file and return it.
	// ICO files contain entries that may be BMP/DIB or PNG encoded images.
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid ico data")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	var bestOffset int
	var bestSize int
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		if entry+16 > len(data) {
			break
		}
		bytesInRes := int(binary.LittleEndian.Uint32(data[entry+8 : entry+12]))
		imageOffset := int(binary.LittleEndian.Uint32(data[entry+12 : entry+16]))
		if bytesInRes == 0 || imageOffset+bytesInRes > len(data) {
			continue
		}
		// PNG files start with the 8-byte signature 89 50 4E 47 0D 0A 1A 0A
		if bytesInRes >= 8 && imageOffset+8 <= len(data) &&
			bytes.Equal(data[imageOffset:imageOffset+8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) {
			if bytesInRes > bestSize {
				bestSize = bytesInRes
				bestOffset = imageOffset
			}
		}
	}
	if bestSize > 0 {
		out := make([]byte, bestSize)
		copy(out, data[bestOffset:bestOffset+bestSize])
		return out, nil
	}
	return nil, fmt.Errorf("no PNG image found in ico")
}

var statusMu sync.Mutex
var lastStatus string

// UpdateStatus updates the status text displayed in the tray menu.
func UpdateStatus(status string) {
	statusMu.Lock()
	defer statusMu.Unlock()
	if statusMenuItem != nil && status != lastStatus {
		lastStatus = status
		statusMenuItem.SetTitle("Status: " + status)
		systray.SetTooltip(trayLabel + " - " + status)

		initDynamicIcons()
		if strings.Contains(status, "Recording") {
			systray.SetIcon(iconRecordingData)
		} else if isProcessingStatus(status) {
			systray.SetIcon(iconTranscribingData)
		} else {
			systray.SetIcon(getDefaultIcon())
		}
	}
}

// isProcessingStatus keeps the yellow microphone visible for every phase
// between releasing the recording shortcut and finishing text insertion.
func isProcessingStatus(status string) bool {
	return strings.Contains(status, "Processing") ||
		strings.Contains(status, "Transcribing") ||
		strings.Contains(status, "Typing")
}

// IncrementTriggerCount increments the troubleshooting counter in the tray.
func IncrementTriggerCount() {
	count := atomic.AddInt32(&triggerCount, 1)
	if triggerCountItem != nil {
		triggerCountItem.SetTitle(fmt.Sprintf("Shortcut detected: %d times", count))
	}
}

// onExit is called when the system tray is shutting down.
func onExit() {
	logger.Info("System tray terminated")
}

func initDynamicIcons() {
	iconsInitOnce.Do(func() {
		white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		red := color.RGBA{R: 255, G: 59, B: 48, A: 255}
		yellow := color.RGBA{R: 255, G: 204, B: 0, A: 255}

		iconPNGData = createMicPNG(white)
		iconRecordingData = createMicPNG(red)
		iconTranscribingData = createMicPNG(yellow)
	})
}

func createMicPNG(c color.Color) []byte {
	const S = 128 // canvas size
	img := image.NewRGBA(image.Rect(0, 0, S, S))
	// All pixels are transparent by default (new RGBA starts with 0 alpha).

	cx, cy := S/2, S/2 // center (64, 50)

	// Let's draw the microphone parts
	for y := 0; y < S; y++ {
		for x := 0; x < S; x++ {
			drawPixel := false

			// 1. Capsule Body (Rounded Rectangle)
			// Capsule center is X=64, Y=50. Width=28 (radius 14), height of straight part = 20 (Y from 40 to 60)
			if x >= 50 && x <= 78 && y >= 40 && y <= 60 {
				drawPixel = true
			} else if y < 40 {
				// Top cap: center (64, 40), radius 14
				dx := float64(x - cx)
				dy := float64(y - 40)
				if dx*dx+dy*dy <= 196 { // 14^2
					drawPixel = true
				}
			} else if y > 60 && y <= 74 {
				// Bottom cap: center (64, 60), radius 14
				dx := float64(x - cx)
				dy := float64(y - 60)
				if dx*dx+dy*dy <= 196 {
					drawPixel = true
				}
			}

			// 2. U-stand
			// Center of U-stand circle is (64, 50).
			// Outer radius = 30, inner radius = 24 (thickness 6)
			// Only draw for Y >= 50 and Y <= 80
			dx := float64(x - cx)
			dy := float64(y - cy)
			distSq := dx*dx + dy*dy
			if y >= 50 && y <= 80 && distSq >= 576 && distSq <= 900 { // 24^2 to 30^2
				drawPixel = true
			}

			// 3. Stem (Vertical line from Y=80 to 100, X=62 to 66)
			if x >= 62 && x <= 66 && y >= 80 && y <= 100 {
				drawPixel = true
			}

			// 4. Base (Horizontal line at Y=100 to 104, X=40 to 88)
			if x >= 40 && x <= 88 && y >= 100 && y <= 104 {
				drawPixel = true
			}

			if drawPixel {
				img.Set(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
