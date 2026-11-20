package tray

import "sync"

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
