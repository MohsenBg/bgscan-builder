package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"bgscan-builder/internal/platform"
)

type release struct {
	Assets []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// resolveAsset evaluates available assets in the target repository to pinpoint
// the optimal binary artifact match for a specified system architecture.
func resolveAsset(
	ctx context.Context,
	info platform.Info,
	repoURL string,
	binaryName string,
	version string,
) (string, error) {
	assets, err := fetchAssets(ctx, repoURL, version)
	if err != nil {
		return "", err
	}

	osToken := strings.ToLower(info.OS.String())
	archTokens := info.Arch.Tokens()

	for _, link := range assets {
		l := strings.ToLower(link)
		binName := strings.ToLower(binaryName)
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
			return link, nil
		}
	}

	return "", fmt.Errorf("no matching asset for %s-%s", info.OS, info.Arch)
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

func fetchAssets(ctx context.Context, repoURL, version string) ([]string, error) {
	cleanRepo := strings.Trim(repoURL, "/")
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", cleanRepo, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(r.Assets))
	for _, a := range r.Assets {
		if strings.HasSuffix(a.BrowserDownloadURL, ".dgst") {
			continue
		}
		out = append(out, a.BrowserDownloadURL)
	}

	return out, nil
}
