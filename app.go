package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wis-free-v3/internal/audio/recorder"
	"wis-free-v3/internal/config"
	"wis-free-v3/internal/hotkey"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"
	"wis-free-v3/internal/services/transcriber"
	"wis-free-v3/internal/services/whisper"
	"wis-free-v3/internal/ui/tray"

	"github.com/go-vgo/robotgo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx             context.Context
	audioRecorder   *recorder.AudioRecorder
	hotkeyListener  *hotkey.Listener
	transcriber     *transcriber.Client
	config          *config.Config
	overlay         platform.Overlay
	recordingPath   string
	isQuitting      bool
	wasMediaPlaying bool
	whisperManager  *whisper.Manager
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// GetConfig returns the app configuration
func (a *App) GetConfig() *config.Config {
	return a.config
}

// Version returns the application version (see scripts/VERSION and build scripts).
func (a *App) Version() string {
	return AppVersion
}

// ShowSettings shows the settings window
func (a *App) ShowSettings() {
	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	title := appTitle
	switch AppVersion {
	case "", "dev":
		title = appTitle + " (dev)"
	default:
		title = appTitle + " v" + AppVersion
	}
	wailsruntime.WindowSetTitle(ctx, title)

	// Initialize components
	a.startupHeadless()

	// Start system tray in a goroutine
	go tray.Start(a)
}

// beforeClose is called when the window is about to close
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.isQuitting {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
}

// Quit handles application exit from tray
func (a *App) Quit() {
	a.isQuitting = true
	logger.Info("Application quitting...")
	logger.Close()
	wailsruntime.Quit(a.ctx)
}

