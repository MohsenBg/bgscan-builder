// Package downloader implements the remote asset fetching, artifact resolution,
// and validation routines for bgscan sidecar components.
package downloader

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

const dependencyRepo = "MohsenBg/dep-bgscan"

func extractChecksumFromFile(ctx context.Context, filename, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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
