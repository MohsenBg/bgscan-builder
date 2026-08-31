package downloader

import (
	"context"
	"net/http"
	"strings"

	"bgscan-builder/internal/netutil"
	"bgscan-builder/internal/platform"
)

const (
	defaultAPIBase      = "https://api.github.com"
	defaultDownloadBase = "https://github.com"
)

// Downloader defines methods for fetching, verifying, and staging remote binary assets.
type Downloader interface {
	DownloadXray(ctx context.Context, info platform.Info, destDir string, version string) (string, error)
	DownloadSlipstream(ctx context.Context, info platform.Info, destDir string) (string, error)
	DownloadFile(ctx context.Context, urlStr, dest string) (string, error)
	VerifyFileChecksum(path, expectedHash string) error
	// ResolveReleaseAsset pinpoints the optimal binary asset for the target
	// platform within the given repository release.
	ResolveReleaseAsset(ctx context.Context, info platform.Info, repoURL, binaryName, version string) (ReleaseAsset, error)
	// FetchChecksum retrieves the SHA-256 checksum for the given asset from
	// the repository's release checksum.txt.
	FetchChecksum(ctx context.Context, repoURL, filename, version string) (string, error)
}

// Option configures the Downloader implementation.
type Option func(*client)

// WithProgressSink routes download progress bars through the provided sink.
// A nil sink disables progress bars.
func WithProgressSink(sink ProgressSink) Option {
	return func(c *client) { c.sink = sink }
}

// WithHTTPClient overrides the HTTP client used for network requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *client) { c.hc = hc }
}

// WithAPIBaseURL overrides the GitHub API base URL (default https://api.github.com).
// Intended for tests; release resolution endpoints are resolved against it.
func WithAPIBaseURL(base string) Option {
	return func(c *client) { c.apiBase = strings.TrimRight(base, "/") }
}

// WithDownloadBaseURL overrides the GitHub release download base URL
// (default https://github.com). Intended for tests.
func WithDownloadBaseURL(base string) Option {
	return func(c *client) { c.downloadBase = strings.TrimRight(base, "/") }
}

// New returns a new Downloader implementation. Unless overridden with
// WithHTTPClient, all network operations share the netutil HTTP client,
// which falls back to public DNS servers when the system resolver fails.
func New(options ...Option) Downloader {
	c := &client{
		hc:           netutil.DefaultHTTPClient(),
		apiBase:      defaultAPIBase,
		downloadBase: defaultDownloadBase,
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}
