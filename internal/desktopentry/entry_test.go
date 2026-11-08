package desktopentry

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationCommands(t *testing.T) {
	tests := []struct {
		name, path, want string
	}{
		{"package", "/usr/bin/wis-free-v3", `"/usr/bin/wis-free-v3"`},
		{"spaces and apostrophe", "/home/Test User/WIS's app", `"/home/Test User/WIS's app"`},
		{"shell characters without spaces", "/home/$USER/app`;&()", `"/home/\\$USER/app\\` + "`" + `;&()"`},
		{"quotes and backslashes", "/home/test/\"WIS\\app", `"/home/test/\\"WIS\\\\app"`},
		{"field codes", "/home/100%/app%f", `"/home/100%%/app%%f"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := Application(test.path, "wis-free-v3")
			for _, want := range []string{
				"\nExec=" + test.want + "\n",
				"\nActions=ToggleRecording;\n",
				"[Desktop Action ToggleRecording]\nName=Toggle recording\nExec=" + test.want + " --action=toggle\n",
				"\nStartupNotify=false\n",
			} {
				if !strings.Contains(content, want) {
					t.Errorf("launcher missing %q:\n%s", want, content)
				}
			}
		})
	}
}

func TestAutostartAndEscapedValues(t *testing.T) {
	content := Autostart("/home/Test User/WIS", "/home/test/icon\nname\\.png")
	for _, want := range []string{
		"Exec=\"/home/Test User/WIS\" --background\n",
		"TryExec=/home/Test User/WIS\n",
		`Icon=/home/test/icon\nname\\.png` + "\n",
		"StartupNotify=false\n",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("autostart entry missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "--action=toggle") {
		t.Error("autostart must not start a recording")
	}
}

func TestAutostartEnabled(t *testing.T) {
	for _, test := range []struct {
		name, content string
		want          bool
	}{
		{"normal", Autostart("/usr/bin/wis-free-v3", "wis-free-v3"), true},
		{"Plasma disabled", "[Desktop Entry]\nHidden=true\n", false},
		{"enabled explicitly", "[Desktop Entry]\nHidden=false\n", true},
		{"GNOME disabled", "[Desktop Entry]\nX-GNOME-Autostart-enabled=false\n", false},
		{"comments and other sections", "[Desktop Entry]\n#Hidden=true\n[Desktop Action Other]\nHidden=true\n", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := AutostartEnabled(test.content); got != test.want {
				t.Errorf("AutostartEnabled() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDesktopEntriesValidate(t *testing.T) {
	validator, err := exec.LookPath("desktop-file-validate")
	if err != nil {
		t.Skip("desktop-file-utils is required for freedesktop validation")
	}
	for _, executable := range []string{"/usr/bin/wis-free-v3", "/home/Test User/WIS's app", "/home/$USER/100%/WIS\"app\\file"} {
		for _, content := range []string{Application(executable, "wis-free-v3"), Autostart(executable, "wis-free-v3")} {
			path := filepath.Join(t.TempDir(), "wis-free-v3.desktop")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(validator, path).CombinedOutput(); err != nil {
				t.Fatalf("desktop-file-validate: %v\n%s\n%s", err, out, content)
			}
		}
	}
}
