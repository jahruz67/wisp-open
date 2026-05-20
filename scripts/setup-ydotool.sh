#!/bin/bash

# Exit on error
set -e

echo "=========================================================="
echo "   ydotool Wayland Auto-Configuration Utility"
echo "=========================================================="
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 1. Package Installation
echo "[1/4] Checking and installing ydotool..."
if ! command_exists ydotool; then
    if command_exists dnf; then
        echo "Fedora detected. Installing ydotool via dnf..."
        sudo dnf install -y ydotool
    elif command_exists apt-get; then
        echo "Debian/Ubuntu detected. Installing ydotool via apt..."
        sudo apt-get update && sudo apt-get install -y ydotool
    elif command_exists pacman; then
        echo "Arch Linux detected. Installing ydotool via pacman..."
        sudo pacman -S --noconfirm ydotool
    else
        echo "ERROR: Unsupported package manager. Please install 'ydotool' manually, then run this script again."
        exit 1
    fi
else
    echo "ydotool is already installed."
fi

# 2. Configure Non-Root /dev/uinput Access via udev
echo "[2/4] Configuring non-root udev rule for /dev/uinput..."
sudo tee /etc/udev/rules.d/80-uinput.rules << 'EOF' > /dev/null
KERNEL=="uinput", SUBSYSTEM=="misc", TAG+="uaccess", OPTIONS+="static_node=uinput"
EOF

echo "Reloading udev rules..."
sudo udevadm control --reload-rules && sudo udevadm trigger

# 3. Locate the Executable and Setup Systemd User Service
echo "[3/4] Designing systemd user-level service..."

# Dynamically locate the binary path to support varying distributions (/usr/bin vs /usr/local/bin)
YDOTOOLD_PATH=$(command -v ydotoold || true)
if [ -z "$YDOTOOLD_PATH" ]; then
    YDOTOOLD_PATH="/usr/bin/ydotoold"
fi

mkdir -p "$HOME/.config/systemd/user"
tee "$HOME/.config/systemd/user/ydotool.service" << EOF > /dev/null
[Unit]
Description=ydotoold key emulation daemon for non-root user

[Service]
Type=simple
ExecStart=$YDOTOOLD_PATH
Restart=always

[Install]
WantedBy=default.target
EOF

# 4. Activate User-Level Daemon
echo "[4/4] Starting ydotool daemon for user: $USER (UID: $(id -u))..."

# Disable 'exit on error' temporarily in case systemd user-manager lacks session initialization
set +e
systemctl --user daemon-reload
systemctl --user enable ydotool.service
systemctl --user start ydotool.service
set -e

# 5. Post-Configuration Diagnostics
echo ""
echo "=========================================================="
echo "   Post-Installation Diagnostics"
echo "=========================================================="
echo "Giving the daemon a moment to spin up..."
sleep 2

# Check if daemon is active
if systemctl --user is-active --quiet ydotool.service; then
    echo "[-] Daemon Status: Active (Running)"
else
    echo "[!] Daemon Status: Warning! The daemon failed to start."
    echo "    Check logs using: journalctl --user -u ydotool.service"
fi

# Check for socket existence
SOCKET_PATH="/run/user/$(id -u)/.ydotool_socket"
if [ -S "$SOCKET_PATH" ]; then
    echo "[-] Socket Location: Found at $SOCKET_PATH"
    echo ""
    echo "SUCCESS: Configuration completed."
    echo "You can now run your Go/Wails application, and paste operations"
    echo "will route seamlessly through ydotool."
else
    echo "[!] Socket Location: Missing at $SOCKET_PATH"
    echo ""
    echo "WARNING: The setup ran but the communications socket is absent."
    echo "This is typically caused by /dev/uinput permissions not applying yet."
    echo "Please log out of your desktop session and log back in, or run:"
    echo "    systemctl --user restart ydotool.service"
fi
echo "=========================================================="
