# bgscan-builder

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)

`bgscan-builder` cross-compiles **bgscan** from a single Go binary. It builds
for multiple OS/architecture targets then stages the Slipstream sidecar
alongside each build.

Installing a release is handled by the separate
[bgscan-installer](https://github.com/MohsenBg/bgscan-installer) tool.

## Features

- **Release pipeline** — cross-compiles for multiple OS/architecture targets
  and fetches, verifies, and extracts the Slipstream sidecar per
  platform.
- **Android cross-compilation** — resolves the NDK's native LLVM toolchain and
  couples it through CGO.

## Requirements

- **Go 1.27** or newer.
- **Android NDK** — required only when `release` targets include `android`.

## Build

```bash
git clone https://github.com/MohsenBg/bgscan-builder.git
cd bgscan-builder
go build -o bgscan-builder ./cmd/builder
```

## Usage

Subcommands: `setup-dev`, `release`.

### setup-dev

Prepares a local development workspace for the current platform, copying
`*.default` templates into `ips` and downloading the Slipstream
sidecar.

```bash
bgscan-builder setup-dev -project-dir /path/to/bgscan
```

### release

Cross-compiles bgscan for one or more OS/architecture targets and stages the
Slipstream sidecar:

```bash
bgscan-builder release -os linux -arch amd64
bgscan-builder release -os android -arch arm64 -ndk-dir /opt/android-ndk
bgscan-builder release -os all -arch all -dest ./out
```

Supported values for `-os`: `linux`, `macos`, `windows`, `android`, `all`.
Supported values for `-arch`: `amd64`, `arm64`, `arm32`, `amd32`, `all`.

## Supporting flags

| Flag            | Applies to      | Description                            |
| --------------- | --------------- | -------------------------------------- |
| `-verbose`      | all subcommands | Enable debug logging                   |
| `-version`      | `release`       | Version embedded into the built binary |
| `-dest`         | `release`       | Release output directory (default `./dist`) |

Run `bgscan-builder <subcommand> -h` for the full flag list.
