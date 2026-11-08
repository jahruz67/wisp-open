// Package recorder provides audio capture functionality using the miniaudio library.
// It records audio from the system's default capture device to WAV files.
package recorder

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"unsafe"
	"wis-free-v3/internal/logger"

	"github.com/gen2brain/malgo"
)

// Audio recording configuration
const (
	SampleRate     = 16000 // Hz - optimal for speech recognition
	NumChannels    = 1     // Mono audio
	BitsPerSample  = 16    // 16-bit PCM
	BytesPerSample = BitsPerSample / 8
)

// MicrophoneInfo contains information about an available audio capture device.
type MicrophoneInfo struct {
	ID        string
	Name      string
	IsDefault bool
}

// VolumeCallback is called when new volume levels are calculated
type VolumeCallback func(level float64)

// Keep native resource ownership behind small interfaces so recovery can be
// exercised without depending on a physical microphone or a running audio server.
type captureDevice interface {
	Start() error
	Stop() error
	Uninit()
}

type captureContext interface {
	Devices(malgo.DeviceType) ([]malgo.DeviceInfo, error)
	InitDevice(malgo.DeviceConfig, malgo.DeviceCallbacks) (captureDevice, error)
	Uninit() error
	Free()
}

type nativeContext struct {
	*malgo.AllocatedContext
}

func (c *nativeContext) InitDevice(config malgo.DeviceConfig, callbacks malgo.DeviceCallbacks) (captureDevice, error) {
	return malgo.InitDevice(c.Context, config, callbacks)
}

func newCaptureContext() (captureContext, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	return &nativeContext{ctx}, nil
}

// AudioRecorder handles audio capture from the system microphone.
type AudioRecorder struct {
	ctx          captureContext
	newContext   func() (captureContext, error)
	deviceConfig malgo.DeviceConfig
	device       captureDevice
	outputFile   *os.File
	dataSize     uint32
	isRecording  bool
	closed       bool
	writing      int32 // atomic: 1 = safe to write to outputFile, 0 = no longer writing
	deviceID     *string
	OnVolume     VolumeCallback
	mu           sync.Mutex
}

// NewRecorder creates a new audio recorder instance.
// Returns an error if the audio context cannot be initialized.
func NewRecorder() (*AudioRecorder, error) {
	ctx, err := newCaptureContext()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio context: %w", err)
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = NumChannels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.Alsa.NoMMap = 1

	return &AudioRecorder{
		ctx:          ctx,
		newContext:   newCaptureContext,
		deviceConfig: deviceConfig,
	}, nil
}

// GetMicrophones returns a list of available audio capture devices.
func (r *AudioRecorder) GetMicrophones() ([]MicrophoneInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx == nil {
		return nil, fmt.Errorf("audio context not initialized")
	}

	devices, err := r.ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate devices: %w", err)
	}

	mics := []MicrophoneInfo{
		{ID: "", Name: "System Default", IsDefault: true},
	}

	for _, info := range devices {
		name := info.Name()
		if name == "" {
			continue
		}

		mics = append(mics, MicrophoneInfo{
			ID:        fmt.Sprintf("%v", info.ID),
			Name:      name,
			IsDefault: false,
		})

		logger.Info("Found microphone: %s", name)
	}

	return mics, nil
}

// SetDevice configures the recorder to use a specific device.
// Pass an empty string to use the system default device.
func (r *AudioRecorder) SetDevice(deviceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if deviceID == "" {
		r.deviceID = nil
	} else {
		r.deviceID = &deviceID
	}
}

