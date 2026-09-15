package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bgscan-builder/internal/archive"
	"bgscan-builder/internal/downloader"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

// newDownloader wires the shared UI progress sink into the downloader so all
// downloads render into the same container as the log output.
func newDownloader(u *ui.UI) downloader.Downloader {
	return downloader.New(downloader.WithProgressSink(u.ProgressSink()))
}

// processSlipstream fetches, expands, and configures the Slipstream tunneling protocol client
// asset workspace configurations natively.
func processSlipstream(ctx context.Context, u *ui.UI, target platform.Info, assetsDir string) error {
	u.Info("fetching Slipstream", "target", target.String())

	slipDir := filepath.Join(assetsDir, "slipstream-client")
	if err := os.MkdirAll(slipDir, 0o755); err != nil {
		return fmt.Errorf("failed to prepare slipstream folder: %w", err)
	}

	archivePath, err := newDownloader(u).DownloadSlipstream(ctx, target, slipDir)
	if err != nil {
		return fmt.Errorf("slipstream download failed: %w", err)
	}
	u.Debug("slipstream bundle downloaded", "archive", archivePath)

	_, err = archive.Extract(archivePath, slipDir)
	if err != nil {
		return fmt.Errorf("slipstream extraction failed: %w", err)
	}

	_ = os.Remove(archivePath)

	ext := ""
	if target.OS == platform.Windows {
		ext = ".exe"
	}

	fixBinaryMapping(slipDir, "slipstream", "slipstream-client"+ext)
	cleanDocumentation(slipDir)
	u.Success("Slipstream staged")
	return nil
}

// fixBinaryMapping looks for a partial match or unmapped executable and enforces standard targets.
func fixBinaryMapping(dir, prefix, expectedTarget string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(strings.ToLower(name), prefix) && name != expectedTarget {
			oldPath := filepath.Join(dir, name)
			newPath := filepath.Join(dir, expectedTarget)
			_ = os.Rename(oldPath, newPath)
			break
		}
	}
}

// cleanDocumentation sweeps a target directory to scrub LICENSE and README variations.
func cleanDocumentation(targetDir string) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		nameLower := strings.ToLower(entry.Name())
		if strings.HasPrefix(nameLower, "license") || strings.HasPrefix(nameLower, "readme") {
			_ = os.Remove(filepath.Join(targetDir, entry.Name()))
		}
	}
}
