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

### 2. Recording shortcut

Copy the full command shown by the app. It contains the real absolute executable path and is safely quoted. Do not replace it with a guessed executable name.

- GNOME / Ubuntu: **Settings → Keyboard → View and Customize Shortcuts → Custom Shortcuts**
- KDE Plasma: **System Settings → Keyboard → Shortcuts → Add New → Command or Script**
- Other desktops: add a custom command in the desktop's keyboard or shortcut settings

One press starts recording and the next press stops it. The command uses `--action=toggle` and can launch WIS from a cold start.

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

## Distribution packaging references

The service detection and dependency rules account for the layouts documented by the distributions:

- [Debian ydotool 1.0.4 file list](https://packages.debian.org/trixie-backports/amd64/ydotool/filelist) (combined client/daemon, user unit, and udev rule)
- [Ubuntu ydotool package search](https://packages.ubuntu.com/ydotool) (older releases split `ydotool` and `ydotoold`)
- [Fedora ydotool package](https://packages.fedoraproject.org/pkgs/ydotool/ydotool/) (system service)
- [Arch Linux ydotool file list](https://archlinux.org/packages/extra/x86_64/ydotool/files/) (user unit and udev rule)
- [openSUSE ydotool package](https://software.opensuse.org/package/ydotool) (official Tumbleweed package)
