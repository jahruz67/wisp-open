# Linux installation and troubleshooting

WIS Free V3 supports X11 and Wayland on `amd64`/`x86_64`. Direct typing uses `ydotool`, which requires both its client and a persistent `ydotoold` service. Linux distributions package that service differently, so WIS Free V3 detects both user and system units.

## Install a release package

### Ubuntu, Debian, Linux Mint, and Pop!_OS

```bash
sudo apt install ./wis-free-v3_<version>-1_amd64.deb
```

Use the leading `./`; without it, APT searches configured repositories instead of installing the downloaded file. The WIS package requires `ydotool` and, on releases that split it out, `ydotoold`.

Release packages also require `pactl` for muting audio during recording and restoring it afterwards. APT installs its `pulseaudio-utils` package automatically; RPM package managers resolve the required `/usr/bin/pactl` provider automatically.

If an earlier `dpkg -i` left the installation incomplete:

```bash
sudo apt -f install
sudo apt install ./wis-free-v3_<version>-1_amd64.deb
```

### Fedora and other DNF-based distributions

```bash
sudo dnf install ./wis-free-v3-<version>-1.x86_64.rpm
```

The WIS RPM includes a user-service integration and uinput permission rule. It also recognizes Fedora's system-level `ydotool.service` if that service is already configured.

### openSUSE Tumbleweed

```bash
sudo zypper install ./wis-free-v3-<version>-1.x86_64.rpm
```

The RPM relies on library/ABI dependencies discovered from the built binary rather than Fedora-specific package names, allowing Zypper to resolve the corresponding openSUSE packages.

## First launch

Open **WIS Free V3** from the application menu. A normal launch opens Settings; `--background` is used only by autostart.

### 1. Direct typing

The app checks all of the following separately:

- the `ydotool` client;
- the `ydotoold` daemon;
- WIS and distribution-provided user services;
- Fedora-style system services;
- the daemon socket and a harmless client connection.

Release packages include `/usr/lib/systemd/user/wis-free-v3-ydotool.service` and `/usr/lib/udev/rules.d/80-wis-free-v3-uinput.rules`. WIS starts this user service automatically when the app launches. Click **Set up direct typing** if the service needs another attempt; on a distro that only supplies a system service, this may display the normal administrator-authentication prompt.

If a package or daemon is missing, the app shows only the command appropriate for the detected distribution. Install it, then click **Check again**.

### 2. Hold-to-record shortcut

WIS defaults to **Ctrl+Super**. Use the **Record** button in WIS Settings to choose another shortcut. The app listens to Linux keyboard state directly, so shortcut setup is the same in KDE, GNOME, and other desktop environments. Keypresses are not consumed; choose a combination that does not conflict with another action.

- Hold the shortcut: a floating pill shows live audio waves while recording.
- Release it: the overlay pulses **gold** during transcription while WIS prepares the text for insertion.
- When insertion finishes: the overlay disappears. Wait for it to disappear before starting another recording.

Enable **Run on System Startup** to launch in the tray at login. Login itself never starts a recording. If WIS reports that it cannot read `/dev/input/event*`, grant your account access to the `input` group and sign out and back in:

```bash
sudo usermod -aG input "$USER"
```

Membership in `input` grants programs running as your user read/write access to Linux input devices, including keyboards. Only make this change if you want to grant that access.

The **WIS app icon stays registered in the system tray whenever WIS is running**, including immediately after a background login launch and while Settings is closed. It uses the same artwork as Windows and stays unchanged during recording and transcription. The floating overlay shows activity without taking keyboard focus or intercepting clicks. WIS uses a native Wayland overlay layer on supported compositors and a separately positioned X11/XWayland overlay on other desktops, including GNOME. Linux packages install both overlay dependencies automatically. The settings window keeps its normal display backend. In Plasma, set WIS to **Always shown** under **System Tray Settings → Entries** if your panel hides it. In GNOME, enable **AppIndicator and KStatusNotifierItem Support** in Extensions. WIS Settings checks whether a tray host is available and shows the relevant setup hint if one is missing.

GNOME may require the AppIndicator extension described below to display the tray icon.

### KDE Plasma integration

The same recording and typing backend is used on Plasma Wayland and Plasma X11. The tray uses AppIndicator/StatusNotifier integration, which Plasma supports without a GNOME extension. If the icon is hidden, expand the system tray or change its visibility in **System Tray Settings → Entries**.

Use **Start with system** in WIS to start in the tray at login. You can also manage the entry in Plasma's **Autostart** settings; WIS recognizes an entry disabled there. Launchers and icons honor `XDG_DATA_HOME`, and autostart honors `XDG_CONFIG_HOME`, with the standard home-directory defaults. Package installs, source installs, and launchers refreshed by the app share the same desktop-entry generator, including the recording action and escaping for paths with spaces or special characters.

WIS listens to the shortcut saved in Settings. Its keyboard shortcut does not depend on KDE shortcut configuration.

