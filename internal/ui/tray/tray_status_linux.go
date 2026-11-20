package tray

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// LinuxHostStatus detects missing tray support instead of making a hidden
// background app look as if it has exited. The shell controls panel placement.
func LinuxHostStatus() HostStatus {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dbus.SessionBus()
	if err == nil {
		var property dbus.Variant
		err = conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher").CallWithContext(ctx,
			"org.freedesktop.DBus.Properties.Get", 0, "org.kde.StatusNotifierWatcher", "IsStatusNotifierHostRegistered").Store(&property)
		var host bool
		if err == nil && property.Store(&host) == nil && host {
			return HostStatus{Ready: true, Message: "Tray support is available. The WIS icon stays in the panel while the app is running. A floating overlay shows recording and transcription activity."}
		}
	}
	desktop := strings.ToUpper(os.Getenv("XDG_CURRENT_DESKTOP"))
	if strings.Contains(desktop, "GNOME") {
		return HostStatus{Message: "Enable AppIndicator and KStatusNotifierItem Support in GNOME Extensions to keep the WIS icon in the panel. Install gnome-shell-extension-appindicator if needed, then click Check again."}
	}
	if strings.Contains(desktop, "KDE") {
		return HostStatus{Message: "Add or enable Plasma's System Tray widget, then set WIS Free V3 to Always shown under System Tray Settings → Entries. Click Check again after changing it."}
	}
	return HostStatus{Message: "No StatusNotifier system tray host is available. Enable your desktop's tray or AppIndicator support, then click Check again."}
}
