// Package main is the entry point for wis-free-v3, a voice dictation application
// that provides global hotkey-triggered audio recording and transcription.
package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// Application constants
const (
	appName   = "wis-free-v3"
	appTitle  = "wis-free-v3 Settings"
	appWidth  = 800
	appHeight = 700
	lockFile  = "wis-free-v3.lock"
	configDir = ".wis-free-v3"
)

// Background color for the application window (matches new deep dark theme #0b0f1a)
var windowBackground = &options.RGBA{R: 11, G: 15, B: 26, A: 255}

//go:embed all:frontend/dist
var assets embed.FS

// Global lock file handle for single instance management
var instanceLock *os.File

// AppVersion is set at link time by scripts/build.bat and scripts/build-linux.sh
// from scripts/VERSION. Default is used for plain `go build` / `wails build`.
var AppVersion = "dev"

var initialAction string

func main() {
	// systray's package init calls runtime.LockOSThread() on the program's startup
	// thread. Undo that so the main goroutine is not permanently bound; the tray
	// goroutine locks itself in tray.Start instead (see internal/ui/tray/tray.go).
	runtime.UnlockOSThread()

	// If we are being used as a helper command (e.g. GNOME custom shortcut),
	// signal the running instance and exit.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--action=") {
			action := strings.TrimPrefix(arg, "--action=")
			initialAction = action
			if tryNotifyRunningInstanceAction(action) {
				os.Exit(0)
			}
		}
	}

	// Ensure only one instance of the application is running
	if !acquireInstanceLock() {
		// Another copy is already running (often left in the tray). Without this,
		// we would exit silently and the user sees "nothing happens" when clicking
		// the launcher again.
		_ = tryNotifyRunningInstanceToShow()
		os.Exit(0)
	}
	runSecondInstanceListener()

	// Initialize the application
	app := NewApp()

	// Configure and run the Wails application
	err := wails.Run(&options.App{
		Title:            appTitle,
		Width:            appWidth,
		Height:           appHeight,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: windowBackground,
		OnStartup:        app.startup,
		OnShutdown:       app.Shutdown,
		OnBeforeClose:    app.beforeClose,
		StartHidden:      true,
		Bind:             []interface{}{app},
		// PLATFORM NOTE: Linux-specific Wails options (sets the program name
		// for desktop integration). These options are only applied on Linux.
		Linux: &linux.Options{
			ProgramName: "wis-free-v3",
		},
	})

	if err != nil {
		logger.Error("Application error: %v", err)
	}

	// Clean up resources on exit
	cleanupSecondInstanceListener()
	releaseInstanceLock()
}

// acquireInstanceLock attempts to acquire an exclusive lock to prevent multiple instances.
// It stores the current process ID in the lock file and checks if any existing lock
// belongs to a still-running process.
//
// Returns true if the lock was acquired successfully, false if another instance is running.
func acquireInstanceLock() bool {
	lockPath, err := getLockPath()
	if err != nil {
		logger.Error("Failed to get lock path: %v", err)
		return true // Allow running if we can't determine the path
	}

	// Ensure the config directory exists
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		logger.Error("Failed to create config directory: %v", err)
		return true
	}

	// Check for existing lock file
	if data, err := os.ReadFile(lockPath); err == nil {
		if pid, err := strconv.Atoi(string(data)); err == nil {
			if platform.IsProcessRunning(pid) {
				logger.Info("Another instance is already running (PID: %d)", pid)
				return false
			}
		}
		// Stale lock file from a crashed process - remove it
		os.Remove(lockPath)
	}

	// Create new lock file with our PID
	instanceLock, err = os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		logger.Error("Failed to create lock file: %v", err)
		return true
	}

	// Write current process ID
	pid := os.Getpid()
	if _, err := instanceLock.WriteString(strconv.Itoa(pid)); err != nil {
		logger.Error("Failed to write PID to lock file: %v", err)
	}
	instanceLock.Sync()

	logger.Info("Instance lock acquired (PID: %d)", pid)
	return true
}

// releaseInstanceLock removes the lock file and closes the file handle.
func releaseInstanceLock() {
	if instanceLock != nil {
		instanceLock.Close()
		instanceLock = nil
	}

	if lockPath, err := getLockPath(); err == nil {
		os.Remove(lockPath)
		logger.Info("Instance lock released")
	}
}

// getLockPath returns the full path to the instance lock file.
func getLockPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, configDir, lockFile), nil
}
