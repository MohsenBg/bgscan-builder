package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bgscan-builder/internal/platform"
)

const (
	ModeDev     = "setup-dev"
	ModeRelease = "release"
)

// Config aggregates the validated configuration state required to run
// the multi-architecture builder routines.
type Config struct {
	Mode       string
	Platforms  []platform.Info
	ProjectDir string
	DestDir    string
	NDKDir     string
	Version    string
	Verbose    bool
}

// ParseCLI evaluates incoming os.Args arguments to determine the execution
// context, delegating work to subcommand parsers.
func ParseCLI() (*Config, error) {
	if len(os.Args) < 2 {
		printUsage()
		return nil, fmt.Errorf("missing subcommand (%s | %s)", ModeDev, ModeRelease)
	}

	switch os.Args[1] {
	case "-v", "--version", "version":
		fmt.Printf("bgscan-builder %s\n", Version)
		os.Exit(0)
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	case ModeDev:
		return parseSetupDev()
	case ModeRelease:
		return parseRelease()
	default:
		printUsage()
		return nil, fmt.Errorf("unknown subcommand %q", os.Args[1])
	}

	return nil, nil // unreachable
}

// printUsage prints top-level usage information for the builder CLI.
func printUsage() {
	fmt.Fprint(os.Stderr, `bgscan-builder — multi-architecture build tool

Usage:
  bgscan-builder <subcommand> [flags]

Subcommands:
  setup-dev    Set up a local development build for the current platform
  release      Build a formal multi-platform release

Run 'bgscan-builder <subcommand> -h' for subcommand-specific flags.
`)
}

// parseSetupDev sets up configuration parameters for the local development profile.
func parseSetupDev() (*Config, error) {
	fs := flag.NewFlagSet(ModeDev, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: bgscan-builder setup-dev -project-dir <path>

Sets up a local development build for the currently detected platform.

Flags:
`)
		fs.PrintDefaults()
	}

	projectDir := fs.String("project-dir", "", "Path to the bgscan project")
	verbose := fs.Bool("verbose", false, "Enable debug logging")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	if *projectDir == "" {
		return nil, fmt.Errorf("project-dir is required")
	}

	cfg := &Config{
		Mode:       ModeDev,
		Platforms:  []platform.Info{platform.Detect()},
		ProjectDir: *projectDir,
		DestDir:    filepath.Join(*projectDir, "dist"),
		Verbose:    *verbose,
	}

	resolvePaths(cfg)
	return cfg, nil
}

// parseRelease handles command flag structures for generating formal
// multi-platform software distribution units.
func parseRelease() (*Config, error) {
	fs := flag.NewFlagSet(ModeRelease, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: bgscan-builder release -os <target> -arch <target> [flags]

Builds release artifacts for one or more OS/architecture combinations.

Examples:
  bgscan-builder release -os linux -arch amd64
  bgscan-builder release -os android -arch arm64 -ndk-dir /opt/android-ndk
  bgscan-builder release -os all -arch all -dest ./out

Flags:
`)
		fs.PrintDefaults()
	}

	targetOS := fs.String("os", "", "Target operating system (linux, windows, macos, android, all)")
	targetArch := fs.String("arch", "", "Target architecture (amd64, arm64, arm32, amd32, all)")
	destDir := fs.String("dest", "./dist", "Release output directory")
	projectDir := fs.String("project-dir", "", "Path to the bgscan project")
	ndkDir := fs.String("ndk-dir", "", "Android NDK root directory")
	version := fs.String("version", "dev", "Build version to embed in binary (e.g. v1.0.0)")
	verbose := fs.Bool("verbose", false, "Enable debug logging")

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
		Mode:       ModeRelease,
		Platforms:  resolvePlatforms(*targetOS, *targetArch),
		DestDir:    *destDir,
		NDKDir:     *ndkDir,
		Version:    *version,
		ProjectDir: *projectDir,
		Verbose:    *verbose,
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

// resolvePlatforms maps a user-supplied os/arch pair (or "all") to concrete
// platform targets.
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
		return platform.GetPlatformSpecificArch(platform.ParseOS(osName))

	default:
		return []platform.Info{
			{
				OS:   platform.ParseOS(osName),
				Arch: platform.ParseArch(archName),
			},
		}
	}
}

// requiresAndroidNDK scans requested platforms to see if an Android CGO
// toolchain lookup is required.
func requiresAndroidNDK(platforms []platform.Info) bool {
	for _, p := range platforms {
		if p.OS == platform.Android {
			return true
		}
	}
	return false
}

// resolvePaths ensures destination paths resolve to fully
// qualified absolute file-system directories.
func resolvePaths(cfg *Config) {
	cfg.DestDir = absPath(cfg.DestDir)
	cfg.ProjectDir = absPath(cfg.ProjectDir)
	cfg.NDKDir = absPath(cfg.NDKDir)
}

// absPath returns the absolute form of p, leaving it untouched when
// empty or unresolvable.
func absPath(p string) string {
	if p == "" {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
