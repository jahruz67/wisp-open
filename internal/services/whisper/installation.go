package whisper

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func validateEngineFile(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Whisper engine is missing; reinstall WIS with its offline engine: %w", err)
	}
	if !stat.Mode().IsRegular() || stat.Size() == 0 || (runtime.GOOS != "windows" && stat.Mode().Perm()&0111 == 0) {
		return fmt.Errorf("Whisper engine is not executable: %s", path)
	}
	return nil
}

// Catch missing runtime libraries before downloading a potentially large model.
func checkEngine(path string) error {
	if err := validateEngineFile(path); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--help")
	cmd.Dir = filepath.Dir(path)
	hideWindowContext(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		hint := "Check the engine's runtime libraries."
		if runtime.GOOS == "linux" && strings.Contains(string(output), "GLIBC_") && strings.Contains(string(output), "not found") {
			hint = "This Whisper engine requires a newer glibc than your Linux system provides. Install a WIS package compatible with your distribution, or build WIS from source on this system."
		} else if runtime.GOOS == "windows" {
			hint = "Check your graphics driver and Microsoft Visual C++ x64 runtime (https://aka.ms/vs/17/release/vc_redist.x64.exe)."
		}
		return fmt.Errorf("Whisper engine could not start: %w. %s %s", err, hint, strings.TrimSpace(string(output)))
	}
	return nil
}

func (m *Manager) installWindowsEngine(progress chan<- DownloadProgress) error {
	if err := checkEngine(m.getBinaryPath()); err == nil {
		return nil
	}
	stage, err := os.MkdirTemp(m.installDir, ".engine-*")
	if err != nil {
		return fmt.Errorf("failed to create engine staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	archivePath := filepath.Join(stage, "engine.zip")
	if err := downloadFile(whisperBinaryURL, archivePath, "Downloading engine", progress); err != nil {
		return fmt.Errorf("failed to download Whisper engine: %w", err)
	}
	reportInstallPhase(progress, "Extracting engine")
	unpacked := filepath.Join(stage, "unpacked")
	if err := extractEngineArchive(archivePath, unpacked); err != nil {
		return fmt.Errorf("failed to extract Whisper engine: %w", err)
	}
	binary := findWindowsBinary(unpacked)
	if binary == "" {
		return fmt.Errorf("downloaded engine archive does not contain whisper-cli.exe or main.exe")
	}
	reportInstallPhase(progress, "Checking engine")
	if err := checkEngine(binary); err != nil {
		return err
	}
	// Keep rollback data outside staging so a failed restore cannot delete it.
	backup := filepath.Join(m.installDir, filepath.Base(stage)+".previous")
	return activateEngine(unpacked, filepath.Join(m.installDir, "engine"), backup)
}

func activateEngine(staged, target, backup string) error {
	hadPrevious := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("failed to move previous engine; stop any active transcription and retry: %w", err)
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, target); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("failed to activate engine: %v; previous engine remains at %s: %w", err, backup, restoreErr)
			}
		}
		return fmt.Errorf("failed to activate engine: %w", err)
	}
	if hadPrevious {
		_ = os.RemoveAll(backup)
	}
	return nil
}

func extractEngineArchive(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		name := filepath.FromSlash(strings.ReplaceAll(entry.Name, `\`, "/"))
		if !filepath.IsLocal(name) || strings.Contains(name, ":") || (!entry.Mode().IsRegular() && !entry.FileInfo().IsDir()) {
			return fmt.Errorf("unsafe engine archive entry: %s", entry.Name)
		}
		target := filepath.Join(destination, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := extractEngineFile(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func extractEngineFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Release archives may contain a wrapper directory or a Release/bin directory.
func findWindowsBinary(root string) string {
	var binary, fallback string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			if strings.EqualFold(entry.Name(), "whisper-cli.exe") {
				binary = path
				return fs.SkipAll
			}
			if strings.EqualFold(entry.Name(), "main.exe") {
				fallback = path
			}
		}
		return nil
	})
	if err != nil {
		return ""
	}
	if binary != "" {
		return binary
	}
	return fallback
}
