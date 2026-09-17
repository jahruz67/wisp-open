//go:build !darwin

package platform

import (
	"fmt"
	"runtime"
)

func GetStatus() Status {
	return Status{
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		SetupRequired: false,
		Microphone:    PermissionUnavailable,
		Accessibility: PermissionUnavailable,
	}
}

func RequestPermission(kind string) error {
	return fmt.Errorf("%s permission is managed by %s", kind, runtime.GOOS)
}

func OpenPermissionSettings(kind string) error {
	return fmt.Errorf("%s permission settings are not available on %s", kind, runtime.GOOS)
}

func SetSettingsWindowVisible(visible bool) {}
