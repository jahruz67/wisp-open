#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
    echo "The Linux release engine must be built on Linux x86_64." >&2
    exit 1
fi

for tool in git cmake c++ glslc; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Missing $tool. Install the Whisper build dependencies in docs/linux-installation.md." >&2
        exit 1
    fi
done

# Match the pinned macOS engine. Models are downloaded only from Settings.
SOURCE_DIR="build/whisper-src"
ENGINE_DIR="build/whisper-linux"
WHISPER_COMMIT="927cfce34f31707e17f2bff35c349632fb9e2c3a"
if [[ ! -d "$SOURCE_DIR/.git" ]]; then
    mkdir -p build
    git clone --depth 1 --branch v1.9.4 https://github.com/ggml-org/whisper.cpp.git "$SOURCE_DIR"
fi
if [[ "$(git -C "$SOURCE_DIR" rev-parse HEAD)" != "$WHISPER_COMMIT" ]]; then
    echo "Unexpected Whisper source revision in $SOURCE_DIR; expected $WHISPER_COMMIT." >&2
    exit 1
fi

# Link Whisper/GGML and the C++ runtime into the CLI. Only system libraries
# (including the Vulkan loader/driver) are needed on the destination machine.
# Avoid host-specific instructions in binaries distributed to other machines.
cmake -S "$SOURCE_DIR" -B "$ENGINE_DIR" \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_EXE_LINKER_FLAGS="-static-libstdc++ -static-libgcc" \
    -DBUILD_SHARED_LIBS=OFF \
    -DGGML_BACKEND_DL=OFF \
    -DGGML_VULKAN=ON \
    -DGGML_CPU=ON \
    -DGGML_NATIVE=OFF \
    -DGGML_SSE42=OFF \
    -DGGML_AVX=OFF \
    -DGGML_AVX2=OFF \
    -DGGML_BMI2=OFF \
    -DGGML_FMA=OFF \
    -DGGML_F16C=OFF \
    -DGGML_OPENMP=OFF \
    -DWHISPER_BUILD_IS_DEV=OFF \
    -DWHISPER_BUILD_TESTS=OFF \
    -DWHISPER_BUILD_SERVER=OFF \
    -DWHISPER_BUILD_EXAMPLES=ON
cmake --build "$ENGINE_DIR" --config Release --target whisper-cli --parallel

mkdir -p build/bin
install -m 0755 "$ENGINE_DIR/bin/whisper-cli" build/bin/whisper-cli
install -m 0644 "$SOURCE_DIR/LICENSE" build/bin/whisper-LICENSE
