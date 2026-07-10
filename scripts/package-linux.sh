#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

APP_NAME="wis-free-v3"
DISPLAY_NAME="WIS Free V3"
COMMENT="Voice Dictation App"
MAINTAINER="${MAINTAINER:-WIS Free V3 Maintainers}"
LICENSE="${LICENSE:-MIT}"
ARCH_DEB="${ARCH_DEB:-amd64}"
ARCH_RPM="${ARCH_RPM:-x86_64}"
BUILD_DIR="build/bin"
EXECUTABLE="$BUILD_DIR/$APP_NAME"
DIST_DIR="dist/packages"
WORK_DIR="build/package-linux"
VERSION_FILE="scripts/VERSION"

APP_VERSION="${APP_VERSION:-dev}"
if [ -f "$VERSION_FILE" ]; then
    APP_VERSION="$(head -n1 "$VERSION_FILE" | tr -d '\r\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
fi
[ -n "$APP_VERSION" ] || APP_VERSION="dev"

PKG_VERSION="$(printf '%s' "$APP_VERSION" | sed 's/[^A-Za-z0-9.+~]/./g')"
PKG_RELEASE="${PKG_RELEASE:-1}"
BUILD_DEB=true
BUILD_RPM=true

for arg in "$@"; do
    case "$arg" in
        --deb)
            BUILD_DEB=true
            BUILD_RPM=false
            ;;
        --rpm)
            BUILD_DEB=false
            BUILD_RPM=true
            ;;
        --help|-h)
            echo "Usage: $0 [--deb|--rpm]"
            echo "  (no flag)  Build both formats when their build tools are installed."
            echo "  --deb      Build only a Debian package."
            echo "  --rpm      Build only an RPM package."
            exit 0
            ;;
        *)
            echo "[ERROR] Unknown option: $arg" >&2
            exit 1
            ;;
    esac
done

echo "========================================"
echo "   $APP_NAME - Linux Package Script"
echo "========================================"
echo ""
echo "Version: $PKG_VERSION"
echo ""

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

require_command() {
    if ! command_exists "$1"; then
        echo "[ERROR] Missing required command: $1"
        echo "        Install it and run this script again."
        exit 1
    fi
}

require_command go
require_command pkg-config
require_command gcc
if [ "$BUILD_DEB" = true ]; then
    require_command dpkg-deb
fi
if [ "$BUILD_RPM" = true ]; then
    require_command rpmbuild
fi

if ! command_exists wails; then
    echo "[INFO] Wails CLI not found. Installing..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
fi

if ! command_exists wails; then
    echo "[ERROR] Wails CLI is still unavailable after install attempt."
    exit 1
fi

webkit2_ok() {
    pkg-config --exists webkit2gtk-4.0 2>/dev/null && return 0
    pkg-config --exists webkit2gtk-4.1 2>/dev/null && return 0
    return 1
}

appindicator_ok() {
    pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null && return 0
    pkg-config --exists appindicator3-0.1 2>/dev/null && return 0
    return 1
}

if pkg-config --exists ayatana-appindicator3-0.1 2>/dev/null; then
    APPINDICATOR_DEB_DEP="libayatana-appindicator3-1"
elif pkg-config --exists appindicator3-0.1 2>/dev/null; then
    APPINDICATOR_DEB_DEP="libappindicator3-1"
else
    APPINDICATOR_DEB_DEP="libayatana-appindicator3-1"
fi

if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    WEBKIT_DEB_DEP="libwebkit2gtk-4.1-0"
else
    WEBKIT_DEB_DEP="libwebkit2gtk-4.0-37"
fi

echo "[1/5] Checking Linux build dependencies..."
if ! pkg-config --exists gtk+-3.0 || ! webkit2_ok || ! pkg-config --exists alsa || ! appindicator_ok; then
    echo "[ERROR] Missing one or more native build dependencies."
    echo ""
    echo "Ubuntu/Debian:"
    echo "  sudo apt update && sudo apt install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.0-dev libasound2-dev libayatana-appindicator3-dev dpkg-dev rpm"
    echo "  sudo apt install -y ydotool  (recommended for direct keyboard injection)"
    echo ""
    echo "Fedora:"
    echo "  sudo dnf install -y gcc gcc-c++ make pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel alsa-lib-devel libayatana-appindicator-gtk3-devel rpm-build"
    echo "  sudo dnf install -y ydotool  (recommended for direct keyboard injection)"
    echo ""
    echo "Arch Linux:"
    echo "  sudo pacman -S base-devel pkgconf gtk3 webkit2gtk alsa-lib libayatana-appindicator"
    echo "  sudo pacman -S ydotool  (recommended for direct keyboard injection)"
    echo ""
    exit 1
fi

echo "[2/5] Building Linux binary with Wails..."

