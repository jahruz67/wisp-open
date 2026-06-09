#!/bin/bash

# Exit on error
set -e

# Change to the root directory of the project
cd "$(dirname "$0")/.."

# Temporary pkg-config shim (Fedora 40+: only webkit2gtk-4.1.pc exists; some tooling still asks for 4.0)
WAILS_PKGCFG_SHIM=""
cleanup_pkgconfig_shim() {
    if [ -n "$WAILS_PKGCFG_SHIM" ] && [ -d "$WAILS_PKGCFG_SHIM" ]; then
        rm -rf "$WAILS_PKGCFG_SHIM"
    fi
}
trap cleanup_pkgconfig_shim EXIT

APP_NAME="wis-free-v3"
BUILD_DIR="build/bin"
EXECUTABLE="$BUILD_DIR/$APP_NAME"

echo "========================================"
echo "   wis-free-v3 - Linux Build Script"
echo "========================================"
echo ""

VERSION_FILE="$(dirname "$0")/VERSION"
APP_VERSION="dev"
if [ -f "$VERSION_FILE" ]; then
    APP_VERSION="$(head -n1 "$VERSION_FILE" | tr -d '\r\n')"
    APP_VERSION="$(echo "$APP_VERSION" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
fi
if [ -z "$APP_VERSION" ]; then
    APP_VERSION="dev"
fi
echo "[0/4] App version: $APP_VERSION"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

print_ydotool_install_help() {
    if command_exists dnf; then
        echo "    sudo dnf install ydotool"
    elif command_exists apt-get; then
        echo "    sudo apt install ydotool"
    elif command_exists pacman; then
        echo "    sudo pacman -S ydotool"
    else
        echo "    Install ydotool with your distribution's package manager."
    fi
}

find_ydotoold() {
    if command_exists ydotoold; then
        command -v ydotoold
        return 0
    fi
    for candidate in /usr/bin/ydotoold /usr/local/bin/ydotoold; do
        if [ -x "$candidate" ]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done
    return 1
}

setup_ydotool_systemd() {
    echo ""
    echo "========================================"
    echo "  Setting up ydotool systemd user service"
    echo "========================================"
    echo ""

    if ! command_exists systemctl; then
        echo "[ERROR] systemctl is not available on this system."
        return 1
    fi

    if ! command_exists ydotool; then
        echo "[ERROR] ydotool is not installed."
        echo "Install it first:"
        print_ydotool_install_help
        return 1
    fi

    YDOTOOLD_BIN="$(find_ydotoold || true)"
    if [ -z "$YDOTOOLD_BIN" ]; then
        echo "[ERROR] ydotoold was not found after installing ydotool."
        echo "Check your distribution's ydotool package or install the daemon package if it is split out."
        return 1
    fi

    SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
    mkdir -p "$SYSTEMD_USER_DIR"
    SERVICE_FILE="$SYSTEMD_USER_DIR/ydotool.service"

    cat > "$SERVICE_FILE" << YDSVCEOF
[Unit]
Description=ydotool daemon for WIS Free V3 direct keyboard injection
Documentation=man:ydotool(1)
After=graphical-session.target

[Service]
Type=simple
Environment=YDOTOOL_SOCKET=%t/.ydotool_socket
ExecStart=$YDOTOOLD_BIN
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
YDSVCEOF

    echo "[INFO] Created $SERVICE_FILE"

    UDEV_RULE_FILE="/etc/udev/rules.d/80-uinput.rules"
    if [ -f "$UDEV_RULE_FILE" ] && grep -q 'KERNEL=="uinput"' "$UDEV_RULE_FILE"; then
        echo "[INFO] uinput udev rule already exists at $UDEV_RULE_FILE"
    else
        echo ""
        echo "  /dev/uinput permission rule is needed for ydotool direct typing:"
        echo ""
        echo '    KERNEL=="uinput", SUBSYSTEM=="misc", TAG+="uaccess", OPTIONS+="static_node=uinput"'
        echo ""
        read -p "  Create or update $UDEV_RULE_FILE now? (requires sudo) (y/N): " CREATE_UDEV
        if [[ "$CREATE_UDEV" == "y" || "$CREATE_UDEV" == "Y" ]]; then
            echo 'KERNEL=="uinput", SUBSYSTEM=="misc", TAG+="uaccess", OPTIONS+="static_node=uinput"' | \
                sudo tee "$UDEV_RULE_FILE" > /dev/null
            sudo udevadm control --reload-rules && sudo udevadm trigger
            echo "[INFO] udev rule created and reloaded."
            echo "       Log out and back in, or reboot, if ydotool still cannot access /dev/uinput."
        else
            echo "  Skipping udev rule creation. ydotoold may not be able to inject keystrokes."
        fi
    fi

    echo ""
    echo "  Reloading systemd user daemon..."
    systemctl --user daemon-reload
    echo "  Enabling ydotool user service..."
    systemctl --user enable ydotool.service
    echo "  Starting ydotool user service..."
    if systemctl --user restart ydotool.service; then
        echo "  ydotool systemd service is running."
    else
        echo "  [WARN] Could not start ydotool.service. Check: systemctl --user status ydotool.service"
    fi
    echo ""
}

