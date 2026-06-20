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
)

// processXray handles the downloading, verification, unpacking, and metadata cleanup
// of the Xray Core binary asset for the specified target architecture platform.
func processXray(ctx context.Context, platform platform.Info, xrayVersion, assetsDir string) error {
	fmt.Printf("Downloading Xray Core (%s)...\n", xrayVersion)

	xrayDir := filepath.Join(assetsDir, "xray")
	if err := os.MkdirAll(xrayDir, 0755); err != nil {
		return fmt.Errorf("failed to prepare xrayDir folder: %w", err)
	}

	archivePath, err := downloader.DownloadXray(ctx, platform, xrayDir, xrayVersion)
	if err != nil {
		return fmt.Errorf("xray download failed: %w", err)
	}

	zipArchiver, err := archive.CreateArchiver(archive.ArchiveZIP)
	if err != nil {
		return fmt.Errorf("failed to initialize zip engine: %w", err)
	}

	_, err = zipArchiver.Decompress(archivePath, xrayDir)
	if err != nil {
		return fmt.Errorf("xray extraction failed: %w", err)
	}

	_ = os.Remove(archivePath)
	cleanDocumentation(xrayDir)
	return nil
}

// processDNSTT handles fetching, verifying, unpacking, and normalizing the binary mapping
// conventions for the DNSTT sidecar execution client.
func processDNSTT(ctx context.Context, platformInfo platform.Info, depVersion, assetsDir string) error {
	fmt.Printf("Downloading DNSTT (%s)...\n", depVersion)

	dnsttDir := filepath.Join(assetsDir, "dnstt-client")
	if err := os.MkdirAll(dnsttDir, 0755); err != nil {
		return fmt.Errorf("failed to prepare dnstt folder: %w", err)
	}

	archivePath, err := downloader.DownloadDNSTT(ctx, platformInfo, dnsttDir, depVersion)
	if err != nil {
		return fmt.Errorf("dnstt download failed: %w", err)
	}

	tarArchiver, err := archive.CreateArchiver(archive.ArchiveTAR)
	if err != nil {
		return fmt.Errorf("failed to initialize tar engine: %w", err)
	}

	_, err = tarArchiver.Decompress(archivePath, dnsttDir)
	if err != nil {
		return fmt.Errorf("dnstt extraction failed: %w", err)
	}

	_ = os.Remove(archivePath)

	ext := ""
	if platformInfo.OS == platform.Windows {
		ext = ".exe"
	}

	fixBinaryMapping(dnsttDir, "dnstt", "dnstt-client"+ext)
	cleanDocumentation(dnsttDir)
	return nil
}

// processSlipstream fetches, expands, and configures the Slipstream tunneling protocol client
// asset workspace configurations natively.
func processSlipstream(ctx context.Context, platformInfo platform.Info, depVersion, assetsDir string) error {
	fmt.Printf("Downloading Slipstream (%s)...\n", depVersion)

	slipDir := filepath.Join(assetsDir, "slipstream-client")
	if err := os.MkdirAll(slipDir, 0755); err != nil {
		return fmt.Errorf("failed to prepare slipstream folder: %w", err)
	}

	archivePath, err := downloader.DownloadSlipstream(ctx, platformInfo, slipDir, depVersion)
	if err != nil {
		return fmt.Errorf("slipstream download failed: %w", err)
	}

	tarArchiver, err := archive.CreateArchiver(archive.ArchiveTAR)
	if err != nil {
		return fmt.Errorf("failed to initialize tar engine: %w", err)
	}

	_, err = tarArchiver.Decompress(archivePath, slipDir)
	if err != nil {
		return fmt.Errorf("slipstream extraction failed: %w", err)
	}

	_ = os.Remove(archivePath)

	ext := ""
	if platformInfo.OS == platform.Windows {
		ext = ".exe"
	}

	fixBinaryMapping(slipDir, "slipstream", "slipstream-client"+ext)
	cleanDocumentation(slipDir)
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
