//go:build linux

// ============================================================
// LINUX-ONLY FILE — This file compiles ONLY on Linux.
// Any changes here will NOT affect the Windows build.
// For the Windows equivalent, see text_insert_nonlinux.go
// ============================================================

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wis-free-v3/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) insertTranscription(text string) {
	a.releaseLinuxInputFocus()

	if err := typeLinuxTextWithYdotool(text); err == nil {
		logger.Info("Typed transcription on Linux using ydotool (%d chars)", utf8.RuneCountInString(text))
		return
	} else {
		logger.Error("Linux direct typing unavailable via ydotool: %v", err)
	}

	logger.Info("Transcription was not inserted; install ydotool with ydotoold/uinput access for direct Linux typing")
	if a.overlay != nil {
		a.overlay.Show("Direct typing unavailable. Check ydotool setup.")
	}
}

func (a *App) releaseLinuxInputFocus() {
	if a.ctx == nil {
		time.Sleep(30 * time.Millisecond)
		return
	}
	wailsruntime.WindowHide(a.ctx)
	time.Sleep(50 * time.Millisecond)
}

func typeLinuxTextWithYdotool(text string) error {
	path, socketPath, err := getYdotoolCommand()
	if err != nil {
		return err
	}

	timeout := linuxTextTyperTimeout(text)
	// ydotool has changed option parsing across releases. Use stdin to avoid
	// shell quoting and argument-size issues, but keep a direct-argument
	// fallback for older packages whose `type --file` support is incomplete.
	// Remove the spacing between typed characters without changing how long
	// each key is held down. Keep default-delay attempts for older versions
	// that do not support the key-delay option.
	attempts := [][]string{
		{"type", "-d", "0", "--file", "-"},
		{"type", "-d", "0", "-f", "-"},
		{"type", "-f", "-"},
		{"type", "--file", "-"},
	}
	var lastErr error
	for _, args := range attempts {
		if err := runLinuxInputCommand(path, args, text, timeout, socketPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	// Do not turn a leading dash into an option. Transcriptions are normally
	// small enough for argv and exec.Command passes this value without a shell.
	if text != "" && !strings.HasPrefix(text, "-") && !strings.ContainsRune(text, '\x00') {
		directAttempts := [][]string{
			{"type", "-d", "0", text},
			{"type", text},
		}
		for _, args := range directAttempts {
			if err := runLinuxInputCommand(path, args, "", timeout, socketPath); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
	}

	return lastErr
}

func getYdotoolCommand() (string, string, error) {
	path, err := findLinuxCommand("ydotool")
	if err != nil {
		return "", "", errors.New("ydotool not found; install ydotool and start its service")
	}

	socketPath, err := getYdotoolSocketPath()
	if err != nil {
		return "", "", err
	}

	return path, socketPath, nil
}

func linuxYdotoolStatus() map[string]interface{} {
	distroID, distroName, distroLike := linuxDistribution()
	status := map[string]interface{}{
		"ready":            false,
		"installed":        false,
		"daemon_installed": false,
		"socket":           false,
		"socket_path":      "",
		"issue":            "not_installed",
		"message":          "",
		"distro":           distroName,
		"setup_commands":   []string{},
	}

	_, err := findLinuxCommand("ydotool")
	if err != nil {
		status["message"] = "The direct-typing helper is not installed."
		status["setup_commands"] = []string{linuxInstallYdotoolCommand(distroID, distroLike)}
		return status
	}
	status["installed"] = true

	if _, err := findLinuxCommand("ydotoold"); err != nil {
		status["issue"] = "daemon_missing"
		status["message"] = "ydotool is installed, but its background service is missing."
		status["setup_commands"] = linuxDaemonInstallCommands(distroID, distroLike)
		return status
	}
	status["daemon_installed"] = true

	socketPath, err := getYdotoolSocketPath()
	if err != nil {
		status["issue"] = "service_inactive"
		status["message"] = linuxYdotoolServiceMessage()
		status["setup_commands"] = linuxYdotoolServiceCommands()
		return status
	}
	status["socket"] = true
	status["socket_path"] = socketPath

	// Exercise the same client/server path used for insertion, with empty input
	// so the readiness check never types a character into the user's window.
	if err := typeLinuxTextWithYdotool(""); err != nil {
		status["issue"] = "daemon_unavailable"
		status["message"] = "The ydotool socket exists, but the app cannot use it: " + err.Error()
		status["setup_commands"] = linuxYdotoolServiceCommands()
		return status
	}

	status["ready"] = true
	status["issue"] = "ready"
	status["message"] = "Transcriptions can be typed into other apps."
	return status
}

// SetupLinuxDirectTyping starts the service supplied by the WIS package or by
// the distribution. A system service (Fedora) is started through polkit only
// after the user explicitly clicks the setup button.
func (a *App) SetupLinuxDirectTyping() map[string]interface{} {
	status := linuxYdotoolStatus()
	if ready, _ := status["ready"].(bool); ready {
		return status
	}
	if installed, _ := status["installed"].(bool); !installed {
		status["action_message"] = "Install the package shown below, then click Check again."
		return status
	}
	if daemonInstalled, _ := status["daemon_installed"].(bool); !daemonInstalled {
		status["action_message"] = "Install the daemon package shown below, then click Check again."
		return status
	}

	if err := startLinuxDirectTypingService(true); err != nil {
		status = linuxYdotoolStatus()
		status["action_message"] = err.Error()
		return status
	}
	time.Sleep(300 * time.Millisecond)
	status = linuxYdotoolStatus()
	if ready, _ := status["ready"].(bool); ready {
		status["action_message"] = "Direct typing is set up."
	}
	return status
}

// startLinuxDirectTypingService is also called during startup. Automatic
// startup only touches unprivileged user services; allowPolkit is reserved for
// the explicit setup button.
func startLinuxDirectTypingService(allowPolkit bool) error {
	if _, err := findLinuxCommand("ydotool"); err != nil {
		return errors.New("ydotool is not installed yet")
	}
	if _, err := findLinuxCommand("ydotoold"); err != nil {
		return errors.New("the ydotoold daemon is not installed yet")
	}
	if linuxYdotoolConnectionReady() {
		return nil
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemd is not available; start ydotoold using your distribution's service manager")
	}

	_ = runSystemctl(2*time.Second, "--user", "daemon-reload")
	for _, service := range []string{"wis-free-v3-ydotool.service", "ydotool.service", "ydotoold.service"} {
		if !systemdUnitExists("--user", service) {
			continue
		}
		args := []string{"--user", "start", service}
		if service != "wis-free-v3-ydotool.service" {
			args = []string{"--user", "enable", "--now", service}
		}
		if err := runSystemctl(15*time.Second, args...); err == nil {
			time.Sleep(200 * time.Millisecond)
			if linuxYdotoolConnectionReady() {
				return nil
			}
		}
	}

	for _, service := range []string{"ydotool.service", "ydotoold.service"} {
		if !systemdUnitExists("", service) {
			continue
		}
		if systemdUnitActive("", service) {
			return fmt.Errorf("the system service %s is running, but its socket is not usable by this account", service)
		}
		if !allowPolkit {
			return fmt.Errorf("the system service %s is installed but stopped", service)
		}
		pkexec, err := exec.LookPath("pkexec")
		if err != nil {
			return fmt.Errorf("run `sudo systemctl enable --now %s` in a terminal", service)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, pkexec, "systemctl", "enable", "--now", service)
		out, runErr := cmd.CombinedOutput()
		cancel()
		if runErr == nil {
			time.Sleep(200 * time.Millisecond)
			if linuxYdotoolConnectionReady() {
				return nil
			}
			return fmt.Errorf("%s started, but its socket is not usable by this account", service)
		}
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = runErr.Error()
		}
		return fmt.Errorf("could not start %s: %s", service, msg)
	}

	return errors.New("no ydotool service unit was found; use the command shown below")
}

func linuxYdotoolConnectionReady() bool {
	return typeLinuxTextWithYdotool("") == nil
}

func findLinuxCommand(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range []string{"/usr/bin", "/usr/local/bin", "/bin"} {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s was not found", name)
}

func runSystemctl(timeout time.Duration, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func systemdUnitExists(scope, service string) bool {
	args := []string{}
	if scope != "" {
		args = append(args, scope)
	}
	args = append(args, "cat", service)
	return runSystemctl(750*time.Millisecond, args...) == nil
}

func systemdUnitActive(scope, service string) bool {
	args := []string{}
	if scope != "" {
		args = append(args, scope)
	}
	args = append(args, "is-active", "--quiet", service)
	return runSystemctl(750*time.Millisecond, args...) == nil
}

func linuxYdotoolServiceMessage() string {
	for _, service := range []string{"wis-free-v3-ydotool.service", "ydotool.service", "ydotoold.service"} {
		if systemdUnitExists("--user", service) {
			if systemdUnitActive("--user", service) {
				return "The user service is running, but its socket is not available yet."
			}
			return "The direct-typing service is installed but not running."
		}
	}
	for _, service := range []string{"ydotool.service", "ydotoold.service"} {
		if systemdUnitExists("", service) {
			if systemdUnitActive("", service) {
				return "The system service is running, but its socket is not accessible."
			}
			return "The direct-typing system service is installed but not running."
		}
	}
	return "ydotool is installed, but no running service was detected."
}

func linuxYdotoolServiceCommands() []string {
	for _, service := range []string{"wis-free-v3-ydotool.service", "ydotool.service", "ydotoold.service"} {
		if systemdUnitExists("--user", service) {
			return []string{"systemctl --user start " + service, "systemctl --user status " + service}
		}
	}
	for _, service := range []string{"ydotool.service", "ydotoold.service"} {
		if systemdUnitExists("", service) {
			return []string{"sudo systemctl enable --now " + service, "systemctl status " + service}
		}
	}
	return []string{"systemctl --user enable --now ydotool.service"}
}

func linuxDistribution() (id, name, like string) {
	id, name = "linux", "Linux"
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return id, name, ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")
		switch key {
		case "ID":
			id = strings.ToLower(value)
		case "PRETTY_NAME":
			name = value
		case "ID_LIKE":
			like = strings.ToLower(value)
		}
	}
	return id, name, like
}

func linuxInstallYdotoolCommand(id, like string) string {
	family := id + " " + like
	switch {
	case strings.Contains(family, "debian"), strings.Contains(family, "ubuntu"):
		return "sudo apt install ydotool"
	case strings.Contains(family, "fedora"), strings.Contains(family, "rhel"), strings.Contains(family, "centos"):
		return "sudo dnf install ydotool"
	case strings.Contains(family, "arch"), strings.Contains(family, "manjaro"):
		return "sudo pacman -S ydotool"
	case strings.Contains(family, "suse"):
		return "sudo zypper install ydotool"
	default:
		return "Install ydotool with your distribution's package manager."
	}
}

func linuxDaemonInstallCommands(id, like string) []string {
	family := id + " " + like
	if strings.Contains(family, "debian") || strings.Contains(family, "ubuntu") {
		return []string{"sudo apt install ydotoold"}
	}
	return []string{linuxInstallYdotoolCommand(id, like)}
}

func getYdotoolSocketPath() (string, error) {
	if socketPath := strings.TrimSpace(os.Getenv("YDOTOOL_SOCKET")); socketPath != "" {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}
		return "", fmt.Errorf("YDOTOOL_SOCKET is set but not accessible: %s", socketPath)
	}

	candidates := make([]string, 0, 6)
	if runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); runtimeDir != "" {
		candidates = append(candidates, filepath.Join(runtimeDir, ".ydotool_socket"))
	}
	candidates = append(candidates,
		filepath.Join("/run/user", fmt.Sprintf("%d", os.Getuid()), ".ydotool_socket"),
		"/tmp/.ydotool_socket",
		"/run/ydotoold/socket",
		"/run/ydotoold/.ydotool_socket",
	)
	for _, socketPath := range candidates {
		if _, err := os.Stat(socketPath); err == nil {
			return socketPath, nil
		}
	}

	return "", fmt.Errorf("ydotoold socket not found; run `systemctl --user enable --now ydotool.service` and log out/in only if the service still cannot access /dev/uinput")
}

func runLinuxInputCommand(path string, args []string, stdin string, timeout time.Duration, socketPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "YDOTOOL_SOCKET="+socketPath)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func linuxTextTyperTimeout(text string) time.Duration {
	timeout := 5*time.Second + time.Duration(utf8.RuneCountInString(text))*30*time.Millisecond
	if timeout > 2*time.Minute {
		return 2 * time.Minute
	}
	return timeout
}
