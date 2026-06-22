package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"bgscan-builder/internal/compiler"
	"bgscan-builder/internal/platform"
)

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
	cfg, err := ParseCLI()
	if err != nil {
		log.Fatalf("CLI Error: %v", err)
	}

	ctx := context.Background()
	switch cfg.Mode {
	case MODE_RELEASE:
		if err := BuildAllPlatforms(ctx, *cfg); err != nil {
			log.Fatalf("Build Error: %v", err)
		}
	case MODE_DEV:
		RunSetupDev(ctx, *cfg)
	}

}

// BuildAllPlatforms executes cross-compilation and downloads core dependencies
// for all targeted architectures.
func BuildAllPlatforms(ctx context.Context, cfg Config) error {
	if len(cfg.Platforms) == 0 {
		return fmt.Errorf("no target platforms specified in configuration")
	}

	for _, platformInfo := range cfg.Platforms {
		platformDirName, exists := platformName[platformInfo]
		if !exists {
			return fmt.Errorf("unsupported orchestration mapping: %s", platformInfo.String())
		}

		targetDestPath := filepath.Join(cfg.DestDir, platformDirName)

		fmt.Println("----------------------------------------------------------------------")
		fmt.Printf("Target Environment: %s\n", platformDirName)
		fmt.Println("----------------------------------------------------------------------")

		if err := os.MkdirAll(targetDestPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory for platform %s: %w", platformDirName, err)
		}

		if err := compiler.Build(platformInfo, targetDestPath, cfg.ProjectDir, cfg.NDKDir); err != nil {
			return fmt.Errorf("build aborted due to compilation failure on %s: %w", platformDirName, err)
		}

		destAssetsDir := filepath.Join(targetDestPath, "assets")

		if err := processXray(ctx, platformInfo, cfg.XrayVersion, destAssetsDir); err != nil {
			return fmt.Errorf("failed fetching Xray for platform %s: %w", platformDirName, err)
		}

		if err := processDNSTT(ctx, platformInfo, cfg.DepVersion, destAssetsDir); err != nil {
			return fmt.Errorf("failed fetching DNSTT for platform %s: %w", platformDirName, err)
		}

		if err := processSlipstream(ctx, platformInfo, cfg.DepVersion, destAssetsDir); err != nil {
			return fmt.Errorf("failed fetching Slipstream for platform %s: %w", platformDirName, err)
		}
	}

	fmt.Println("\nExecution completed: All platform targets and dependencies deployed successfully.")
	return nil
}

func RunSetupDev(ctx context.Context, cfg Config) error {
	fmt.Println("------------------------------------------------------")
	fmt.Println("DEV SETUP START")
	fmt.Println("------------------------------------------------------")

	// prepare local dev workspace
	if err := compiler.PrepareDevProjectFiles(cfg.ProjectDir); err != nil {
		return fmt.Errorf("dev prep failed: %w", err)
	}

	assetsDir := filepath.Join(cfg.ProjectDir, "assets")

	// ensure assets folder exists
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return fmt.Errorf("create assets dir: %w", err)
	}

	fmt.Println("Downloading dependencies...")

	// download xray
	if err := processXray(ctx, platform.Detect(), cfg.XrayVersion, assetsDir); err != nil {
		return fmt.Errorf("xray setup failed: %w", err)
	}

	// download dnstt
	if err := processDNSTT(ctx, platform.Detect(), cfg.DepVersion, assetsDir); err != nil {
		return fmt.Errorf("dnstt setup failed: %w", err)
	}

	// download slipstream
	if err := processSlipstream(ctx, platform.Detect(), cfg.DepVersion, assetsDir); err != nil {
		return fmt.Errorf("slipstream setup failed: %w", err)
	}

	fmt.Println("------------------------------------------------------")
	fmt.Println("DEV SETUP DONE")
	fmt.Println("------------------------------------------------------")

	return nil
}
