package globalshortcuts

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// The fake emits portal responses before returning method handles, exercising
// the race that otherwise makes very fast (restored) bindings hang at login.
type fakePortal struct {
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	subscribers  map[chan<- *dbus.Signal]bool
	session      dbus.ObjectPath
	bound        []binding
	responseCode uint32
	hang         bool
	started      chan struct{}
	closes       int
}

func newFakePortal() *fakePortal {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakePortal{ctx: ctx, cancel: cancel, subscribers: make(map[chan<- *dbus.Signal]bool),
		session: "/org/freedesktop/portal/desktop/session/test/wis", started: make(chan struct{}, 1)}
}

func (f *fakePortal) Object(_ string, path dbus.ObjectPath) dbus.BusObject {
	return fakeObject{f: f, path: path}
}
func (f *fakePortal) AddMatchSignal(...dbus.MatchOption) error { return nil }
func (f *fakePortal) Names() []string                          { return []string{":1.42"} }
func (f *fakePortal) Context() context.Context                 { return f.ctx }
func (f *fakePortal) Close() error                             { f.cancel(); return nil }
func (f *fakePortal) Signal(ch chan<- *dbus.Signal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subscribers[ch] = true
}
func (f *fakePortal) RemoveSignal(ch chan<- *dbus.Signal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.subscribers, ch)
}
func (f *fakePortal) emit(path dbus.ObjectPath, name string, body ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for ch := range f.subscribers {
		ch <- &dbus.Signal{Path: path, Name: name, Body: body}
	}
}

type fakeObject struct {
	dbus.BusObject
	f    *fakePortal
	path dbus.ObjectPath
}

func (o fakeObject) CallWithContext(_ context.Context, method string, _ dbus.Flags, args ...interface{}) *dbus.Call {
	f := o.f
	switch method {
	case "org.freedesktop.DBus.Properties.Get":
		return &dbus.Call{Body: []interface{}{dbus.MakeVariant(uint32(2))}}
	case sessionInterface + ".Close", requestInterface + ".Close":
		f.mu.Lock()
		f.closes++
		f.mu.Unlock()
		return &dbus.Call{}
	case shortcutInterface + ".CreateSession", shortcutInterface + ".BindShortcuts":
		options := args[len(args)-1].(map[string]dbus.Variant)
		path := dbus.ObjectPath(string(portalPath) + "/request/1_42/" + options["handle_token"].Value().(string))
		results := map[string]dbus.Variant{"session_handle": dbus.MakeVariant(string(f.session))}
		if method == shortcutInterface+".BindShortcuts" {
			f.mu.Lock()
			f.bound = args[1].([]binding)
			f.mu.Unlock()
			results = map[string]dbus.Variant{"shortcuts": dbus.MakeVariant([]binding{{shortcutID, map[string]dbus.Variant{
				"trigger_description": dbus.MakeVariant("Alt+Z"),
			}}})}
		}
		select {
		case f.started <- struct{}{}:
		default:
		}
		if !f.hang {
			f.emit(path, requestInterface+".Response", f.responseCode, results)
		}
		return &dbus.Call{Body: []interface{}{path}}
	}
	return &dbus.Call{Err: dbus.NewError("org.freedesktop.DBus.Error.UnknownMethod", nil)}
}

func fakeManager(f *fakePortal, press, release func()) *Manager {
	m := New(press, release, nil)
	m.connect = func() (portalConnection, error) { return f, nil }
	return m
}

func await(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for " + what)
	}
}

