//go:build windows
package windows

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// Windows Registry constants
const (
	registryPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	appName      = "WISNative"
)

// AddToStartup adds the current executable to Windows startup.
// The application will start automatically when the user logs in.
func AddToStartup() error {
	exePath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		registryPath,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(appName, exePath); err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}

// RemoveFromStartup removes the application from Windows startup.
func RemoveFromStartup() error {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		registryPath,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(appName); err != nil {
		// Ignore error if value doesn't exist
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete registry value: %w", err)
		}
	}

	return nil
}

// IsInStartup checks if the application is configured to start with Windows.
func IsInStartup() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		registryPath,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	return err == nil
}

// getExecutablePath returns the absolute path to the current executable.
func getExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}


