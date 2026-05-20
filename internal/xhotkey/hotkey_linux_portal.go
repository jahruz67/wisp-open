//go:build linux

package hotkey

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"wis-free-v3/internal/logger"
)

func unwrapVariant(val interface{}) interface{} {
	for {
		if v, ok := val.(dbus.Variant); ok {
			val = v.Value()
		} else {
			break
		}
	}
	return val
}

const (
	portalBusName                    = "org.freedesktop.portal.Desktop"
	portalObjectPath                 = "/org/freedesktop/portal/desktop"
	ifaceGlobalShortcuts             = "org.freedesktop.portal.GlobalShortcuts"
	ifaceRequest                     = "org.freedesktop.portal.Request"
	ifaceSession                     = "org.freedesktop.portal.Session"
	wisfreeGlobalShortcutID          = "com.wisfree.push-to-record"
	envForcePortal                   = "WISFREE_USE_PORTAL_HOTKEY"
)

func usePortalBackend() bool {
	if os.Getenv(envForcePortal) == "0" {
		return false
	}
	return true
}

func randomPortalToken() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "tok" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func triggerForPortalSpec(mods []Modifier, key Key) string {
	var ctrl, shift, alt, super bool
	for _, m := range mods {
		switch m {
		case ModCtrl:
			ctrl = true
		case ModShift:
			shift = true
		case Mod1:
			alt = true
		case Mod4:
			super = true
		}
	}
	keyName := portalKeySpecName(key)
	if keyName == "" {
		return ""
	}
	var parts []string
	if ctrl {
		parts = append(parts, "Control")
	}
	if alt {
		parts = append(parts, "Alt")
	}
	if shift {
		parts = append(parts, "Shift")
	}
	if super {
		parts = append(parts, "Super")
	}
	if len(parts) == 0 {
		return keyName
	}
	return strings.Join(parts, "+") + "+" + keyName
}

func portalKeySpecName(key Key) string {
	if key >= KeyA && key <= KeyZ {
		return string(rune(key))
	}
	if key >= Key0 && key <= Key9 {
		return string(rune(key))
	}
	switch key {
	case KeySpace:
		return "Space"
	case KeyReturn:
		return "Return"
	case KeyEscape:
		return "Escape"
	case KeyDelete:
		return "Delete"
	case KeyTab:
		return "Tab"
	case KeyLeft:
		return "Left"
	case KeyRight:
		return "Right"
	case KeyUp:
		return "Up"
	case KeyDown:
		return "Down"
	case KeyF1:
		return "F1"
	case KeyF2:
		return "F2"
	case KeyF3:
		return "F3"
	case KeyF4:
		return "F4"
	case KeyF5:
		return "F5"
	case KeyF6:
		return "F6"
	case KeyF7:
		return "F7"
	case KeyF8:
		return "F8"
	case KeyF9:
		return "F9"
	case KeyF10:
		return "F10"
	case KeyF11:
		return "F11"
	case KeyF12:
		return "F12"
	default:
		return ""
	}
}

func portalWaitRequest(conn *dbus.Conn, reqPath dbus.ObjectPath) (uint32, map[string]dbus.Variant, error) {
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	rule := fmt.Sprintf(
		"type='signal',path='%s',interface='%s',member='Response'",
		string(reqPath), ifaceRequest,
	)
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Store(); err != nil {
		return 0, nil, err
	}
	defer func() {
		_ = conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule).Store()
	}()

	timeout := time.NewTimer(45 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case sig := <-ch:
			if sig == nil || sig.Path != reqPath {
				continue
			}
			if !strings.HasSuffix(sig.Name, ".Response") {
				continue
			}
			if len(sig.Body) < 2 {
				continue
			}
			rawCode := unwrapVariant(sig.Body[0])
			var code uint32
			switch x := rawCode.(type) {
			case uint32:
				code = x
			case int:
				code = uint32(x)
			case int32:
				code = uint32(x)
			case uint8:
				code = uint32(x)
			default:
				continue
			}
			rawResults := unwrapVariant(sig.Body[1])
			var results map[string]dbus.Variant
			switch resMap := rawResults.(type) {
			case map[string]dbus.Variant:
				results = resMap
			case map[string]interface{}:
				results = make(map[string]dbus.Variant)
				for k, val := range resMap {
					results[k] = dbus.MakeVariant(val)
				}
			}
			return code, results, nil
		case <-timeout.C:
			return 0, nil, fmt.Errorf("portal request timed out")
		}
	}
}

