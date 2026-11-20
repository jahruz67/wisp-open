#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != Linux || "$(uname -m)" != x86_64 ]]; then
    echo "The prebuilt Linux engine requires Linux x86_64." >&2
    exit 1
fi

for tool in curl unzip sha256sum; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "Missing $tool. Install the Linux build dependencies in docs/linux-installation.md." >&2
        exit 1
    fi
done

# Pin both the repository revision and archive contents. Models are installed
# separately from Settings; this download contains only the engine and license.
ENGINE_URL="https://raw.githubusercontent.com/jahruz67/wisp-open/cd468ed0bf13ae7108b4639c16656ceae9dfe5da/assets/linuz_bin_cpp/whisper-linux-amd64-vulkan-v1.9.4.zip"
ENGINE_SHA256="41f654fa772873a1e1a16b115ad15f8df7f598544f222b1298d16b55cd5d9c88"
ENGINE_STAGE="$(mktemp -d "${TMPDIR:-/tmp}/wis-whisper.XXXXXX")"
trap 'rm -rf -- "$ENGINE_STAGE"' EXIT

echo "Downloading prebuilt Linux Whisper v1.9.4 (CPU/Vulkan)..."
curl --fail --location --retry 3 --connect-timeout 20 \
    --speed-limit 1 --speed-time 60 \
    --output "$ENGINE_STAGE/engine.zip" "$ENGINE_URL"
if ! printf '%s  %s\n' "$ENGINE_SHA256" "$ENGINE_STAGE/engine.zip" | sha256sum --check --status; then
    echo "Linux Whisper engine checksum mismatch; refusing to install it." >&2
    exit 1
fi

# Extract only the expected files without trusting archive paths or permissions.
unzip -p "$ENGINE_STAGE/engine.zip" whisper-cli > "$ENGINE_STAGE/whisper-cli"
unzip -p "$ENGINE_STAGE/engine.zip" whisper-LICENSE > "$ENGINE_STAGE/whisper-LICENSE"
if [[ ! -s "$ENGINE_STAGE/whisper-cli" || ! -s "$ENGINE_STAGE/whisper-LICENSE" ]]; then
    echo "Linux Whisper archive is missing its engine or license." >&2
    exit 1
fi
mkdir -p build/bin
install -m 0755 "$ENGINE_STAGE/whisper-cli" build/bin/whisper-cli
install -m 0644 "$ENGINE_STAGE/whisper-LICENSE" build/bin/whisper-LICENSE