## Offline Whisper and GPU acceleration

Linux release packages include whisper.cpp v1.9.4 with Vulkan support, using the same engine and GGML model format as the Windows Vulkan version. No model is bundled or downloaded during app installation.

Linux release builds target glibc 2.35 and newer. They download the prebuilt engine from a pinned GitHub commit and verify its SHA-256 checksum before packaging it. If **Install Offline Whisper** fails with `GLIBC_x.y not found`, the installed engine was built against a newer glibc than your system provides. Install a compatible WIS release package or a compatible manually built engine. Retrying the model download or rebuilding only the app will not fix an incompatible engine.

In Settings, open **Offline Whisper**, choose a model size, and click **Install Offline Whisper**. The download runs inside Settings with progress and verifies the model's size and SHA-256 checksum. Then select **Local — [model] (offline)** in the Whisper Model dropdown. The engine and model run locally; no transcription API key is needed. Set **AI Refinement** to **None** to keep the entire dictation flow offline.

Settings reports **Engine already installed** when a working engine is present. If it is missing or cannot start, WIS downloads the pinned Linux engine with **Downloading engine** progress, verifies its SHA-256 checksum, and checks it before downloading the model. Downloaded engines are kept in `~/.wis-free-v3/whisper/engine/`; an existing installation is preserved if a replacement download fails.

GPU acceleration requires a compatible Vulkan graphics driver. For Intel/AMD, install your distribution's Mesa Vulkan drivers (`mesa-vulkan-drivers` on Debian/Ubuntu and Fedora; `vulkan-intel` or `vulkan-radeon` on Arch). For NVIDIA, use your distribution's supported NVIDIA driver with Vulkan support. A Vulkan loader alone does not supply a GPU driver. The packaged engine also includes the CPU backend for machines without a usable Vulkan GPU.

If GPU acceleration is unavailable, inspect `vulkaninfo --summary` (from `vulkan-tools`) and the WIS logs under `~/.wis-free-v3/logs/`. Whisper backend diagnostics in the log show which device was detected; installation success alone does not confirm GPU inference. Report the GPU model, driver, and diagnostic output when troubleshooting.

Release packages install the engine in `/usr/lib/wis-free-v3/whisper-cli`. Source installs use `/usr/local/lib/wis-free-v3/whisper-cli` or `~/.local/lib/wis-free-v3/whisper-cli`. A manually installed `whisper-cli` on `PATH` is also accepted when the bundled engine is absent; its GPU support depends on how it was built. Models remain in `~/.wis-free-v3/whisper/`.

