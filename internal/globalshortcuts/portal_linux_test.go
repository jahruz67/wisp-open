package globalshortcuts

import (
	"os"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

type wirePortal struct {
	conn    *dbus.Conn
	session dbus.ObjectPath
	closed  chan struct{}
}
type wireProperties struct{}

func (wireProperties) Get(string, string) (dbus.Variant, *dbus.Error) {
	return dbus.MakeVariant(uint32(2)), nil
}
func (p *wirePortal) respond(options map[string]dbus.Variant, sender dbus.Sender, results map[string]dbus.Variant) dbus.ObjectPath {
	name := strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_")
	path := dbus.ObjectPath(string(portalPath) + "/request/" + name + "/" + options["handle_token"].Value().(string))
	_ = p.conn.Emit(path, requestInterface+".Response", uint32(0), results)
	return path
}
func (p *wirePortal) CreateSession(options map[string]dbus.Variant, sender dbus.Sender) (dbus.ObjectPath, *dbus.Error) {
	_ = p.conn.Export(p, p.session, sessionInterface)
	return p.respond(options, sender, map[string]dbus.Variant{"session_handle": dbus.MakeVariant(string(p.session))}), nil
}
func (p *wirePortal) BindShortcuts(_ dbus.ObjectPath, shortcuts []binding, _ string, options map[string]dbus.Variant, sender dbus.Sender) (dbus.ObjectPath, *dbus.Error) {
	shortcuts[0].Properties["trigger_description"] = dbus.MakeVariant("Alt+Z")
	return p.respond(options, sender, map[string]dbus.Variant{"shortcuts": dbus.MakeVariant(shortcuts)}), nil
}
func (p *wirePortal) Close() *dbus.Error { close(p.closed); return nil }

// Run under dbus-run-session in CI. This tests actual D-Bus signatures, signal
// sender matching and early responses without opening a real permission UI.
func TestPortalOverSessionBus(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("run under dbus-run-session")
	}
	server, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	reply, err := server.RequestName(service, dbus.NameFlagDoNotQueue)
	if err != nil {
		t.Fatal(err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		t.Skip("a real desktop portal already owns this session bus")
	}
	p := &wirePortal{server, "/org/freedesktop/portal/desktop/session/wire/test", make(chan struct{})}
	if err := server.Export(p, portalPath, shortcutInterface); err != nil {
		t.Fatal(err)
	}
	if err := server.Export(wireProperties{}, portalPath, "org.freedesktop.DBus.Properties"); err != nil {
		t.Fatal(err)
	}
	pressed, released := make(chan struct{}), make(chan struct{})
	m := New(func() { close(pressed) }, func() { close(released) }, nil)
	t.Cleanup(m.Close)
	if status := m.Start("ALT+z"); !status.Ready {
		t.Fatalf("portal wire setup failed: %+v", status)
	}
	if err := server.Emit(portalPath, shortcutInterface+".Activated", p.session, shortcutID, uint64(1), map[string]dbus.Variant{}); err != nil {
		t.Fatal(err)
	}
	await(t, pressed, "wire press")
	if err := server.Emit(portalPath, shortcutInterface+".Deactivated", p.session, shortcutID, uint64(2), map[string]dbus.Variant{}); err != nil {
		t.Fatal(err)
	}
	await(t, released, "wire release")
	m.Close()
	await(t, p.closed, "portal session cleanup")
}
