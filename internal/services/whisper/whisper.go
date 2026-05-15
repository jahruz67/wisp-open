// Package whisper provides local offline transcription using whisper.cpp
package whisper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"wis-free-v3/internal/logger"
)

// Download URLs
const (
	// Whisper.cpp Windows binary from ggml-org GitHub releases
	// Using Vulkan build to support Intel iGPU, AMD, and Nvidia
	whisperBinaryURL = "https://github.com/jerryshell/whisper.cpp-windows-vulkan-bin/releases/download/v1.0.0/whisper.cpp-windows-vulkan.zip"
	// Model URLs from Hugging Face
	modelBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main"
)

// Available models
var Models = map[string]struct {
	Filename string
	Size     string
}{
	"tiny":   {"ggml-tiny.bin", "75 MB"},
	"base":   {"ggml-base.bin", "150 MB"},
	"small":  {"ggml-small.bin", "500 MB"},
	"medium": {"ggml-medium.bin", "1.5 GB"},
}

// InstalledInfo stores information about the installed whisper
type InstalledInfo struct {
	Version     string `json:"version"`
	Model       string `json:"model"`
	InstallPath string `json:"install_path"`
}

// Manager handles whisper installation and execution
type Manager struct {
	installDir string
}

// NewManager creates a new whisper manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	installDir := filepath.Join(homeDir, ".wis-free-v3", "whisper")

	return &Manager{
		installDir: installDir,
	}, nil
}

// IsInstalled checks if whisper is installed
func (m *Manager) IsInstalled() bool {
	infoPath := filepath.Join(m.installDir, "installed.json")
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		return false
	}

	// Check if model exists
	info, err := m.GetInstalledInfo()
	if err != nil {
		return false
	}

	modelInfo, ok := Models[info.Model]
	if !ok || modelInfo.Filename == "" {
		return false
	}

	modelPath := filepath.Join(m.installDir, modelInfo.Filename)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// GetInstalledInfo returns information about the installed whisper
