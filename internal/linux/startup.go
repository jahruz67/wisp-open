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
const desktopFileTemplate = `[Desktop Entry]
Type=Application
Name=WIS Free V3
%s
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
`

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

func AddToStartup() error {
	autostartPath, err := getAutostartPath()
	if err != nil {
		return fmt.Errorf("failed to determine autostart path: %w", err)
	}

	exePath, err := getExecutablePath()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	content := fmt.Sprintf(desktopFileTemplate, desktopExecField(exePath))
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
