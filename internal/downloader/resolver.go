package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"bgscan-builder/internal/platform"
)

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ReleaseAsset identifies a resolved release asset together with the version
// it was published under.
type ReleaseAsset struct {
	Version string // release tag, e.g. v2.9.1
	Name    string // asset filename, e.g. bgscan-linux-64.zip
	URL     string // browser download URL for the asset
}

// ResolveReleaseAsset evaluates available assets in the target repository to
// pinpoint the optimal binary artifact match for a specified platform.
// An empty or "latest" version resolves the latest release; any other value
// resolves the matching tagged release.
func (c *client) ResolveReleaseAsset(
	ctx context.Context,
	info platform.Info,
	repoURL string,
	binaryName string,
	version string,
) (ReleaseAsset, error) {
	links, tag, err := c.fetchAssets(ctx, repoURL, version)
	if err != nil {
		return ReleaseAsset{}, err
	}

	osToken := strings.ToLower(info.OS.String())
	archTokens := info.Arch.Tokens()
	binName := strings.ToLower(binaryName)

	for _, link := range links {
		l := strings.ToLower(link)
		if !strings.Contains(l, binName) {
			continue
		}

		osTokens := []string{osToken}
		if osToken == platform.MacOS.String() {
			osTokens = append(osTokens, "darwin")
		}

		if !matchTokens(l, osTokens) {
			continue
		}

		if matchTokens(l, archTokens) {
			return ReleaseAsset{
				Version: tag,
				Name:    filepath.Base(link),
				URL:     link,
			}, nil
		}
	}

	return ReleaseAsset{}, fmt.Errorf("no matching asset for %s-%s", info.OS, info.Arch)
}

// resolveAsset is a URL-only convenience wrapper for internal callers.
func (c *client) resolveAsset(
	ctx context.Context,
	info platform.Info,
	repoURL string,
	binaryName string,
	version string,
) (string, error) {
	asset, err := c.ResolveReleaseAsset(ctx, info, repoURL, binaryName, version)
	if err != nil {
		return "", err
	}
	return asset.URL, nil
}

func matchTokens(text string, tokens []string) bool {
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == '/'
	})

	for _, part := range parts {
		// Explicitly skip legacy ARM v5 and v6 variants
		if part == "v5" || part == "v6" || part == "armv5" || part == "armv6" {
			return false
		}

		if slices.Contains(tokens, part) {
			return true
		}
	}

	return false
}

// fetchAssets returns the browser download URLs advertised by the release
// together with the release tag name.
func (c *client) fetchAssets(ctx context.Context, repoURL, version string) ([]string, string, error) {
	cleanRepo := strings.Trim(repoURL, "/")

	var url string
	if version == "" || strings.EqualFold(version, "latest") {
		url = fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase, cleanRepo)
	} else {
		url = fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.apiBase, cleanRepo, version)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", &statusError{Code: resp.StatusCode, Status: resp.Status}
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, "", err
	}

	out := make([]string, 0, len(r.Assets))
	for _, a := range r.Assets {
		if strings.HasSuffix(a.BrowserDownloadURL, ".dgst") {
			continue
		}
		out = append(out, a.BrowserDownloadURL)
	}

	return out, r.TagName, nil
}
