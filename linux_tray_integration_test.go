//go:build linux

package main

import (
	"context"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

type registeredTrayItem struct {
	destination string
	path        dbus.ObjectPath
}
type testTrayWatcher struct{ registered chan registeredTrayItem }

func (w *testTrayWatcher) RegisterStatusNotifierItem(item string, sender dbus.Sender) *dbus.Error {
	registration := registeredTrayItem{item, "/StatusNotifierItem"}
	if strings.HasPrefix(item, "/") {
		registration = registeredTrayItem{string(sender), dbus.ObjectPath(item)}
	}
	w.registered <- registration
	return nil
}

// This checks the real packaged app's idle indicator, not just the PNG helper.
// CI runs it with Xvfb and a private D-Bus session, without a physical desktop.
func TestLinuxBackgroundLaunchKeepsWhiteTrayIcon(t *testing.T) {
	if os.Getenv("WIS_TEST_LINUX_TRAY") != "1" {
		t.Skip("set WIS_TEST_LINUX_TRAY=1 to launch the app in an isolated test desktop")
	}
	if os.Getenv("DISPLAY") == "" || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("requires Xvfb and dbus-run-session")
	}
	binary, err := filepath.Abs("build/bin/wis-free-v3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skip("build the Linux release binary first")
	}
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()
	const watcherInterface = "org.kde.StatusNotifierWatcher"
	reply, err := bus.RequestName(watcherInterface, dbus.NameFlagDoNotQueue)
	if err != nil {
		t.Fatal(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skip("a real desktop tray is already running")
	}
	watcher := &testTrayWatcher{make(chan registeredTrayItem, 8)}
	if err := bus.Export(watcher, "/StatusNotifierWatcher", watcherInterface); err != nil {
		t.Fatal(err)
	}
	if _, err := prop.Export(bus, "/StatusNotifierWatcher", prop.Map{watcherInterface: {
		"IsStatusNotifierHostRegistered": {Value: true, Emit: prop.EmitConst},
		"ProtocolVersion":                {Value: int32(0), Emit: prop.EmitConst},
		"RegisteredStatusNotifierItems":  {Value: []string{}, Emit: prop.EmitConst},
	}}); err != nil {
		t.Fatal(err)
	}

	directory := t.TempDir()
	logPath := filepath.Join(directory, "app.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--background")
	// Keep first-run configuration, recordings and icons isolated from the user.
	cmd.Env = append(os.Environ(), "HOME="+directory, "XDG_CONFIG_HOME="+filepath.Join(directory, "config"),
		"XDG_DATA_HOME="+filepath.Join(directory, "data"), "XDG_RUNTIME_DIR="+directory,
		"TMPDIR="+directory, "GDK_BACKEND=x11")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			content, _ := os.ReadFile(logPath)
			t.Logf("background app output:\n%s", content)
		}
	}()
	var item registeredTrayItem
	select {
	case item = <-watcher.registered:
	case <-ctx.Done():
		t.Fatal("background app never registered a tray icon")
	}
	object := bus.Object(item.destination, item.path)
	read := func(key string) dbus.Variant {
		requestCtx, requestCancel := context.WithTimeout(ctx, time.Second)
		defer requestCancel()
		var value dbus.Variant
		_ = object.CallWithContext(requestCtx, "org.freedesktop.DBus.Properties.Get", 0, "org.kde.StatusNotifierItem", key).Store(&value)
		return value
	}
	for {
		if ctx.Err() != nil {
			t.Fatal("idle tray never exposed its white microphone")
		}
		icon, _ := read("IconName").Value().(string)
		file, err := os.Open(icon)
		if err == nil {
			img, decodeErr := png.Decode(file)
			file.Close()
			if decodeErr == nil {
				got := color.RGBAModel.Convert(img.At(64, 50)).(color.RGBA)
				if got != (color.RGBA{255, 255, 255, 255}) {
					t.Fatalf("idle microphone color = %v, want white", got)
				}
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Ready must remain Active (a passive status lets the shell hide the item).
	for i := 0; i < 3; i++ {
		if got := read("Status").Value(); got != "Active" {
			t.Fatalf("idle tray status = %v, want Active", got)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
