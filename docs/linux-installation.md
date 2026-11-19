# Linux installation and troubleshooting

WIS Free V3 supports X11 and Wayland on `amd64`/`x86_64`. Direct typing uses `ydotool`, which requires both its client and a persistent `ydotoold` service. Linux distributions package that service differently, so WIS Free V3 detects both user and system units.

## Install a release package

### Ubuntu, Debian, Linux Mint, and Pop!_OS

```bash
sudo apt install ./wis-free-v3_<version>-1_amd64.deb
```

Use the leading `./`; without it, APT searches configured repositories instead of installing the downloaded file. The WIS package requires `ydotool` and, on releases that split it out, `ydotoold`.

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

Click **Set up hold-to-record** and approve the desktop's shortcut dialog. Choose the recording key there; WIS suggests **Ctrl+Super** (Control + Command / Windows key). Remove any old custom command bound to the same keys. WIS uses the desktop's Global Shortcuts portal to receive separate press and release events on both Wayland and X11.

- Hold the shortcut: the microphone turns **red** and records.
- Release it: the microphone turns **yellow** while the clip is finalized, transcribed, refined, and typed into the focused app.
- When insertion finishes: the microphone returns to **white**. Wait for white before starting another recording.

Enable **Run on System Startup** to launch in the tray at login. After the shortcut has been approved, WIS restores it at subsequent launches. Login itself never starts a recording. KDE supports the portal through `xdg-desktop-portal-kde`; GNOME requires GNOME 48 or newer and `xdg-desktop-portal-gnome`. The app shows a setup error if the Global Shortcuts portal is unavailable; it does not pretend that a command shortcut reports key release.

The **white microphone stays registered in the system tray whenever WIS is running**, including immediately after a background login launch and while Settings is closed. WIS changes its color rather than removing the icon while idle. In Plasma, set WIS to **Always shown** under **System Tray Settings → Entries** if your panel hides it. In GNOME, enable **AppIndicator and KStatusNotifierItem Support** in Extensions. WIS Settings checks whether a tray host is available and shows the relevant setup hint if one is missing.

Use **Change shortcut** to open the desktop's configuration UI. On older portal versions without that UI method, change the binding in the desktop's application shortcut settings. GNOME may require the AppIndicator extension described below to display the tray icon.

### Optional toggle shortcut

The command under **Optional toggle shortcut**, and KDE's native **Toggle recording** desktop action, are alternatives for desktops without a suitable portal. They start on one press and stop on the next. They do **not** implement hold and release; the setup button above is the supported hold-to-record path.

Copy the full command shown by the app. It contains the real absolute executable path and is safely quoted. Do not replace it with a guessed executable name.

- GNOME / Ubuntu: **Settings → Keyboard → View and Customize Shortcuts → Custom Shortcuts**
- KDE Plasma 6: **System Settings → Keyboard → Shortcuts → Add New → Application → WIS Free V3**, then assign a key to **Toggle recording** and click **Apply**. The launcher includes this native action; choose it rather than the action that opens Settings. Alternatively, choose **Command or Script** and paste the command.
- KDE Plasma 5: **System Settings → Shortcuts → Custom Shortcuts → Edit → New → Global Shortcut → Command/URL**, then paste the command under **Action** and set the key under **Trigger**.
- Other desktops: add a custom command in the desktop's keyboard or shortcut settings

The optional command uses `--action=toggle` and can launch WIS from a cold start.

### KDE Plasma integration

The same recording and typing backend is used on Plasma Wayland and Plasma X11. The tray uses AppIndicator/StatusNotifier integration, which Plasma supports without a GNOME extension. If the icon is hidden, expand the system tray or change its visibility in **System Tray Settings → Entries**.

Use **Start with system** in WIS to start in the tray at login. You can also manage the entry in Plasma's **Autostart** settings; WIS recognizes an entry disabled there. Launchers and icons honor `XDG_DATA_HOME`, and autostart honors `XDG_CONFIG_HOME`, with the standard home-directory defaults. Package installs, source installs, and launchers refreshed by the app share the same desktop-entry generator, including the recording action and escaping for paths with spaces or special characters.