INSTALL_MODE="none"  # none, system, user
INSTALL_SYSTEMD=false
SHOW_HELP=false

for arg in "$@"; do
    case "$arg" in
        --install)
            INSTALL_MODE="system"
            ;;
        --install-user)
            INSTALL_MODE="user"
            ;;
        --install-systemd)
            INSTALL_SYSTEMD=true
            ;;
        --help|-h)
            SHOW_HELP=true
            ;;
        *)
            echo "[ERROR] Unknown option: $arg"
            echo "Usage: $0 [--install|--install-user|--install-systemd|--help]"
            exit 1
            ;;
    esac
done

if [ "$SHOW_HELP" = true ]; then
    echo "Usage: $0 [--install|--install-user|--install-systemd|--help]"
    echo ""
    echo "  (no flags)          Build only, then offer interactive install prompts"
    echo "  --install           Build and install system-wide (/usr/local/bin)"
    echo "  --install-user      Build and install per-user (~/.local/bin)"
    echo "  --install-systemd   Generate, reload, enable, and start ydotool user service"
    echo "  --help              Show this message"
    echo ""
    exit 0
fi

if [ "$INSTALL_SYSTEMD" = true ] && [ "$INSTALL_MODE" = "none" ]; then
    setup_ydotool_systemd
    exit $?
fi

# 1. Check for basic tools
if ! command_exists go; then
    echo "[ERROR] Go is not installed. Please install Go 1.23+."
    exit 1
fi

if ! command_exists wails; then
    echo "[INFO] Wails CLI not found. Installing..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    export PATH=$PATH:$(go env GOPATH)/bin
fi

# 2. Check for Linux dependencies
echo "[1/4] Checking system dependencies..."
MISSING_DEPS=0

