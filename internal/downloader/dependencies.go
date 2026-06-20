// Package downloader implements the remote asset fetching, artifact resolution,
// and validation routines for bgscan sidecar components.
package downloader

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"bgscan-builder/internal/platform"
)

const dependencyRepo = "MohsenBg/dep-bgscan"

// DownloadDNSTT fetches, verifies, and stages the DNSTT client module for the target platform architecture.
func DownloadDNSTT(ctx context.Context, info platform.Info, destDir string, version string) (string, error) {
	return resolveAndDownloadDependency(ctx, info, "dnstt-client", destDir, version)
}

// DownloadSlipstream fetches, verifies, and stages the Slipstream client module for the target platform architecture.
func DownloadSlipstream(ctx context.Context, info platform.Info, destDir string, version string) (string, error) {
	return resolveAndDownloadDependency(ctx, info, "slipstream-client", destDir, version)
}

func resolveAndDownloadDependency(
	ctx context.Context,
	info platform.Info,
	binaryName string,
	destPath string,
	version string,
) (string, error) {
	binaryURL, err := resolveAsset(ctx, info, dependencyRepo, binaryName, version)
	if err != nil {
		return "", err
	}

	cleanRepo := strings.Trim(dependencyRepo, "/")
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksum.txt", cleanRepo, version)

	finalBinaryPath, err := DownloadFile(ctx, binaryURL, destPath)
	if err != nil {
		return "", err
	}

	hash, err := extractChecksumFromFile(ctx, filepath.Base(binaryURL), checksumURL)
	if err != nil {
		return "", err
	}

	if err := VerifyFileChecksum(finalBinaryPath, hash); err != nil {
		return "", err
	}

	return finalBinaryPath, nil
}

func extractChecksumFromFile(ctx context.Context, filename, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum fetch error: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, filename) {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		if parts[1] == filename {
			return parts[0], nil
		}
		return parts[0], nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("checksum not found for %s", filename)
}

