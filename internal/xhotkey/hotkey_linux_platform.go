//go:build linux

package hotkey

import "sync"

type platformHotkey struct {
	mu         sync.Mutex
	registered bool
}
