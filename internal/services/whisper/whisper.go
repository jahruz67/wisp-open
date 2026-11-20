// Package whisper provides local offline transcription using whisper.cpp
package whisper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wis-free-v3/internal/logger"
)

// Download URLs
const (
	// Whisper.cpp Windows binary from ggml-org GitHub releases
	// Using Vulkan build to support Intel iGPU, AMD, and Nvidia
	whisperBinaryURL = "https://github.com/jerryshell/whisper.cpp-windows-vulkan-bin/releases/download/v1.0.0/whisper.cpp-windows-vulkan.zip"
	// Model files are pinned to an immutable Hugging Face revision so the
	// expected byte sizes and SHA-256 digests below remain reproducible.
	modelBaseURL = "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1"
)

// Available models
var Models = map[string]struct {
	Filename string
	Size     string
	Bytes    int64
	SHA256   string
}{
	"tiny":   {"ggml-tiny.bin", "75 MB", 77691713, "be07e048e1e599ad46341c8d2a135645097a538221678b7acdd1b1919c6e1b21"},
	"base":   {"ggml-base.bin", "150 MB", 147951465, "60ed5bc3dd14eea856493d334349b405782ddcaf0028d4b5df4088345fba2efe"},
	"small":  {"ggml-small.bin", "500 MB", 487601967, "1be3a9b2063867b937e64e2ec7483364a79917e157fa98c5d94b5c1fffea987b"},
	"medium": {"ggml-medium.bin", "1.5 GB", 1533763059, "6c14d5adee5f86394037b4e4e8b59f1673b6cee10e3cf0b11bbdbee79c156208"},
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
	if stat, err := os.Stat(modelPath); err != nil || !stat.Mode().IsRegular() || stat.Size() == 0 {
		return false
	}
	if stat, err := os.Stat(m.getBinaryPath()); err != nil || !stat.Mode().IsRegular() || (runtime.GOOS != "windows" && stat.Mode().Perm()&0111 == 0) {
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

// Install downloads and installs Whisper with the specified model. Windows
// opens its existing terminal installer; Linux and macOS report progress through the
// channel accepted by InstallWithProgress.
func (m *Manager) Install(model string) error {
	return m.InstallWithProgress(model, nil)
}

// InstallWithProgress installs a user-selected local model. Linux and macOS ship
// the engine with the application, so only the explicitly selected model is
// downloaded. No model download occurs during application installation.
func (m *Manager) InstallWithProgress(model string, progress chan<- DownloadProgress) error {
	if _, ok := Models[model]; !ok {
		return fmt.Errorf("unknown model: %s", model)
	}

	// Create install directory
	if err := os.MkdirAll(m.installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		return m.installLocalModel(model, progress)
	}

	// Create install script (Windows batch file)
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

func (m *Manager) installLocalModel(model string, progress chan<- DownloadProgress) error {
	if runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("local Whisper is supported only on Apple Silicon")
	}
	if stat, err := os.Stat(m.getBinaryPath()); err != nil {
		return fmt.Errorf("Whisper engine is missing; reinstall WIS with its offline engine: %w", err)
	} else if !stat.Mode().IsRegular() || stat.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("Whisper engine is not executable: %s", m.getBinaryPath())
	}

	modelInfo := Models[model]
	modelURL := fmt.Sprintf("%s/%s", modelBaseURL, modelInfo.Filename)
	tempPath := filepath.Join(m.installDir, modelInfo.Filename+".download")
	finalPath := filepath.Join(m.installDir, modelInfo.Filename)
	_ = os.Remove(tempPath)
	if err := downloadFile(modelURL, tempPath, progress); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to download %s model: %w", model, err)
	}
	if stat, err := os.Stat(tempPath); err != nil || stat.Size() != modelInfo.Bytes {
		_ = os.Remove(tempPath)
		if err != nil {
			return fmt.Errorf("failed to inspect downloaded model: %w", err)
		}
		return fmt.Errorf("model size mismatch: got %d bytes, want %d", stat.Size(), modelInfo.Bytes)
	}

	file, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to verify model: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	actualHash := fmt.Sprintf("%x", hash.Sum(nil))
	if !strings.EqualFold(actualHash, modelInfo.SHA256) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("model checksum mismatch: got %s", actualHash)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to activate downloaded model: %w", err)
	}

	old, _ := m.GetInstalledInfo()
	version := "bundled-arm64"
	if runtime.GOOS == "linux" {
		version = "linux-whisper.cpp"
	}
	info := InstalledInfo{Version: version, Model: model, InstallPath: m.installDir}
	data, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	infoPath := filepath.Join(m.installDir, "installed.json")
	if err := os.WriteFile(infoPath+".tmp", data, 0600); err != nil {
		return err
	}
	if err := os.Rename(infoPath+".tmp", infoPath); err != nil {
		return err
	}
	// Keep the previous installation usable until its metadata is replaced too.
	if old != nil && old.Model != model {
		if oldInfo, ok := Models[old.Model]; ok {
			_ = os.Remove(filepath.Join(m.installDir, oldInfo.Filename))
		}
	}
	logger.Info("Local Whisper %s model installed", model)
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
func (m *Manager) Transcribe(audioPath string, language string) (string, error) {
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

	if language == "" {
		language = "auto"
	}

	logger.Info("Running whisper: %s -m %s -l %s -f %s", binaryPath, modelPath, language, audioPath)

	// Run whisper with simple arguments: ./main -m model.bin -l language -f audio.wav
	// Vulkan build uses GPU by default, no need for -ngl
	// Use a 5-minute timeout to prevent infinite hangs on long audio or stuck processes
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, "-m", modelPath, "-l", language, "-f", audioPath)

	// Set working directory to the binary's location so it can find DLLs
	cmd.Dir = filepath.Dir(binaryPath)

	// Hide the console window on Windows
	hideWindowContext(cmd)

	// GPU/backend diagnostics go to stderr and must never become dictated text.
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	output, err := cmd.Output()
	outputStr := string(output)

	if err != nil {
		logger.Error("Whisper failed: %v, output: %s, diagnostics: %s", err, outputStr, diagnostics.String())
		return "", fmt.Errorf("transcription failed: %w: %s", err, strings.TrimSpace(diagnostics.String()))
	}
	if runtime.GOOS == "linux" {
		logger.Debug("Whisper backend diagnostics: %s", strings.TrimSpace(diagnostics.String()))
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
	if runtime.GOOS == "darwin" {
		return m.darwinBinaryPath()
	}
	if runtime.GOOS == "linux" {
		return m.linuxBinaryPath()
	}
	// whisper.cpp extracts to a Release subdirectory
	releaseDir := filepath.Join(m.installDir, "Release")

	// Try different possible binary names (platform-aware)
	// On Windows: whisper-cli.exe / main.exe
	// On Linux/macOS: whisper-cli / main
	possibleNames := []string{
		"whisper-cli",
		"main",
	}
	if runtime.GOOS == "windows" {
		possibleNames = []string{
			"whisper-cli.exe",
			"main.exe",
		}
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

	// Default path
	if runtime.GOOS == "windows" {
		return filepath.Join(releaseDir, "main.exe")
	}
	return filepath.Join(releaseDir, "main")
}

// linuxBinaryPath covers release packages, source installs, and development.
func (m *Manager) linuxBinaryPath() string {
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		binDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(binDir, "..", "lib", "wis-free-v3", "whisper-cli"),
			filepath.Join(binDir, "whisper-cli"))
	}
	candidates = append(candidates,
		filepath.Join(m.installDir, "whisper-cli"),
		filepath.Join(m.installDir, "Release", "whisper-cli"))
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.Mode().IsRegular() && stat.Mode().Perm()&0111 != 0 {
			return filepath.Clean(candidate)
		}
	}
	if binary, err := exec.LookPath("whisper-cli"); err == nil {
		return binary
	}
	return "/usr/lib/wis-free-v3/whisper-cli"
}

