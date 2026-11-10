package recorder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
)

type fakeDevice struct {
	startErr error
	stopErr  error
	starts   int
	stops    int
	uninit   int
}

func (d *fakeDevice) Start() error { d.starts++; return d.startErr }
func (d *fakeDevice) Stop() error  { d.stops++; return d.stopErr }
func (d *fakeDevice) Uninit()      { d.uninit++ }

type fakeContext struct {
	t         *testing.T
	devices   []malgo.DeviceInfo
	open      func() (*fakeDevice, error)
	opened    []*fakeDevice
	ids       []*malgo.DeviceID
	callbacks malgo.DeviceCallbacks
	uninit    int
	freed     int
}

func (c *fakeContext) Devices(malgo.DeviceType) ([]malgo.DeviceInfo, error) {
	return c.devices, nil
}

func (c *fakeContext) InitDevice(config malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (captureDevice, error) {
	c.callbacks = callbacks
	var id *malgo.DeviceID
	if config.Capture.DeviceID != nil {
		copyID := *(*malgo.DeviceID)(config.Capture.DeviceID)
		id = &copyID
	}
	c.ids = append(c.ids, id)
	d, err := c.open()
	if err == nil {
		c.opened = append(c.opened, d)
	}
	return d, err
}

func (c *fakeContext) Uninit() error {
	for _, device := range c.opened {
		if device.uninit != 1 {
			c.t.Errorf("context closed before device was released: uninit=%d", device.uninit)
		}
	}
	c.uninit++
	return nil
}

func (c *fakeContext) Free() {
	if c.uninit != 1 {
		c.t.Error("context freed before uninitialization")
	}
	c.freed++
}

func testRecorder(t *testing.T, initial *fakeContext, factory func() (captureContext, error)) *AudioRecorder {
	t.Helper()
	r := &AudioRecorder{ctx: initial, newContext: factory}
	t.Cleanup(r.Cleanup)
	return r
}

func checkWAV(t *testing.T, filename string, samples []byte) {
	t.Helper()
	wav, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if len(wav) != 44+len(samples) || string(wav[:4]) != "RIFF" ||
		binary.LittleEndian.Uint32(wav[40:44]) != uint32(len(samples)) ||
		string(wav[44:]) != string(samples) {
		t.Fatalf("recording was not finalized correctly: %x", wav)
	}
}

func TestRecordingAfterIdleUsesFreshEndpoint(t *testing.T) {
	ctx := &fakeContext{t: t, open: func() (*fakeDevice, error) { return &fakeDevice{}, nil }}
	r := testRecorder(t, ctx, func() (captureContext, error) {
		t.Fatal("healthy context should not need recovery")
		return nil, nil
	})
	for i := 0; i < 2; i++ {
		filename := filepath.Join(t.TempDir(), "recording.wav")
		if err := r.Start(filename); err != nil {
			t.Fatal(err)
		}
		samples := []byte{1, 0, 2, 0}
		ctx.callbacks.Data(nil, samples, 2)
		if err := r.Stop(); err != nil {
			t.Fatal(err)
		}
		checkWAV(t, filename, samples)
		if r.device != nil || r.outputFile != nil || r.IsRecording() {
			t.Fatal("recording resources retained while idle")
		}
		// Simulate the previous endpoint expiring while the app is idle.
		ctx.opened[i].startErr = errors.New("endpoint invalidated during idle")
	}
	if len(ctx.opened) != 2 || ctx.opened[0].starts != 1 {
		t.Fatal("reused an expired capture endpoint")
	}
}

func TestStartRecoversStaleAudioContext(t *testing.T) {
	for _, stage := range []string{"init", "start"} {
		for _, selected := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/selected=%t", stage, selected), func(t *testing.T) {
				staleErr := errors.New("audio service restarted while idle")
				failedDevice := &fakeDevice{startErr: staleErr}
				initial := &fakeContext{t: t, open: func() (*fakeDevice, error) {
					if stage == "init" {
						return nil, staleErr
					}
					return failedDevice, nil
				}}
				recovered := &fakeContext{t: t, open: func() (*fakeDevice, error) { return &fakeDevice{}, nil }}
				refreshes := 0
				r := testRecorder(t, initial, func() (captureContext, error) {
					refreshes++
					return recovered, nil
				})
				var id malgo.DeviceID
				id[0] = 42
				if selected {
					initial.devices = []malgo.DeviceInfo{{ID: id}}
					recovered.devices = initial.devices
					r.SetDevice(id.String())
				}
				filename := filepath.Join(t.TempDir(), "recording.wav")
				if err := r.Start(filename); err != nil {
					t.Fatal(err)
				}
				if refreshes != 1 || initial.uninit != 1 || initial.freed != 1 || !r.IsRecording() {
					t.Fatal("capture did not recover with a fresh audio context")
				}
				if selected && (recovered.ids[0] == nil || *recovered.ids[0] != id) {
					t.Fatal("recovery lost the selected microphone")
				}
				if !selected && recovered.ids[0] != nil {
					t.Fatal("system default should be resolved by the new context")
				}
				samples := []byte{7, 0, 8, 0}
				recovered.callbacks.Data(nil, samples, 2)
				r.Stop()
				checkWAV(t, filename, samples)
			})
		}
	}
}

