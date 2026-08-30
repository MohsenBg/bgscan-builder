package downloader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const dependencyRepo = "MohsenBg/dep-bgscan"

// ErrChecksumUnavailable reports that no checksum could be resolved for a
// release asset: the checksum file is missing or does not list the asset.
var ErrChecksumUnavailable = errors.New("checksum unavailable for release asset")

// FetchChecksum retrieves the SHA-256 checksum for the given asset filename
// from the repository's release checksum.txt.
func (c *client) FetchChecksum(ctx context.Context, repoURL, filename, version string) (string, error) {
	cleanRepo := strings.Trim(repoURL, "/")

	var checksumURL string
	if version == "" || strings.EqualFold(version, "latest") {
		checksumURL = fmt.Sprintf("%s/%s/releases/latest/download/checksum.txt", c.downloadBase, cleanRepo)
	} else {
		checksumURL = fmt.Sprintf("%s/%s/releases/download/%s/checksum.txt", c.downloadBase, cleanRepo, version)
	}

	hash, ok, err := c.scanChecksumFile(ctx, filename, checksumURL)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrChecksumUnavailable, filename)
	}
	return hash, nil
}

func (c *client) scanChecksumFile(ctx context.Context, filename, url string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", false, ErrChecksumUnavailable
		}
		return "", false, fmt.Errorf("checksum fetch error: %s", resp.Status)
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

		return parts[0], true, nil
	}

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return "", false, nil
}
