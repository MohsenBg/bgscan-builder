package downloader

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"bgscan-builder/internal/platform"
)

// client implements the Downloader interface.
type client struct {
	sink         ProgressSink
	hc           *http.Client
	apiBase      string
	downloadBase string
}

// DownloadSlipstream fetches, verifies, and stages the Slipstream client module for the target platform architecture.
func (c *client) DownloadSlipstream(ctx context.Context, info platform.Info, destDir string) (string, error) {
	return c.resolveAndDownloadDependency(ctx, info, "slipstream-client", destDir, "latest")
}

// DownloadFile downloads a file from a URL into destDir.
// It deliberately applies no timeout: slow links keep downloading until the
// user cancels (Ctrl+C) or the transfer naturally ends.
func (c *client) DownloadFile(ctx context.Context, urlStr, destDir string) (string, error) {
	filename := filenameFromURL(urlStr)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(destDir, ".tmp-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()

	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	// Wrap the response body with a progress bar when a sink is configured.
	var reader io.Reader = resp.Body
	var bar FileBar
	if c.sink != nil {
		bar = c.sink.AddFileBar(resp.ContentLength)
		proxy := bar.ProxyReader(resp.Body)
		defer func() { _ = proxy.Close() }()
		reader = proxy
	}

	n, err := io.Copy(tmpFile, reader)
	if err != nil {
		if bar != nil {
			bar.Abort()
		}
		return "", err
	}

	if bar != nil {
		// ContentLength may be -1 for unknown sizes; finalize the bar with
		// the actual number of bytes written.
		bar.SetTotal(n)
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	finalPath := filepath.Join(destDir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", err
	}

	return finalPath, nil
}

// VerifyFileChecksum checks the SHA256 hash of a file against an expected hex-encoded value.
func (c *client) VerifyFileChecksum(path, expectedHash string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(expectedHash, actual) {
		return fmt.Errorf("checksum mismatch: expected %s got %s for file %s", expectedHash, actual, path)
	}

	return nil
}

func (c *client) resolveAndDownloadDependency(
	ctx context.Context,
	info platform.Info,
	binaryName string,
	destPath string,
	version string,
) (string, error) {
	binaryURL, err := c.resolveAsset(ctx, info, dependencyRepo, binaryName, version)
	if err != nil {
		return "", err
	}

	finalBinaryPath, err := c.DownloadFile(ctx, binaryURL, destPath)
	if err != nil {
		return "", err
	}

	hash, err := c.FetchChecksum(ctx, dependencyRepo, filepath.Base(binaryURL), version)
	if err != nil {
		return "", err
	}

	if err := c.VerifyFileChecksum(finalBinaryPath, hash); err != nil {
		return "", err
	}

	return finalBinaryPath, nil
}

// filenameFromURL derives the output filename from the last URL path
// segment, falling back to "file" when the URL carries no usable name.
func filenameFromURL(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "file"
	}

	name := filepath.Base(u.Path)
	if name == "" || name == "/" || name == "." {
		return "file"
	}
	return name
}