// StartRecording starts the audio recording
func (a *App) StartRecording() {
	logger.Info("StartRecording triggered")

	// 1. Show overlay immediately (most visible feedback)
	if a.overlay != nil {
		a.overlay.Show("Recording...")
	}

	// 2. Start recorder as soon as possible
	if a.audioRecorder == nil {
		logger.Error("Recorder not initialized")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	// Prepare path
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102_150405")
	a.recordingPath = filepath.Join(tempDir, fmt.Sprintf("wis_recording_%s.wav", timestamp))

	// Start recording
	err := a.audioRecorder.Start(a.recordingPath)
	if err != nil {
		logger.Error("Failed to start recording: %v", err)
		// IDIOT-PROOFING: Fallback to default
		if a.config.MicrophoneDevice != nil {
			a.audioRecorder.SetDevice("")
			err = a.audioRecorder.Start(a.recordingPath)
		}
		if err != nil {
			if a.overlay != nil {
				a.overlay.Hide()
			}
			return
		}
	}

	// 3. Handle secondary tasks in background
	go func() {
		// Update tray status
		tray.UpdateStatus("Recording...")

		// Pause media if playing (this is slow due to PowerShell)
		a.wasMediaPlaying = platform.PauseMedia()
		if a.wasMediaPlaying {
			logger.Info("Media paused for recording")
		}
	}()
}

// StopRecording stops the audio recording and triggers transcription
func (a *App) StopRecording() {
	logger.Info("StopRecording triggered")

	// Resume media if it was playing before
	platform.ResumeMedia(a.wasMediaPlaying)
	if a.wasMediaPlaying {
		logger.Info("Media resumed after recording")
	}

	if a.audioRecorder == nil {
		return
	}

	err := a.audioRecorder.Stop()
	if err != nil {
		logger.Error("Failed to stop recording: %v", err)
		return
	}

	// Transcribe in a goroutine to avoid blocking
	go a.processRecording()
}

// processRecording handles transcription and pasting
func (a *App) processRecording() {
	if a.recordingPath == "" {
		logger.Error("No recording path set")
		tray.UpdateStatus("Ready")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	logger.Info("Transcribing audio...")
	tray.UpdateStatus("Transcribing...")
	if a.overlay != nil {
		a.overlay.Show("Transcribing...")
	}

	var text string
	var err error

	// Check if using local whisper
	isLocal := strings.HasPrefix(a.config.WhisperModel, "local-")

	// IDIOT-PROOFING: Validate API key before attempting cloud transcription
	if !isLocal && !strings.HasPrefix(a.config.APIKey, "gsk_") {
		err = fmt.Errorf("invalid API key - must start with gsk_")
	} else if isLocal {
		// Use local whisper.cpp
		if a.whisperManager == nil {
			var mgrErr error
			a.whisperManager, mgrErr = whisper.NewManager()
			if mgrErr != nil {
				logger.Error("Failed to create whisper manager: %v", mgrErr)
				err = mgrErr
			}
		}

		if a.whisperManager != nil {
			text, err = a.whisperManager.Transcribe(a.recordingPath)
		}
	} else {
		// Use cloud API
		text, err = a.transcriber.TranscribeAudio(a.recordingPath, a.config.Language)
	}

	if err != nil {
		logger.Error("Transcription failed: %v", err)
		tray.UpdateStatus("Ready")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	logger.Info("Transcribed: %s", text)

	// Refine text (optional)
	refinedText, err := a.transcriber.RefineText(text)
	if err != nil {
		logger.Error("Refinement failed: %v", err)
		// Fallback to original text
		refinedText = text
	} else {
		logger.Info("Refined: %s", refinedText)
	}

	// Save to history
	historyItem := config.HistoryItem{
		Text:      refinedText,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	// Prepend to history
	a.config.History = append([]config.HistoryItem{historyItem}, a.config.History...)
	// Keep only last 50 items
	if len(a.config.History) > 50 {
		a.config.History = a.config.History[:50]
	}
	if err := config.Save(a.config, ""); err != nil {
		logger.Error("Failed to save config after history update: %v", err)
	} else if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "history:updated")
	}

	// Copy to clipboard
	wailsruntime.ClipboardSetText(a.ctx, refinedText)

	// Paste
	a.pasteText()

	// Clean up the recording file
	os.Remove(a.recordingPath)
	logger.Info("Processing complete!")
	tray.UpdateStatus("Ready")

	// Hide overlay after a short delay
	time.Sleep(1 * time.Second)
	if a.overlay != nil {
		a.overlay.Hide()
	}
}

// pasteText simulates Ctrl+V to paste from clipboard
func (a *App) pasteText() {
	// Give a substantial delay for Linux GTK clipboard sync
	time.Sleep(200 * time.Millisecond)

	// Simulate Ctrl+V using modern robotgo API
	robotgo.KeyTap("v", "ctrl")
}

// GetSettings returns the current configuration
func (a *App) GetSettings() map[string]interface{} {
	conf := make(map[string]interface{})
	conf["api_key"] = a.config.APIKey
	conf["shortcut"] = a.config.Shortcut
	conf["whisper_model"] = a.config.WhisperModel
	conf["ai_model"] = a.config.AIModel
	conf["ai_prompt"] = a.config.AIPrompt
	conf["language"] = a.config.Language
	conf["microphone_device"] = a.config.MicrophoneDevice
	conf["history"] = a.config.History
	conf["startup"] = platform.IsInStartup()
	conf["app_version"] = AppVersion
	return conf
}

// SaveSettings updates the configuration
func (a *App) SaveSettings(settings map[string]interface{}) string {
	if val, ok := settings["api_key"].(string); ok {
		a.config.APIKey = val
	}
	if val, ok := settings["shortcut"].(string); ok {
		_, _, modOnly, ok := hotkey.ParseShortcut(val)
		if !ok {
			logger.Error("Invalid shortcut: %s (rejected)", val)
			return "Invalid shortcut - use modifiers plus a key (e.g. ctrl+k), or modifier-only on Windows (e.g. ctrl+win)"
		}
		if modOnly && runtime.GOOS != "windows" {
			return "Modifier-only shortcuts (like ctrl+win) are only supported on Windows"
		}

		a.config.Shortcut = val
		// Update existing listener with new shortcut (hot-swap)
		if a.hotkeyListener != nil {
			a.hotkeyListener.UpdateShortcut(val)
		} else {
			// Should not happen if app started correctly, but just in case
			a.hotkeyListener = hotkey.NewListener(val, a.StartRecording, a.StopRecording)
			a.hotkeyListener.Start()
		}
	}
	if val, ok := settings["whisper_model"].(string); ok {
		a.config.WhisperModel = val
	}
	if val, ok := settings["ai_model"].(string); ok {
		a.config.AIModel = val
	}
	if val, ok := settings["ai_prompt"].(string); ok {
		a.config.AIPrompt = val
	}
	if val, ok := settings["language"].(string); ok {
		a.config.Language = val
	}
	if val, ok := settings["microphone_device"]; ok {
		if val == nil {
			a.config.MicrophoneDevice = nil
		} else {
			// Handle float64 from JSON/JS
			if f, ok := val.(float64); ok {
				i := int(f)
				a.config.MicrophoneDevice = &i
			}
		}
	}

	// Save to file
	config.Save(a.config, "")

	// Re-init transcriber with new settings
	a.transcriber = transcriber.NewClient(
		a.config.APIKey,
		a.config.WhisperModel,
		a.config.AIModel,
		a.config.AIPrompt,
	)

	logger.Info("Settings saved")
	return "Settings saved successfully"
}

// GetMicrophones returns available microphone devices
func (a *App) GetMicrophones() []map[string]interface{} {
	var result []map[string]interface{}

	if a.audioRecorder != nil {
		mics, err := a.audioRecorder.GetMicrophones()
		if err != nil {
			logger.Error("Failed to enumerate microphones: %v", err)
		} else {
			for i, mic := range mics {
				result = append(result, map[string]interface{}{
					"index": i - 1, // -1 for default, 0+ for specific devices
					"name":  mic.Name,
					"id":    mic.ID,
				})
			}
			return result
		}
	}

	// Fallback to just default
	return []map[string]interface{}{
		{"index": -1, "name": "System Default", "id": ""},
	}
}

// ToggleStartup toggles Windows startup status
func (a *App) ToggleStartup(enable bool) string {
	var err error
	if enable {
		err = platform.AddToStartup()
	} else {
		err = platform.RemoveFromStartup()
	}

	if err != nil {
		logger.Error("Startup toggle error: %v", err)
		return fmt.Sprintf("Error: %v", err)
	}
	return "Success"
}

// ClearHistory clears the transcription history
func (a *App) ClearHistory() {
	a.config.History = []config.HistoryItem{}
	config.Save(a.config, "")
	logger.Info("History cleared")
}

// startupHeadless initializes the app without Wails context
func (a *App) startupHeadless() {
	// Initialize Logger
	err := logger.Init()
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
	}

	// Load configuration
	configPath, err := config.GetConfigPath()
	if err != nil {
		logger.Error("Failed to get config path: %v", err)
		a.config = config.DefaultConfig()
	} else {
		a.config, err = config.Load(configPath)
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			a.config = config.DefaultConfig()
		}
	}

	// Initialize heavy components synchronously instead of in background.
	// This prevents a known ALSA/GTK initialization race condition bug on Linux
	// where Miniaudio and GTK try to probe audio devices concurrently resulting in SIGABRT.

	// Initialize transcriber
	a.transcriber = transcriber.NewClient(
		a.config.APIKey,
		a.config.WhisperModel,
		a.config.AIModel,
		a.config.AIPrompt,
	)

	// Initialize whisper manager
	a.whisperManager, _ = whisper.NewManager()

	// Initialize Audio Recorder
	rec, err := recorder.NewRecorder()
	if err != nil {
		logger.Error("Error initializing recorder: %v", err)
	} else {
		a.audioRecorder = rec
	}

	// Initialize Overlay
	a.overlay = platform.NewOverlay()

	// Connect volume feedback from recorder to overlay
	if a.audioRecorder != nil && a.overlay != nil {
		a.audioRecorder.OnVolume = func(level float64) {
			a.overlay.SetVolume(level)
		}
	}

	// Initialize Hotkey Listener
	a.hotkeyListener = hotkey.NewListener(a.config.Shortcut, a.StartRecording, a.StopRecording)
	a.hotkeyListener.Start()

	logger.Info("Components initialized successfully!")

	logger.Info("Basic app components loaded, continuing startup...")
}

