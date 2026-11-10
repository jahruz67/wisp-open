//go:build !darwin

package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"wis-free-v3/internal/ui/tray"
)

type blockingRecordingOverlay struct {
	shown   chan struct{}
	proceed chan struct{}
}

func (o *blockingRecordingOverlay) Show(string)       { close(o.shown); <-o.proceed }
func (o *blockingRecordingOverlay) Hide()             {}
func (o *blockingRecordingOverlay) SetVolume(float64) {}
func (o *blockingRecordingOverlay) Close()            {}

func TestShortcutReleaseWaitsForRecordingStartup(t *testing.T) {
	overlay := &blockingRecordingOverlay{shown: make(chan struct{}), proceed: make(chan struct{})}
	a := &App{overlay: overlay}
	started := make(chan struct{})
	go func() { a.StartRecording(); close(started) }()
	<-overlay.shown // startup is in progress, before capture is ready
	if got := tray.CurrentStatus(); got != "Recording..." {
		t.Fatalf("held shortcut status = %q", got)
	}
	stopped := make(chan struct{})
	go func() { a.StopRecording(); close(stopped) }()
	select {
	case <-stopped:
		t.Error("shortcut release overtook recording startup")
	case <-time.After(50 * time.Millisecond):
	}
	if atomic.LoadInt32(&a.recording) != 1 {
		t.Error("shortcut release changed state before startup completed")
	}
	close(overlay.proceed)
	for _, done := range []<-chan struct{}{started, stopped} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("recording lifecycle did not finish")
		}
	}
}

func TestProcessingRemainsYellowThroughSlowTextInsertion(t *testing.T) {
	a := &App{transcribing: 1}
	tray.UpdateStatus("Processing...")
	inserting, finishTyping, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		a.completeProcessing(func() {
			tray.UpdateStatus("Transcribing...")
			a.finishTextInsertion(func() { close(inserting); <-finishTyping })
		})
		close(done)
	}()
	<-inserting
	if got := tray.CurrentStatus(); got != "Typing..." {
		t.Errorf("status during text insertion = %q", got)
	}
	// A fresh hold while busy must not turn the icon red or create a new clip.
	a.StartRecording()
	if atomic.LoadInt32(&a.recording) != 0 || atomic.LoadInt32(&a.transcribing) != 1 {
		t.Error("new recording interrupted text insertion")
	}
	if got := tray.CurrentStatus(); got != "Typing..." {
		t.Errorf("busy hold changed status to %q", got)
	}
	close(finishTyping)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("typing did not finish")
	}
	if got := tray.CurrentStatus(); got != "Ready" {
		t.Errorf("completed status = %q", got)
	}
	if atomic.LoadInt32(&a.transcribing) != 0 {
		t.Error("app remained busy after insertion")
	}
}

func TestDiscardedRecordingReturnsToReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tiny.wav")
	if err := os.WriteFile(path, []byte("tiny clip"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{transcribing: 1}
	tray.UpdateStatus("Processing...")
	a.processRecording(path)
	if got := tray.CurrentStatus(); got != "Ready" {
		t.Errorf("discarded clip status = %q", got)
	}
	if atomic.LoadInt32(&a.transcribing) != 0 {
		t.Error("discarded clip left app busy")
	}
}

func TestRecordingStartupFailureReturnsToReady(t *testing.T) {
	a := &App{}
	a.StartRecording()
	if got := tray.CurrentStatus(); got != "Ready" {
		t.Errorf("failed start status = %q", got)
	}
	if atomic.LoadInt32(&a.recording) != 0 {
		t.Error("failed microphone startup left recording active")
	}
}
