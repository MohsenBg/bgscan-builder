#!/usr/bin/env bash
# ==============================================================================
#  bgscan-builder — cross-compile script (CI/CD ONLY)
# ------------------------------------------------------------------------------
#  Compiles bgscan-builder for every supported platform/architecture and
#  writes the resulting binaries into ./dist/.
#
#  ⚠️  Designed for GitHub Actions. Not intended for manual use.
#
#  Usage:
#    build.sh <target> [version]
#
#  Targets:
#    linux    → linux-64, linux-32, linux-arm64, linux-arm32-v7a
#    macos    → macos-64, macos-arm64
#    windows  → windows-64.exe, windows-arm64.exe
#    android  → android-arm64-v8a, android-armeabi-v7a, android-x86_64, android-x86
#    all      → all of the above (sequential)
#
#  Output:
#    dist/bgscan-builder-<platform>-<arch>[.exe]
# ==============================================================================
set -euo pipefail

# ------------------------------------------------------------------------------
# Arguments
# ------------------------------------------------------------------------------
TARGET="${1:-}"
VERSION="${2:-dev}"

if [[ -z "$TARGET" ]]; then
    echo "Usage: $0 {linux|macos|windows|android|all} [version]" >&2
    exit 1
fi

# ------------------------------------------------------------------------------
# Paths
# ------------------------------------------------------------------------------
ROOT_DIR="$PWD"
DIST_DIR="$ROOT_DIR/dist"
MAIN_PKG="./cmd/builder"

mkdir -p "$DIST_DIR"

# ------------------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------------------
log() {
    echo
    echo "======================================"
    echo "$*"
    echo "======================================"
}

# ------------------------------------------------------------------------------
# Standard CGO-free build
# ------------------------------------------------------------------------------
build_go() {
    local goos="$1"
    local goarch="$2"
    local suffix="$3"
    local output="$DIST_DIR/bgscan-builder-${suffix}"

    log "BUILD => ${goos}/${goarch} -> bgscan-builder-${suffix}"

    GOOS="$goos" \
    GOARCH="$goarch" \
    CGO_ENABLED=0 \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "$output" \
        "$MAIN_PKG"
}

# ------------------------------------------------------------------------------
# Android CGO build (requires NDK on PATH)
# ------------------------------------------------------------------------------
build_android() {
    local goarch="$1"   # arm64 | arm | amd64 | 386
    local suffix="$2"   # android-arm64-v8a | android-armeabi-v7a | …
    local output="$DIST_DIR/bgscan-builder-${suffix}"

    local cc
    case "$goarch" in
        arm64) cc="aarch64-linux-android21-clang"      ;;
        arm)   cc="armv7a-linux-androideabi21-clang"   ;;
        386)   cc="i686-linux-android21-clang"         ;;
        amd64) cc="x86_64-linux-android21-clang"       ;;
        *)
            echo "error: unsupported Android arch: ${goarch}" >&2
            exit 1
            ;;
    esac

    log "BUILD => android/${goarch} -> bgscan-builder-${suffix}"

    GOOS=android \
    GOARCH="$goarch" \
    CGO_ENABLED=1 \
    CC="$cc" \
    go build \
        -trimpath \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "$output" \
        "$MAIN_PKG"
}

# ------------------------------------------------------------------------------
# Android NDK setup (CI only — downloads NDK r27d)
# ------------------------------------------------------------------------------
setup_android_ndk() {
    local ndk_version="r27d"
    local ndk_dir="$ROOT_DIR/android-ndk-${ndk_version}"
    local toolchain="$ndk_dir/toolchains/llvm/prebuilt/linux-x86_64"

    log "ANDROID: setting up NDK ${ndk_version}"

    sudo apt-get update -y -qq
    sudo apt-get install -y -qq wget unzip build-essential

    if [[ ! -d "$ndk_dir" ]]; then
        local zip="$ROOT_DIR/ndk.zip"
        wget -q \
            "https://dl.google.com/android/repository/android-ndk-${ndk_version}-linux.zip" \
            -O "$zip"
        unzip -q "$zip" -d "$ROOT_DIR"
        rm -f "$zip"
    fi

    export PATH="${toolchain}/bin:$PATH"
}

# ------------------------------------------------------------------------------
# Build router
# ------------------------------------------------------------------------------
case "$TARGET" in

    linux)
        log "TARGET: LINUX"
        build_go linux amd64 linux-64
        build_go linux 386   linux-32
        build_go linux arm64 linux-arm64
        build_go linux arm   linux-arm32-v7a
        ;;

    macos)
        log "TARGET: MACOS"
        build_go darwin amd64 macos-64
        build_go darwin arm64 macos-arm64
        ;;

    windows)
        log "TARGET: WINDOWS"
        build_go windows amd64 windows-64.exe
        build_go windows arm64 windows-arm64.exe
        ;;

    android)
        log "TARGET: ANDROID"
        setup_android_ndk
        build_android arm64 android-arm64-v8a
        build_android arm   android-armeabi-v7a
        build_android amd64 android-x86_64
        build_android 386   android-x86
        ;;

    all)
        log "TARGET: ALL"
        bash "$0" linux   "$VERSION"
        bash "$0" macos   "$VERSION"
        bash "$0" windows "$VERSION"
        bash "$0" android "$VERSION"
        ;;

    *)
        echo "Usage: $0 {linux|macos|windows|android|all} [version]" >&2
        exit 1
        ;;
esac

# ------------------------------------------------------------------------------
log "BUILD COMPLETE"
echo "Artifacts in ${DIST_DIR}:"
ls -lh "$DIST_DIR"