func variantToObjectPath(v dbus.Variant) (dbus.ObjectPath, bool) {
	val := unwrapVariant(v)
	switch x := val.(type) {
	case dbus.ObjectPath:
		return x, true
	case string:
		return dbus.ObjectPath(x), true
	default:
		return "", false
	}
}

// registerPortal binds a global shortcut via org.freedesktop.portal.GlobalShortcuts (Wayland / desktop-agnostic).
func (hk *Hotkey) registerPortal() error {
	trigger := triggerForPortalSpec(hk.mods, hk.key)
	if trigger == "" {
		return fmt.Errorf("unsupported key for portal global shortcuts")
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		return fmt.Errorf("dbus session: %w", err)
	}

	portal := conn.Object(portalBusName, portalObjectPath)

	createOpts := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(randomPortalToken()),
		"session_handle_token": dbus.MakeVariant(randomPortalToken()),
		// Required by xdg-desktop-portal on some desktops (e.g. Fedora/GNOME).
		// This should match the app's .desktop file id when possible.
		"app_id": dbus.MakeVariant("wis-free-v3"),
	}

	var createReqPath dbus.ObjectPath
	if err := portal.Call(ifaceGlobalShortcuts+".CreateSession", 0, createOpts).Store(&createReqPath); err != nil {
		_ = conn.Close()
		return fmt.Errorf("CreateSession: %w", err)
	}

	code, results, err := portalWaitRequest(conn, createReqPath)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("CreateSession wait: %w", err)
	}
	if code != 0 {
		_ = conn.Close()
		return fmt.Errorf("CreateSession rejected (code %d)", code)
	}
	v, ok := results["session_handle"]
	if !ok {
		_ = conn.Close()
		return errors.New("CreateSession: missing session_handle")
	}
	sessPath, okp := variantToObjectPath(v)
	if !okp || sessPath == "" {
		_ = conn.Close()
		return errors.New("CreateSession: invalid session_handle")
	}

	type portalShortcut struct {
		ID      string
		Details map[string]dbus.Variant
	}
	shortcutsArg := []portalShortcut{
		{
			ID: wisfreeGlobalShortcutID,
			Details: map[string]dbus.Variant{
				"description":       dbus.MakeVariant("Hold to dictate; release to transcribe (WIS Free)"),
				"preferred_trigger": dbus.MakeVariant(trigger),
			},
		},
	}

	bindOpts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(randomPortalToken()),
	}

	var bindReqPath dbus.ObjectPath
	if err := portal.Call(ifaceGlobalShortcuts+".BindShortcuts", 0, sessPath, shortcutsArg, "", bindOpts).Store(&bindReqPath); err != nil {
		_ = conn.Object(portalBusName, sessPath).Call(ifaceSession+".Close", 0).Store()
		_ = conn.Close()
		return fmt.Errorf("BindShortcuts: %w", err)
	}

	code, results, err = portalWaitRequest(conn, bindReqPath)
	if err != nil {
		_ = conn.Object(portalBusName, sessPath).Call(ifaceSession+".Close", 0).Store()
		_ = conn.Close()
		return err
	}
	if code != 0 {
		_ = conn.Object(portalBusName, sessPath).Call(ifaceSession+".Close", 0).Store()
		_ = conn.Close()
		return fmt.Errorf("BindShortcuts rejected (code %d); install a desktop with GlobalShortcuts portal support (e.g. recent KDE Plasma or GNOME)", code)
	}
	if sc, ok := results["shortcuts"]; ok {
		val := sc.Value()
		isEmpty := false
		switch v := val.(type) {
		case []interface{}:
			isEmpty = len(v) == 0
		case [][]interface{}:
			isEmpty = len(v) == 0
		case []map[string]interface{}:
			isEmpty = len(v) == 0
		}
		if isEmpty {
			_ = conn.Object(portalBusName, sessPath).Call(ifaceSession+".Close", 0).Store()
			_ = conn.Close()
			return fmt.Errorf("BindShortcuts returned empty shortcut list (desktop declined the binding)")
		}
	}

	hk.mu.Lock()
	if hk.registered {
		hk.mu.Unlock()
		_ = conn.Object(portalBusName, sessPath).Call(ifaceSession+".Close", 0).Store()
		_ = conn.Close()
		return errors.New("hotkey already registered.")
	}
	hk.backend = linuxHKPortal
	hk.registered = true
	hk.portalConn = conn
	hk.sessionPath = sessPath
	hk.portalStop = make(chan struct{})
	hk.portalDone = make(chan struct{})
	hk.mu.Unlock()

	go hk.portalSignalLoop()
	return nil
}

