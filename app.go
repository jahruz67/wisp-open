package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"wis-free-v3/internal/audio/recorder"
	"wis-free-v3/internal/config"
	"wis-free-v3/internal/hotkey"
	"wis-free-v3/internal/logger"
	"wis-free-v3/internal/platform"
	"wis-free-v3/internal/services/transcriber"
	"wis-free-v3/internal/services/whisper"
	"wis-free-v3/internal/ui/tray"

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
	recording       int32
	isQuitting      bool
	wasMediaPlaying bool
	whisperManager  *whisper.Manager
	tempDir         string
	transcribing    int32 // atomic: 1 = transcription in progress, prevents concurrent
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

	// Create and clean private temp directory for recordings (Cleanup in background)
	a.tempDir = filepath.Join(os.TempDir(), "wis-free-v3-recordings")
	os.MkdirAll(a.tempDir, 0700)

	go func() {
		// Clean up any orphaned files from previous sessions
		if files, err := os.ReadDir(a.tempDir); err == nil {
			for _, f := range files {
				os.Remove(filepath.Join(a.tempDir, f.Name()))
			}
		}
		logger.Info("Orphaned temp files cleaned")
	}()

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
	a.startLinuxPressDaemon()

	// When the user launches the app again while it is already running (tray-only),
	// the second process signals us here so the settings window becomes visible.
	go func() {
		for range secondInstanceWake {
			wailsruntime.WindowShow(ctx)
		}
	}()

	// Accept explicit commands from helper invocations (Linux desktop shortcuts).
	go func() {
		for cmd := range secondInstanceCommand {
			switch cmd {
			case instanceCmdShow:
				wailsruntime.WindowShow(ctx)
			case instanceCmdStart:
				a.StartRecording()
			case instanceCmdStop:
				a.StopRecording()
			case instanceCmdToggle:
				a.ToggleRecording()
			default:
			}
		}
	}()

	// Register callback so the tray can notify the frontend when the startup
	// menu item is toggled directly from the tray icon context menu.
	tray.SetOnStartupChanged(func(enabled bool) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "startup:changed", enabled)
		}
	})

	// Start system tray in a goroutine
	// PLATFORM NOTE: On Linux, Wails' GTK main loop needs tray.Start() to be
	// called synchronously (uses systray.Register). On Windows (and macOS),
	// tray.Start() blocks for the Win32 message pump, so it must run in a goroutine.
	if runtime.GOOS == "linux" {
		tray.Start(a)
	} else {
		go tray.Start(a)
	}

	if initialAction != "" {
		action := initialAction
		initialAction = ""
		go func() {
			switch action {
			case "show":
				wailsruntime.WindowShow(ctx)
			case "start":
				a.StartRecording()
			case "stop":
				a.StopRecording()
			case "toggle":
				a.ToggleRecording()
			default:
			}
		}()
	}
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

var lastToggleUnixNano int64

func (a *App) ToggleRecording() {
	// Debounce toggle calls to prevent rapid firing from double-binds or Wayland glitches
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&lastToggleUnixNano)
	if last != 0 && time.Duration(now-last) < 250*time.Millisecond {
		return
	}
	atomic.StoreInt64(&lastToggleUnixNano, now)

	if atomic.LoadInt32(&a.recording) == 1 {
		a.StopRecording()
		return
	}
	a.StartRecording()
}

