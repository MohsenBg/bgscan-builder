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

const duplicateNameFormat = "%s_%d"

// client implements the Downloader interface.
type client struct {
	sink         ProgressSink
	hc           *http.Client
	apiBase      string
	downloadBase string
}

// DownloadXray resolves, downloads, and validates the Xray Core release asset matching
// the given platform specification using its remote digest signature.
func (c *client) DownloadXray(
	ctx context.Context,
	info platform.Info,
	destDir string,
	version string,
) (string, error) {
	// Xray releases no Android builds for 32-bit CPUs; use the Linux build.
	if platform.Android == info.OS && (platform.ARM32 == info.Arch || platform.AMD32 == info.Arch) {
		info.OS = platform.Linux
	}

	binaryURL, err := c.resolveAsset(ctx, info, xrayRepo, "Xray", version)
	if err != nil {
		return "", err
	}

	dgstURL := binaryURL + ".dgst"

	binaryPath, err := c.DownloadFile(ctx, binaryURL, destDir)
	if err != nil {
		return "", err
	}

	if dgstURL != "" {
		hash, err := c.extractSHA256(ctx, dgstURL)
		if err != nil {
			return "", err
		}

		if err := c.VerifyFileChecksum(binaryPath, hash); err != nil {
			return "", err
		}
	}

	return binaryPath, nil
}

// DownloadSlipstream fetches, verifies, and stages the Slipstream client module for the target platform architecture.
func (c *client) DownloadSlipstream(ctx context.Context, info platform.Info, destDir string) (string, error) {
	return c.resolveAndDownloadDependency(ctx, info, "slipstream-client", destDir, "latest")
}

// DownloadFile downloads a file from a URL into a target path or target directory.
// It deliberately applies no timeout: slow links keep downloading until the
// user cancels (Ctrl+C) or the transfer naturally ends.
func (c *client) DownloadFile(ctx context.Context, urlStr, dest string) (string, error) {
	filename, err := getFilename(urlStr, dest)
	if err != nil {
		return "", err
	}

	dir := dest
	fi, err := os.Stat(dest)
	if err != nil || !fi.IsDir() {
		dir = filepath.Dir(dest)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
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
		bar = c.sink.AddFileBar(filename, resp.ContentLength)
		proxy, proxyErr := bar.ProxyReader(resp.Body)
		if proxyErr != nil {
			return "", proxyErr
		}
		defer func() { _ = proxy.Close() }()
		reader = proxy
	}

	n, err := io.Copy(tmpFile, reader)
	if err != nil {
		if bar != nil {
			bar.Abort(true)
		}
		return "", err
	}

	if bar != nil {
		// ContentLength may be -1 for unknown sizes; finalize the bar with
		// the actual number of bytes written before waiting on it.
		bar.SetTotal(n, true)
		bar.Wait()
	}

	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	finalPath := filepath.Join(dir, filename)
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

func getFilename(urlStr, dest string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	urlName := filepath.Base(u.Path)
	if urlName == "" || urlName == "/" || urlName == "." {
		urlName = "file"
	}

	if dest == "" {
		return resolveFilenameConflict(".", urlName)
	}

	fi, err := os.Stat(dest)
	if err == nil && fi.IsDir() {
		return resolveFilenameConflict(dest, urlName)
	}

	if strings.HasSuffix(dest, string(os.PathSeparator)) {
		return resolveFilenameConflict(dest, urlName)
	}

	base := filepath.Base(dest)
	return resolveFilenameConflict(filepath.Dir(dest), base)
}

func resolveFilenameConflict(dir, filename string) (string, error) {
	existing := make(map[string]struct{})

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return filename, nil
		}
		return "", err
	}

	for _, e := range entries {
		if !e.IsDir() {
			existing[e.Name()] = struct{}{}
		}
	}

	if _, ok := existing[filename]; !ok {
		return filename, nil
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for i := 1; ; i++ {
		newName := fmt.Sprintf(duplicateNameFormat, base, i) + ext
		if _, exists := existing[newName]; !exists {
			return newName, nil
		}
	}
}
