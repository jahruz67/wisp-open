// Package globalshortcuts implements hold-to-record with the XDG GlobalShortcuts
// portal. Desktop command shortcuts cannot report a physical key release.
package globalshortcuts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	service           = "org.freedesktop.portal.Desktop"
	portalPath        = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	shortcutInterface = "org.freedesktop.portal.GlobalShortcuts"
	requestInterface  = "org.freedesktop.portal.Request"
	sessionInterface  = "org.freedesktop.portal.Session"
	shortcutID        = "dictate"
)

type Status struct {
	Ready   bool   `json:"ready"`
	Busy    bool   `json:"busy"`
	Message string `json:"message"`
	Trigger string `json:"trigger"`
}

type binding struct {
	ID         string
	Properties map[string]dbus.Variant
}

type portalConnection interface {
	Object(string, dbus.ObjectPath) dbus.BusObject
	AddMatchSignal(...dbus.MatchOption) error
	Signal(chan<- *dbus.Signal)
	RemoveSignal(chan<- *dbus.Signal)
	Names() []string
	Context() context.Context
	Close() error
}

// Manager owns a private bus connection and one portal session. Callbacks run
// in order: a release received during slow microphone startup is never lost.
type Manager struct {
	opMu               sync.Mutex
	mu                 sync.RWMutex
	status             Status
	conn               portalConnection
	session            dbus.ObjectPath
	version            uint32
	cancel             context.CancelFunc
	done               chan struct{}
	closed             bool
	onPress, onRelease func()
	onStatus           func(Status)
	connect            func() (portalConnection, error)
}

func New(onPress, onRelease func(), onStatus func(Status)) *Manager {
	return &Manager{onPress: onPress, onRelease: onRelease, onStatus: onStatus,
		connect: func() (portalConnection, error) { return dbus.ConnectSessionBus() },
		status:  Status{Message: "Set up hold-to-record and approve the shortcut in your desktop's dialog."}}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(status Status) {
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
	if m.onStatus != nil {
		m.onStatus(status)
	}
}

// Start binds the action. Call on login only after the user has completed
// setup; the desktop remembers permission and the user's chosen trigger.
func (m *Manager) Start(preferredTrigger string) Status {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	closed, live := m.closed, m.conn != nil
	m.mu.RUnlock()
	if closed || live {
		return m.Status()
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		return m.Status()
	}
	m.cancel, m.done = cancel, done
	m.mu.Unlock()
	m.setStatus(Status{Busy: true, Message: "Waiting for desktop shortcut permission…"})
	conn, session, version, signals, err := openSession(ctx, preferredTrigger, m.connect)
	if err != nil {
		cancel()
		m.setStatus(Status{Message: err.Error()})
		close(done)
		return m.Status()
	}
	m.mu.Lock()
	m.conn, m.session, m.version = conn, session.path, version
	m.mu.Unlock()
	m.setStatus(bindingStatus(session.shortcuts))
	go m.listen(ctx, conn, session.path, signals, done)
	return m.Status()
}

// Configure uses the desktop's UI, so the effective shortcut and conflict
// handling stay under the desktop's control instead of silently rebinding it.
func (m *Manager) Configure(preferredTrigger string) Status {
	m.mu.RLock()
	conn, session, version := m.conn, m.session, m.version
	m.mu.RUnlock()
	if conn == nil {
		return m.Start(preferredTrigger)
	}
	if version < 2 {
		status := m.Status()
		status.Message = "Change Hold to record in your desktop's application shortcut settings."
		return status
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Object(service, portalPath).CallWithContext(ctx, shortcutInterface+".ConfigureShortcuts", 0,
		session, "", map[string]dbus.Variant{}).Err; err != nil {
		status := m.Status()
		status.Message = "Could not open shortcut settings: " + err.Error()
		return status
	}
	return m.Status()
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	cancel, done := m.cancel, m.done
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

type sessionInfo struct {
	path      dbus.ObjectPath
	shortcuts []binding
}

func openSession(parent context.Context, trigger string, connect func() (portalConnection, error)) (conn portalConnection, session sessionInfo, version uint32, signals chan *dbus.Signal, err error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	conn, err = connect()
	if err != nil {
		return nil, session, 0, nil, fmt.Errorf("Hold-to-record cannot connect to the desktop session: %w", err)
	}
	defer func() {
		if err != nil {
			if session.path.IsValid() {
				closeObject(conn, session.path, sessionInterface)
			}
			conn.Close()
		}
	}()
	var property dbus.Variant
	err = conn.Object(service, portalPath).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0,
		shortcutInterface, "version").Store(&property)
	if err != nil {
		err = fmt.Errorf("Hold-to-record needs the Global Shortcuts portal. Install or update xdg-desktop-portal and your desktop backend (GNOME 48+ or KDE Plasma). Details: %w", err)
		return
	}
	if err = property.Store(&version); err != nil {
		return
	}
	signals = make(chan *dbus.Signal, 128)
	conn.Signal(signals)
	for _, iface := range []string{shortcutInterface, sessionInterface, requestInterface} {
		if err = conn.AddMatchSignal(dbus.WithMatchSender(service), dbus.WithMatchInterface(iface)); err != nil {
			return
		}
	}
	if err = conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, service)); err != nil {
		return
	}
	var results map[string]dbus.Variant
	results, err = request(ctx, conn, "CreateSession", nil, map[string]dbus.Variant{"session_handle_token": dbus.MakeVariant(token())})
	if err != nil {
		return
	}
	var path string
	if err = results["session_handle"].Store(&path); err != nil {
		return
	}
	session.path = dbus.ObjectPath(path)
	if !session.path.IsValid() {
		err = fmt.Errorf("desktop returned an invalid shortcut session")
		return
	}
	shortcuts := []binding{{shortcutID, map[string]dbus.Variant{
		"description":       dbus.MakeVariant("Hold to record"),
		"preferred_trigger": dbus.MakeVariant(trigger),
	}}}
	results, err = request(ctx, conn, "BindShortcuts", []interface{}{session.path, shortcuts, ""}, nil)
	if err != nil {
		return
	}
	err = results["shortcuts"].Store(&session.shortcuts)
	return
}

