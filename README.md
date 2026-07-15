# WIS Free V3

WIS Free V3 is a desktop voice-dictation app built with Go and Wails. It records audio from a global shortcut, transcribes it with Groq Whisper or a local backend, optionally refines the result, and types the text into the focused application.

Windows and Linux are supported. Linux packages and source builds currently target `amd64`/`x86_64`.

## Linux installation

Install a package with the native package manager so it resolves runtime dependencies. Do not use `dpkg -i` on its own.

### Debian and Ubuntu

Download the `.deb` for your architecture, then run:

```bash
sudo apt install ./wis-free-v3_<version>_amd64.deb
```

If a previous `dpkg -i` attempt left dependencies unfinished, repair them first:

```bash
sudo apt -f install
sudo apt install ./wis-free-v3_<version>_amd64.deb
```

### Fedora and other RPM-based distributions

Install the RPM with the distribution package manager:

```bash
sudo dnf install ./wis-free-v3-<version>-1.x86_64.rpm
```

For openSUSE, use `sudo zypper install ./wis-free-v3-<version>.x86_64.rpm`.

After installing, launch **WIS Free V3** from the application launcher or run `wis-free-v3` in a terminal. A reboot is not required.

## Linux first-run setup

### Global shortcut

Linux uses your desktop environment's custom shortcut feature as the primary global shortcut method. Copy the **Linux System Shortcut** command shown in Settings and bind it to your preferred key combination:

- GNOME: **Settings -> Keyboard -> Custom Shortcuts**
- KDE Plasma: **System Settings -> Shortcuts -> Command/URL**

That command toggles recording: one press starts and the next press stops. It is shell-quoted automatically, including when the app is installed in a path containing spaces. It also works from a cold start because it uses `--action=toggle`.

### Direct typing with ydotool

WIS Free V3 uses `ydotool` to type transcriptions into the active application. This works on both Wayland and X11, but it needs the persistent `ydotoold` service.

Install the dependency, then enable the service for your user:

```bash
# Debian / Ubuntu
sudo apt install ydotool

# Fedora
sudo dnf install ydotool

# Arch Linux
sudo pacman -S ydotool

# All distributions that provide the standard user unit
systemctl --user enable --now ydotool.service
```

Distribution packages normally install the required `/dev/uinput` rule and the `ydotool.service` user unit. Do not add an extra manual udev rule unless your distribution's ydotool documentation says its package does not provide one.

Open Settings again and check the **Direct typing** status. If the socket is still unavailable after enabling the service, log out and back in once, then run:

```bash
systemctl --user restart ydotool.service
```

For a ydotool installation without a packaged user unit, the source checkout can create a separate WIS Free V3 user unit without overwriting distribution files:

```bash
bash scripts/build-linux.sh --install-systemd
```

To diagnose it, use:

```bash
systemctl --user status ydotool.service
journalctl --user -u ydotool.service -b
```

The app detects sockets from the standard user-runtime directory, `/tmp/.ydotool_socket`, and common system-service locations. You can override detection with `YDOTOOL_SOCKET=/path/to/socket` before launching WIS Free V3.

### GNOME tray icon

GNOME needs the **AppIndicator and KStatusNotifierItem Support** extension for a traditional system-tray icon. The app still runs without it, but use the launcher or a configured shortcut to open Settings. On Debian/Ubuntu:

```bash
sudo apt install gnome-shell-extension-appindicator
```

Enable the extension in **Settings -> Extensions**. Other major desktops generally expose the tray icon without an extra extension.

## Build from source on Linux

Install Go 1.24+, Node.js 20+, and native dependencies. The build script handles WebKit 4.0 versus 4.1 and Wails tags automatically.

### Debian / Ubuntu

```bash
sudo apt update
sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libasound2-dev libayatana-appindicator3-dev
```

If your release does not provide `libwebkit2gtk-4.1-dev`, install `libwebkit2gtk-4.0-dev` instead.

### Fedora

```bash
sudo dnf install -y gcc gcc-c++ make pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel alsa-lib-devel libayatana-appindicator-gtk3-devel
```

### Arch Linux

```bash
sudo pacman -S base-devel pkgconf gtk3 webkit2gtk alsa-lib libayatana-appindicator
```

Then clone the repository and build:

```bash
git clone <repository-url>
cd wisp-open
go install github.com/wailsapp/wails/v2/cmd/wails@latest
bash scripts/build-linux.sh --install-user
```

Use `--install` for `/usr/local/bin`, `--install-user` for `~/.local/bin`, or `--no-install` for a non-interactive build. The binary is written to `build/bin/wis-free-v3`.

## Building packages

Create distribution packages from a Linux checkout:

```bash
# Build only the format available on your build host
bash scripts/package-linux.sh --deb
bash scripts/package-linux.sh --rpm
```

With no flag, the script builds both formats and therefore requires both `dpkg-deb` and `rpmbuild`. Packages are placed in `dist/packages/`. The package metadata records the WebKit and AppIndicator ABI selected during the build, preventing a package built against one implementation from silently depending on the other.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| The package installed but the app will not launch | Install it with `apt install ./file.deb` or `dnf install ./file.rpm` so runtime libraries are resolved. Launch `wis-free-v3` from a terminal once to see any loader error. |
| The shortcut does nothing | Copy the Linux System Shortcut command from Settings and bind it in your desktop's custom shortcut settings. |
| Transcription completes but text is not inserted | Open Settings and complete the ydotool setup. Check `systemctl --user status ydotool.service`. |
| `ydotoold socket not found` | Enable the user service, then log out/in once only if the service still cannot access `/dev/uinput`. |
| The app runs but no tray icon appears on GNOME | Install and enable the AppIndicator extension. |

## Configuration

Settings are available from the tray menu or the Settings window. The configuration file is stored in `~/.wis-free-v3/config.json`.

## License

Distributed under the MIT License.
