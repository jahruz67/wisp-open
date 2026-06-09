// Package recorder provides audio capture functionality using the miniaudio library.
// It records audio from the system's default capture device to WAV files.
package recorder

import (
	"encoding/binary"
	"fmt"
	"os"
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

// AudioRecorder handles audio capture from the system microphone.
type AudioRecorder struct {
	ctx          *malgo.AllocatedContext
	deviceConfig malgo.DeviceConfig
	device       *malgo.Device
	outputFile   *os.File
	dataSize     uint32
	isRecording  bool
	deviceID     *string
	OnVolume     VolumeCallback
	mu           sync.Mutex
}

// NewRecorder creates a new audio recorder instance.
// Returns an error if the audio context cannot be initialized.
func NewRecorder() (*AudioRecorder, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
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

	devices, err := r.ctx.Context.Devices(malgo.Capture)
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

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	r.outputFile = file
	r.dataSize = 0

	// Write placeholder WAV header
	if err := r.writeWAVHeader(0); err != nil {
		file.Close()
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Determine device ID pointer
	var targetDeviceID unsafe.Pointer
	if r.deviceID != nil {
		devices, err := r.ctx.Context.Devices(malgo.Capture)
		if err == nil {
			for _, info := range devices {
				if fmt.Sprintf("%v", info.ID) == *r.deviceID {
					targetDeviceID = info.ID.Pointer()
					break
				}
			}
		}
	}

	// Check if we need to re-initialize the device
	configChanged := false
	if r.deviceConfig.Capture.DeviceID != targetDeviceID {
		configChanged = true
	}

	if r.device != nil && configChanged {
		logger.Info("Audio device config changed, re-initializing...")
		r.device.Uninit()
		r.device = nil
	}

	if r.device == nil {
		r.deviceConfig.Capture.DeviceID = targetDeviceID
		callbacks := malgo.DeviceCallbacks{
			Data: r.onAudioData,
		}
		device, err := malgo.InitDevice(r.ctx.Context, r.deviceConfig, callbacks)
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to initialize audio device: %w", err)
		}
		r.device = device
		logger.Info("Audio device initialized")
	}

	// Start capturing
	if err := r.device.Start(); err != nil {
		r.device.Uninit()
		r.device = nil
		file.Close()
		return fmt.Errorf("failed to start audio capture: %w", err)
	}

	r.isRecording = true
	logger.Info("Recording started: %s", filename)
	return nil
}

// Stop ends the current recording and finalizes the WAV file.
func (r *AudioRecorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRecording {
		return nil
	}

	// Stop the capture device. We keep the device initialized between recordings
	// to avoid the expensive re-initialization cycle. ALSA privacy indicators
	// will still turn off because we've stopped the stream.
	if r.device != nil {
		if err := r.device.Stop(); err != nil {
			logger.Error("Failed to stop audio device: %v", err)
		}
		// Do NOT Uninit the device between recordings. Keeping it alive
		// avoids expensive ALSA device re-probe and reduces CPU spikes.
		// The device will be fully cleaned up in Cleanup().
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

	if r.device != nil {
		r.device.Uninit()
		r.device = nil
	}

	if r.ctx != nil {
		r.ctx.Free()
		r.ctx = nil
	}

	logger.Info("Audio recorder cleaned up")
}

// onAudioData is called by miniaudio when audio data is available.
func (r *AudioRecorder) onAudioData(_, inputSamples []byte, _ uint32) {
	if r.outputFile != nil && len(inputSamples) > 0 {
		n, _ := r.outputFile.Write(inputSamples)
		atomic.AddUint32(&r.dataSize, uint32(n))

		// Calculate volume if callback is set
		if r.OnVolume != nil {
			r.calculateVolume(inputSamples[:n])
		}
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
	r.outputFile.Write([]byte("RIFF"))
	binary.Write(r.outputFile, binary.LittleEndian, uint32(36+dataSize))
	r.outputFile.Write([]byte("WAVE"))

	// fmt sub-chunk
	r.outputFile.Write([]byte("fmt "))
	binary.Write(r.outputFile, binary.LittleEndian, uint32(16)) // Subchunk1Size
	binary.Write(r.outputFile, binary.LittleEndian, uint16(1))  // AudioFormat (PCM)
	binary.Write(r.outputFile, binary.LittleEndian, numChannels)
	binary.Write(r.outputFile, binary.LittleEndian, sampleRate)
	binary.Write(r.outputFile, binary.LittleEndian, byteRate)
	binary.Write(r.outputFile, binary.LittleEndian, blockAlign)
	binary.Write(r.outputFile, binary.LittleEndian, bitsPerSample)

	// data sub-chunk
	r.outputFile.Write([]byte("data"))
	binary.Write(r.outputFile, binary.LittleEndian, dataSize)

	return nil
}