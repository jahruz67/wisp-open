package tray

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"runtime"
)

// All platforms use the Windows artwork. GTK and AppKit receive its embedded PNG.
//
//go:embed icon.ico
var iconData []byte

var iconPNGData = func() []byte {
	data, err := icoToPNG(iconData)
	if err != nil {
		panic(err) // The bundled ICO must contain a PNG for non-Windows trays.
	}
	return data
}()

func DefaultIconBytes() []byte {
	if runtime.GOOS == "windows" {
		return iconData
	}
	return iconPNGData
}

func icoToPNG(data []byte) ([]byte, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("invalid ico data")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	var best []byte
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		if entry+16 > len(data) {
			break
		}
		size := uint64(binary.LittleEndian.Uint32(data[entry+8 : entry+12]))
		offset := uint64(binary.LittleEndian.Uint32(data[entry+12 : entry+16]))
		if size < 8 || offset+size > uint64(len(data)) {
			continue
		}
		candidate := data[offset : offset+size]
		if len(candidate) > len(best) && bytes.Equal(candidate[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) {
			best = candidate
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no PNG image found in ico")
	}
	return best, nil
}
