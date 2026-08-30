package ui

import (
	"io"
	"time"

	"bgscan-builder/internal/downloader"

	"github.com/schollz/progressbar/v3"
)

// progressSink adapts a progressbar to the downloader.ProgressSink interface
// so every download renders as a live, self-updating progress bar on its own
// dedicated line below the "Downloading …" title. A plain (non-terminal) UI
// never creates bars.
type progressSink struct{ w io.Writer }

func (s *progressSink) AddFileBar(_ string, total int64) downloader.FileBar {
	bar := progressbar.NewOptions64(total,
		progressbar.OptionSetWriter(s.w),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerPadding: "░",
			BarStart:      "",
			BarEnd:        "",
		}),
		progressbar.OptionSetWidth(40),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionThrottle(25*time.Millisecond),
		progressbar.OptionUseANSICodes(true),
	)
	return &fileBar{bar: bar, w: s.w}
}

// fileBar adapts the schollz bar to the downloader.FileBar contract.
type fileBar struct {
	bar *progressbar.ProgressBar
	w   io.Writer
}

// ProxyReader wraps the response body, advancing the bar as bytes arrive.
func (b *fileBar) ProxyReader(r io.Reader) (io.ReadCloser, error) {
	return &countingReader{bar: b.bar, src: r}, nil
}

// SetTotal normalizes the bar total on completion and renders the final line.
func (b *fileBar) SetTotal(total int64, forceComplete bool) {
	if !forceComplete || b.bar.IsFinished() {
		return
	}
	if total >= 0 {
		b.bar.ChangeMax64(total)
		_ = b.bar.Set64(total)
	}
	b.finish()
}

// Abort finalizes the bar early on failure.
func (b *fileBar) Abort(bool) { b.finish() }

// finish renders the completed bar and moves to the next line so following
// log output is not appended to the progress line.
func (b *fileBar) finish() {
	_ = b.bar.Finish()
	_, _ = io.WriteString(b.w, "\n")
}

// Wait is a no-op; the bar renders synchronously.
func (b *fileBar) Wait() {}

// countingReader proxies reads and feeds their size into the progress bar.
type countingReader struct {
	bar *progressbar.ProgressBar
	src io.Reader
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		_ = r.bar.Add64(int64(n))
	}
	return n, err
}

func (r *countingReader) Close() error {
	if c, ok := r.src.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