// StartRecording starts the audio recording
func (a *App) StartRecording() {
	if !atomic.CompareAndSwapInt32(&a.recording, 0, 1) {
		return
	}
	logger.Info("StartRecording triggered")

	// 1. Show overlay immediately (most visible feedback)
	if a.overlay != nil {
		a.overlay.Show("Recording...")
	}

	tray.IncrementTriggerCount()

	// 2. Start recorder as soon as possible
	if a.audioRecorder == nil {
		logger.Error("Recorder not initialized")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		atomic.StoreInt32(&a.recording, 0)
		return
	}

	// Prepare path safely in our private temp directory
	tempFile, err := os.CreateTemp(a.tempDir, "rec_*.wav")
	if err != nil {
		logger.Error("Failed to create temp file: %v", err)
		if a.overlay != nil {
			a.overlay.Hide()
		}
		atomic.StoreInt32(&a.recording, 0)
		return
	}
	a.recordingPath = tempFile.Name()
	tempFile.Close() // We just need the path, recorder will open it

	// Start recording
	err = a.audioRecorder.Start(a.recordingPath)
	if err != nil {
		logger.Error("Failed to start recording: %v", err)
		// IDIOT-PROOFING: Fallback to default
		if a.config.MicrophoneDevice != nil {
			a.audioRecorder.SetDevice("")
			err = a.audioRecorder.Start(a.recordingPath)
		}
		if err != nil {
			// Clean up the orphaned temp file since recording failed to start
			os.Remove(a.recordingPath)
			if a.overlay != nil {
				a.overlay.Hide()
			}
			atomic.StoreInt32(&a.recording, 0)
			return
		}
	}

	// 3. Update tray status
	tray.UpdateStatus("Recording...")

	// Pause media if playing (do synchronously so wasMediaPlaying is ready before Stop)
	a.wasMediaPlaying = platform.PauseMedia()
	if a.wasMediaPlaying {
		logger.Info("Media paused for recording")
	}
}

// StopRecording stops the audio recording and triggers transcription
func (a *App) StopRecording() {
	if !atomic.CompareAndSwapInt32(&a.recording, 1, 0) {
		return
	}
	logger.Info("StopRecording triggered")

	// Resume media if it was playing before
	platform.ResumeMedia(a.wasMediaPlaying)
	if a.wasMediaPlaying {
		logger.Info("Media resumed after recording")
	}

	if a.audioRecorder == nil {
		return
	}

	// Capture the path BEFORE stopping the recorder, so it can't be
	// overwritten by a concurrent StartRecording (which sets a.recordingPath).
	pathToProcess := a.recordingPath

	err := a.audioRecorder.Stop()
	if err != nil {
		logger.Error("Failed to stop recording: %v", err)
		return
	}

	// Transcribe in a goroutine to avoid blocking
	go a.processRecording(pathToProcess)
}

