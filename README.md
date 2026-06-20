# bgscan-builder

A lightweight cross-compilation pipeline tool written in Go. It automates fetching sidecar dependencies (like Xray Core, DNSTT, and Slipstream), preparing asset workspaces, and targeting multiple OS and architecture platforms.

## Features

* **Subcommand CLI Profile:** Seamless switching between local rapid development workspaces and production deployment targets.
* **Automated Asset Resolution:** Fetches, decompresses, and validates sidecar dependency binary hashes from remote distribution channels.
* **Android Cross-Compilation Integration:** Automatically checks host dependencies and couples native LLVM toolchains using CGO targets.

## Quick Start

### Prerequisites

* **Go Compiler:** Version `1.26.3` or higher is required.
* **Android NDK:** Required only when compiling distribution layers targeting the `android` platform.

### Installation

Clone the orchestrator tool suite into your system workspace environment:

```bash
git clone [https://github.com/MohsenBg/bgscan-builder.git](https://github.com/MohsenBg/bgscan-builder.git)
cd bgscan-builder
