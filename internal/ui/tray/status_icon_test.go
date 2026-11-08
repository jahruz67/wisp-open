//go:build !darwin

package tray

import (
	"bytes"
	"image/color"
	"image/png"
	"runtime"
	"testing"
)

func TestMicrophoneStatusColors(t *testing.T) {
	tests := []struct {
		status string
		want   color.RGBA
	}{
		{"Recording...", color.RGBA{255, 59, 48, 255}},
		{"Processing...", color.RGBA{255, 204, 0, 255}},
		{"Transcribing...", color.RGBA{255, 204, 0, 255}},
		{"Typing...", color.RGBA{255, 204, 0, 255}},
	}
	if runtime.GOOS == "linux" {
		tests = append(tests, struct {
			status string
			want   color.RGBA
		}{"Ready", color.RGBA{255, 255, 255, 255}})
	}
	for _, test := range tests {
		t.Run(test.status, func(t *testing.T) {
			img, err := png.Decode(bytes.NewReader(statusIcon(test.status)))
			if err != nil {
				t.Fatal(err)
			}
			got := color.RGBAModel.Convert(img.At(64, 50)).(color.RGBA)
			if got != test.want {
				t.Errorf("microphone color = %v, want %v", got, test.want)
			}
		})
	}
}