Configure **Hold to record** through WIS's setup button. The launcher's **Toggle recording** action remains available as a separate optional mode. If that optional action is missing after upgrading, launch WIS once and reopen the Shortcuts settings page; WIS refreshes its per-user launcher on startup.

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

For hold-to-record, check its status in WIS Settings and click **Set up hold-to-record** or **Change shortcut**. Confirm that the desktop has assigned a key to **Hold to record**, and that an old toggle command is not using the same keys. If permission was cancelled, the setup button can request it again. A portal/session failure releases an active recording and shows a disconnected status in Settings.

For the optional toggle command:

Copy the command again from Settings. Older builds displayed the incorrect name `wisp-open`; the correct release executable is `wis-free-v3`, and current builds copy its absolute path.

Run the copied command in a terminal once. If the app is already running, it should toggle recording. If it is not running, it should launch in the background and begin recording.

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
sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libasound2-dev libayatana-appindicator3-dev ydotool
```

On Debian/Ubuntu releases where `ydotoold` is a separate package, also run `sudo apt install ydotoold`. If `libwebkit2gtk-4.1-dev` is unavailable, use `libwebkit2gtk-4.0-dev`.

### Fedora

```bash
sudo dnf install -y gcc gcc-c++ make pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel alsa-lib-devel libayatana-appindicator-gtk3-devel ydotool
```

### Arch Linux and Manjaro

```bash
sudo pacman -S base-devel pkgconf gtk3 webkit2gtk alsa-lib libayatana-appindicator ydotool
```

Then build and install:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
bash scripts/build-linux.sh --install-user
bash scripts/build-linux.sh --install-systemd
```

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

Automated tests cover press/release ordering, repeated activations, permission cancellation, session loss, busy-state lifetime, and the red/yellow/white tray icons. Linux CI additionally launches the packaged app with `--background` under Xvfb and checks that its real StatusNotifier item exposes a white microphone with status **Active** while idle. It also tests the portal protocol over an isolated D-Bus session, desktop-entry escaping, native launcher execution through GLib, XDG locations, disabled autostart entries, and the packaged launcher with `desktop-file-validate`. These checks do not exercise KWin, the microphone, or physical keyboard input.

Before claiming a release has been tested on Plasma, run these checks in both a **Plasma Wayland** session and a **Plasma X11** session where available. Record the distribution, Plasma version, session type (`echo "$XDG_SESSION_TYPE"`), package version, and result. A Windows build or a headless launcher test does not count as a completed Plasma session check.

| Check | Expected result |
| --- | --- |
| Launch from the application menu, then launch again | Settings opens; there is one WIS process and one tray entry. |
| Set up **Hold to record**, hold the key, and release it | Red while held; yellow immediately on release and throughout transcription/refinement/typing; white after insertion. |
| Hold the key long enough for keyboard auto-repeat | The app keeps recording until release; repeated activation never toggles it off. |
| Press the shortcut again while text is still being inserted | The app stays yellow and finishes inserting the current result once. |
| Restart WIS or log in after completing setup | The approved hold shortcut is restored without beginning a recording. |
| Optional: trigger **Toggle recording** with WIS fully exited | WIS starts in the background and records; a second press stops it. |
| Dictate into Kate, a browser, and an XWayland app on Wayland | Text reaches the focused field once, with the intended punctuation and keyboard layout. |
| Change the default microphone in Plasma and record again | The selected/default device records usable audio. |
| Open Settings from the tray, close it, and dictate again | The window hides; the app remains running and text reaches the intended app. |
| Enable autostart, log out, and log back in | One tray instance starts without opening Settings or beginning a recording. |
| Leave WIS idle after login with Settings closed | The white microphone remains in the tray throughout the idle period. |
| Disable WIS in Plasma Autostart settings, then reopen WIS Settings | **Start with system** is off. |

## Desktop integration references

- [KDE's desktop-action shortcut implementation](https://invent.kde.org/plasma/kglobalacceld/-/blob/master/src/kserviceactioncomponent.cpp)
- [Global Shortcuts portal press/release API](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.GlobalShortcuts.html)
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
