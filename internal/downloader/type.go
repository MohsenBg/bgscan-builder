package downloader

import (
	"context"

	"bgscan-builder/internal/platform"
)

// Downloader defines methods for fetching, verifying, and staging remote binary assets.
type Downloader interface {
	DownloadXray(ctx context.Context, info platform.Info, destDir string, version string) (string, error)
	DownloadSlipstream(ctx context.Context, info platform.Info, destDir string, version string) (string, error)
	DownloadFile(ctx context.Context, urlStr, dest string) (string, error)
	VerifyFileChecksum(path, expectedHash string) error
}

// New returns a new Downloader implementation.
func New() Downloader {
	return &client{}
}