// Shutdown cleans up resources
func (a *App) Shutdown(ctx context.Context) {
	if a.hotkeyListener != nil {
		a.hotkeyListener.Stop()
	}
	if a.audioRecorder != nil {
		a.audioRecorder.Cleanup()
	}
	if a.overlay != nil {
		a.overlay.Close()
	}
	logger.Close()
}

// Greet returns a greeting for the given name (for frontend testing)
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// CheckOnline checks if internet connection is available
func (a *App) CheckOnline() bool {
	return whisper.CheckOnline()
}

// IsWhisperInstalled checks if offline whisper is installed
func (a *App) IsWhisperInstalled() bool {
	mgr, err := whisper.NewManager()
	if err != nil {
		return false
	}
	return mgr.IsInstalled()
}

// GetWhisperInfo returns information about installed whisper
func (a *App) GetWhisperInfo() map[string]interface{} {
	mgr, err := whisper.NewManager()
	if err != nil {
		return map[string]interface{}{"installed": false}
	}

	if !mgr.IsInstalled() {
		return map[string]interface{}{"installed": false}
	}

	info, err := mgr.GetInstalledInfo()
	if err != nil {
		return map[string]interface{}{"installed": false}
	}

	return map[string]interface{}{
		"installed": true,
		"model":     info.Model,
		"version":   info.Version,
	}
}

// InstallWhisper starts the whisper installation process
func (a *App) InstallWhisper(model string) string {
	mgr, err := whisper.NewManager()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if err := mgr.Install(model); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return "Installation started - check the terminal window"
}

// UninstallWhisper removes the whisper installation
func (a *App) UninstallWhisper() string {
	mgr, err := whisper.NewManager()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if err := mgr.Uninstall(); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return "Whisper uninstalled successfully"
}

// GetAvailableWhisperModels returns list of available whisper models
func (a *App) GetAvailableWhisperModels() []map[string]string {
	var models []map[string]string
	for name, info := range whisper.Models {
		models = append(models, map[string]string{
			"name": name,
			"size": info.Size,
		})
	}
	return models
}
