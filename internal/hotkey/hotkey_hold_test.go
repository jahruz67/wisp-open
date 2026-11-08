package hotkey

import (
	"testing"
	"time"

	xhk "wis-free-v3/internal/xhotkey"
)

func TestHeldShortcutIgnoresRepeatedActivationAfterSlowStart(t *testing.T) {
	down := make(chan xhk.Event)
	up := make(chan xhk.Event)
	started := make(chan struct{})
	allowStart := make(chan struct{})
	stopped := make(chan struct{}, 4)
	done := make(chan struct{})
	l := NewListener("alt+z", func() {
		close(started)
		<-allowStart // microphone initialization is slow after an idle period
	}, func() { stopped <- struct{}{} })
	go func() { l.runEventLoop(down, up); close(done) }()
	t.Cleanup(func() {
		close(down)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("hotkey event loop did not exit")
		}
	})
	down <- xhk.Event{}
	<-started
	close(allowStart)
	down <- xhk.Event{}
	select {
	case <-stopped:
		t.Fatal("repeat activation stopped recording while shortcut was held")
	case <-time.After(100 * time.Millisecond):
	}
	up <- xhk.Event{}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("physical shortcut release did not stop recording")
	}
}
