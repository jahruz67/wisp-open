package whisper

import (
	"encoding/hex"
	"testing"
)

func TestModelDownloadMetadata(t *testing.T) {
	filenames := make(map[string]bool)
	for name, model := range Models {
		if model.Filename == "" || model.Size == "" || model.Bytes <= 0 {
			t.Fatalf("model %q has incomplete download metadata", name)
		}
		if filenames[model.Filename] {
			t.Fatalf("model filename %q is duplicated", model.Filename)
		}
		filenames[model.Filename] = true
		digest, err := hex.DecodeString(model.SHA256)
		if err != nil || len(digest) != 32 {
			t.Fatalf("model %q has invalid SHA-256 %q", name, model.SHA256)
		}
	}
}