if [ -d "frontend/node_modules/.bin" ]; then
    chmod +x frontend/node_modules/.bin/* 2>/dev/null || true
fi

WAILS_PKGCFG_SHIM=""
cleanup() {
    if [ -n "$WAILS_PKGCFG_SHIM" ] && [ -d "$WAILS_PKGCFG_SHIM" ]; then
        rm -rf "$WAILS_PKGCFG_SHIM"
    fi
}
trap cleanup EXIT

WAILS_WEBKIT_TAGS=()
if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    WAILS_WEBKIT_TAGS=(-tags webkit2_41)
fi

if ! pkg-config --exists webkit2gtk-4.0 2>/dev/null && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    WEBKIT41_PC=""
    WEBKIT41_PKGCONFIG_DIR="$(pkg-config --variable=pcfiledir webkit2gtk-4.1 2>/dev/null || true)"
    if [ -n "$WEBKIT41_PKGCONFIG_DIR" ] && [ -f "$WEBKIT41_PKGCONFIG_DIR/webkit2gtk-4.1.pc" ]; then
        WEBKIT41_PC="$WEBKIT41_PKGCONFIG_DIR/webkit2gtk-4.1.pc"
    fi
    for dir in \
        $(printf '%s' "${PKG_CONFIG_PATH:-}" | tr ':' '\n') \
        /usr/lib64/pkgconfig \
        /usr/lib/pkgconfig \
        /usr/local/lib64/pkgconfig \
        /usr/local/lib/pkgconfig; do
        [ -n "$WEBKIT41_PC" ] && break
        [ -z "$dir" ] && continue
        [ -f "$dir/webkit2gtk-4.1.pc" ] || continue
        WEBKIT41_PC="$dir/webkit2gtk-4.1.pc"
        break
    done
    if [ -n "$WEBKIT41_PC" ]; then
        WAILS_PKGCFG_SHIM="$(mktemp -d "${TMPDIR:-/tmp}/wails-pkgcfg-shim.XXXXXX")"
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

pkill -f "$EXECUTABLE" 2>/dev/null || true

echo "[3/5] Preparing package payload..."
rm -rf "$WORK_DIR"
mkdir -p "$WORK_DIR/root/usr/bin"
mkdir -p "$WORK_DIR/root/usr/share/applications"
mkdir -p "$WORK_DIR/root/usr/share/pixmaps"
mkdir -p "$WORK_DIR/root/usr/share/doc/$APP_NAME"
mkdir -p "$DIST_DIR"

install -m 0755 "$EXECUTABLE" "$WORK_DIR/root/usr/bin/$APP_NAME"

if [ -f "build/appicon.png" ]; then
    install -m 0644 "build/appicon.png" "$WORK_DIR/root/usr/share/pixmaps/$APP_NAME.png"
elif [ -f "frontend/src/assets/images/logo-universal.png" ]; then
    install -m 0644 "frontend/src/assets/images/logo-universal.png" "$WORK_DIR/root/usr/share/pixmaps/$APP_NAME.png"
else
    echo "[ERROR] Application icon was not found; refusing to build a package with a broken desktop entry."
    exit 1
fi

cat > "$WORK_DIR/root/usr/share/applications/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=$DISPLAY_NAME
Comment=$COMMENT
Exec=/usr/bin/$APP_NAME
TryExec=/usr/bin/$APP_NAME
Icon=$APP_NAME
StartupWMClass=$APP_NAME
Terminal=false
Categories=Utility;Audio;
EOF

if [ -f README.md ]; then
    install -m 0644 README.md "$WORK_DIR/root/usr/share/doc/$APP_NAME/README.md"
fi

if [ "$BUILD_DEB" = true ]; then
echo "[4/5] Building .deb package..."
DEB_ROOT="$WORK_DIR/deb"
rm -rf "$DEB_ROOT"
mkdir -p "$DEB_ROOT/DEBIAN"
cp -a "$WORK_DIR/root/." "$DEB_ROOT/"

INSTALLED_SIZE="$(du -sk "$DEB_ROOT/usr" | awk '{print $1}')"
cat > "$DEB_ROOT/DEBIAN/control" <<EOF
Package: $APP_NAME
Version: $PKG_VERSION
Section: utils
Priority: optional
Architecture: $ARCH_DEB
Maintainer: $MAINTAINER
Installed-Size: $INSTALLED_SIZE
Depends: libgtk-3-0, $WEBKIT_DEB_DEP, libasound2, $APPINDICATOR_DEB_DEP
Recommends: ydotool
Description: $COMMENT
 WIS Free V3 is a desktop voice dictation app built with Go and Wails.
 On Wayland, install ydotool for direct keyboard injection.
EOF

dpkg-deb --build "$DEB_ROOT" "$DIST_DIR/${APP_NAME}_${PKG_VERSION}-${PKG_RELEASE}_${ARCH_DEB}.deb"
fi

if [ "$BUILD_RPM" = true ]; then
echo "[5/5] Building .rpm package..."
RPM_TOP="$WORK_DIR/rpm"
RPM_PAYLOAD="$(pwd)/$WORK_DIR/root"
RPM_SPEC="$(pwd)/$WORK_DIR/$APP_NAME.spec"
rm -rf "$RPM_TOP"
mkdir -p "$RPM_TOP/BUILD" "$RPM_TOP/BUILDROOT" "$RPM_TOP/RPMS" "$RPM_TOP/SOURCES" "$RPM_TOP/SPECS" "$RPM_TOP/SRPMS"

cat > "$RPM_SPEC" <<EOF
Name:           $APP_NAME
Version:        $PKG_VERSION
Release:        $PKG_RELEASE%{?dist}
Summary:        $COMMENT
License:        $LICENSE
Requires:       gtk3
Requires:       alsa-lib
Recommends:     ydotool

%description
WIS Free V3 is a desktop voice dictation app built with Go and Wails.
On Wayland, install ydotool for direct keyboard injection.

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}
cp -a "$RPM_PAYLOAD"/. %{buildroot}/

%files
%attr(0755,root,root) /usr/bin/$APP_NAME
/usr/share/applications/$APP_NAME.desktop
/usr/share/pixmaps/$APP_NAME.png
/usr/share/doc/$APP_NAME/README.md
EOF

rpmbuild --target "$ARCH_RPM" --define "_topdir $(pwd)/$RPM_TOP" -bb "$RPM_SPEC"
find "$RPM_TOP/RPMS" -type f -name "*.rpm" -exec cp {} "$DIST_DIR/" \;
fi

echo ""
echo "Packages written to:"
find "$DIST_DIR" -maxdepth 1 -type f \( -name "*.deb" -o -name "*.rpm" \) -print | sort