Upstream reference: [whisper.cpp Vulkan GPU support](https://github.com/ggml-org/whisper.cpp/blob/v1.9.4/README.md#vulkan-gpu-support).

## Troubleshooting

### Direct typing is not ready

Start with the status and command shown in the app. For manual diagnostics, first determine which service your distribution installed:

```bash
systemctl --user status wis-free-v3-ydotool.service
systemctl --user status ydotool.service
systemctl status ydotool.service
```

Only one of these needs to exist and run. Relevant logs are available with:

```bash
journalctl --user -u wis-free-v3-ydotool.service -b
journalctl --user -u ydotool.service -b
journalctl -u ydotool.service -b
```

If the service reports that it cannot open `/dev/uinput`, log out and back in once so the desktop session receives the new udev access rule. A full reboot is rarely necessary.

The app checks the standard per-user socket, `/tmp/.ydotool_socket`, and common system-service locations. An advanced custom service can override detection by launching WIS with `YDOTOOL_SOCKET=/path/to/socket`.

### The shortcut does nothing

For hold-to-record, check the status in WIS Settings. If it says it cannot read `/dev/input/event*`, the current user needs input-device access. Add the user to the `input` group with the command above, sign out and back in, then restart WIS. This grants programs running as that user read/write access to all input devices.

### The app appears not to launch

Current builds show Settings after a normal application-menu launch. If no window appears, run:

```bash
wis-free-v3
```

Any missing runtime-library message will be visible in that terminal. On GNOME, a missing tray icon does not stop the Settings window or shortcut from working. To enable the tray icon on Debian/Ubuntu:

```bash
sudo apt install gnome-shell-extension-appindicator
```

Then enable **AppIndicator and KStatusNotifierItem Support** in the Extensions app.

## Build from source

Install Go 1.24+, Node.js 20+, and native dependencies.

### Debian and Ubuntu

```bash
sudo apt update
sudo apt install -y build-essential pkg-config git curl unzip libvulkan1 libgtk-3-dev libgtk-layer-shell-dev libx11-dev libxext-dev libxrandr-dev xwayland libwebkit2gtk-4.1-dev libasound2-dev libayatana-appindicator3-dev pulseaudio-utils ydotool
```

On Debian/Ubuntu releases where `ydotoold` is a separate package, also run `sudo apt install ydotoold`. If `libwebkit2gtk-4.1-dev` is unavailable, use `libwebkit2gtk-4.0-dev`.

### Fedora

```bash
sudo dnf install -y gcc gcc-c++ make pkgconf-pkg-config git curl unzip vulkan-loader gtk3-devel gtk-layer-shell-devel libX11-devel libXext-devel libXrandr-devel xorg-x11-server-Xwayland webkit2gtk4.1-devel alsa-lib-devel libayatana-appindicator-gtk3-devel pulseaudio-utils ydotool
```

### Arch Linux and Manjaro

```bash
sudo pacman -S base-devel pkgconf git curl unzip vulkan-icd-loader gtk3 gtk-layer-shell libx11 libxext libxrandr xorg-xwayland webkit2gtk alsa-lib libayatana-appindicator libpulse ydotool
```

Then build and install:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
bash scripts/build-linux.sh --install-user
bash scripts/build-linux.sh --install-systemd
```

Both package and source builds run `scripts/download-whisper-linux.sh` to download the prebuilt CPU/Vulkan engine and license from the pinned GitHub archive. They verify the archive's SHA-256 checksum and install the same engine without recompiling Whisper or its shader compiler. CMake, glslc, and Vulkan development headers are no longer required for normal Linux app builds. No model is downloaded by the build scripts.

The service setup prefers a distribution-provided user unit. If none exists, it creates a WIS-specific user unit without overwriting distribution files.

## Uninstall

```bash
# Debian / Ubuntu
sudo apt remove wis-free-v3

# Fedora / DNF
sudo dnf remove wis-free-v3

# openSUSE
sudo zypper remove wis-free-v3
```

User settings and transcription history remain in `~/.wis-free-v3/` so upgrades do not erase them. Remove that directory manually only if you also want to delete the saved configuration and history.

## KDE release checks

Automated checks cover recording state transitions and the shared tray artwork. Linux CI additionally launches the packaged app with `--background` under Xvfb and checks that its real StatusNotifier item exposes the app icon with status **Active** while idle. These checks do not exercise the overlay, physical keyboard input, or `/dev/input` permissions.

Before claiming a release has been tested on Plasma, run these checks in both a **Plasma Wayland** session and a **Plasma X11** session where available. Record the distribution, Plasma version, session type (`echo "$XDG_SESSION_TYPE"`), package version, and result. A Windows build or a headless launcher test does not count as a completed Plasma session check.

| Check | Expected result |
| --- | --- |
| Launch from the application menu, then launch again | Settings opens; there is one WIS process and one tray entry. |
| Record a shortcut in WIS Settings, hold it, and release it | Live audio waves while held; gold pulses during transcription; overlay disappears after insertion. The tray icon stays unchanged. |
| Hold a shortcut with a trigger key long enough for keyboard auto-repeat | The app keeps recording until release. |
| Press the shortcut again while text is still being inserted | The overlay remains visible and WIS finishes inserting the current result once. |
| Restart WIS after enabling input-device access | The shortcut saved in Settings is ready without beginning a recording. |
| Dictate into Kate, a browser, and an XWayland app on Wayland | Text reaches the focused field once, with the intended punctuation and keyboard layout. |
| Change the default microphone in Plasma and record again | The selected/default device records usable audio. |
| Open Settings from the tray, close it, and dictate again | The window hides; the app remains running and text reaches the intended app. |
| Enable autostart, log out, and log back in | One tray instance starts without opening Settings or beginning a recording. |
| Leave WIS idle after login with Settings closed | The app icon remains in the tray throughout the idle period; the overlay stays hidden. |
| Disable WIS in Plasma Autostart settings, then reopen WIS Settings | **Start with system** is off. |

## Desktop integration references

- [KDE's desktop-action shortcut implementation](https://invent.kde.org/plasma/kglobalacceld/-/blob/master/src/kserviceactioncomponent.cpp)
- [GNOME 48 Global Shortcuts support](https://release.gnome.org/48/developers/)
- [Desktop Entry command escaping](https://specifications.freedesktop.org/desktop-entry/latest/exec-variables.html)
- [Desktop Application Autostart specification](https://specifications.freedesktop.org/autostart/latest/)

## Distribution packaging references

The service detection and dependency rules account for the layouts documented by the distributions:

- [Debian ydotool 1.0.4 file list](https://packages.debian.org/trixie-backports/amd64/ydotool/filelist) (combined client/daemon, user unit, and udev rule)
- [Ubuntu ydotool package search](https://packages.ubuntu.com/ydotool) (older releases split `ydotool` and `ydotoold`)
- [Fedora ydotool package](https://packages.fedoraproject.org/pkgs/ydotool/ydotool/) (system service)
- [Arch Linux ydotool file list](https://archlinux.org/packages/extra/x86_64/ydotool/files/) (user unit and udev rule)
- [openSUSE ydotool package](https://software.opensuse.org/package/ydotool) (official Tumbleweed package)
