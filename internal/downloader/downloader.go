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
	"time"

	"bgscan-builder/internal/platform"
)

const (
	passFix = "%s_%d"
	timeout = 30 * time.Second
)

// client implements the Downloader interface.
type client struct{}

// DownloadXray resolves, downloads, and validates the Xray Core release asset matching
// the given platform specification using its remote digest signature.
func (c *client) DownloadXray(
	ctx context.Context,
	info platform.Info,
	destDir string,
	version string,
) (string, error) {
	// xray don't have build for android arm32-va7 amd amd32 so switch to linux build
	if platform.Android == info.OS && (platform.ARM32 == info.Arch || platform.AMD32 == info.Arch) {
		info.OS = platform.Linux
	}

	binaryURL, err := resolveAsset(ctx, info, xrayRepo, "Xray", version)
	if err != nil {
		return "", err
	}

	dgstURL := binaryURL + ".dgst"

	binaryPath, err := c.DownloadFile(ctx, binaryURL, destDir)
	if err != nil {
		return "", err
	}

	if dgstURL != "" {
		hash, err := extractSHA256(ctx, dgstURL)
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
func (c *client) DownloadSlipstream(ctx context.Context, info platform.Info, destDir string, version string) (string, error) {
	return c.resolveAndDownloadDependency(ctx, info, "slipstream-client", destDir, version)
}

// DownloadFile downloads a file from a URL into a target path or target directory.
func (c *client) DownloadFile(ctx context.Context, urlStr, dest string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	filename, err := getFilename(urlStr, dest)
	if err != nil {
		return "", err
	}

	dir := dest
	fi, err := os.Stat(dest)
	if err != nil || !fi.IsDir() {
		dir = filepath.Dir(dest)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
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
	binaryURL, err := resolveAsset(ctx, info, dependencyRepo, binaryName, version)
	if err != nil {
		return "", err
	}

	cleanRepo := strings.Trim(dependencyRepo, "/")
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksum.txt", cleanRepo, version)

	finalBinaryPath, err := c.DownloadFile(ctx, binaryURL, destPath)
	if err != nil {
		return "", err
	}

	hash, err := extractChecksumFromFile(ctx, filepath.Base(binaryURL), checksumURL)
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
		newName := fmt.Sprintf(passFix, base, i) + ext
		if _, exists := existing[newName]; !exists {
			return newName, nil
		}
	}
}
