#!/bin/bash

# Exit on error
set -e

# Change to the root directory of the project
cd "$(dirname "$0")/.."

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
echo "[0/3] App version: $APP_VERSION"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

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
echo "[1/3] Checking system dependencies..."
MISSING_DEPS=0
DEPS=("gcc" "pkg-config")

for dep in "${DEPS[@]}"; do
    if ! command_exists $dep; then
        echo "[WARNING] Missing basic build tool: $dep"
        MISSING_DEPS=1
    fi
done

# We can't easily check C headers, but we can try to find them with pkg-config
if command_exists pkg-config; then
    if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.0; then
        echo "[WARNING] Missing Wails dependencies (GTK3 / WebKit2GTK)."
        MISSING_DEPS=1
    fi
    if ! pkg-config --exists x11 xtst xcb xkbcommon-x11; then
        echo "[WARNING] Missing gohook dependencies (X11 / Xtst / Xcb / Xkbcommon)."
        MISSING_DEPS=1
    fi
    if ! pkg-config --exists alsa; then
        echo "[WARNING] Missing audio dependencies (ALSA)."
        MISSING_DEPS=1
    fi
    if ! pkg-config --exists ayatana-appindicator3-0.1; then
        echo "[WARNING] Missing systray dependencies (ayatana-appindicator3)."
        MISSING_DEPS=1
    fi
fi

if [ $MISSING_DEPS -eq 1 ]; then
    echo ""
    echo "It looks like you are missing some required libraries."
    
    DEBIAN_DEPS="build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev libx11-dev libx11-xcb-dev libxtst-dev libasound2-dev libayatana-appindicator3-dev libxkbcommon-x11-dev"
    ARCH_DEPS="base-devel pkgconf gtk3 webkit2gtk libx11 libxtst alsa-lib libayatana-appindicator libxkbcommon-x11"
    
    echo "The full list of dependencies needed:"
    echo "  [Ubuntu/Debian]: sudo apt update && sudo apt install -y $DEBIAN_DEPS"
    echo "  [Arch Linux]:    sudo pacman -S $ARCH_DEPS"
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
    else
        read -p "Press Enter to attempt build anyway, or Ctrl+C to cancel..."
    fi
fi

# 3. Build the application
echo "[2/3] Building with Wails..."

# Pre-emptively fix npm bin permissions if they got messed up (common issue on some systems)
if [ -d "frontend/node_modules/.bin" ]; then
    chmod +x frontend/node_modules/.bin/* 2>/dev/null || true
fi

wails build -platform linux/amd64 -clean -ldflags "-X main.AppVersion=${APP_VERSION}"

if [ ! -f "$EXECUTABLE" ]; then
    echo "[ERROR] Build failed. Binary not found at $EXECUTABLE."
    exit 1
fi

echo "[3/3] Build successful!"
echo "      Output: $EXECUTABLE"
echo ""

# 4. Optional Installation
read -p "Would you like to install it globally to /usr/local/bin and add a desktop shortcut? (y/n): " INSTALL
if [[ "$INSTALL" == "y" || "$INSTALL" == "Y" ]]; then
    echo "Installing..."
    
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
    
    cat << EOF > /tmp/$APP_NAME.desktop
[Desktop Entry]
Type=Application
Name=WIS Free V3
Comment=Voice Dictation App
Exec=$APP_NAME
Icon=$APP_NAME
Terminal=false
Categories=Utility;Audio;
EOF

    sudo mv /tmp/$APP_NAME.desktop "$DESKTOP_FILE"
    sudo chmod 644 "$DESKTOP_FILE"
    
    echo ""
    echo "Installation complete! You can now launch 'WIS Free V3' from your app launcher,"
    echo "or by typing '$APP_NAME' in your terminal."
else
    echo "Skipping installation. You can run the app directly via: ./$EXECUTABLE"
fi