func TestFailedRecoveryReleasesFileAndAllowsNextRecording(t *testing.T) {
	unavailable := errors.New("microphone unavailable")
	initial := &fakeContext{t: t, open: func() (*fakeDevice, error) { return nil, unavailable }}
	recovered := &fakeContext{t: t, open: func() (*fakeDevice, error) { return &fakeDevice{startErr: unavailable}, nil }}
	refreshes := 0
	r := testRecorder(t, initial, func() (captureContext, error) { refreshes++; return recovered, nil })
	defer r.Stop()
	filename := filepath.Join(t.TempDir(), "recording.wav")
	if err := r.Start(filename); !errors.Is(err, unavailable) {
		t.Fatalf("expected microphone error, got %v", err)
	}
	if refreshes != 1 || r.IsRecording() || r.device != nil || r.outputFile != nil || atomic.LoadInt32(&r.writing) != 0 {
		t.Fatal("failed capture left resources active or retried without a limit")
	}
	if err := os.Remove(filename); err != nil {
		t.Fatalf("failed capture left the WAV file open: %v", err)
	}
	recovered.open = func() (*fakeDevice, error) { return &fakeDevice{}, nil }
	if err := r.Start(filename); err != nil {
		t.Fatalf("subsequent shortcut could not start: %v", err)
	}
}

func TestContextRefreshFailureCanRecoverOnNextShortcut(t *testing.T) {
	unavailable := errors.New("audio service unavailable")
	initial := &fakeContext{t: t, open: func() (*fakeDevice, error) { return nil, unavailable }}
	recovered := &fakeContext{t: t, open: func() (*fakeDevice, error) { return &fakeDevice{}, nil }}
	refreshes := 0
	r := testRecorder(t, initial, func() (captureContext, error) {
		refreshes++
		if refreshes == 1 {
			return nil, unavailable
		}
		return recovered, nil
	})
	defer r.Stop()
	filename := filepath.Join(t.TempDir(), "recording.wav")
	if err := r.Start(filename); !errors.Is(err, unavailable) {
		t.Fatalf("expected refresh error, got %v", err)
	}
	if err := r.Start(filename); err != nil {
		t.Fatalf("recorder remained unusable after audio service returned: %v", err)
	}
}

func TestCleanupFinalizesActiveRecording(t *testing.T) {
	ctx := &fakeContext{t: t, open: func() (*fakeDevice, error) {
		return &fakeDevice{stopErr: errors.New("device disconnected")}, nil
	}}
	r := testRecorder(t, ctx, nil)
	filename := filepath.Join(t.TempDir(), "recording.wav")
	if err := r.Start(filename); err != nil {
		t.Fatal(err)
	}
	samples := []byte{1, 0}
	ctx.callbacks.Data(nil, samples, 1)
	r.Cleanup()
	ctx.callbacks.Data(nil, samples, 1) // late callbacks must not touch the closed file
	checkWAV(t, filename, samples)
	if r.IsRecording() || ctx.freed != 1 || r.outputFile != nil {
		t.Fatal("cleanup left recording resources active")
	}
	if err := r.Start(filename); err == nil {
		t.Fatal("closed recorder accepted a new capture")
	}
}

func TestNativeCaptureLifecycleWithSelectedDevice(t *testing.T) {
	// The null backend exercises the real C bindings without recording a mic.
	// malgo v0.11.24 omits ma_backend_custom from its Go enum; its bundled C
	// header places ma_backend_null at 14, one after the Go BackendNull value.
	const nullBackend malgo.Backend = malgo.BackendNull + 1
	ctx, err := malgo.InitContext([]malgo.Backend{nullBackend}, malgo.ContextConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := &AudioRecorder{
		ctx:          &nativeContext{ctx},
		newContext:   newCaptureContext,
		deviceConfig: malgo.DefaultDeviceConfig(malgo.Capture),
	}
	t.Cleanup(r.Cleanup)
	r.deviceConfig.Capture.Format = malgo.FormatS16
	r.deviceConfig.Capture.Channels = NumChannels
	r.deviceConfig.SampleRate = SampleRate
	devices, err := ctx.Devices(malgo.Capture)
	if err != nil || len(devices) == 0 {
		t.Fatalf("null backend did not enumerate a capture device: %v", err)
	}
	r.SetDevice(devices[0].ID.String())
	for i := 0; i < 2; i++ {
		filename := filepath.Join(t.TempDir(), "recording.wav")
		if err := r.Start(filename); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for atomic.LoadUint32(&r.dataSize) == 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		r.Stop()
		if r.dataSize == 0 {
			t.Fatal("native capture did not deliver audio callbacks")
		}
		checkWAV(t, filename, make([]byte, r.dataSize))
	}
}