// processRecording handles transcription and pasting.
// Uses an atomic guard to prevent concurrent transcriptions.
func (a *App) processRecording(recordingPath string) {
	if recordingPath == "" {
		logger.Error("No recording path set")
		tray.UpdateStatus("Ready")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	// Prevent concurrent transcriptions: only one goroutine can process at a time.
	// If another transcription is already in progress, discard this recording.
	if !atomic.CompareAndSwapInt32(&a.transcribing, 0, 1) {
		logger.Info("Another transcription already in progress, discarding recording: %s", recordingPath)
		os.Remove(recordingPath)
		return
	}
	defer atomic.StoreInt32(&a.transcribing, 0)

	// IDIOT-PROOFING: Ignore extremely short recordings (less than ~100ms or ~3KB)
	// that are likely accidental clicks or hardware glitches.
	stat, statErr := os.Stat(recordingPath)
	if statErr != nil {
		logger.Error("Failed to stat recording file: %v", statErr)
		if a.overlay != nil {
			a.overlay.Hide()
		}
		tray.UpdateStatus("Ready")
		return
	}
	if stat.Size() < 4000 {
		logger.Info("Discarding tiny recording (%d bytes)", stat.Size())
		os.Remove(recordingPath)
		if a.overlay != nil {
			a.overlay.Hide()
		}
		tray.UpdateStatus("Ready")
		return
	}

	logger.Info("Transcribing audio (%d bytes)...", stat.Size())
	tray.UpdateStatus("Transcribing...")
	if a.overlay != nil {
		a.overlay.Show("Transcribing...")
	}

	startTranscribe := time.Now()
	var text string
	var err error

	// Check if using local whisper
	isLocal := strings.HasPrefix(a.config.WhisperModel, "local-")

	// IDIOT-PROOFING: Validate API key before attempting cloud transcription
	if !isLocal {
		// Get the appropriate API key for the whisper model
		apiKey := a.config.GetAPIKey(transcriber.GetProviderForModel(a.config.WhisperModel))
		if apiKey == "" {
			err = fmt.Errorf("API key is missing for the selected provider")
		}
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
			text, err = a.whisperManager.Transcribe(recordingPath, a.config.Language)
		}
	} else {
		// Use cloud API
		text, err = a.transcriber.TranscribeAudio(recordingPath, a.config.Language)
	}
	transcribeDuration := time.Since(startTranscribe)
	logger.Info("Transcription completed in %v", transcribeDuration)

	if err != nil {
		logger.Error("Transcription failed: %v", err)
		tray.UpdateStatus("Ready")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	logger.Info("Transcribed %d characters", len(text))

	// robotgo queries X11 directly. Do not invoke it from a Wayland session;
	// the Linux helper returns an empty context instead of making transcription
	// fail after the audio has already been captured.
	activeWindow := activeWindowTitle()
	logger.Info("Active window for context: %s", activeWindow)

	// Refine text (optional)
	startRefine := time.Now()
	refinedText, err := a.transcriber.RefineText(text, activeWindow)
	if err != nil {
		logger.Error("Refinement failed: %v", err)
		// Fallback to original text
		refinedText = text
	} else {
		refineDuration := time.Since(startRefine)
		logger.Info("AI Refinement completed in %v (%d chars)", refineDuration, len(refinedText))
	}

	// Save to history
	a.config.AddHistoryItem(refinedText, time.Now().Format(time.RFC3339))

	if err := config.Save(a.config, ""); err != nil {
		logger.Error("Failed to save config after history update: %v", err)
	} else if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "history:updated")
	}

	a.insertTranscription(refinedText)

	// Clean up the recording file
	if removeErr := os.Remove(recordingPath); removeErr != nil {
		logger.Error("Failed to remove temporary recording file: %v", removeErr)
	}
	logger.Info("Processing complete!")
	tray.UpdateStatus("Ready")

	// Hide overlay after a short delay
	time.Sleep(1 * time.Second)
	if a.overlay != nil {
		a.overlay.Hide()
	}
}

// GetSettings returns the current configuration
func (a *App) GetSettings() map[string]interface{} {
	conf := make(map[string]interface{})
	// Mask the API keys: only reveal the last 4 characters so the user can verify
	// which key is configured without exposing the full secret to the frontend.
	if a.config.GroqAPIKey != "" {
		key := a.config.GroqAPIKey
		if len(key) > 4 {
			key = "****" + key[len(key)-4:]
		}
		conf["groq_api_key"] = key
	} else {
		conf["groq_api_key"] = ""
	}
	if a.config.MistralAPIKey != "" {
		key := a.config.MistralAPIKey
		if len(key) > 4 {
			key = "****" + key[len(key)-4:]
		}
		conf["mistral_api_key"] = key
	} else {
		conf["mistral_api_key"] = ""
	}
	conf["shortcut"] = a.config.Shortcut
	conf["whisper_model"] = a.config.WhisperModel
	conf["ai_model"] = a.config.AIModel
	conf["ai_prompt"] = a.config.AIPrompt
	conf["language"] = a.config.Language
	conf["microphone_device"] = a.config.MicrophoneDevice
	conf["history"] = a.config.History
	conf["startup"] = platform.IsInStartup()
	conf["app_version"] = AppVersion
	// PLATFORM NOTE: Linux-only settings — press daemon command and ydotool status.
	// These are not included in the Windows build. See linux_press_daemon.go
	// and text_insert_linux.go for the implementations.
	if runtime.GOOS == "linux" {
		if exePath, err := os.Executable(); err == nil {
			conf["linux_press_command"] = linuxShortcutCommand(exePath)
		}
		conf["linux_press_mode"] = true
		conf["linux_ydotool_status"] = linuxYdotoolStatus()
	}
	return conf
}