func TestPortalHoldReleaseAndRepeatedActivation(t *testing.T) {
	f := newFakePortal()
	pressed := make(chan struct{}, 8)
	released := make(chan struct{}, 8)
	m := fakeManager(f, func() { pressed <- struct{}{} }, func() { released <- struct{}{} })
	t.Cleanup(m.Close)
	if status := m.Start("ALT+z"); !status.Ready || status.Trigger != "Alt+Z" {
		t.Fatalf("not ready: %+v", status)
	}
	if dbus.SignatureOf(f.bound).String() != "a(sa{sv})" {
		t.Fatal("invalid shortcut D-Bus signature")
	}
	if got := f.bound[0].Properties["preferred_trigger"].Value(); got != "ALT+z" {
		t.Fatalf("preferred trigger = %v", got)
	}
	f.emit(portalPath, shortcutInterface+".Activated", dbus.ObjectPath("/other"), shortcutID)
	f.emit(portalPath, shortcutInterface+".Activated", f.session, "other_action")
	f.emit(portalPath, shortcutInterface+".Activated", f.session, shortcutID)
	await(t, pressed, "press")
	for i := 0; i < 5; i++ {
		f.emit(portalPath, shortcutInterface+".Activated", f.session, shortcutID)
	}
	f.emit(portalPath, shortcutInterface+".Deactivated", f.session, shortcutID)
	await(t, released, "release")
	select {
	case <-pressed:
		t.Fatal("auto-repeat started another recording")
	default:
	}
	select {
	case <-released:
		t.Fatal("auto-repeat toggled recording off")
	default:
	}
}

func TestPortalReleaseDuringSlowMicrophoneStartup(t *testing.T) {
	f := newFakePortal()
	started, allowStart, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m := fakeManager(f, func() { close(started); <-allowStart }, func() { close(stopped) })
	if !m.Start("ALT+z").Ready {
		t.Fatal("not ready")
	}
	f.emit(portalPath, shortcutInterface+".Activated", f.session, shortcutID)
	await(t, started, "slow start")
	f.emit(portalPath, shortcutInterface+".Activated", f.session, shortcutID)
	f.emit(portalPath, shortcutInterface+".Deactivated", f.session, shortcutID)
	close(allowStart)
	await(t, stopped, "release queued during startup")
	m.Close()
}

func TestPortalSessionLossStopsHeldRecording(t *testing.T) {
	for _, event := range []string{sessionInterface + ".Closed", "org.freedesktop.DBus.NameOwnerChanged"} {
		t.Run(event, func(t *testing.T) {
			f := newFakePortal()
			pressed, released := make(chan struct{}), make(chan struct{})
			var stops atomic.Int32
			m := fakeManager(f, func() { close(pressed) }, func() { stops.Add(1); close(released) })
			if !m.Start("ALT+z").Ready {
				t.Fatal("not ready")
			}
			f.emit(portalPath, shortcutInterface+".Activated", f.session, shortcutID)
			await(t, pressed, "press")
			f.emit(f.session, event)
			await(t, released, "release on session loss")
			m.Close()
			if m.Status().Ready || stops.Load() != 1 {
				t.Fatalf("bad disconnected state: %+v, stops=%d", m.Status(), stops.Load())
			}
		})
	}
}

func TestPortalPermissionCancellation(t *testing.T) {
	f := newFakePortal()
	f.responseCode = 1
	m := fakeManager(f, func() { t.Error("recording without permission") }, func() {})
	status := m.Start("ALT+z")
	if status.Ready || status.Busy || !strings.Contains(status.Message, "cancelled") {
		t.Fatalf("unexpected status: %+v", status)
	}
	m.Close()
}

func TestPortalCloseCancelsPendingPermission(t *testing.T) {
	f := newFakePortal()
	f.hang = true
	m := fakeManager(f, func() {}, func() {})
	finished := make(chan struct{})
	go func() { m.Start("ALT+z"); close(finished) }()
	await(t, f.started, "pending request")
	m.Close()
	await(t, finished, "cancelled setup")
	f.mu.Lock()
	closes := f.closes
	f.mu.Unlock()
	if closes == 0 {
		t.Fatal("pending desktop permission request was not closed")
	}
}

func TestBindingWithoutKeyIsNotReady(t *testing.T) {
	if status := bindingStatus([]binding{{shortcutID, map[string]dbus.Variant{}}}); status.Ready {
		t.Fatal("unbound shortcut reported ready")
	}
}
