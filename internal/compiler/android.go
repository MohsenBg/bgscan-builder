package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetNDKPath resolves the root location of the Android NDK by inspecting the provided
// path parameter, falling back to the NDK_DIR environment variable if empty.
func GetNDKPath(ndkDir string) (string, error) {
	if ndkDir == "" {
		ndkDir = os.Getenv("NDK_DIR")
	}

	if ndkDir == "" {
		return "", fmt.Errorf("missing required Android NDK path; please pass it via CLI flag or export NDK_DIR")
	}

	if _, err := os.Stat(ndkDir); os.IsNotExist(err) {
		return "", fmt.Errorf("the resolved Android NDK path %q does not exist", ndkDir)
	}

	return ndkDir, nil
}

// GetAndroidCompilerPath resolves and validates the absolute path to the prebuilt
// LLVM Clang toolchain binary matching the specified host OS and target architecture matrix.
func GetAndroidCompilerPath(ndkDir string, arch string, apiLevel int) (string, error) {
	var hostTag string
	var ext string

	switch runtime.GOOS {
	case "windows":
		hostTag = "windows-x86_64"
		ext = ".cmd"
	case "darwin":
		hostTag = "darwin-x86_64"
		ext = ""
	case "linux":
		hostTag = "linux-x86_64"
		ext = ""
	default:
		return "", fmt.Errorf("unsupported host operating system: %s", runtime.GOOS)
	}

	var binaryName string
	switch arch {
	case "arm64":
		binaryName = fmt.Sprintf("aarch64-linux-android%d-clang%s", apiLevel, ext)
	case "arm", "arm32":
		binaryName = fmt.Sprintf("armv7a-linux-androideabi%d-clang%s", apiLevel, ext)
	case "386", "amd32":
		binaryName = fmt.Sprintf("i686-linux-android%d-clang%s", apiLevel, ext)
	case "amd64":
		binaryName = fmt.Sprintf("x86_64-linux-android%d-clang%s", apiLevel, ext)
	default:
		return "", fmt.Errorf("unsupported target architecture: %s", arch)
	}

	compilerPath := filepath.Join(ndkDir, "toolchains", "llvm", "prebuilt", hostTag, "bin", binaryName)

	if _, err := os.Stat(compilerPath); os.IsNotExist(err) {
		return "", fmt.Errorf("android compiler binary not found at path: %s", compilerPath)
	}

	return compilerPath, nil
}

