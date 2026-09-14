package downloader

import "io"

// ProgressSink creates progress bars for downloads. Implemented by the
// caller-provided UI; a nil sink disables progress bars (plain copy).
type ProgressSink interface {
	AddFileBar(total int64) FileBar
}

// FileBar is the minimal surface the downloader needs from a progress bar.
type FileBar interface {
	ProxyReader(r io.Reader) io.ReadCloser
	SetTotal(total int64)
	Abort()
}
