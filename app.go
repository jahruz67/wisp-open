package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	recording       int32
	isQuitting      bool
	wasMediaPlaying bool
	whisperManager  *whisper.Manager
	tempDir         string
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

	// Start system tray in a goroutine
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
			if a.overlay != nil {
				a.overlay.Hide()
			}
			atomic.StoreInt32(&a.recording, 0)
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
		atomic.StoreInt32(&a.recording, 0)
		return
	}

	err := a.audioRecorder.Stop()
	if err != nil {
		logger.Error("Failed to stop recording: %v", err)
		// Attempt to keep state consistent: if stop failed, we are likely still recording.
		atomic.StoreInt32(&a.recording, 1)
		return
	}

	// Capture the path before it can be overwritten by another immediate start
	pathToProcess := a.recordingPath
	// Transcribe in a goroutine to avoid blocking
	go a.processRecording(pathToProcess)
}

// processRecording handles transcription and pasting
func (a *App) processRecording(recordingPath string) {
	if recordingPath == "" {
		logger.Error("No recording path set")
		tray.UpdateStatus("Ready")
		if a.overlay != nil {
			a.overlay.Hide()
		}
		return
	}

	// IDIOT-PROOFING: Ignore extremely short recordings (less than ~100ms or ~3KB)
	// that are likely accidental clicks or hardware glitches.
	stat, statErr := os.Stat(recordingPath)
	if statErr == nil && stat.Size() < 4000 {
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

	logger.Info("Transcribed: %s", text)

	activeWindow := robotgo.GetTitle()
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
		logger.Info("AI Refinement completed in %v (Refined: %s)", refineDuration, refinedText)
	}

	// Save to history
	a.config.AddHistoryItem(refinedText, time.Now().Format(time.RFC3339))

	if err := config.Save(a.config, ""); err != nil {
		logger.Error("Failed to save config after history update: %v", err)
	} else if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "history:updated")
	}

	if len(refinedText) < 50 {
		// Type short text directly to avoid clipboard interference
		robotgo.TypeStr(refinedText)
	} else {
		// Save old clipboard
		oldClip, clipErr := wailsruntime.ClipboardGetText(a.ctx)

		// Copy to clipboard
		wailsruntime.ClipboardSetText(a.ctx, refinedText)

		// Paste
		a.pasteText()

		// Restore old clipboard after a short delay, but ONLY if the user
		// hasn't manually copied something else or another burst hasn't finished.
		if clipErr == nil && oldClip != "" {
			go func() {
				time.Sleep(1000 * time.Millisecond)
				current, _ := wailsruntime.ClipboardGetText(a.ctx)
				if current == refinedText {
					wailsruntime.ClipboardSetText(a.ctx, oldClip)
					logger.Info("Clipboard history restored")
				}
			}()
		}
	}

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
	if runtime.GOOS == "linux" {
		if exePath, err := os.Executable(); err == nil {
			conf["linux_press_command"] = exePath + " --press"
		}
		conf["linux_press_mode"] = true
	}
	return conf
}

// SaveSettings updates the configuration
func (a *App) SaveSettings(settings map[string]interface{}) string {
	if val, ok := settings["api_key"].(string); ok {
		a.config.APIKey = val
	}
	if runtime.GOOS != "linux" {
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
				if runtime.GOOS != "windows" {
					a.hotkeyListener.SetRegistrationErrorCallback(func(err error) {
						logger.Error("Linux hotkey registration failed: %v", err)
						go func() {
							time.Sleep(2 * time.Second)
							if a.overlay != nil {
								a.overlay.Show("Shortcut registration failed. Please add a custom system shortcut calling 'wis-free-v3 --action=toggle' as a fallback.")
							}
						}()
					})
				}
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

	// Ensure desktop integration for Wayland portals
	if err := platform.EnsureDesktopFile(tray.DefaultIconBytes()); err != nil {
		logger.Error("Failed to ensure desktop file: %v", err)
	}

	// Initialize Hotkey Listener
	if shouldStartBuiltInHotkeyListener() {
		a.hotkeyListener = hotkey.NewListener(a.config.Shortcut, a.StartRecording, a.StopRecording)
		if runtime.GOOS != "windows" {
			a.hotkeyListener.SetRegistrationErrorCallback(func(err error) {
				logger.Error("Linux hotkey registration failed: %v", err)
				go func() {
					time.Sleep(2 * time.Second)
					if a.overlay != nil {
						a.overlay.Show("Shortcut registration failed. Add a custom system shortcut with the command shown in Settings.")
					}
				}()
			})
		}
		a.hotkeyListener.Start()
	} else {
		logger.Info("Linux portal hotkey disabled; use the --press command from Settings for GNOME shortcuts")
	}

	logger.Info("Components initialized successfully!")

	logger.Info("Basic app components loaded, continuing startup...")
}

func shouldStartBuiltInHotkeyListener() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	return os.Getenv("WISFREE_USE_PORTAL_HOTKEY") == "1"
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
