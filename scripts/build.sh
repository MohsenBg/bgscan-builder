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
#     bgscan-builder-linux-*
#     bgscan-builder-windows-*
#     bgscan-builder-android-*
#     bgscan-builder-macos-*
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
  local name="bgscan-builder-$3"
  local cgo="${4:-0}"

  log "BUILD => $goos/$goarch -> $name"

  export GOOS="$goos"
  export GOARCH="$goarch"
  export CGO_ENABLED="$cgo"

  go build -trimpath -ldflags="-s -w" \
    -o "$DIST_DIR/$name" \
    ./cmd/builder
}

build_android() {
  local arch="$1"
  local triple="$2"
  local name="bgscan-builder-$3"

  export GOOS=android
  export GOARCH="$arch"
  export CGO_ENABLED=1

  case "$arch" in
  arm64)
    export CC=aarch64-linux-android21-clang
    ;;
  arm)
    export CC=armv7a-linux-androideabi21-clang
    ;;
  386)
    export CC=i686-linux-android21-clang
    ;;
  amd64)
    export CC=x86_64-linux-android21-clang
    ;;
  esac

  log "BUILD => android/$arch -> $name"

  go build -trimpath -ldflags="-s -w" \
    -o "$DIST_DIR/$name" \
    ./cmd/builder
}

# ==============================================================================
# ANDROID NDK SETUP (CI ONLY)
# ==============================================================================
setup_android_ndk() {
  set -e

  API=21
  NDK_VERSION="r27d"
  NDK_DIR="$ROOT_DIR/android-ndk-$NDK_VERSION"

  log "ANDROID: setting up NDK"

  sudo apt-get update -y >/dev/null
  sudo apt-get install -y wget unzip curl build-essential >/dev/null

  if [ ! -d "$NDK_DIR" ]; then
    wget -q \
      "https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip" \
      -O "$ROOT_DIR/ndk.zip"

    unzip -q "$ROOT_DIR/ndk.zip" -d "$ROOT_DIR"
    rm -f "$ROOT_DIR/ndk.zip"
  fi

  export NDK="$NDK_DIR"
  export TOOLCHAIN="$NDK/toolchains/llvm/prebuilt/linux-x86_64"

  export PATH="$TOOLCHAIN/bin:$PATH"
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

  build_android arm64 arm64 android-arm64-v8a
  build_android arm arm android-armeabi-v7a
  build_android amd64 amd64 android-x86_64
  build_android 386 386 android-x86
  ;;

all)
  log "TARGET: ALL"

  bash "$0" linux
  bash "$0" macos
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
