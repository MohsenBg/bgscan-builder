package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"bgscan-builder/internal/compiler"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

// Version is set at build time via -ldflags "-X main.Version=vx.x.x"
var Version = "dev"

var platformName = map[platform.Info]string{
	{OS: platform.Linux, Arch: platform.ARM64}:   "bgscan-linux-arm64",
	{OS: platform.Linux, Arch: platform.ARM32}:   "bgscan-linux-arm32-v7a",
	{OS: platform.Linux, Arch: platform.AMD64}:   "bgscan-linux-64",
	{OS: platform.Linux, Arch: platform.AMD32}:   "bgscan-linux-32",
	{OS: platform.Android, Arch: platform.ARM64}: "bgscan-android-arm64-v8a",
	{OS: platform.Android, Arch: platform.ARM32}: "bgscan-android-armeabi-v7a",
	{OS: platform.Android, Arch: platform.AMD64}: "bgscan-android-x86_64",
	{OS: platform.Android, Arch: platform.AMD32}: "bgscan-android-x86",
	{OS: platform.MacOS, Arch: platform.ARM64}:   "bgscan-macos-arm64",
	{OS: platform.MacOS, Arch: platform.AMD64}:   "bgscan-macos-64",
	{OS: platform.Windows, Arch: platform.AMD64}: "bgscan-windows-64",
	{OS: platform.Windows, Arch: platform.ARM64}: "bgscan-windows-arm64",
}

func main() {
	u := ui.New(os.Stderr)

	cfg, err := ParseCLI()
	if err != nil {
		u.Fail(err.Error())
		os.Exit(1)
	}

	if cfg.Verbose {
		u.SetLevel(slog.LevelDebug)
	}

	mode := "multi-platform release pipeline"
	switch cfg.Mode {
	case ModeDev:
		mode = "local development setup"
	case ModeInstall:
		mode = "installer"
	case ModeUpdate:
		mode = "updater"
	}
	u.Brand(Version, mode)

	// Ctrl+C cancels the running operation, allowing temporary files to be
	// cleaned up before exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var runErr error
	switch cfg.Mode {
	case ModeRelease:
		runErr = BuildAllPlatforms(ctx, u, *cfg)
	case ModeDev:
		runErr = RunSetupDev(ctx, u, *cfg)
	case ModeInstall:
		runErr = Install(ctx, u, *cfg)
	case ModeUpdate:
		runErr = Update(ctx, u, *cfg)
	}

	if runErr != nil {
		if cfg.Mode != ModeInstall && cfg.Mode != ModeUpdate {
			u.Fail(runErr.Error())
		}
		os.Exit(1)
	}

	// Flush the progress container after all output has been written.
}

// BuildAllPlatforms executes cross-compilation and downloads core dependencies
// for all targeted architectures.
func BuildAllPlatforms(ctx context.Context, u *ui.UI, cfg Config) error {
	if len(cfg.Platforms) == 0 {
		return fmt.Errorf("no target platforms specified in configuration")
	}

	u.Info("resolved build matrix", "platforms", len(cfg.Platforms), "dest", cfg.DestDir)

	summary := make([]ui.Row, 0, len(cfg.Platforms))

	for _, platformInfo := range cfg.Platforms {
		dirName, ok := platformName[platformInfo]
		if !ok {
			return fmt.Errorf("unsupported orchestration mapping: %s", platformInfo.String())
		}

		u.Step(fmt.Sprintf("%s (%s)", dirName, platformInfo.String()))

		dest := filepath.Join(cfg.DestDir, dirName)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("failed to create directory for platform %s: %w", dirName, err)
		}

		if err := compiler.New().Build(platformInfo, dest, cfg.ProjectDir, cfg.NDKDir, cfg.Version); err != nil {
			return fmt.Errorf("build aborted due to compilation failure on %s: %w", dirName, err)
		}
		u.Success(fmt.Sprintf("compiled %s", dirName))

		destAssetsDir := filepath.Join(dest, "assets")

		if err := processXray(ctx, u, platformInfo, cfg.XrayVersion, destAssetsDir); err != nil {
			return fmt.Errorf("failed fetching Xray for platform %s: %w", dirName, err)
		}

		if err := processSlipstream(ctx, u, platformInfo, destAssetsDir); err != nil {
			return fmt.Errorf("failed fetching Slipstream for platform %s: %w", dirName, err)
		}

		summary = append(summary, ui.Row{Target: dirName, Status: "done", OK: true})
	}

	u.Summary("summary", summary)
	return nil
}

func RunSetupDev(ctx context.Context, u *ui.UI, cfg Config) error {
	u.Info("preparing dev workspace")
	if err := compiler.New().PrepareDevProjectFiles(cfg.ProjectDir); err != nil {
		return fmt.Errorf("dev prep failed: %w", err)
	}

	assetsDir := filepath.Join(cfg.ProjectDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return fmt.Errorf("create assets dir: %w", err)
	}

	if err := processXray(ctx, u, platform.Detect(), cfg.XrayVersion, assetsDir); err != nil {
		return fmt.Errorf("xray setup failed: %w", err)
	}

	if err := processSlipstream(ctx, u, platform.Detect(), assetsDir); err != nil {
		return fmt.Errorf("slipstream setup failed: %w", err)
	}

	u.Success(fmt.Sprintf("dev workspace ready at %s", cfg.ProjectDir))
	return nil
}
