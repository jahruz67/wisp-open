package tray

import (
	"bytes"
	"image/png"
	"runtime"
	"testing"
)

func TestTrayUsesWindowsArtwork(t *testing.T) {
	want := iconPNGData
	if runtime.GOOS == "windows" {
		want = iconData
	}
	if !bytes.Equal(DefaultIconBytes(), want) {
		t.Fatal("tray icon does not match the Windows artwork")
	}
	if _, err := png.Decode(bytes.NewReader(iconPNGData)); err != nil {
		t.Fatalf("Windows ICO must contain a usable PNG: %v", err)
	}
}
