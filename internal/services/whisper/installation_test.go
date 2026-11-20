package whisper

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureModel(t *testing.T) (string, []byte) {
	t.Helper()
	name := "test-fixture"
	data := []byte("verified model")
	model := Models["tiny"]
	model.Filename = "fixture.bin"
	model.Bytes = int64(len(data))
	model.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
	Models[name] = model
	t.Cleanup(func() { delete(Models, name) })
	return name, data
}

func writeInstallFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyModelRejectsCorruption(t *testing.T) {
	model, data := fixtureModel(t)
	for _, test := range []struct {
		name, content, wantError string
	}{
		{"complete", string(data), ""},
		{"truncated", string(data[:len(data)-1]), "size mismatch"},
		{"same size corruption", strings.Repeat("x", len(data)), "checksum mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "model.download")
			writeInstallFixture(t, path, test.content)
			err := verifyModel(model, path)
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("got %v; want %q", err, test.wantError)
			}
		})
	}
}

func TestInstallationRejectsIncompleteModel(t *testing.T) {
	model, _ := fixtureModel(t)
	mgr := &Manager{installDir: t.TempDir()}
	info, err := json.Marshal(InstalledInfo{Model: model})
	if err != nil {
		t.Fatal(err)
	}
	writeInstallFixture(t, filepath.Join(mgr.installDir, "installed.json"), string(info))
	writeInstallFixture(t, filepath.Join(mgr.installDir, Models[model].Filename), "partial")
	if mgr.IsInstalled() {
		t.Fatal("partial model was reported as installed")
	}
	if err := mgr.ValidateInstallation(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("missing incomplete-model explanation: %v", err)
	}
}

func TestFailedModelDownloadPreservesPreviousInstallation(t *testing.T) {
	model, data := fixtureModel(t)
	mgr := &Manager{installDir: t.TempDir()}
	modelPath := filepath.Join(mgr.installDir, Models[model].Filename)
	infoPath := filepath.Join(mgr.installDir, "installed.json")
	writeInstallFixture(t, modelPath, string(data))
	writeInstallFixture(t, infoPath, `{"model":"previous"}`)
	mockDownload(t, http.StatusOK, strings.Repeat("x", len(data)))
	if err := mgr.installLocalModel(model, nil); err == nil {
		t.Fatal("corrupt replacement model was accepted")
	}
	for path, want := range map[string]string{modelPath: string(data), infoPath: `{"model":"previous"}`} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != want {
			t.Fatalf("failed installation changed %s: %q, %v", path, content, err)
		}
	}
	partial, err := filepath.Glob(filepath.Join(mgr.installDir, ".model-*.download"))
	if err != nil || len(partial) != 0 {
		t.Fatalf("failed installation left partial downloads: %v, %v", partial, err)
	}
}

func TestInstallRejectsUnknownModelWithoutCreatingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	mgr := &Manager{installDir: dir}
	if err := mgr.Install("unknown-model"); err == nil {
		t.Fatal("unknown model was accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("invalid request created the install directory: %v", err)
	}
}

func TestExtractEngineArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, test := range []struct {
		name string
		mode os.FileMode
	}{
		{"../escape.exe", 0644},
		{`..\escape.exe`, 0644},
		{"/escape.exe", 0644},
		{"C:/escape.exe", 0644},
		{"symlink", os.ModeSymlink | 0777},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "engine.zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			archive := zip.NewWriter(file)
			header := &zip.FileHeader{Name: test.name}
			header.SetMode(test.mode)
			entry, err := archive.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(entry, "unsafe"); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := extractEngineArchive(path, filepath.Join(root, "unpacked")); err == nil {
				t.Fatal("unsafe archive entry was accepted")
			}
		})
	}
}

func TestExtractEngineArchivePreservesNestedRuntime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "engine.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, name := range []string{"main.exe", "wrapper/Release/whisper-cli.exe", "wrapper/Release/whisper.dll"} {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	unpacked := filepath.Join(root, "unpacked")
	if err := extractEngineArchive(path, unpacked); err != nil {
		t.Fatal(err)
	}
	wantBinary := filepath.Join(unpacked, "wrapper", "Release", "whisper-cli.exe")
	if got := findWindowsBinary(unpacked); got != wantBinary {
		t.Fatalf("got engine %q; want %q", got, wantBinary)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(wantBinary), "whisper.dll")); err != nil {
		t.Fatalf("engine runtime was not preserved: %v", err)
	}
}

func TestActivateEngineRestoresPreviousOnFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "engine")
	backup := filepath.Join(root, "previous")
	writeInstallFixture(t, filepath.Join(target, "whisper-cli.exe"), "previous")
	if err := activateEngine(filepath.Join(root, "missing"), target, backup); err == nil {
		t.Fatal("activation of a missing engine succeeded")
	}
	content, err := os.ReadFile(filepath.Join(target, "whisper-cli.exe"))
	if err != nil || string(content) != "previous" {
		t.Fatalf("previous engine was not restored: %q, %v", content, err)
	}
}

type offlineTestTransport func(*http.Request) (*http.Response, error)

func (transport offlineTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func mockDownload(t *testing.T, status int, body string) {
	t.Helper()
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: offlineTestTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, ContentLength: -1, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestDownloadRejectsHTTPErrorWithoutOverwritingFile(t *testing.T) {
	mockDownload(t, http.StatusBadGateway, "server error")
	path := filepath.Join(t.TempDir(), "model")
	writeInstallFixture(t, path, "previous")
	if err := downloadFile("https://offline.invalid/model", path, "Downloading model", nil); err == nil {
		t.Fatal("HTTP failure was accepted")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "previous" {
		t.Fatalf("HTTP failure replaced an existing file: %q, %v", content, err)
	}
}

func TestDownloadReportsProgressWithoutContentLength(t *testing.T) {
	mockDownload(t, http.StatusOK, "model bytes")
	path := filepath.Join(t.TempDir(), "model.download")
	progress := make(chan DownloadProgress, 4)
	if err := downloadFile("https://offline.invalid/model", path, "Downloading model", progress); err != nil {
		t.Fatal(err)
	}
	close(progress)
	var last DownloadProgress
	for update := range progress {
		last = update
	}
	if last.Phase != "Downloading model" || last.Downloaded != int64(len("model bytes")) {
		t.Fatalf("missing download phase/progress: %+v", last)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "model bytes" {
		t.Fatalf("incorrect download contents: %q, %v", content, err)
	}
}
