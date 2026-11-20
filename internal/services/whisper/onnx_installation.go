package whisper

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const sherpaReleaseURL = "https://github.com/k2-fsa/sherpa-onnx/releases/download/"

type sherpaDownload struct {
	Name   string
	SHA256 string
}

func sherpaArchive() (sherpaDownload, error) {
	platform := runtime.GOOS + "/" + runtime.GOARCH
	var target, checksum string
	switch platform {
	case "windows/amd64":
		target = "win-x64-shared-no-tts"
		checksum = "8ac784e14582e9e36a952fb70d8e71f4e1b65cc219f27afb7b016661613602a4"
	case "linux/amd64":
		target = "linux-x64-shared-no-tts"
		checksum = "80f6d7e933aa0f1f43351b0d68b4245f17ab7a59855594a66973764e7aa44744"
	case "darwin/arm64":
		target = "osx-universal2-shared-no-tts"
		checksum = "9dae274edf1d5957b199943dab2d5df74402c3466a690560c6e47ed2288b8cf3"
	default:
		return sherpaDownload{}, fmt.Errorf("these local models are not supported on %s", platform)
	}
	return sherpaDownload{"sherpa-onnx-" + sherpaVersion + "-" + target + ".tar.bz2", checksum}, nil
}

func (m *Manager) sherpaBinary() (string, error) {
	name := "sherpa-onnx-offline"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var binary string
	err := filepath.WalkDir(filepath.Join(m.installDir, "sherpa-"+sherpaVersion), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && entry.Name() == name {
			binary = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("offline engine is missing; reinstall the local model: %w", err)
	}
	if binary == "" {
		return "", fmt.Errorf("offline engine is missing %s; reinstall the local model", name)
	}
	return binary, validateEngineFile(binary)
}

func (m *Manager) installSherpaEngine(progress chan<- DownloadProgress) error {
	archive, err := sherpaArchive()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.installDir, 0755); err != nil {
		return err
	}
	if _, err := m.sherpaBinary(); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := m.runSherpa(ctx, "--help"); err == nil {
			return nil
		}
	}
	stage, err := os.MkdirTemp(m.installDir, ".sherpa-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	url := sherpaReleaseURL + sherpaVersion + "/" + archive.Name
	if err := downloadONNXArchive(url, archive.SHA256, stage, "Downloading local engine", progress); err != nil {
		return err
	}
	target := filepath.Join(m.installDir, "sherpa-"+sherpaVersion)
	if err := activateEngine(filepath.Join(stage, "unpacked"), target, target+".previous"); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if output, err := m.runSherpa(ctx, "--help"); err != nil {
		return fmt.Errorf("offline engine could not start; check its runtime libraries: %w: %s", err, output)
	}
	return nil
}

