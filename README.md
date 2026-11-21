# WIS Free V3

WIS Free V3 is a desktop voice-dictation app built with Go and Wails. It records audio, transcribes it with Groq or Mistral, optionally cleans up the result, and types it into the focused application.

Windows and Linux are supported. An Apple Silicon-only macOS 13+ preview is also available. Current Linux release packages target `amd64`/`x86_64`.

## Install on macOS

Download the Apple Silicon preview DMG, drag **WIS Free V3** to Applications, and complete the in-app Microphone and Accessibility checklist. Intel Macs are not supported. The current preview is ad-hoc signed, so follow the Gatekeeper and permission notes in [macOS installation](docs/macos-installation.md).

Local models are never bundled or downloaded automatically. In Settings, use **Offline Transcription** to install Whisper, NVIDIA Parakeet TDT v2/v3, Moonshine Tiny/Base, or SenseVoice Small. The picker shows languages and approximate download sizes. Parakeet, Moonshine, and SenseVoice use an engine downloaded on demand and run on CPU on Windows x64, Linux x64, and Apple Silicon macOS. Select the installed model under **Transcription Model**; set **AI Model** to **None** for fully offline dictation.

Linux packages include the same whisper.cpp Vulkan backend used on Windows, enabling offline GPU inference with a compatible Intel, AMD, or NVIDIA Vulkan driver. Choose a Whisper model under **Offline Transcription** in Settings, click **Install Selected Model**, then select the installed local model. See [offline setup and GPU requirements](docs/linux-installation.md#offline-whisper-and-gpu-acceleration).

## Install on Linux

Download the package for your distribution from the release page and install it with your normal package manager. The package manager installs the app and its direct-typing dependency together.

To upgrade, install the newer package with the same command; uninstalling first is unnecessary. Release builds add the workflow run number to the patch in `scripts/VERSION`, giving each run an increasing app and package version. Workflow reruns retain the same version.

| Distribution | Package | Install command |
| --- | --- | --- |
| Ubuntu, Debian, Linux Mint, Pop!_OS | `.deb` | `sudo apt install ./wis-free-v3_<version>-1_amd64.deb` |
| Fedora and compatible DNF desktops | `.rpm` | `sudo dnf install ./wis-free-v3-<version>-1.x86_64.rpm` |
| openSUSE Tumbleweed | `.rpm` | `sudo zypper install ./wis-free-v3-<version>-1.x86_64.rpm` |
| Arch, Manjaro | source | Follow [Build from source](docs/linux-installation.md#build-from-source) |

Do not use `dpkg -i` by itself: it does not resolve dependencies. If you already did, run `sudo apt -f install`, then repeat the `apt install ./...deb` command above.

Launch **WIS Free V3** from the application menu. The Settings window now opens on a normal launch and shows a two-step Linux checklist:

1. Confirm that **Direct typing** says it is ready. Packaged installs start the included user service automatically. If it is not ready, the app identifies the missing package or stopped service and shows the command for the detected distribution.
2. Hold **Ctrl+Super** to record; a floating overlay shows live audio waves. Release to transcribe and type; the overlay pulses gold during transcription and disappears when finished. The tray keeps the same app icon throughout. If keyboard access is denied, the status explains how to enable it.

Enable **Run on System Startup** to launch in the tray at login. On Linux, choose or record the hold-to-record shortcut in WIS Settings; the same in-app shortcut listener works across desktops. GNOME also needs its AppIndicator extension to display tray icons. See the [Linux setup instructions](docs/linux-installation.md#2-hold-to-record-shortcut).

No reboot is normally required. A logout/login is only needed if `/dev/uinput` permissions do not refresh after installation.

Full distro-specific instructions and diagnostics are in [Linux installation and troubleshooting](docs/linux-installation.md).

## Build from source

Install Go 1.24+, Node.js 20+, and the native build dependencies listed in the [Linux guide](docs/linux-installation.md#build-from-source), then run:

```bash
git clone https://github.com/Daishir0/wisp-open.git
cd wisp-open
go install github.com/wailsapp/wails/v2/cmd/wails@latest
bash scripts/build-linux.sh --install-user
```

Use `--install` for `/usr/local/bin`, `--install-user` for `~/.local/bin`, or `--no-install` for a non-interactive build. The binary is written to `build/bin/wis-free-v3`.

## Build Linux packages

```bash
bash scripts/package-linux.sh --deb
bash scripts/package-linux.sh --rpm
```

With no flag, the script builds both formats and requires both `dpkg-deb` and `rpmbuild`. Packages are written to `dist/packages/`.

## Configuration

Settings are available from the Settings window and tray menu. Configuration is stored in `~/.wis-free-v3/config.json`.

The default recording shortcut is **Ctrl+Super**: Ctrl+Windows on Windows and Ctrl+Command on macOS.

## License

Distributed under the MIT License.