func (m *Manager) GetInstalledInfo() (*InstalledInfo, error) {
	infoPath := filepath.Join(m.installDir, "installed.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info InstalledInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// Install downloads and installs whisper with the specified model
// It opens a terminal window to show progress
func (m *Manager) Install(model string) error {
	if _, ok := Models[model]; !ok {
		return fmt.Errorf("unknown model: %s", model)
	}

	// Create install directory
	if err := os.MkdirAll(m.installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Create install script
	scriptPath := filepath.Join(m.installDir, "install.bat")
	script := m.generateInstallScript(model)
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write install script: %w", err)
	}

	// Run install script in a visible terminal
	cmd := exec.Command("cmd", "/c", "start", "cmd", "/k", scriptPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %w", err)
	}

	logger.Info("Whisper installation started in terminal")
	return nil
}

// generateInstallScript creates a batch script for installation
func (m *Manager) generateInstallScript(model string) string {
	modelInfo := Models[model]
	modelURL := fmt.Sprintf("%s/%s", modelBaseURL, modelInfo.Filename)
	modelPath := filepath.Join(m.installDir, modelInfo.Filename)
	infoPath := filepath.Join(m.installDir, "installed.json")
	zipPath := filepath.Join(m.installDir, "whisper.zip")

	script := fmt.Sprintf(`@echo off
title wis-free-v3 - Installing Offline Whisper
color 0A
echo.
echo ===============================================
echo    wis-free-v3 - Offline Whisper Installer
echo ===============================================
echo.
echo Install directory: %s
echo Model: %s (%s)
echo.

echo Checking prerequisites...
if not exist "C:\Windows\System32\msvcp140.dll" (
    echo.
    echo ERROR: Microsoft Visual C++ Redistributable is missing.
    echo This is required for the offline model to run.
    echo.
    echo Please download and install it from:
    echo https://aka.ms/vs/17/release/vc_redist.x64.exe
    echo.
    echo After installing, restart wis-free-v3 and try again.
    echo.
    pause
    exit /b 1
)

echo.
echo [1/4] Creating directories...
mkdir "%s" 2>nul

echo.
echo [2/4] Downloading whisper.cpp binary...
echo URL: %s
curl -L --retry 3 --progress-bar -o "%s" "%s"
if %%ERRORLEVEL%% neq 0 (
    echo.
    echo ERROR: Failed to download whisper.cpp
    pause
    exit /b 1
)

echo.
echo [3/4] Extracting whisper.cpp...
powershell -Command "Expand-Archive -Path '%s' -DestinationPath '%s' -Force"
if %%ERRORLEVEL%% neq 0 (
    echo ERROR: Failed to extract whisper.cpp
    pause
    exit /b 1
)
del "%s"

echo.
echo [4/4] Downloading Whisper %s model (%s)...
echo URL: %s
echo.
echo This may take several minutes depending on your internet speed...
echo.
curl -L --retry 3 --progress-bar -o "%s" "%s"
if %%ERRORLEVEL%% neq 0 (
    echo.
    echo ERROR: Failed to download model
    pause
    exit /b 1
)

echo.
echo Verifying installation...
if not exist "%s" (
    echo ERROR: Model file not found
    pause
    exit /b 1
)

echo.
echo Creating installation info...
echo {"version":"1.8.2","model":"%s","install_path":"%s"} > "%s"

echo.
echo ===============================================
echo    Installation Complete!
echo ===============================================
echo.
echo Whisper.cpp and %s model installed successfully!
echo.
echo NEXT STEPS:
echo 1. Close this window
echo 2. Restart wis-free-v3
echo 3. Select "Local Whisper" in settings
echo.
pause
exit
`,
		m.installDir,
		model, modelInfo.Size,
		m.installDir,
		whisperBinaryURL,
		zipPath, whisperBinaryURL,
		zipPath, m.installDir,
		zipPath,
		model, modelInfo.Size,
		modelURL,
		modelPath, modelURL,
		modelPath,
		model, strings.ReplaceAll(m.installDir, `\`, `\\`), infoPath,
		model,
	)

	return script
}

// Uninstall removes whisper installation
func (m *Manager) Uninstall() error {
	if err := os.RemoveAll(m.installDir); err != nil {
		return fmt.Errorf("failed to remove whisper directory: %w", err)
	}
	logger.Info("Whisper uninstalled")
	return nil
}

// Transcribe runs local whisper transcription
func (m *Manager) Transcribe(audioPath string) (string, error) {
	if !m.IsInstalled() {
		return "", fmt.Errorf("whisper is not installed")
	}

	info, err := m.GetInstalledInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get installation info: %w", err)
	}

	binaryPath := m.getBinaryPath()
	modelFilename := Models[info.Model].Filename

	// Check for model in Release directory first, then main directory
	modelPath := filepath.Join(m.installDir, "Release", modelFilename)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		modelPath = filepath.Join(m.installDir, modelFilename)
	}

	logger.Info("Running whisper: %s -m %s -f %s", binaryPath, modelPath, audioPath)

	// Run whisper with simple arguments: ./main -m model.bin -f audio.wav
	// Vulkan build uses GPU by default, no need for -ngl
	cmd := exec.Command(binaryPath, "-m", modelPath, "-f", audioPath)

	// Set working directory to the binary's location so it can find DLLs
	cmd.Dir = filepath.Dir(binaryPath)

	// Hide the console window on Windows
	hideWindowContext(cmd)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		logger.Error("Whisper failed: %v, output: %s", err, outputStr)
		return "", fmt.Errorf("transcription failed: %w", err)
	}

	// Parse the output - whisper.cpp outputs transcribed text to stdout
	lines := strings.Split(outputStr, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip all system/debug info
		if strings.Contains(line, "whisper_") ||
			strings.Contains(line, "main:") ||
			strings.Contains(line, "system_info:") ||
			strings.Contains(line, "ld_") ||
			strings.Contains(line, "cuda_") ||
			strings.Contains(line, "ggml_vulkan") {
			continue
		}

		// Handle timestamped lines like: [00:00:00.000 --> 00:00:07.280] Text
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]") {
			// Extract text after the timestamp
			parts := strings.SplitN(line, "]", 2)
			if len(parts) > 1 {
				text := strings.TrimSpace(parts[1])
				if text != "" {
					textLines = append(textLines, text)
				}
			}
			continue
		}

		// Fallback for non-prefixed text lines (if any)
		textLines = append(textLines, line)
	}

	result := strings.Join(textLines, " ")
	result = strings.TrimSpace(result)

	if result == "" {
		// If still no text, return simple fallback or raw output if short
		if len(outputStr) < 200 {
			return strings.TrimSpace(outputStr), nil
		}
		// Return empty string rather than spamming user with massive logs
		return "", nil
	}

	return result, nil
}

// getBinaryPath returns the path to the whisper binary
func (m *Manager) getBinaryPath() string {
	// whisper.cpp extracts to a Release subdirectory
	releaseDir := filepath.Join(m.installDir, "Release")

	// Try different possible binary names
	// whisper-cli.exe is the new standard (main.exe is deprecated)
	possibleNames := []string{
		"whisper-cli.exe",
		"main.exe",
	}

	// First check in Release subdirectory
	for _, name := range possibleNames {
		path := filepath.Join(releaseDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Then check in main install directory
	for _, name := range possibleNames {
		path := filepath.Join(m.installDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to Release/main.exe
	return filepath.Join(releaseDir, "main.exe")
}

// CheckOnline checks if internet is available
func CheckOnline() bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("https://api.groq.com")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}

// DownloadProgress represents download progress
type DownloadProgress struct {
	Downloaded int64
	Total      int64
	Percent    float64
}

// downloadFile downloads a file with progress reporting
func downloadFile(url, dest string, progress chan<- DownloadProgress) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			downloaded += int64(n)
			if progress != nil && total > 0 {
				progress <- DownloadProgress{
					Downloaded: downloaded,
					Total:      total,
					Percent:    float64(downloaded) / float64(total) * 100,
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

