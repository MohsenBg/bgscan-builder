package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"bgscan-builder/internal/platform"
)

const (
	ModeInstall = "install"
	ModeUpdate  = "update"
	ModeDev     = "setup-dev"
	ModeRelease = "release"

	defaultInstallDir  = "./bgscan"
	defaultXrayVersion = "v26.7.28"
)

// Config aggregates the validated configuration state required to run
// the multi-architecture builder routines.
type Config struct {
	Mode        string
	Platforms   []platform.Info
	ProjectDir  string
	DestDir     string
	InstallDir  string
	NDKDir      string
	Version     string
	XrayVersion string
	Verbose     bool
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
	case ModeInstall:
		return parseInstall()
	case ModeUpdate:
		return parseUpdate()
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
  install      Install the latest bgscan release into ./bgscan
  update       Update bgscan in place, keeping your ips/assets files
  setup-dev    Set up a local development build for the current platform
  release      Build a formal multi-platform release

Run 'bgscan-builder <subcommand> -h' for subcommand-specific flags.
`)
}

// installFlags parses the shared install/update flag surface.
func installFlags(mode, usage string) (*Config, error) {
	fs := flag.NewFlagSet(mode, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.PrintDefaults()
	}

	version := fs.String("version", "latest", "Version to install (e.g. v2.10.0; 'latest' resolves the newest release)")
	installDir := fs.String("dir", defaultInstallDir, "Installation directory")
	verbose := fs.Bool("verbose", false, "Enable debug logging")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	cfg := &Config{
		Mode:       mode,
		InstallDir: *installDir,
		Version:    *version,
		Verbose:    *verbose,
	}
	resolvePaths(cfg)
	return cfg, nil
}

// parseInstall handles command flag structures for installing the latest
// (or a pinned) bgscan release into the target directory.
func parseInstall() (*Config, error) {
	return installFlags(ModeInstall, `Usage: bgscan-builder install [flags]

Installs the latest (or a specific version of) bgscan into ./bgscan.

Examples:
  bgscan-builder install
  bgscan-builder install --version v2.10.0
  bgscan-builder install --version latest --dir ./bgscan

Flags:
`)
}

// parseUpdate handles command flag structures for refreshing an existing
// bgscan installation in place without removing user-added ips/assets files.
func parseUpdate() (*Config, error) {
	return installFlags(ModeUpdate, `Usage: bgscan-builder update [flags]

Refreshes bgscan in place: the binary and release-provided ips/assets are
updated, while files you added to ips/assets are preserved.

Examples:
  bgscan-builder update
  bgscan-builder update --version v2.10.0 --dir ./bgscan

Flags:
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
		Mode:        ModeDev,
		Platforms:   []platform.Info{platform.Detect()},
		ProjectDir:  *projectDir,
		DestDir:     filepath.Join(*projectDir, "dist"),
		XrayVersion: defaultXrayVersion,
		Verbose:     *verbose,
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
	xrayVersion := fs.String("xray-version", defaultXrayVersion, "Xray version tag")
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
		Mode:        ModeRelease,
		Platforms:   resolvePlatforms(*targetOS, *targetArch),
		DestDir:     *destDir,
		NDKDir:      *ndkDir,
		Version:     *version,
		XrayVersion: *xrayVersion,
		ProjectDir:  *projectDir,
		Verbose:     *verbose,
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

// resolvePaths ensures internal destination paths resolve to fully
// qualified absolute file-system directories.
func resolvePaths(cfg *Config) {
	if abs, err := filepath.Abs(cfg.DestDir); err == nil {
		cfg.DestDir = abs
	}

	if cfg.InstallDir != "" {
		if abs, err := filepath.Abs(cfg.InstallDir); err == nil {
			cfg.InstallDir = abs
		}
	}

	if cfg.NDKDir != "" {
		if abs, err := filepath.Abs(cfg.NDKDir); err == nil {
			cfg.NDKDir = abs
		}
	}
}
