package whisper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const sherpaVersion = "v1.12.17"

type onnxModel struct {
	Label       string
	Size        string
	Description string
	Directory   string
	SHA256      string
	Kind        string
	Files       map[string]string // CLI option to model filename
}

var parakeetFiles = map[string]string{
	"encoder": "encoder.int8.onnx", "decoder": "decoder.int8.onnx",
	"joiner": "joiner.int8.onnx", "tokens": "tokens.txt",
}

var moonshineFiles = map[string]string{
	"moonshine-preprocessor": "preprocess.onnx", "moonshine-encoder": "encode.int8.onnx",
	"moonshine-uncached-decoder": "uncached_decode.int8.onnx",
	"moonshine-cached-decoder": "cached_decode.int8.onnx", "tokens": "tokens.txt",
}

// Model archives and checksums published by k2-fsa/sherpa-onnx (asr-models release).
var onnxModels = map[string]onnxModel{
	"parakeet-v3": {
		Label: "NVIDIA Parakeet TDT v3", Size: "487 MB",
		Description: "25 European languages; automatic language detection and punctuation. Runs on CPU.",
		Directory: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8",
		SHA256: "1495e91b59d7fc53ddca91f3431758ec99f8fa253fdb0e1e851be0f72b14b0f9",
		Kind: "parakeet", Files: parakeetFiles,
	},
	"parakeet-v2": {
		Label: "NVIDIA Parakeet TDT v2", Size: "482 MB",
		Description: "English only; automatic punctuation. Runs on CPU.",
		Directory: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8",
		SHA256: "6fc4426b54aa549a0019b8a75b8dea59164e71790a37ac56a045ad57f915f714",
		Kind: "parakeet", Files: parakeetFiles,
	},
	"moonshine-tiny": {
		Label: "Moonshine Tiny", Size: "108 MB",
		Description: "English only; lightweight model for short dictation. Runs on CPU.",
		Directory: "sherpa-onnx-moonshine-tiny-en-int8",
		SHA256: "d5fe6ec4334fef36255b2a4010412cad4c007e33103fec62fb5d17cad88086f2",
		Kind: "moonshine", Files: moonshineFiles,
	},
	"moonshine-base": {
		Label: "Moonshine Base", Size: "251 MB",
		Description: "English only; larger Moonshine model for dictation. Runs on CPU.",
		Directory: "sherpa-onnx-moonshine-base-en-int8",
		SHA256: "21870cecaa2e44e4e2bf63e02d1072bed183ccd10284871353bd9d24dad14e5e",
		Kind: "moonshine", Files: moonshineFiles,
	},
	"sensevoice": {
		Label: "SenseVoice Small", Size: "166 MB",
		Description: "Chinese, Cantonese, English, Japanese and Korean; automatic punctuation. Runs on CPU.",
		Directory: "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2025-09-09",
		SHA256: "7305f7905bfcf77fa0b39388a313f3da35c68d971661a65475b56fb2162c8e63",
		Kind: "sensevoice", Files: map[string]string{"sense-voice-model": "model.int8.onnx", "tokens": "tokens.txt"},
	},
}

// ModelLabel preserves existing local model IDs while giving every family its own name.
func ModelLabel(name string) string {
	if model, ok := onnxModels[name]; ok {
		return model.Label
	}
	if _, ok := Models[name]; ok {
		return "Whisper " + strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

func AvailableModels() []map[string]string {
	models := make([]map[string]string, 0, len(Models)+len(onnxModels))
	for name, model := range Models {
		models = append(models, map[string]string{
			"name": name, "label": ModelLabel(name), "size": model.Size, "engine": "Whisper",
			"description": "Multilingual. Uses GPU acceleration when available. Larger models need more memory and processing time.",
		})
	}
	if _, err := sherpaArchive(); err == nil {
		for name, model := range onnxModels {
			models = append(models, map[string]string{
				"name": name, "label": model.Label, "size": model.Size, "engine": "Other local models",
				"description": model.Description,
			})
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i]["label"] < models[j]["label"] })
	return models
}

func (m *Manager) transcribeONNX(model onnxModel, audioPath, language string) (string, error) {
	if (model.Kind == "moonshine" || model.Directory == onnxModels["parakeet-v2"].Directory) &&
		language != "" && language != "auto" && language != "en" {
		return "", fmt.Errorf("%s supports English only; select English or Auto-Detect", model.Label)
	}
	args := []string{"--provider=cpu", "--num-threads=2", "--debug=false"}
	for flag, filename := range model.Files {
		args = append(args, "--"+flag+"="+filepath.Join(m.installDir, "models", model.Directory, filename))
	}
	if model.Kind == "parakeet" {
		args = append(args, "--model-type=nemo_transducer")
	}
	if model.Kind == "sensevoice" {
		if language == "" {
			language = "auto"
		}
		switch language {
		case "auto", "zh", "yue", "en", "ja", "ko":
		default:
			return "", fmt.Errorf("SenseVoice supports Chinese, Cantonese, English, Japanese and Korean; choose a supported language or Auto-Detect")
		}
		args = append(args, "--sense-voice-use-itn=true", "--sense-voice-language="+language)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	output, err := m.runSherpa(ctx, append(args, audioPath)...)
	if ctx.Err() != nil {
		return "", fmt.Errorf("local transcription did not finish: %w", ctx.Err())
	}
	if err != nil {
		return "", fmt.Errorf("%s transcription failed: %w: %s", model.Label, err, output)
	}
	// The CLI writes one JSON result to stderr, alongside its diagnostics.
	for _, line := range strings.Split(output, "\n") {
		var result struct {
			Text *string `json:"text"`
		}
		if json.Unmarshal([]byte(line), &result) == nil && result.Text != nil {
			return strings.Join(strings.Fields(*result.Text), " "), nil
		}
	}
	return "", fmt.Errorf("%s returned no transcription result", model.Label)
}

func (m *Manager) runSherpa(ctx context.Context, args ...string) (string, error) {
	binary, err := m.sherpaBinary()
	if err != nil {
		return "", err
	}
	// Release binaries keep their runtime libraries in the adjacent lib directory.
	libDir := filepath.Join(filepath.Dir(binary), "..", "lib")
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = filepath.Dir(binary)
	hideWindowContext(cmd)
	cmd.Env = os.Environ()
	key := "LD_LIBRARY_PATH"
	if runtime.GOOS == "windows" {
		key = "PATH"
	} else if runtime.GOOS == "darwin" {
		key = "DYLD_LIBRARY_PATH"
	}
	cmd.Env = append(cmd.Env, key+"="+libDir+string(os.PathListSeparator)+os.Getenv(key))
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
