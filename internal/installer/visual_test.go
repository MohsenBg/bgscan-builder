package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bgscan-builder/internal/downloader"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

// TestVisualRun exercises the full install flow against a local release
// server (no network) and renders it to the real terminal. Run under a pty
// with BGSCAN_VISUAL=1 to inspect the polished output:
//
//	BGSCAN_VISUAL=1 go test ./internal/installer -run TestVisualRun -v
//
// heavyZip builds a release zip large enough for the live progress bar to
// animate during a visual run. The filler is incompressible so the payload
// stays meaningfully large.
func heavyZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	header := func(name string, mode os.FileMode) *zip.FileHeader {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(mode)
		return h
	}
	write := func(name string, mode os.FileMode, content string) {
		t.Helper()
		w, err := zw.CreateHeader(header(name, mode))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("bgscan-linux-64/bgscan", 0o755, "#!/bin/sh\necho bgscan\n")
	write("bgscan-linux-64/ips/iran.csv", 0o644, "v2-iran\n")

	// ~700 KiB of deterministic, hard-to-compress data.
	rng := rand.New(rand.NewSource(42))
	filler := make([]byte, 700*1024)
	if _, err := rng.Read(filler); err != nil {
		t.Fatal(err)
	}
	w, err := zw.CreateHeader(header("bgscan-linux-64/assets/large.dat", 0o644))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(filler); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestVisualRun(t *testing.T) {
	if os.Getenv("BGSCAN_VISUAL") == "" {
		t.Skip("set BGSCAN_VISUAL=1 to run")
	}

	run := func(t *testing.T, before func(dir string), input string) {
		fixed := newFixture(t, heavyZip(t), func(f *fixture) { f.slow = true })
		u := ui.New(os.Stderr)
		dl := downloader.New(
			downloader.WithHTTPClient(fixed.srv.Client()),
			downloader.WithAPIBaseURL(fixed.srv.URL),
			downloader.WithDownloadBaseURL(fixed.srv.URL),
			downloader.WithProgressSink(u.ProgressSink()),
		)
		inst := New(u, dl)
		dir := filepath.Join(t.TempDir(), "bgscan")
		if before != nil {
			before(dir)
		}
		err := inst.Install(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, "latest", dir, strings.NewReader(input))
		if err != nil && err != ErrCancelled {
			t.Fatalf("Install: %v", err)
		}
		t.Log("install dir:", dir)
	}

	t.Run("success-fresh", func(t *testing.T) {
		run(t, nil, "\n")
	})

	t.Run("update-existing", func(t *testing.T) {
		fixed := newFixture(t, heavyZip(t), nil)
		u := ui.New(os.Stderr)
		dl := downloader.New(
			downloader.WithHTTPClient(fixed.srv.Client()),
			downloader.WithAPIBaseURL(fixed.srv.URL),
			downloader.WithDownloadBaseURL(fixed.srv.URL),
			downloader.WithProgressSink(u.ProgressSink()),
		)
		inst := New(u, dl)
		dir := filepath.Join(t.TempDir(), "bgscan")
		if err := os.MkdirAll(filepath.Join(dir, "ips"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ips", "custom.txt"), []byte("mine"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.Update(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, "latest", dir); err != nil {
			t.Fatalf("Update: %v", err)
		}
	})
	t.Run("existing-backup", func(t *testing.T) {
		run(t, func(dir string) {
			if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "old-marker.txt"), []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "3\n")
	})
	t.Run("existing-cancel", func(t *testing.T) {
		run(t, func(dir string) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}, "4\n")
	})
	t.Run("checksum-fail", func(t *testing.T) {
		fixed := newFixture(t, makeZip(t), func(f *fixture) { f.wrongSum = true })
		u := ui.New(os.Stderr)
		dl := downloader.New(
			downloader.WithHTTPClient(fixed.srv.Client()),
			downloader.WithAPIBaseURL(fixed.srv.URL),
			downloader.WithDownloadBaseURL(fixed.srv.URL),
			downloader.WithProgressSink(u.ProgressSink()),
		)
		inst := New(u, dl)
		dir := filepath.Join(t.TempDir(), "bgscan")
		err := inst.Install(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, "latest", dir, strings.NewReader(""))
		if err == nil {
			t.Fatal("expected error")
		}
		var se *StageError
		if !errors.As(err, &se) {
			t.Fatalf("expected StageError, got %v", err)
		}
		u.FailPanel(se.Op(), se.Reason(), se.Note())
		t.Log("cleanup done")
	})
}
