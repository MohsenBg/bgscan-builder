package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bgscan-builder/internal/platform"
)

// Config aggregates the validated configuration states required to run
// the multi-architecture builder routines.
type Config struct {
	Mode        string
	Platforms   []platform.Info
	DestDir     string
	NDKDir      string
	DepVersion  string
	XrayVersion string
}

// ParseCLI evaluates incoming os.Args arguments to determine the execution
// context, delegating work to subcommand parsers.
func ParseCLI() (*Config, error) {
	if len(os.Args) < 2 {
		return nil, fmt.Errorf("missing subcommand (setup-dev | release)")
	}

	switch os.Args[1] {
	case "setup-dev":
		return parseSetupDev()
	case "release":
		return parseRelease()
	default:
		return nil, fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
}

// parseSetupDev sets up configuration parameters for the local development profile.
func parseSetupDev() (*Config, error) {
	cfg := &Config{
		Mode:        "setup-dev",
		Platforms:   []platform.Info{platform.Detect()},
		DestDir:     ".",
		DepVersion:  "v1.0",
		XrayVersion: "v26.3.27",
	}

	resolvePaths(cfg)
	return cfg, nil
}

// parseRelease handles command flag structures for generating formal multi-platform software distribution units.
func parseRelease() (*Config, error) {
	fs := flag.NewFlagSet("release", flag.ExitOnError)

	targetOS := fs.String(
		"os",
		"",
		"Target operating system (linux, windows, macos, android, all)",
	)
	targetArch := fs.String(
		"arch",
		"",
		"Target architecture (amd64, arm64, arm32, amd32, all)",
	)
	destDir := fs.String(
		"dest",
		"./dist",
		"Release output directory",
	)
	ndkDir := fs.String(
		"ndk-dir",
		"",
		"Android NDK root directory",
	)
	depVersion := fs.String(
		"dep-version",
		"v1.0",
		"Dependencies version tag",
	)
	xrayVersion := fs.String(
		"xray-version",
		"v26.3.27",
		"Xray version tag",
	)

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	if *targetOS == "" {
		return nil, fmt.Errorf("-os is required")
	}
	if *targetArch == "" {
		return nil, fmt.Errorf("-arch is required")
	}

	cfg := &Config{
		Mode:        "release",
		Platforms:   resolvePlatforms(*targetOS, *targetArch),
		DestDir:     *destDir,
		NDKDir:      *ndkDir,
		DepVersion:  *depVersion,
		XrayVersion: *xrayVersion,
	}

	if len(cfg.Platforms) == 0 {
		return nil, fmt.Errorf("no matching platform targets found")
	}

	if requiresAndroidNDK(cfg.Platforms) && cfg.NDKDir == "" {
		return nil, fmt.Errorf("-ndk-dir is required for Android builds")
	}

	resolvePaths(cfg)
	return cfg, nil
}

// resolvePlatforms maps string inputs down to formal, distinct architecture definitions.
func resolvePlatforms(osName, archName string) []platform.Info {
	allBuilds := platform.GetAllBuilds()

	switch {
	case osName == "all" && archName == "all":
		return allBuilds

	case osName == "all":
		arch := platform.ParseArch(archName)
		var builds []platform.Info
		for _, build := range allBuilds {
			if build.Arch == arch {
				builds = append(builds, build)
			}
		}
		return builds

	case archName == "all":
		return platform.GetPlatformSpecificArch(
			platform.ParseOS(osName),
		)

	default:
		return []platform.Info{
			{
				OS:   platform.ParseOS(osName),
				Arch: platform.ParseArch(archName),
			},
		}
	}
}

// requiresAndroidNDK scans requested parameters to see if an Android CGO chain lookup sequence is requested.
func requiresAndroidNDK(platforms []platform.Info) bool {
	for _, p := range platforms {
		if p.OS == platform.Android {
			return true
		}
	}
	return false
}

// resolvePaths ensures internal destination paths parse correctly into fully qualified absolute file-system directories.
func resolvePaths(cfg *Config) {
	if abs, err := filepath.Abs(cfg.DestDir); err == nil {
		cfg.DestDir = abs
	}

	if cfg.NDKDir != "" {
		if abs, err := filepath.Abs(cfg.NDKDir); err == nil {
			cfg.NDKDir = abs
		}
	}
}

