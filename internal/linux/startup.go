//go:build linux

// ============================================================
// LINUX-ONLY FILE — Linux startup (autostart) management via
// XDG Desktop Entry files (.desktop). Creates/removes autostart
// entries and manages the application's .desktop file.
// The Windows equivalent is in internal/windows/
// ============================================================

package linux

import (
	"fmt"
	"os"
	"path/filepath"

	"wis-free-v3/internal/desktopentry"
)

const appName = "wis-free-v3"

func EnsureDesktopFile(iconBytes []byte) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dataDir := xdgDirectory("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	appsDir := filepath.Join(dataDir, "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return err
	}

	desktopPath := filepath.Join(appsDir, appName+".desktop")

	exePath, err := getExecutablePath()
	if err != nil {
		return err
	}

	// Write icon to local pixmaps so the desktop file Icon= field resolves
	pixmapsDir := filepath.Join(dataDir, "pixmaps")
	if err := os.MkdirAll(pixmapsDir, 0755); err != nil {
		return err
	}
	iconPath := filepath.Join(pixmapsDir, appName+".png")
	if err := os.WriteFile(iconPath, iconBytes, 0644); err != nil {
		return fmt.Errorf("failed to write icon to pixmaps: %w", err)
	}

	content := desktopentry.Application(exePath, iconPath)
	return os.WriteFile(desktopPath, []byte(content), 0644)
}

func getAutostartPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := xdgDirectory("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	autostartDir := filepath.Join(configDir, "autostart")

	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(autostartDir, appName+".desktop"), nil
}

func getExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

// XDG paths must be absolute. Ignore relative values as required by the base
// directory specification, rather than writing relative to the launcher's cwd.
func xdgDirectory(name, fallback string) string {
	if directory := os.Getenv(name); filepath.IsAbs(directory) {
		return directory
	}
	return fallback
}

func AddToStartup() error {
	autostartPath, err := getAutostartPath()
	if err != nil {
		return fmt.Errorf("failed to determine autostart path: %w", err)
	}

	exePath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Prefer a concrete icon file path if we already created one in EnsureDesktopFile.
	// Fall back to the icon name (theme lookup) if it doesn't exist.
	iconValue := appName
	if home, err := os.UserHomeDir(); err == nil {
		dataDir := xdgDirectory("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
		iconPath := filepath.Join(dataDir, "pixmaps", appName+".png")
		if _, statErr := os.Stat(iconPath); statErr == nil {
			iconValue = iconPath
		}
	}

	content := desktopentry.Autostart(exePath, iconValue)
	if err := os.WriteFile(autostartPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write autostart file: %w", err)
	}

	return nil
}

func RemoveFromStartup() error {
	autostartPath, err := getAutostartPath()
	if err != nil {
		return fmt.Errorf("failed to determine autostart path: %w", err)
	}

	if err := os.Remove(autostartPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove autostart file: %w", err)
		}
	}

	return nil
}

func IsInStartup() bool {
	autostartPath, err := getAutostartPath()
	if err != nil {
		return false
	}

	content, err := os.ReadFile(autostartPath)
	return err == nil && desktopentry.AutostartEnabled(string(content))
}