func (hk *Hotkey) portalSignalLoop() {
	defer close(hk.portalDone)

	hk.mu.Lock()
	conn := hk.portalConn
	sess := hk.sessionPath
	hk.mu.Unlock()
	if conn == nil {
		return
	}

	sessStr := string(sess)
	var token string
	if idx := strings.LastIndex(sessStr, "/"); idx != -1 {
		token = sessStr[idx+1:]
	} else {
		token = sessStr
	}

	ch := make(chan *dbus.Signal, 32)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	rule := fmt.Sprintf(
		"type='signal',interface='%s'",
		ifaceGlobalShortcuts,
	)
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Store(); err != nil {
		logger.Error("wis-free-v3 hotkey: AddMatch GlobalShortcuts: %v", err)
		return
	}
	defer func() { _ = conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, rule).Store() }()

	logger.Info("Listening for global shortcut signals. Session token: %s", token)

	for {
		select {
		case <-hk.portalStop:
			return
		case sig, ok := <-ch:
			if !ok || sig == nil {
				return
			}
			if len(sig.Body) < 2 {
				continue
			}
			rawSess := unwrapVariant(sig.Body[0])
			var sessVar dbus.ObjectPath
			switch x := rawSess.(type) {
			case dbus.ObjectPath:
				sessVar = x
			case string:
				sessVar = dbus.ObjectPath(x)
			default:
				continue
			}

			sessVarStr := string(sessVar)
			match := false
			if sessVar == sess {
				match = true
			} else if token != "" && (strings.HasSuffix(sessVarStr, "/"+token) || strings.Contains(sessVarStr, token)) {
				match = true
			}

			if !match {
				continue
			}

			rawID := unwrapVariant(sig.Body[1])
			id, ok := rawID.(string)
			if !ok || id != wisfreeGlobalShortcutID {
				continue
			}

			name := sig.Name
			logger.Info("Matched global shortcut signal: name=%s", name)

			switch {
			case name == "Activated" || strings.HasSuffix(name, ".Activated"):
				go func() { hk.keydownIn <- Event{} }()
			case name == "Deactivated" || strings.HasSuffix(name, ".Deactivated"):
				go func() { hk.keyupIn <- Event{} }()
			}
		}
	}
}

// cleanupPortal stops the portal listener and closes the session (unlock before call).
func (hk *Hotkey) cleanupPortal() error {
	hk.mu.Lock()
	stopCh := hk.portalStop
	doneCh := hk.portalDone
	conn := hk.portalConn
	sess := hk.sessionPath
	hk.portalStop = nil
	hk.portalDone = nil
	hk.portalConn = nil
	hk.sessionPath = ""
	hk.backend = linuxHKNone
	hk.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
	}
	if doneCh != nil {
		<-doneCh
	}
	if conn != nil && sess != "" {
		_ = conn.Object(portalBusName, sess).Call(ifaceSession+".Close", 0).Store()
		_ = conn.Close()
	}
	return nil
}
