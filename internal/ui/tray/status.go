package tray

import (
	"strings"
	"sync"
)

var statusMu sync.Mutex
var lastStatus = "Ready"

type HostStatus struct {
	Ready   bool   `json:"ready"`
	Message string `json:"message"`
}

// CurrentStatus also reflects transitions before the native tray has loaded.
func CurrentStatus() string {
	statusMu.Lock()
	defer statusMu.Unlock()
	return lastStatus
}

// Every phase after key release stays yellow until text insertion completes.
func isProcessingStatus(status string) bool {
	return strings.Contains(status, "Processing") || strings.Contains(status, "Transcribing") || strings.Contains(status, "Typing")
}