// SaveSettings updates the configuration
func (a *App) SaveSettings(settings map[string]interface{}) string {
	if val, ok := settings["groq_api_key"].(string); ok {
		// Only update the API key if it's not the masked value returned by GetSettings.
		// GetSettings masks the key as "****abcd" so the frontend can show the last 4 chars.
		// If the user didn't change it and sent back the masked value, preserve the real key.
		if !strings.HasPrefix(val, "****") {
			a.config.GroqAPIKey = val
		}
	}
	if val, ok := settings["mistral_api_key"].(string); ok {
		// Only update the API key if it's not the masked value returned by GetSettings.
		if !strings.HasPrefix(val, "****") {
			a.config.MistralAPIKey = val
		}
	}
	if val, ok := settings["shortcut"].(string); ok {
		if runtime.GOOS == "linux" {
			logger.Info("Ignoring in-app shortcut recording on Linux; use the desktop custom-shortcut command shown in Settings")
		} else {
			_, _, modOnly, ok := hotkey.ParseShortcut(val)
			if !ok {
				logger.Error("Invalid shortcut: %s (rejected)", val)
				return "Invalid shortcut - use modifiers plus a key (e.g. ctrl+k), or modifier-only on Windows (e.g. ctrl+win)"
			}
			if modOnly && runtime.GOOS != "windows" {
				return "Modifier-only shortcuts (like ctrl+win) are only supported on Windows"
			}

			a.config.Shortcut = val
			if a.hotkeyListener != nil {
				a.hotkeyListener.UpdateShortcut(val)
			} else {
				a.hotkeyListener = hotkey.NewListener(val, a.StartRecording, a.StopRecording)
				a.hotkeyListener.Start()
			}
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
	if err := config.Save(a.config, ""); err != nil {
		logger.Error("Failed to save config: %v", err)
		return fmt.Sprintf("Error saving settings: %v", err)
	}

	// Re-init transcriber with new settings.
	// Note: a.config.GroqAPIKey is already updated by the api_key field above.
	// Since GetSettings masks the key, we only update if it's not still the masked value.
	a.transcriber = transcriber.NewClient(
		a.config.GroqAPIKey,
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

	// Keep the tray menu item in sync when changed from the settings UI
	tray.SetStartupChecked(enable)

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
		a.config.GroqAPIKey,
		a.config.MistralAPIKey,
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

	// Ensure desktop integration for Linux custom shortcuts and tray metadata.
	if err := platform.EnsureDesktopFile(tray.DefaultIconBytes()); err != nil {
		logger.Error("Failed to ensure desktop file: %v", err)
	}

	// Linux uses the desktop custom-shortcut command shown in Settings instead
	// of an in-app global hotkey backend. Other platforms keep their native
	// listeners.
	if runtime.GOOS != "linux" {
		a.hotkeyListener = hotkey.NewListener(a.config.Shortcut, a.StartRecording, a.StopRecording)
		a.hotkeyListener.Start()
	} else {
		logger.Info("Linux shortcut setup uses the command shown in Settings; configure it in GNOME/KDE custom shortcuts")
	}

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
	// Gracefully shut down the Linux press daemon HTTP server (no-op on Windows)
	stopLinuxPressDaemon()
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

// IsWhisperInstalled checks if offline whisper is installed.
// Reuses the cached whisperManager if available.
func (a *App) IsWhisperInstalled() bool {
	mgr := a.whisperManager
	if mgr == nil {
		var err error
		mgr, err = whisper.NewManager()
		if err != nil {
			return false
		}
	}
	return mgr.IsInstalled()
}

// GetWhisperInfo returns information about installed whisper.
// Reuses the cached whisperManager if available.
func (a *App) GetWhisperInfo() map[string]interface{} {
	mgr := a.whisperManager
	if mgr == nil {
		var err error
		mgr, err = whisper.NewManager()
		if err != nil {
			return map[string]interface{}{"installed": false}
		}
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

// GetAvailableWhisperModels returns list of available whisper models,
// sorted by name for consistent UI display.
func (a *App) GetAvailableWhisperModels() []map[string]string {
	var names []string
	for name := range whisper.Models {
		names = append(names, name)
	}
	sort.Strings(names)

	models := make([]map[string]string, 0, len(names))
	for _, name := range names {
		info := whisper.Models[name]
		models = append(models, map[string]string{
			"name": name,
			"size": info.Size,
		})
	}
	return models
}