# 2a. GNOME tray icon support check (before build — avoids launching app if this will fail)
# AppIndicator icons need the GNOME Shell extension on GNOME desktop environments.
# Other DEs (KDE Plasma, XFCE, Cinnamon, etc.) usually support them natively.
if [ "$XDG_CURRENT_DESKTOP" = "GNOME" ] || [ "$XDG_CURRENT_DESKTOP" = "ubuntu:GNOME" ]; then
    EXT_INSTALLED=false
    if command_exists dnf; then
        if dnf list installed gnome-shell-extension-appindicator &>/dev/null; then
            EXT_INSTALLED=true
        fi
    elif command_exists apt-get; then
        if dpkg -s gnome-shell-extension-appindicator &>/dev/null; then
            EXT_INSTALLED=true
        fi
    elif command_exists pacman; then
        if pacman -Qi gnome-shell-extension-appindicator &>/dev/null; then
            EXT_INSTALLED=true
        fi
    fi

    if [ "$EXT_INSTALLED" = false ]; then
        echo ""
        echo "==============================================================="
        echo "  WARNING: GNOME AppIndicator extension is not installed"
        echo "==============================================================="
        echo ""
        echo "  GNOME does not show system tray icons without this extension."
        echo "  The app will run but the tray icon will be invisible."
        echo ""
        if command_exists dnf; then
            echo "  Install it with:"
            echo ""
            echo "    sudo dnf install gnome-shell-extension-appindicator"
            echo ""
        elif command_exists apt-get; then
            echo "  Install it with:"
            echo ""
            echo "    sudo apt install gnome-shell-extension-appindicator"
            echo ""
        else
            echo "  Install the 'gnome-shell-extension-appindicator' package"
            echo "  for your distribution."
            echo ""
        fi
        echo "  Then log out and back in, and enable it in:"
        echo "    Settings > Extensions > AppIndicator and KStatusNotifierItem Support"
        echo ""
        echo "==============================================================="
        # Do NOT exit — just warn and continue. The user can still use the app
        # with a custom shortcut even without the tray icon.
        echo "  Continuing (the app will work, but the tray icon may be hidden)."
        echo ""
    else
        echo "[INFO] GNOME AppIndicator extension is installed."
        echo "       Make sure it's enabled in Settings > Extensions."
    fi
fi

DEPS=("gcc" "pkg-config")

for dep in "${DEPS[@]}"; do
    if ! command_exists $dep; then
        echo "[WARNING] Missing basic build tool: $dep"
        MISSING_DEPS=1
    fi
done

# ydotool check for direct text injection
if ! command_exists ydotool; then
    echo ""
    echo "==============================================================="
    echo "  WARNING: ydotool is not installed"
    echo "==============================================================="
    echo ""
    echo "  ydotool is required for direct keyboard injection on Wayland."
    echo "  Without it, WIS Free V3 cannot type transcribed text automatically"
    echo "  into the active Linux window."
    echo ""
    echo "  Install ydotool and configure it:"
    echo ""
    print_ydotool_install_help
    echo ""
    echo "  Then set up udev rules for /dev/uinput access:"
    echo "    echo 'KERNEL==\"uinput\", SUBSYSTEM==\"misc\", TAG+=\"uaccess\", OPTIONS+=\"static_node=uinput\"' |"
    echo "      sudo tee /etc/udev/rules.d/80-uinput.rules"
    echo "    sudo udevadm control --reload-rules && sudo udevadm trigger"
    echo ""
    echo "  Then enable the ydotool user service (see the --install-systemd flag below)."
    echo "==============================================================="
    echo ""
fi

# We can't easily check C headers, but we can try to find them with pkg-config
# (Fedora often ships webkit2gtk-4.1.pc; Debian/Ubuntu often use webkit2gtk-4.0.pc)
webkit2_ok() {
    pkg-config --exists webkit2gtk-4.0 2>/dev/null && return 0
    pkg-config --exists webkit2gtk-4.1 2>/dev/null && return 0
    return 1
}

# Tray: Debian uses ayatana-appindicator3; Fedora package names differ but .pc is often the same
appindicator_ok() {
    pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null && return 0
    pkg-config --exists appindicator3-0.1 2>/dev/null && return 0
    return 1
}

if command_exists pkg-config; then
    if ! pkg-config --exists gtk+-3.0 || ! webkit2_ok; then
        echo "[WARNING] Missing Wails dependencies (GTK3 / WebKit2GTK)."
        MISSING_DEPS=1
    fi
    if ! pkg-config --exists alsa; then
        echo "[WARNING] Missing audio dependencies (ALSA)."
        MISSING_DEPS=1
    fi
    if ! appindicator_ok; then
        echo "[WARNING] Missing systray dependencies (appindicator / ayatana)."
        MISSING_DEPS=1
    fi
