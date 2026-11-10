//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFilesUseXDGDirectories(t *testing.T) {
	home := t.TempDir()
	data := filepath.Join(home, "custom data")
	config := filepath.Join(home, "custom config")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", config)
	if err := EnsureDesktopFile([]byte("icon")); err != nil {
		t.Fatal(err)
	}
	launcher, err := os.ReadFile(filepath.Join(data, "applications", appName+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(launcher), "[Desktop Action ToggleRecording]") {
		t.Fatal("app startup dropped the packaged recording action")
	}
	if _, err := os.Stat(filepath.Join(data, "pixmaps", appName+".png")); err != nil {
		t.Fatal(err)
	}
	if err := AddToStartup(); err != nil {
		t.Fatal(err)
	}
	if !IsInStartup() {
		t.Fatal("new autostart entry is not enabled")
	}
	autostart := filepath.Join(config, "autostart", appName+".desktop")
	if err := os.WriteFile(autostart, []byte("[Desktop Entry]\nHidden=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if IsInStartup() {
		t.Fatal("Plasma's disabled autostart entry is reported as enabled")
	}
	if err := RemoveFromStartup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(autostart); !os.IsNotExist(err) {
		t.Fatalf("autostart file was not removed: %v", err)
	}
}

func TestXDGDirectoryRejectsRelativePaths(t *testing.T) {
	fallback := t.TempDir()
	for _, value := range []string{"", "relative/path"} {
		t.Setenv("XDG_DATA_HOME", value)
		if got := xdgDirectory("XDG_DATA_HOME", fallback); got != fallback {
			t.Fatalf("xdgDirectory(%q) = %q, want %q", value, got, fallback)
		}
	}
}
