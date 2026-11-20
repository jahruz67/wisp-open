package whisper

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Match the pinned archive used by scripts/download-whisper-linux.sh.
const linuxEngineURL = "https://raw.githubusercontent.com/jahruz67/wisp-open/cd468ed0bf13ae7108b4639c16656ceae9dfe5da/assets/linuz_bin_cpp/whisper-linux-amd64-vulkan-v1.9.4.zip"
const linuxEngineSHA256 = "41f654fa772873a1e1a16b115ad15f8df7f598544f222b1298d16b55cd5d9c88"

func (m *Manager) installLinuxEngine(progress chan<- DownloadProgress) error {
	if err := checkEngine(m.linuxBinaryPath()); err == nil {
		reportInstallPhase(progress, "Engine already installed")
		return nil
	}
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("the downloadable Linux Whisper engine requires x86-64")
	}
	stage, err := os.MkdirTemp(m.installDir, ".engine-*")
	if err != nil {
		return fmt.Errorf("failed to create engine staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, "engine.zip")
	if err := downloadFile(linuxEngineURL, archive, "Downloading engine", progress); err != nil {
		return fmt.Errorf("failed to download Linux Whisper engine: %w", err)
	}
	reportInstallPhase(progress, "Verifying engine")
	if err := verifyLinuxEngineArchive(archive); err != nil {
		return err
	}
	reportInstallPhase(progress, "Extracting engine")
	unpacked := filepath.Join(stage, "unpacked")
	if err := extractEngineArchive(archive, unpacked); err != nil {
		return fmt.Errorf("failed to extract Linux Whisper engine: %w", err)
	}
	binary := filepath.Join(unpacked, "whisper-cli")
	if err := os.Chmod(binary, 0755); err != nil {
		return fmt.Errorf("downloaded engine is missing whisper-cli: %w", err)
	}
	reportInstallPhase(progress, "Checking engine")
	if err := checkEngine(binary); err != nil {
		return err
	}
	backup := filepath.Join(m.installDir, filepath.Base(stage)+".previous")
	return activateEngine(unpacked, filepath.Join(m.installDir, "engine"), backup)
}

func verifyLinuxEngineArchive(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != linuxEngineSHA256 {
		return fmt.Errorf("Linux Whisper engine checksum mismatch; retry installing the offline model")
	}
	return nil
}
