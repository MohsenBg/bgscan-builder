package downloader

import (
	"bgscan-builder/internal/platform"
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

const xrayRepo = "XTLS/Xray-core/"

// DownloadXray resolves, downloads, and validates the Xray Core release asset matching
// the given platform specification using its remote digest signature.
func DownloadXray(
	ctx context.Context,
	info platform.Info,
	destDir string,
	version string,
) (string, error) {
	binaryURL, err := resolveAsset(ctx, info, xrayRepo, "Xray", version)
	if err != nil {
		return "", err
	}

	dgstURL := binaryURL + ".dgst"

	binaryPath, err := DownloadFile(ctx, binaryURL, destDir)
	if err != nil {
		return "", err
	}

	if dgstURL != "" {
		hash, err := extractSHA256(ctx, dgstURL)
		if err != nil {
			return "", err
		}

		if err := VerifyFileChecksum(binaryPath, hash); err != nil {
			return "", err
		}
	}

	return binaryPath, nil
}

func extractSHA256(ctx context.Context, url string) (string, error) {
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
		return "", fmt.Errorf("dgst fetch error: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "SHA2-256=") {
			parts := strings.Split(line, "=")
			if len(parts) != 2 {
				continue
			}
			return strings.TrimSpace(parts[1]), nil
		}

		if err := scanner.Err(); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("sha256 not found in dgst")
}