func token() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	return "wis_" + hex.EncodeToString(data[:])
}

func request(ctx context.Context, conn portalConnection, method string, args []interface{}, options map[string]dbus.Variant) (map[string]dbus.Variant, error) {
	if options == nil {
		options = map[string]dbus.Variant{}
	}
	requestToken := token()
	options["handle_token"] = dbus.MakeVariant(requestToken)
	// Subscribe before the method call: a fast backend may emit Response before
	// the method's reply reaches us. A separate channel preserves key events.
	responses := make(chan *dbus.Signal, 16)
	conn.Signal(responses)
	defer conn.RemoveSignal(responses)
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	path := dbus.ObjectPath(string(portalPath) + "/request/" + sender + "/" + requestToken)
	defer func() {
		if ctx.Err() != nil {
			closeObject(conn, path, requestInterface)
		}
	}()
	if err := conn.Object(service, portalPath).CallWithContext(ctx, shortcutInterface+"."+method, 0,
		append(args, options)...).Store(&path); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("shortcut setup was cancelled or timed out: %w", ctx.Err())
		case signal, ok := <-responses:
			if !ok {
				return nil, fmt.Errorf("desktop shortcut connection closed")
			}
			if signal.Name != requestInterface+".Response" || signal.Path != path {
				continue
			}
			var code uint32
			var result map[string]dbus.Variant
			if err := dbus.Store(signal.Body, &code, &result); err != nil {
				return nil, err
			}
			if code == 1 {
				return nil, fmt.Errorf("Shortcut permission was cancelled. Click Set up hold-to-record to try again.")
			}
			if code != 0 {
				return nil, fmt.Errorf("Desktop shortcut setup failed (response %d). Check your desktop portal service.", code)
			}
			return result, nil
		}
	}
}

func closeObject(conn portalConnection, path dbus.ObjectPath, iface string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = conn.Object(service, path).CallWithContext(ctx, iface+".Close", 0).Err
}

func bindingStatus(shortcuts []binding) Status {
	for _, shortcut := range shortcuts {
		if shortcut.ID == shortcutID {
			trigger, _ := shortcut.Properties["trigger_description"].Value().(string)
			if strings.TrimSpace(trigger) != "" {
				return Status{Ready: true, Trigger: trigger, Message: "Hold " + trigger + " to record. Release to transcribe and type."}
			}
		}
	}
	return Status{Message: "Choose a key for Hold to record in your desktop's shortcut settings."}
}

func (m *Manager) listen(ctx context.Context, conn portalConnection, session dbus.ObjectPath, signals <-chan *dbus.Signal, done chan struct{}) {
	held := false
	defer func() {
		// Explicit Close is app shutdown; recorder cleanup stops the microphone
		// without launching a new transcription while the app is quitting.
		if held && ctx.Err() == nil {
			m.onRelease()
		}
		closeObject(conn, session, sessionInterface)
		conn.Close()
		m.opMu.Lock()
		defer m.opMu.Unlock()
		m.mu.Lock()
		m.conn = nil
		m.mu.Unlock()
		m.setStatus(Status{Message: "Hold-to-record is disconnected. Set up the shortcut again."})
		close(done)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-conn.Context().Done():
			return
		case signal, ok := <-signals:
			if !ok {
				return
			}
			if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" {
				return
			}
			if signal.Name == sessionInterface+".Closed" && signal.Path == session {
				return
			}
			if len(signal.Body) < 2 || signal.Body[0] != session {
				continue
			}
			if signal.Name == shortcutInterface+".ShortcutsChanged" {
				var shortcuts []binding
				if dbus.Store(signal.Body[1:], &shortcuts) == nil {
					status := bindingStatus(shortcuts)
					if !status.Ready && held {
						held = false
						m.onRelease()
					}
					m.setStatus(status)
				}
				continue
			}
			if signal.Body[1] != shortcutID {
				continue
			}
			switch signal.Name {
			case shortcutInterface + ".Activated":
				if !held && m.Status().Ready {
					held = true
					m.onPress()
				}
			case shortcutInterface + ".Deactivated":
				if held {
					held = false
					m.onRelease()
				}
			}
		}
	}
}