// Start begins recording audio to the specified WAV file.
// Returns an error if recording is already in progress or initialization fails.
func (r *AudioRecorder) Start(filename string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		return fmt.Errorf("recording already in progress")
	}
	if r.closed {
		return fmt.Errorf("audio recorder is closed")
	}
	if r.ctx == nil {
		ctx, err := r.newContext()
		if err != nil {
			return fmt.Errorf("failed to initialize audio context: %w", err)
		}
		r.ctx = ctx
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	r.outputFile = file
	r.dataSize = 0
	started := false
	defer func() {
		if !started {
			file.Close()
			r.outputFile = nil
		}
	}()

	// Write placeholder WAV header
	if err := r.writeWAVHeader(0); err != nil {
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Audio services and endpoints can disappear while the app is idle (sleep,
	// USB/Bluetooth reconnects, or an audio-server restart). Retry the same
	// recording once with a fresh context, including when using System Default.
	if err := r.startCapture(); err != nil {
		logger.Info("Audio capture failed, refreshing audio context: %v", err)
		r.closeContext()
		ctx, contextErr := r.newContext()
		if contextErr != nil {
			return fmt.Errorf("failed to refresh audio context after %v: %w", err, contextErr)
		}
		r.ctx = ctx
		if err := r.startCapture(); err != nil {
			return err
		}
	}

	r.isRecording = true
	atomic.StoreInt32(&r.writing, 1)
	started = true
	logger.Info("Recording started: %s", filename)
	return nil
}

// startCapture opens a fresh endpoint for each recording. The caller holds mu.
func (r *AudioRecorder) startCapture() error {
	// Determine device ID pointer
	var targetDeviceID *malgo.DeviceID
	if r.deviceID != nil {
		devices, err := r.ctx.Devices(malgo.Capture)
		if err == nil {
			for _, info := range devices {
				if fmt.Sprintf("%v", info.ID) == *r.deviceID {
					id := info.ID
					targetDeviceID = &id
					break
				}
			}
		}
	}

	config := r.deviceConfig
	// Pin the ID only for initialization, which copies it into the native
	// device. DeviceID.Pointer() allocates C memory that would leak each time.
	var pin runtime.Pinner
	if targetDeviceID != nil {
		pin.Pin(targetDeviceID)
		defer pin.Unpin()
	}
	config.Capture.DeviceID = unsafe.Pointer(targetDeviceID)
	device, err := r.ctx.InitDevice(config, malgo.DeviceCallbacks{Data: r.onAudioData})
	if err != nil {
		return fmt.Errorf("failed to initialize audio device: %w", err)
	}

	// Start capturing
	if err := device.Start(); err != nil {
		device.Uninit()
		return fmt.Errorf("failed to start audio capture: %w", err)
	}
	r.device = device
	return nil
}

// Stop ends the current recording and finalizes the WAV file.
func (r *AudioRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.stop()
}

// stop finalizes the recording and releases its endpoint. The caller holds mu.
func (r *AudioRecorder) stop() error {
	if !r.isRecording {
		return nil
	}

	// Mark writing as unsafe BEFORE stopping device to prevent in-flight
	// callbacks from writing to a closed file.
	atomic.StoreInt32(&r.writing, 0)

	// Stop the capture device.
	if r.device != nil {
		if err := r.device.Stop(); err != nil {
			logger.Error("Failed to stop audio device: %v", err)
		}
		// Do not retain a stopped endpoint across idle periods. Its native
		// handle may no longer be usable when the next shortcut is pressed.
		r.device.Uninit()
		r.device = nil
	}

	// Finalize WAV file
	if r.outputFile != nil {
		// Rewrite header with correct data size
		r.outputFile.Seek(0, 0)
		finalSize := atomic.LoadUint32(&r.dataSize)
		if err := r.writeWAVHeader(finalSize); err != nil {
			logger.Error("Failed to update WAV header: %v", err)
		}
		r.outputFile.Close()
		r.outputFile = nil
	}

	r.isRecording = false
	logger.Info("Recording stopped: %d bytes captured", r.dataSize)
	return nil
}

// IsRecording returns true if recording is currently in progress.
func (r *AudioRecorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRecording
}

// Cleanup releases all audio resources.
func (r *AudioRecorder) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stop()
	r.closeContext()
	r.closed = true

	logger.Info("Audio recorder cleaned up")
}

// closeContext must run only after all of its devices have been uninitialized.
func (r *AudioRecorder) closeContext() {
	if r.ctx != nil {
		if err := r.ctx.Uninit(); err != nil {
			logger.Error("Failed to uninitialize audio context: %v", err)
		}
		r.ctx.Free()
		r.ctx = nil
	}
}

// onAudioData is called by miniaudio when audio data is available.
// Safe to call without r.mu: the atomic writing flag prevents writes after
// Stop() clears it, and r.device.Stop() waits for in-flight callbacks to
// complete before returning, so r.outputFile is guaranteed valid here.
func (r *AudioRecorder) onAudioData(_, inputSamples []byte, _ uint32) {
	if atomic.LoadInt32(&r.writing) == 0 || len(inputSamples) == 0 {
		return
	}

	// Double-check outputFile under the assumption that Stop() has already
	// set writing=0 before touching the file handle.
	f := r.outputFile
	if f == nil {
		return
	}

	n, err := f.Write(inputSamples)
	if err == nil {
		atomic.AddUint32(&r.dataSize, uint32(n))
	}

	// Calculate volume if callback is set (uses atomic-read-only fields)
	if r.OnVolume != nil {
		r.calculateVolume(inputSamples[:n])
	}
}

// calculateVolume processes audio samples to compute the current volume level.
// Extracted to avoid allocations in the hot audio callback path.
func (r *AudioRecorder) calculateVolume(samples []byte) {
	// S16LE: 2 bytes per sample
	sampleCount := len(samples) / 2
	if sampleCount == 0 {
		return
	}

	var maxAmplitude float64
	// Use direct slice indexing to avoid per-sample allocations
	for i := 0; i < sampleCount; i++ {
		offset := i * 2
		// Read as int16 via inlined LittleEndian to avoid function call overhead
		val := int16(samples[offset]) | int16(samples[offset+1])<<8
		absVal := float64(val)
		if absVal < 0 {
			absVal = -absVal
		}
		if absVal > maxAmplitude {
			maxAmplitude = absVal
		}
	}

	// Normalize to 0.0 - 1.0 (max for int16 is 32767)
	level := maxAmplitude / 32767.0
	r.OnVolume(level)
}

// writeWAVHeader writes a standard RIFF WAV header to the output file.
func (r *AudioRecorder) writeWAVHeader(dataSize uint32) error {
	sampleRate := uint32(SampleRate)
	numChannels := uint16(NumChannels)
	bitsPerSample := uint16(BitsPerSample)

	byteRate := sampleRate * uint32(numChannels) * uint32(bitsPerSample/8)
	blockAlign := numChannels * (bitsPerSample / 8)

	// RIFF chunk
	if _, err := r.outputFile.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		return err
	}
	if _, err := r.outputFile.Write([]byte("WAVE")); err != nil {
		return err
	}

	// fmt sub-chunk
	if _, err := r.outputFile.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, numChannels); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, sampleRate); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	if err := binary.Write(r.outputFile, binary.LittleEndian, bitsPerSample); err != nil {
		return err
	}

	// data sub-chunk
	if _, err := r.outputFile.Write([]byte("data")); err != nil {
		return err
	}
	return binary.Write(r.outputFile, binary.LittleEndian, dataSize)
}
