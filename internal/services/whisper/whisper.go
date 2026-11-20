// Package whisper provides local offline transcription using whisper.cpp and sherpa-onnx.
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
	// Windows x64 Vulkan engine and backend DLLs from a pinned release.
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
	Version     string           `json:"version"`
	Model       string           `json:"model"`
	InstallPath string           `json:"install_path"`
	Files       map[string]int64 `json:"files,omitempty"`
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

// IsInstalled checks the engine and the complete model, not just metadata.
func (m *Manager) IsInstalled() bool {
	return m.ValidateInstallation() == nil
}

// ValidateInstallation explains why an installation cannot be used.
func (m *Manager) ValidateInstallation() error {
	info, err := m.GetInstalledInfo()
	if err != nil {
		return fmt.Errorf("cannot read Whisper installation info: %w", err)
	}
	if model, ok := onnxModels[info.Model]; ok {
		return m.validateONNXInstallation(model, info)
	}

	modelInfo, ok := Models[info.Model]
	if !ok || modelInfo.Filename == "" {
		return fmt.Errorf("unknown installed Whisper model: %s", info.Model)
	}

	modelPath := filepath.Join(m.installDir, modelInfo.Filename)
	if stat, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("Whisper model is missing: %w", err)
	} else if !stat.Mode().IsRegular() || stat.Size() != modelInfo.Bytes {
		return fmt.Errorf("Whisper %s model is incomplete; reinstall the offline model", info.Model)
	}
	if err := validateEngineFile(m.getBinaryPath()); err != nil {
		return err
	}

	return nil
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

// Install downloads and verifies Whisper, waiting for installation to finish.
func (m *Manager) Install(model string) error {
	return m.InstallWithProgress(model, nil)
}

// InstallWithProgress installs a user-selected local model and its engine if
// needed. Linux and macOS bundle the Whisper engine; other engines are downloaded
// on demand. No model download occurs during application installation.
func (m *Manager) InstallWithProgress(model string, progress chan<- DownloadProgress) error {
	if info, ok := onnxModels[model]; ok {
		return m.installONNXModel(model, info, progress)
	}
	if _, ok := Models[model]; !ok {
		return fmt.Errorf("unknown model: %s", model)
	}

	// Create install directory
	if err := os.MkdirAll(m.installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	if runtime.GOOS == "darwin" && runtime.GOARCH != "arm64" {
		return fmt.Errorf("local Whisper is supported only on Apple Silicon")
	}
	reportInstallPhase(progress, "Checking engine")
	if runtime.GOOS == "windows" {
		if runtime.GOARCH != "amd64" {
			return fmt.Errorf("the Windows offline engine requires the x64 version of WIS")
		}
		if err := m.installWindowsEngine(progress); err != nil {
			return err
		}
	} else if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if err := checkEngine(m.getBinaryPath()); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("offline Whisper is not supported on %s", runtime.GOOS)
	}
	return m.installLocalModel(model, progress)
}

