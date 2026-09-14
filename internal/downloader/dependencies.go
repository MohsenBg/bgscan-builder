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

// statusError reports a non-200 HTTP response.
type statusError struct {
	Code   int
	Status string
}

func (e *statusError) Error() string { return "request failed: " + e.Status }

// get performs a GET request, returning the open response body on 200 OK.
// Any other status is reported as a *statusError; the body is already
// closed in that case.
func (c *client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, &statusError{Code: resp.StatusCode, Status: resp.Status}
	}
	return resp, nil
}

func (c *client) scanChecksumFile(ctx context.Context, filename, url string) (string, bool, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		var statusErr *statusError
		if errors.As(err, &statusErr) && statusErr.Code == http.StatusNotFound {
			return "", false, ErrChecksumUnavailable
		}
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

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