fi

if [ $MISSING_DEPS -eq 1 ]; then
    echo ""
    echo "It looks like you are missing some required libraries."
    echo ""
    echo "Wayland note: global hotkeys use the XDG GlobalShortcuts portal (xdg-desktop-portal"
    echo "with a supporting compositor, e.g. KDE Plasma or GNOME)."
    echo ""
    
    DEBIAN_DEPS="build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev libasound2-dev libayatana-appindicator3-dev"
    # Runtime nicety (optional): playerctl pauses media while recording
    DEBIAN_RUNTIME_OPT="playerctl ydotool"
    # Fedora 40+: WebKit2GTK 4.0 packages are gone; use 4.1 + Wails -tags webkit2_41 (see wails build below).
    # pkgconf-pkg-config provides `pkg-config` on Fedora.
    FEDORA_DEPS="gcc gcc-c++ make pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel alsa-lib-devel libayatana-appindicator-gtk3-devel"
    # Same as FEDORA_DEPS but classic libappindicator (some spins/repos lack Ayatana -devel)
    FEDORA_DEPS_ALT="gcc gcc-c++ make pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel alsa-lib-devel libappindicator-gtk3-devel"
    FEDORA_RUNTIME_OPT="playerctl xdg-desktop-portal ydotool"
    ARCH_DEPS="base-devel pkgconf gtk3 webkit2gtk alsa-lib libayatana-appindicator"
    ARCH_RUNTIME_OPT="playerctl ydotool"
    
    echo "The full list of dependencies needed:"
    echo "  [Ubuntu/Debian]: sudo apt update && sudo apt install -y $DEBIAN_DEPS"
    echo "  [Ubuntu/Debian] optional: sudo apt install -y $DEBIAN_RUNTIME_OPT"
    echo "  [Fedora]:        sudo dnf install -y $FEDORA_DEPS"
    echo "                   (if Ayatana devel is missing, use libappindicator-gtk3-devel — see FEDORA_DEPS_ALT in this script)"
    echo "  [Fedora] optional: sudo dnf install -y $FEDORA_RUNTIME_OPT"
    echo "  [Arch Linux]:    sudo pacman -S $ARCH_DEPS"
    echo "  [Arch Linux] optional: sudo pacman -S $ARCH_RUNTIME_OPT"
    echo ""
    
    if command_exists apt-get; then
        read -p "Would you like to automatically install missing dependencies now? (Requires sudo) (y/N): " INSTALL_DEPS
        if [[ "$INSTALL_DEPS" == "y" || "$INSTALL_DEPS" == "Y" ]]; then
            echo "Installing dependencies..."
            sudo apt update && sudo apt install -y $DEBIAN_DEPS
        else
            echo "Skipping installation."
            read -p "Press Enter to attempt build anyway, or Ctrl+C to cancel..."
        fi
    elif command_exists dnf; then
        read -p "Would you like to automatically install missing dependencies with dnf now? (Requires sudo) (y/N): " INSTALL_DEPS
        if [[ "$INSTALL_DEPS" == "y" || "$INSTALL_DEPS" == "Y" ]]; then
            echo "Installing dependencies (Fedora)..."
            if ! sudo dnf install -y $FEDORA_DEPS; then
                echo "[INFO] Retrying dnf with libappindicator-gtk3-devel instead of Ayatana..."
                sudo dnf install -y $FEDORA_DEPS_ALT
            fi
        else
            echo "Skipping installation."
            read -p "Press Enter to attempt build anyway, or Ctrl+C to cancel..."
        fi
    else
        read -p "Press Enter to attempt build anyway, or Ctrl+C to cancel..."
    fi
fi

# 3. Build the application
echo "[2/4] Building with Wails..."

