#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ "$(uname -s)" != "Darwin" || "$(uname -m)" != "arm64" ]]; then
    echo "The macOS release must be built on an Apple Silicon Mac." >&2
    exit 1
fi

APP_VERSION="$(tr -d '[:space:]' < scripts/VERSION)"
APP_BUNDLE="build/bin/wis-free-v3.app"
ENGINE_SOURCE="${WIS_WHISPER_CLI:-build/whisper-macos/bin/whisper-cli}"
ENGINE_DEST="$APP_BUNDLE/Contents/Resources/bin/whisper-cli"
DIST_DIR="dist/macos"
STAGE_DIR="build/package-macos"
DMG_PATH="$DIST_DIR/wis-free-v3_${APP_VERSION}_macos-arm64-preview.dmg"

if [[ ! -x "$ENGINE_SOURCE" ]]; then
    echo "Missing Apple Silicon whisper-cli at $ENGINE_SOURCE" >&2
    echo "Build the pinned whisper.cpp engine first or set WIS_WHISPER_CLI." >&2
    exit 1
fi

export CGO_CFLAGS="${CGO_CFLAGS:-} -mmacosx-version-min=13.0"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -mmacosx-version-min=13.0"
wails build -platform darwin/arm64 -clean -skipbindings -ldflags "-X main.AppVersion=$APP_VERSION"

mkdir -p "$(dirname "$ENGINE_DEST")" "$DIST_DIR"
install -m 0755 "$ENGINE_SOURCE" "$ENGINE_DEST"

# Ad-hoc signing keeps the preview internally consistent on Apple Silicon. It
# is intentionally not presented as Developer ID signing or notarization.
codesign --force --sign - --options runtime --timestamp=none "$ENGINE_DEST"
codesign --force --sign - --options runtime --timestamp=none \
    --entitlements build/darwin/entitlements.plist "$APP_BUNDLE"
codesign --verify --deep --strict --verbose=2 "$APP_BUNDLE"

rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -R "$APP_BUNDLE" "$STAGE_DIR/WIS Free V3.app"
ln -s /Applications "$STAGE_DIR/Applications"
rm -f "$DMG_PATH"
hdiutil create -volname "WIS Free V3 Preview" -srcfolder "$STAGE_DIR" \
    -ov -format UDZO "$DMG_PATH"

echo "Created $DMG_PATH"
