// Package platform handles system target runtime diagnostics, architecture enums,
// token normalizations, and matrix cross-compilation configurations.
package platform

import (
	"os"
	"runtime"
	"strings"
)

// Detect discovers and returns the host platform context info in a unified call.
func Detect() Info {
	return Info{
		OS:   detectOS(),
		Arch: detectArch(),
	}
}

func detectOS() OS {
	switch runtime.GOOS {
	case "linux":
		if isAndroid() {
			return Android
		}
		return Linux
	case "darwin":
		return MacOS
	case "windows":
		return Windows
	default:
		return UnknownOS
	}
}

func detectArch() Arch {
	switch runtime.GOARCH {
	case "arm64":
		return ARM64
	case "arm":
		return ARM32
	case "amd64":
		return AMD64
	case "386":
		return AMD32
	default:
		return UnknownArch
	}
}

func isAndroid() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	if os.Getenv("TERMUX_VERSION") != "" {
		return true
	}

	if os.Getenv("ANDROID_ROOT") != "" {
		return true
	}

	paths := []string{
		"/system/build.prop",
		"/system/bin/app_process",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}

	return false
}

// ParseArch maps user-provided string labels down to standard internal Arch enums.
func ParseArch(s string) Arch {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "arm64", "aarch64", "armv8":
		return ARM64
	case "armv7", "arm32", "arm":
		return ARM32
	case "x86_64", "amd64", "64":
		return AMD64
	case "x86", "i386", "386", "32", "amd32":
		return AMD32
	default:
		return UnknownArch
	}
}

// ParseOS maps user-provided string labels down to standard internal OS enums.
func ParseOS(s string) OS {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "linux":
		return Linux
	case "android":
		return Android
	case "macos", "darwin":
		return MacOS
	case "windows":
		return Windows
	default:
		return UnknownOS
	}
}

// Tokens generates specific variant naming markers required for match checks during asset resolution.
func (a Arch) Tokens() []string {
	switch a {
	case ARM64:
		return []string{"arm64", "aarch64", "armv8"}
	case ARM32:
		return []string{"armv7", "arm32"}
	case AMD64:
		return []string{"x86_64", "amd64", "64"}
	case AMD32:
		return []string{"x86", "i386", "386", "32"}
	default:
		return []string{}
	}
}

// GetAllBuilds provides a comprehensive list of all verified targets supported by the orchestration engine.
func GetAllBuilds() []Info {
	return []Info{
		{OS: Android, Arch: ARM64},
		{OS: Android, Arch: ARM32},
		{OS: Android, Arch: AMD64},
		{OS: Android, Arch: AMD32},
		{OS: Linux, Arch: ARM64},
		{OS: Linux, Arch: ARM32},
		{OS: Linux, Arch: AMD64},
		{OS: Linux, Arch: AMD32},
		{OS: MacOS, Arch: ARM64},
		{OS: MacOS, Arch: AMD64},
		{OS: Windows, Arch: AMD64},
		{OS: Windows, Arch: ARM64},
	}
}

// GetPlatformSpecificArch queries all support paths to filter out target builds unique to a specific OS.
func GetPlatformSpecificArch(targetOS OS) []Info {
	all := GetAllBuilds()
	var filtered []Info
	for _, build := range all {
		if build.OS == targetOS {
			filtered = append(filtered, build)
		}
	}
	return filtered
}

// GetAllArchForEveryOS aggregates and returns the internal compilation layout profile mapped directly by operating system keys.
func GetAllArchForEveryOS() map[OS][]Arch {
	m := make(map[OS][]Arch)
	all := GetAllBuilds()
	for _, build := range all {
		m[build.OS] = append(m[build.OS], build.Arch)
	}
	return m
}

