//go:build linux
package linux

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "wis-free-v3"

// desktopFileTemplate is filled with execLine built from the absolute binary path.
// Paths with spaces must be quoted per the Desktop Entry spec.
const baseDesktopFileTemplate = `[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Recording and Transcription
%s
%s
Icon=%s
StartupWMClass=%s
Terminal=false
Categories=Utility;
Keywords=voice;recorder;transcription;
`

const autostartDesktopFileTemplate = `[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Recording and Transcription
%s
%s
Icon=%s
StartupWMClass=%s
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
`

func EnsureDesktopFile(iconBytes []byte) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	appsDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return err
	}

	desktopPath := filepath.Join(appsDir, appName+".desktop")

	exePath, err := getExecutablePath()
	if err != nil {
		return err
	}

	// Write icon to local pixmaps so the desktop file Icon= field resolves
	pixmapsDir := filepath.Join(home, ".local", "share", "pixmaps")
	if err := os.MkdirAll(pixmapsDir, 0755); err != nil {
		return err
	}
	iconPath := filepath.Join(pixmapsDir, appName+".png")
	if err := os.WriteFile(iconPath, iconBytes, 0644); err != nil {
		return fmt.Errorf("failed to write icon to pixmaps: %w", err)
	}

	content := fmt.Sprintf(baseDesktopFileTemplate, desktopExecField(exePath), desktopTryExecField(exePath), iconPath, appName)
	return os.WriteFile(desktopPath, []byte(content), 0644)
}

func getAutostartPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}
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

// desktopExecField returns one line: Exec=/path or Exec="/path with spaces"
func desktopExecField(exePath string) string {
	needsQuote := false
	for _, r := range exePath {
		if r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '\\' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return "Exec=" + exePath
	}
	escaped := ""
	for _, r := range exePath {
		if r == '"' || r == '`' || r == '$' || r == '\\' {
			escaped += `\`
		}
		escaped += string(r)
	}
	return `Exec="` + escaped + `"`
}

// desktopTryExecField returns one line: TryExec=/path (desktop entries do not accept quoting for TryExec)
func desktopTryExecField(exePath string) string {
	return "TryExec=" + exePath
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
		iconPath := filepath.Join(home, ".local", "share", "pixmaps", appName+".png")
		if _, statErr := os.Stat(iconPath); statErr == nil {
			iconValue = iconPath
		}
	}

	content := fmt.Sprintf(autostartDesktopFileTemplate, desktopExecField(exePath), desktopTryExecField(exePath), iconValue, appName)
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
	
	_, err = os.Stat(autostartPath)
	return err == nil
}