# Pre-emptively fix npm bin permissions if they got messed up (common issue on some systems)
if [ -d "frontend/node_modules/.bin" ]; then
    chmod +x frontend/node_modules/.bin/* 2>/dev/null || true
fi

# Fedora 40+ / rolling: only webkit2gtk-4.1 is shipped. Wails v2 uses -tags webkit2_41 for that API.
# Also symlink webkit2gtk-4.0.pc -> 4.1.pc so CGO / other deps that still probe "webkit2gtk-4.0" resolve.
WAILS_WEBKIT_TAGS=()
if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    WAILS_WEBKIT_TAGS=(-tags webkit2_41)
fi

if ! pkg-config --exists webkit2gtk-4.0 2>/dev/null && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    WEBKIT41_PC=""
    for dir in \
        $(printf '%s' "${PKG_CONFIG_PATH:-}" | tr ':' '\n') \
        /usr/lib64/pkgconfig \
        /usr/lib/pkgconfig \
        /usr/local/lib64/pkgconfig \
        /usr/local/lib/pkgconfig; do
        [ -z "$dir" ] && continue
        [ -f "$dir/webkit2gtk-4.1.pc" ] || continue
        WEBKIT41_PC="$dir/webkit2gtk-4.1.pc"
        break
    done
    if [ -n "$WEBKIT41_PC" ]; then
        WAILS_PKGCFG_SHIM=$(mktemp -d "${TMPDIR:-/tmp}/wails-pkgcfg-shim.XXXXXX")
        ln -sf "$WEBKIT41_PC" "$WAILS_PKGCFG_SHIM/webkit2gtk-4.0.pc"
        export PKG_CONFIG_PATH="$WAILS_PKGCFG_SHIM${PKG_CONFIG_PATH:+:}${PKG_CONFIG_PATH:-}"
        echo "[INFO] Using PKG_CONFIG_PATH shim: webkit2gtk-4.0.pc -> $(basename "$WEBKIT41_PC")"
    fi
fi

wails build -platform linux/amd64 -clean "${WAILS_WEBKIT_TAGS[@]}" -ldflags "-X main.AppVersion=${APP_VERSION}"

if [ ! -f "$EXECUTABLE" ]; then
    echo "[ERROR] Build failed. Binary not found at $EXECUTABLE."
    exit 1
fi

# wails build sometimes auto-launches the binary on Linux — kill it so it
# doesn't block the terminal while we ask about installation.
pkill -f "$EXECUTABLE" 2>/dev/null || true

echo "[3/4] Build successful!"
echo "      Output: $EXECUTABLE"
echo ""

# 4. Optional Installation
echo "[4/4] Installation options"
echo ""
echo "  (i) Install systemwide:  sudo ./$0 --install"
echo "  (u) Install user-local:  ./$0 --install-user"
echo "  (s) Install systemd service (ydotool):  ./$0 --install-systemd"
echo "  (h) Show this help"
echo ""
echo "  Without flags, the build-only mode finishes here."
echo "  Binary is ready at: $EXECUTABLE"
echo ""

# If no install flags were passed, do interactive prompt (backward-compatible)
if [ "$INSTALL_MODE" = "none" ] && [ "$INSTALL_SYSTEMD" = false ]; then
    read -p "Would you like to install it system-wide to /usr/local/bin? (y/n): " INSTALL
    if [[ "$INSTALL" == "y" || "$INSTALL" == "Y" ]]; then
        INSTALL_MODE="system"
    else
        read -p "Install per-user to ~/.local/bin? (y/n): " INSTALL_USER
        if [[ "$INSTALL_USER" == "y" || "$INSTALL_USER" == "Y" ]]; then
            INSTALL_MODE="user"
        fi
    fi

    read -p "Would you like to set up the ydotool systemd user service? (y/n): " SETUP_SYSTEMD
    if [[ "$SETUP_SYSTEMD" == "y" || "$SETUP_SYSTEMD" == "Y" ]]; then
        INSTALL_SYSTEMD=true
    fi
fi

# --- Systemd ydotool service setup ---
if [ "$INSTALL_SYSTEMD" = true ]; then
    setup_ydotool_systemd
fi

# --- Binary installation ---
if [ "$INSTALL_MODE" = "system" ]; then
    echo "Installing system-wide..."

    # Needs sudo
    sudo cp "$EXECUTABLE" "/usr/local/bin/$APP_NAME"
    sudo chmod +x "/usr/local/bin/$APP_NAME"

    # Create Desktop shortcut
    DESKTOP_FILE="/usr/share/applications/$APP_NAME.desktop"

    # Try to grab the icon from the Wails build directory if available
    ICON_PATH="/usr/share/pixmaps/$APP_NAME.png"
    if [ -f "build/appicon.png" ]; then
        sudo cp "build/appicon.png" "$ICON_PATH"
    fi

    # Absolute Exec path: some desktop environments do not put /usr/local/bin on PATH
    # for .desktop launches, so "Exec=wis-free-v3" can fail with no visible error.
    cat << EOF > /tmp/$APP_NAME.desktop
[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Dictation App
Exec=/usr/local/bin/$APP_NAME
TryExec=/usr/local/bin/$APP_NAME
Icon=$APP_NAME
StartupWMClass=$APP_NAME
Terminal=false
Categories=Utility;Audio;
EOF

    sudo mv /tmp/$APP_NAME.desktop "$DESKTOP_FILE"
    sudo chmod 644 "$DESKTOP_FILE"

    echo ""
    echo "Installation complete! You can now launch 'WIS Free V3' from your app launcher,"
    echo "or by typing '$APP_NAME' in your terminal."
    echo ""

elif [ "$INSTALL_MODE" = "user" ]; then
    echo "Installing per-user..."

    LOCAL_BIN_DIR="$HOME/.local/bin"
    mkdir -p "$LOCAL_BIN_DIR"

    cp "$EXECUTABLE" "$LOCAL_BIN_DIR/$APP_NAME"
    chmod +x "$LOCAL_BIN_DIR/$APP_NAME"

    # Add to PATH warning
    case ":${PATH}:" in
        *:"$LOCAL_BIN_DIR":*) ;;
        *)
            echo "[WARNING] $LOCAL_BIN_DIR is not in your PATH."
            echo "         Add it to your shell profile:"
            echo '           export PATH="$HOME/.local/bin:$PATH"'
            echo ""
            ;;
    esac

    # Create user-local desktop file
    DESKTOP_DIR="$HOME/.local/share/applications"
    mkdir -p "$DESKTOP_DIR"
    ICONS_DIR="$HOME/.local/share/pixmaps"
    mkdir -p "$ICONS_DIR"

    if [ -f "build/appicon.png" ]; then
        cp "build/appicon.png" "$ICONS_DIR/$APP_NAME.png"
    fi

    cat > "$DESKTOP_DIR/$APP_NAME.desktop" << EOF
[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Dictation App
Exec=$LOCAL_BIN_DIR/$APP_NAME
TryExec=$LOCAL_BIN_DIR/$APP_NAME
Icon=$APP_NAME
StartupWMClass=$APP_NAME
Terminal=false
Categories=Utility;Audio;
EOF
    chmod 644 "$DESKTOP_DIR/$APP_NAME.desktop"

    echo ""
    echo "User-local installation complete!"
    echo "  Binary: $LOCAL_BIN_DIR/$APP_NAME"
    echo "  Desktop: $DESKTOP_DIR/$APP_NAME.desktop"
    echo ""

elif [ "$INSTALL_MODE" = "none" ] && [ "$INSTALL_SYSTEMD" = false ]; then
    echo "Skipping installation. You can run the app directly via: ./$EXECUTABLE"
fi

echo "========================================"
echo "  Build finished successfully!"
echo "========================================"
