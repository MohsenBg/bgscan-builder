package downloader

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
)

const xrayRepo = "XTLS/Xray-core/"

func (c *client) extractSHA256(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

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
