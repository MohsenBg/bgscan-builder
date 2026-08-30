# bgscan-builder

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)](LICENSE)

`bgscan-builder` installs, updates, and cross-compiles **bgscan** from a single
Go binary. It detects the host platform, resolves the matching GitHub release
asset, verifies its SHA-256 checksum, and downloads it with live progress —
then stages the sidecar dependencies (Xray Core and Slipstream) alongside the
build.

## Features

- **Native installer** — installs `bgscan` on Linux, macOS, and Termux/Android
  with checksum verification and no shelling out to `curl`/`unzip`.
- **Safe updates** — refreshes the binary and release-provided `ips`/`assets`
  while preserving files you added; timestamped backups never overwrite an
  existing backup.
- **Release pipeline** — cross-compiles for multiple OS/architecture targets
  and fetches, verifies, and extracts the Xray + Slipstream sidecars per
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

Subcommands: `install`, `update`, `setup-dev`, `release`.

```text
bgscan-builder install [--version latest|v2.10.0] [--dir ./bgscan]
bgscan-builder update  [--version latest|v2.10.0] [--dir ./bgscan]
```

### install

Detects the host platform, resolves the latest release, verifies the checksum,
and installs `bgscan` into `./bgscan` by default. When an installation already
exists you choose how to proceed:

```text
[1]  Update installation (keeps your ips/assets/settings)
[2]  Clean install (removes existing installation)
[3]  Back up existing installation and install new version
[4]  Cancel
```

Choosing **3** moves the previous installation to a timestamped backup such as
`bgscan_bck_20260830_083015` and never deletes an existing backup; collisions
get a numeric suffix.

### update

Refreshes an existing installation in place: the binary and release-provided
`ips`/`assets` are replaced while user-added files are preserved. If no
installation exists, a fresh copy is installed.

An empty or `latest` version resolves the GitHub latest release; any other
value resolves the matching tagged release.

### setup-dev

Prepares a local development workspace for the current platform, copying
`*.default` templates into `ips` and downloading the Xray + Slipstream
sidecars.

```bash
bgscan-builder setup-dev -project-dir /path/to/bgscan
```

### release

Cross-compiles bgscan for one or more OS/architecture targets and stages the
sidecar dependencies:

```bash
bgscan-builder release -os linux -arch amd64
bgscan-builder release -os android -arch arm64 -ndk-dir /opt/android-ndk
bgscan-builder release -os all -arch all -dest ./out
```

Supported values for `-os`: `linux`, `macos`, `windows`, `android`, `all`.
Supported values for `-arch`: `amd64`, `arm64`, `arm32`, `amd32`, `all`.

## Supporting flags

| Flag            | Applies to             | Description                                 |
| --------------- | ---------------------- | ------------------------------------------- |
| `-verbose`      | all subcommands        | Enable debug logging                        |
| `-dir`          | `install`, `update`    | Installation directory (default `./bgscan`) |
| `-version`      | `install`, `update`    | Release to install (default `latest`)       |
| `-version`      | `release`              | Version embedded into the built binary      |
| `-xray-version` | `release`              | Xray release tag                            |
| `-dest`         | `release`              | Release output directory (default `./dist`) |

Run `bgscan-builder <subcommand> -h` for the full flag list.