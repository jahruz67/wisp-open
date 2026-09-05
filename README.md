# WIS Free V3

WIS Free V3 is a desktop voice-dictation app built with Go and Wails. It records audio, transcribes it with Groq or Mistral, optionally cleans up the result, and types it into the focused application.

Windows and Linux are supported. Current Linux release packages target `amd64`/`x86_64`.

## Install on Linux

Download the package for your distribution from the release page and install it with your normal package manager. The package manager installs the app and its direct-typing dependency together.

| Distribution | Package | Install command |
| --- | --- | --- |
| Ubuntu, Debian, Linux Mint, Pop!_OS | `.deb` | `sudo apt install ./wis-free-v3_<version>-1_amd64.deb` |
| Fedora and compatible DNF desktops | `.rpm` | `sudo dnf install ./wis-free-v3-<version>-1.x86_64.rpm` |
| openSUSE Tumbleweed | `.rpm` | `sudo zypper install ./wis-free-v3-<version>-1.x86_64.rpm` |
| Arch, Manjaro | source | Follow [Build from source](docs/linux-installation.md#build-from-source) |

Do not use `dpkg -i` by itself: it does not resolve dependencies. If you already did, run `sudo apt -f install`, then repeat the `apt install ./...deb` command above.

Launch **WIS Free V3** from the application menu. The Settings window now opens on a normal launch and shows a two-step Linux checklist:

1. Confirm that **Direct typing** says it is ready. Packaged installs start the included user service automatically. If it is not ready, the app identifies the missing package or stopped service and shows the command for the detected distribution.
2. Copy the exact **Recording shortcut** command and assign it in your desktop's custom keyboard-shortcut settings.

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

## License

Distributed under the MIT License.
