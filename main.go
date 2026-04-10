// Package main is the entry point for wis-free-v3, a voice dictation application
// that provides global hotkey-triggered audio recording and transcription.
package main

import (
	"embed"
	"os"
	"path/filepath"
	"strconv"

	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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

func main() {
	// Ensure only one instance of the application is running
	if !acquireInstanceLock() {
		os.Exit(0)
	}

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
	})

	if err != nil {
		logger.Error("Application error: %v", err)
	}

	// Clean up resources on exit
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