func (m *Manager) installONNXModel(name string, model onnxModel, progress chan<- DownloadProgress) error {
	reportInstallPhase(progress, "Checking local engine")
	if err := m.installSherpaEngine(progress); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(m.installDir, ".onnx-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	url := sherpaReleaseURL + "asr-models/" + model.Directory + ".tar.bz2"
	if err := downloadONNXArchive(url, model.SHA256, stage, "Downloading "+model.Label, progress); err != nil {
		return err
	}
	modelPath := filepath.Join(stage, "unpacked", model.Directory)
	files := make(map[string]int64, len(model.Files))
	for _, filename := range model.Files {
		stat, err := os.Stat(filepath.Join(modelPath, filename))
		if err != nil {
			return fmt.Errorf("downloaded model is missing %s: %w", filename, err)
		}
		if !stat.Mode().IsRegular() || stat.Size() == 0 {
			return fmt.Errorf("downloaded model has an invalid %s", filename)
		}
		files[filename] = stat.Size()
	}
	target := filepath.Join(m.installDir, "models", model.Directory)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := activateEngine(modelPath, target, target+".previous"); err != nil {
		return err
	}
	reportInstallPhase(progress, "Finishing installation")
	old, _ := m.GetInstalledInfo()
	info := InstalledInfo{Version: "sherpa-onnx-" + sherpaVersion, Model: name, InstallPath: m.installDir, Files: files}
	if err := m.validateONNXInstallation(model, &info); err != nil {
		return err
	}
	if err := m.saveInstalledInfo(info); err != nil {
		return err
	}
	m.removePreviousModel(old, name)
	return nil
}

func (m *Manager) validateONNXInstallation(model onnxModel, info *InstalledInfo) error {
	for _, filename := range model.Files {
		stat, err := os.Stat(filepath.Join(m.installDir, "models", model.Directory, filename))
		if err != nil {
			return fmt.Errorf("%s is missing %s; reinstall the local model: %w", model.Label, filename, err)
		}
		if !stat.Mode().IsRegular() || info.Files[filename] <= 0 || stat.Size() != info.Files[filename] {
			return fmt.Errorf("%s is incomplete; reinstall the local model", model.Label)
		}
	}
	_, err := m.sherpaBinary()
	return err
}

func (m *Manager) saveInstalledInfo(info InstalledInfo) error {
	data, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.installDir, "installed.json")
	defer os.Remove(path + ".tmp")
	if err := os.WriteFile(path+".tmp", data, 0600); err != nil {
		return fmt.Errorf("failed to save installation info: %w", err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return fmt.Errorf("failed to activate installation info: %w", err)
	}
	return nil
}

func (m *Manager) removePreviousModel(old *InstalledInfo, current string) {
	if old == nil || old.Model == current {
		return
	}
	if model, ok := Models[old.Model]; ok {
		_ = os.Remove(filepath.Join(m.installDir, model.Filename))
	} else if model, ok := onnxModels[old.Model]; ok {
		_ = os.RemoveAll(filepath.Join(m.installDir, "models", model.Directory))
	}
}

func downloadONNXArchive(url, checksum, stage, phase string, progress chan<- DownloadProgress) error {
	path := filepath.Join(stage, "download.tar.bz2")
	if err := downloadFile(url, path, phase, progress); err != nil {
		return err
	}
	reportInstallPhase(progress, "Verifying download")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != checksum {
		return fmt.Errorf("download checksum mismatch; retry installing the local model")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reportInstallPhase(progress, "Extracting download")
	return extractLocalArchive(bzip2.NewReader(file), filepath.Join(stage, "unpacked"))
}

// Delay links until all regular files are extracted, so no archive member can
// write through a link. Every link must resolve inside the staging directory.
func extractLocalArchive(source io.Reader, destination string) error {
	archive := tar.NewReader(source)
	links := make(map[string]string)
	for {
		entry, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.FromSlash(strings.ReplaceAll(entry.Name, `\`, "/"))
		if !filepath.IsLocal(name) || strings.Contains(name, ":") {
			return fmt.Errorf("unsafe archive entry: %s", entry.Name)
		}
		target := filepath.Join(destination, name)
		switch entry.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0755)
		case tar.TypeReg, tar.TypeRegA:
			err = extractLocalFile(archive, target, entry.Mode&0111 != 0)
		case tar.TypeSymlink:
			link := filepath.FromSlash(strings.ReplaceAll(entry.Linkname, `\`, "/"))
			if filepath.IsAbs(link) || strings.Contains(link, ":") || !filepath.IsLocal(filepath.Join(filepath.Dir(name), link)) {
				return fmt.Errorf("unsafe archive link: %s", entry.Name)
			}
			links[target] = link
		default:
			return fmt.Errorf("unsupported archive entry: %s", entry.Name)
		}
		if err != nil {
			return err
		}
	}
	if _, err := io.Copy(io.Discard, source); err != nil {
		return err
	}
	for path, target := range links {
		if err := os.Symlink(target, path); err != nil {
			return err
		}
	}
	for path := range links {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(destination, resolved)
		if err != nil || !filepath.IsLocal(relative) {
			return fmt.Errorf("archive link escapes installation: %s", path)
		}
	}
	return nil
}

func extractLocalFile(source io.Reader, target string, executable bool) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0644)
	if executable {
		mode = 0755
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
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