func (m *Manager) darwinBinaryPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	// Wails places the executable in App.app/Contents/MacOS and resources in
	// App.app/Contents/Resources. Prefer the bundled engine in release builds.
	binDir := filepath.Dir(executable)
	bundled := filepath.Clean(filepath.Join(binDir, "..", "Resources", "bin", "whisper-cli"))
	if filepath.Base(binDir) == "MacOS" && filepath.Base(filepath.Dir(binDir)) == "Contents" {
		return bundled
	}
	// Wails development mode runs outside the bundle, from the project root.
	for _, candidate := range []string{
		filepath.Join(binDir, "whisper-cli"),
		filepath.Join("build", "whisper-macos", "bin", "whisper-cli"),
	} {
		if stat, err := os.Stat(candidate); err == nil && stat.Mode().IsRegular() && stat.Mode().Perm()&0111 != 0 {
			if absolute, err := filepath.Abs(candidate); err == nil {
				return absolute
			}
		}
	}
	return bundled
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

	// Drain the response body to allow connection reuse and prevent resource leak
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: server returned HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64
	lastProgress := time.Time{}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if progress != nil && total > 0 && (lastProgress.IsZero() || time.Since(lastProgress) >= 250*time.Millisecond || downloaded == total) {
				progress <- DownloadProgress{
					Downloaded: downloaded,
					Total:      total,
					Percent:    float64(downloaded) / float64(total) * 100,
				}
				lastProgress = time.Now()
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
