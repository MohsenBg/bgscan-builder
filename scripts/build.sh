#!/usr/bin/env bash

# ==============================================================================
# bgscan-builder release script
# ------------------------------------------------------------------------------
# This script is part of the bgscan-builder CI/CD pipeline.
#
# PURPOSE:
#   - Cross-compile bgscan-builder for multiple OS/ARCH targets
#   - Produce CI-ready binaries for release distribution
#
# IMPORTANT:
#   - This script is NOT intended for end users
#   - It is designed to be executed ONLY in GitHub Actions (CI environment)
#   - Running it manually is not recommended unless you know what you are doing
#
# OUTPUT:
#   dist/
#     bgscan-linux-*
#     bgscan-windows-*
#     bgscan-android-*
#
# REQUIREMENTS:
#   - Go toolchain installed
#   - Internet access (for Android NDK download in CI)
#   - Linux-based CI environment (GitHub Actions recommended)
# ==============================================================================

set -euo pipefail

TARGET="${1:-linux}"

ROOT_DIR="$PWD"
DIST_DIR="$ROOT_DIR/dist"

log() {
  echo
  echo "======================================"
  echo "$*"
  echo "======================================"
}

run() {
  echo "+ $*"
  "$@"
}

mkdir -p "$DIST_DIR"

# ==============================================================================
# GO BUILD FUNCTION
# ==============================================================================
build_go() {
  local goos="$1"
  local goarch="$2"
  local name="$3"
  local cgo="${4:-0}"

  log "BUILD => $goos/$goarch -> bgscan-$name"

  export GOOS="$goos"
  export GOARCH="$goarch"
  export CGO_ENABLED="$cgo"

  go build -trimpath -ldflags="-s -w" \
    -o "$DIST_DIR/bgscan-$name" \
    ./cmd/builder
}

# ==============================================================================
# ANDROID NDK SETUP (CI ONLY)
# ==============================================================================
setup_android_ndk() {
  API=21
  NDK_VERSION="r27d"
  NDK_DIR="$ROOT_DIR/android-ndk-$NDK_VERSION"

  log "ANDROID: Preparing NDK environment"

  sudo apt-get update -y >/dev/null
  sudo apt-get install -y git wget unzip curl build-essential >/dev/null

  if [ ! -d "$NDK_DIR" ]; then
    log "Downloading Android NDK $NDK_VERSION"

    wget --progress=bar:force:noscroll \
      "https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip" \
      -O "$ROOT_DIR/ndk.zip"

    unzip -q "$ROOT_DIR/ndk.zip" -d "$ROOT_DIR"
    rm -f "$ROOT_DIR/ndk.zip"
  fi

  export TOOLCHAIN_BIN="$NDK_DIR/toolchains/llvm/prebuilt/linux-x86_64/bin"
  export PATH="$TOOLCHAIN_BIN:$PATH"
}

# ==============================================================================
# BUILD ROUTER
# ==============================================================================
case "$TARGET" in

linux)
  log "TARGET: LINUX"

  build_go linux amd64 linux-64
  build_go linux 386 linux-32
  build_go linux arm64 linux-arm64
  build_go linux arm linux-arm32-v7a
  ;;

windows)
  log "TARGET: WINDOWS"

  build_go windows amd64 windows-64.exe
  build_go windows arm64 windows-arm64.exe
  ;;

android)
  log "TARGET: ANDROID"

  setup_android_ndk

  build_go android arm64 android-arm64-v8a 1
  build_go android arm android-armeabi-v7a 1
  build_go android amd64 android-x86_64 1
  build_go android 386 android-x86 1
  ;;

all)
  log "TARGET: ALL"

  bash "$0" linux
  bash "$0" windows
  bash "$0" android
  ;;

*)
  echo "Usage: $0 {linux|windows|android|all}"
  exit 1
  ;;
esac

# ==============================================================================
log "BUILD COMPLETE"
echo "Artifacts:"
ls -lh "$DIST_DIR"