func (m *Manager) installLocalModel(model string, progress chan<- DownloadProgress) error {
	modelInfo := Models[model]
	modelURL := fmt.Sprintf("%s/%s", modelBaseURL, modelInfo.Filename)
	tempFile, err := os.CreateTemp(m.installDir, ".model-*.download")
	if err != nil {
		return fmt.Errorf("failed to create model download: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if err := tempFile.Close(); err != nil {
		return err
	}
	finalPath := filepath.Join(m.installDir, modelInfo.Filename)
	if err := downloadFile(modelURL, tempPath, "Downloading model", progress); err != nil {
		return fmt.Errorf("failed to download %s model: %w", model, err)
	}
	reportInstallPhase(progress, "Verifying model")
	if err := verifyModel(model, tempPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("failed to activate downloaded model: %w", err)
	}

	reportInstallPhase(progress, "Finishing installation")
	old, _ := m.GetInstalledInfo()
	version := "bundled-arm64"
	if runtime.GOOS == "linux" {
		version = "linux-whisper.cpp"
	} else if runtime.GOOS == "windows" {
		version = "windows-vulkan"
	}
	info := InstalledInfo{Version: version, Model: model, InstallPath: m.installDir}
	if err := m.saveInstalledInfo(info); err != nil {
		return err
	}
	if err := m.ValidateInstallation(); err != nil {
		return err
	}
	// Keep the previous model usable until its metadata is replaced too.
	m.removePreviousModel(old, model)
	logger.Info("Local Whisper %s model installed", model)
	return nil
}

func verifyModel(model, tempPath string) error {
	modelInfo, ok := Models[model]
	if !ok {
		return fmt.Errorf("unknown model: %s", model)
	}
	if stat, err := os.Stat(tempPath); err != nil || stat.Size() != modelInfo.Bytes {
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
		return fmt.Errorf("failed to verify model: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	actualHash := fmt.Sprintf("%x", hash.Sum(nil))
	if !strings.EqualFold(actualHash, modelInfo.SHA256) {
		return fmt.Errorf("model checksum mismatch: got %s", actualHash)
	}
	return nil
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
	if err := m.ValidateInstallation(); err != nil {
		return "", err
	}

	info, err := m.GetInstalledInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get installation info: %w", err)
	}
	if model, ok := onnxModels[info.Model]; ok {
		return m.transcribeONNX(model, audioPath, language)
	}

	binaryPath := m.getBinaryPath()
	modelFilename := Models[info.Model].Filename

	modelPath := filepath.Join(m.installDir, modelFilename)

	if language == "" {
		language = "auto"
	}

	logger.Info("Running whisper: %s -m %s -l %s -f %s", binaryPath, modelPath, language, audioPath)

	// GPU acceleration is the default; retry once on CPU if a GPU backend fails.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	args := []string{"-m", modelPath, "-l", language, "-f", audioPath, "-np", "-nt"}
	output, diagnostics, err := runWhisper(ctx, binaryPath, args)
	if err != nil && ctx.Err() == nil {
		logger.Info("Whisper failed with GPU enabled; retrying once with CPU: %s", diagnostics)
		output, diagnostics, err = runWhisper(ctx, binaryPath, append(args, "-ng"))
	}

	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("local transcription did not finish: %w", ctx.Err())
		}
		logger.Error("Whisper failed: %v, diagnostics: %s", err, diagnostics)
		return "", fmt.Errorf("transcription failed: %w: %s", err, diagnostics)
	}
	logger.Debug("Whisper backend diagnostics: %s", diagnostics)
	// -np/-nt produce only dictated text; do not filter out real spoken words.
	return strings.Join(strings.Fields(string(output)), " "), nil
}

func runWhisper(ctx context.Context, binaryPath string, args []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = filepath.Dir(binaryPath)
	hideWindowContext(cmd)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	output, err := cmd.Output()
	return output, strings.TrimSpace(diagnostics.String()), err
}

// getBinaryPath returns the path to the whisper binary
func (m *Manager) getBinaryPath() string {
	if runtime.GOOS == "darwin" {
		return m.darwinBinaryPath()
	}
	if runtime.GOOS == "linux" {
		return m.linuxBinaryPath()
	}
	if binary := findWindowsBinary(filepath.Join(m.installDir, "engine")); binary != "" {
		return binary
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
	Phase      string
	Downloaded int64
	Total      int64
	Percent    float64
}

// downloadFile downloads a file with progress reporting
func downloadFile(url, dest, phase string, progress chan<- DownloadProgress) (err error) {
	reportInstallPhase(progress, phase)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Limit idle time rather than total duration so slow, active downloads work.
	idle := time.AfterFunc(time.Minute, cancel)
	defer idle.Stop()
	defer func() {
		if err != nil && ctx.Err() != nil {
			err = fmt.Errorf("download stalled for one minute: %w", err)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
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
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()

	total := resp.ContentLength
	var downloaded int64
	lastProgress := time.Time{}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			idle.Reset(time.Minute)
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)
			if progress != nil && (lastProgress.IsZero() || time.Since(lastProgress) >= 250*time.Millisecond || downloaded == total) {
				percent := float64(0)
				if total > 0 {
					percent = float64(downloaded) / float64(total) * 100
				}
				progress <- DownloadProgress{
					Phase:      phase,
					Downloaded: downloaded,
					Total:      total,
					Percent:    percent,
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

func reportInstallPhase(progress chan<- DownloadProgress, phase string) {
	if progress != nil {
		progress <- DownloadProgress{Phase: phase}
	}
}
