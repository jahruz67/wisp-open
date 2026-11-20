#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Darwin || "$(uname -m)" != arm64 ]]; then
    echo "The macOS release engine must be built on Apple Silicon." >&2
    exit 1
fi

for tool in git cmake clang; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Missing Whisper build tool: $tool" >&2
        exit 1
    fi
done

SOURCE_DIR="build/whisper-src"
ENGINE_DIR="build/whisper-macos"
WHISPER_COMMIT="927cfce34f31707e17f2bff35c349632fb9e2c3a"
if [[ ! -d "$SOURCE_DIR/.git" ]]; then
    mkdir -p build
    git clone --depth 1 --branch v1.9.4 https://github.com/ggml-org/whisper.cpp.git "$SOURCE_DIR"
fi
if [[ "$(git -C "$SOURCE_DIR" rev-parse HEAD)" != "$WHISPER_COMMIT" ]]; then
    echo "Unexpected Whisper source revision in $SOURCE_DIR; expected $WHISPER_COMMIT." >&2
    exit 1
fi

# Embed Metal shaders and link the backends statically so the installed app
# needs neither the source tree nor Homebrew libraries. Target all Apple Silicon.
cmake -S "$SOURCE_DIR" -B "$ENGINE_DIR" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_OSX_ARCHITECTURES=arm64 \
    -DCMAKE_OSX_DEPLOYMENT_TARGET=13.0 \
    -DBUILD_SHARED_LIBS=OFF \
    -DGGML_BACKEND_DL=OFF \
    -DGGML_NATIVE=OFF \
    -DGGML_OPENMP=OFF \
    -DGGML_METAL=ON \
    -DGGML_METAL_EMBED_LIBRARY=ON \
    -DWHISPER_BUILD_IS_DEV=OFF \
    -DWHISPER_BUILD_TESTS=OFF \
    -DWHISPER_BUILD_SERVER=OFF \
    -DWHISPER_BUILD_EXAMPLES=ON
cmake --build "$ENGINE_DIR" --config Release --target whisper-cli --parallel
install -m 0644 "$SOURCE_DIR/LICENSE" "$ENGINE_DIR/whisper-LICENSE"
