// Package desktopentry builds freedesktop launchers shared by Linux installs
// and the running app. Exec values have both Desktop Entry string escaping
// and command-line escaping; shell quoting alone is not sufficient.
package desktopentry

import (
	"fmt"
	"strings"
)

const applicationTemplate = `[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Recording and Transcription
Exec=%s
TryExec=%s
Icon=%s
StartupWMClass=wis-free-v3
StartupNotify=false
Terminal=false
Categories=Utility;Audio;
Keywords=dictation;speech;voice;transcription;whisper;
Actions=ToggleRecording;

[Desktop Action ToggleRecording]
Name=Toggle recording
Exec=%s --action=toggle
`

// Application includes an action Plasma can expose in System Settings >
// Keyboard > Shortcuts. No default key binding is claimed on the user's behalf.
func Application(executable, icon string) string {
	command := execArgument(executable)
	return fmt.Sprintf(applicationTemplate, command, stringValue(executable), stringValue(icon), command)
}

// Autostart starts in the tray without opening Settings or requesting focus.
func Autostart(executable, icon string) string {
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Recording and Transcription
Exec=%s --background
TryExec=%s
Icon=%s
StartupWMClass=wis-free-v3
StartupNotify=false
Terminal=false
X-GNOME-Autostart-enabled=true
`, execArgument(executable), stringValue(executable), stringValue(icon))
}

func execArgument(value string) string {
	// The launcher unescapes the string value before parsing the quoted
	// argument, then expands field codes. A literal percent must survive that
	// last step even when the binary's directory contains e.g. "%f".
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`, "%", "%%").Replace(value)
	return stringValue(`"` + escaped + `"`)
}

func stringValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value)
}

// AutostartEnabled respects a desktop's disabled entry instead of treating
// every existing .desktop file as enabled (Plasma writes Hidden=true).
func AutostartEnabled(content string) bool {
	inDesktopEntry := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inDesktopEntry = line == "[Desktop Entry]"
			continue
		}
		if !inDesktopEntry {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if (key == "Hidden" && strings.EqualFold(value, "true")) ||
			(key == "X-GNOME-Autostart-enabled" && strings.EqualFold(value, "false")) {
			return false
		}
	}
	return true
}
