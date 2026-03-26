# wis-free-v3

A high-performance, native Windows voice dictation application built in Go using the Wails framework. **wis-free-v3** provides instant speech-to-text with AI-powered refinement, operating as a background service with global hotkey support.

---

## 🚀 Features

- **Blazing Fast**: Native implementation ensures zero lag during recording and transcription.
- **Global Accessibility**: Trigger from anywhere via configurable global hotkeys.
- **AI-Powered Refinement**: Integrates Groq (Whisper + Llama) for intelligent punctuation and grammar fixing.
- **Offline Capability**: Supports local Whisper.cpp for sensitive or offline workflows.
- **Micro-Automation**: Automatically pastes transcribed text directly into your active window.
- **Ultra-Leighton**: Low memory footprint while running in the system tray.

## 📂 Project Structure

```text
.
├── assets/             # Branding and icons
├── build/              # Wails build artifacts and manifests
├── frontend/           # Svelte/Vue/React settings UI
├── internal/           # Private application logic
│   ├── audio/          # Sound capture and processing
│   ├── config/         # Persistent configuration management
│   ├── hotkey/         # Global keyboard hooks
│   ├── logger/         # Structured logging utilities
│   ├── services/       # Cloud and local AI providers
│   ├── system/         # Windows OS integration (Startup/Tray)
│   └── ui/             # Native overlay and window management
├── scripts/            # Development and deployment automation
├── app.go              # Application lifecycle management
├── main.go             # Entry point
└── wails.json          # Project configuration
```

## 🛠️ Getting Started

### Prerequisites

- **Go**: 1.23 or higher
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Compiler**: GCC (TDM-GCC recommended for Windows)

### Installation & Build

1. Clone the repository.
2. Run the build script:
   ```powershell
   .\scripts\build.bat
   ```
3. The executable will be available in `build\bin/wis-free-v3.exe`.

## ⚙️ Configuration

Settings are managed via the built-in UI (Right-click tray → Settings) or manually in `%USERPROFILE%\.wis-free-v3\config.json`.

```json
{
    "api_key": "gsk_...",
    "shortcut": "alt+z",
    "whisper_model": "whisper-large-v3-turbo",
    "ai_model": "llama-3.3-70b-versatile"
}
```

## 🤝 Contributing

This project is maintained with a focus on code quality and modular architecture. Please ensure all logic remains within the `internal/` package to maintain clean boundaries.

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
