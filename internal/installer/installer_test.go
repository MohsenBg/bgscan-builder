package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"bgscan-builder/internal/downloader"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

const fixtureTag = "v2.9.1"

var fixtureAssets = []string{
	"bgscan-android-arm64-v8a.zip",
	"bgscan-android-armeabi-v7a.zip",
	"bgscan-android-x86.zip",
	"bgscan-android-x86_64.zip",
	"bgscan-linux-32.zip",
	"bgscan-linux-64.zip",
	"bgscan-linux-arm32-v7a.zip",
	"bgscan-linux-arm64.zip",
	"bgscan-macos-64.zip",
	"bgscan-macos-arm64.zip",
	"bgscan-windows-64.zip",
	"bgscan-windows-arm64.zip",
}

// fixture is an in-process GitHub release end-to-end stand-in.
type fixture struct {
	srv *httptest.Server

	tag      string
	assets   []string
	body     []byte
	checksum string
	noSum    bool
	wrongSum bool
	badZip   bool
	status   int  // HTTP status served for asset downloads (0 = 200)
	slow     bool // stream /assets in small chunks to animate the progress bar

	mu      sync.Mutex
	apiHits []string
	dlHits  []string
}

func (f *fixture) hits() (api, dl []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.apiHits...), append([]string(nil), f.dlHits...)
}

func newFixture(t *testing.T, body []byte, configure func(*fixture)) *fixture {
	t.Helper()
	f := &fixture{tag: fixtureTag, assets: append([]string(nil), fixtureAssets...), body: body}
	if configure != nil {
		configure(f)
	}
	if f.badZip {
		f.body = []byte("this is not a zip archive")
	}
	if !f.noSum {
		sum := sha256.Sum256(f.body)
		if f.wrongSum {
			sum[0] ^= 0xff
		}
		f.checksum = fmt.Sprintf("%x  %s", sum, "bgscan-linux-64.zip")
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/"+Repository+"/releases/"):
			f.mu.Lock()
			f.apiHits = append(f.apiHits, r.URL.Path)
			f.mu.Unlock()
			fmt.Fprintf(w, `{"tag_name": %q, "assets": [`, f.tag)
			for i, name := range f.assets {
				if i > 0 {
					fmt.Fprint(w, ",")
				}
				fmt.Fprintf(w, `{"name": %q, "browser_download_url": %q}`, name, srv.URL+"/assets/"+name)
			}
			fmt.Fprint(w, "]}")
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			f.mu.Lock()
			f.dlHits = append(f.dlHits, r.URL.Path)
			f.mu.Unlock()
			if f.status != 0 {
				w.WriteHeader(f.status)
				return
			}
			if f.slow {
				body := f.body
				for len(body) > 0 {
					n := min(len(body), 32*1024)
					_, _ = w.Write(body[:n])
					body = body[n:]
					if fl, ok := w.(http.Flusher); ok {
						fl.Flush()
					}
					time.Sleep(8 * time.Millisecond)
				}
				return
			}
			_, _ = w.Write(f.body)
		case strings.Contains(r.URL.Path, "/checksum.txt"):
			f.mu.Lock()
			f.dlHits = append(f.dlHits, r.URL.Path)
			f.mu.Unlock()
			if f.noSum {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			fmt.Fprint(w, f.checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	f.srv = srv
	return f
}

func newTestInstaller(t *testing.T, f *fixture) (*Installer, *ui.UI, *bytes.Buffer) {
	t.Helper()
	dl := downloader.New(
		downloader.WithHTTPClient(f.srv.Client()),
		downloader.WithAPIBaseURL(f.srv.URL),
		downloader.WithDownloadBaseURL(f.srv.URL),
	)
	out := new(bytes.Buffer)
	u := ui.New(out)
	inst := New(u, dl)
	inst.now = func() time.Time { return time.Date(2026, 8, 29, 19, 15, 30, 0, time.UTC) }
	return inst, u, out
}

func makeZip(t *testing.T) []byte {
	return makeZipWith(t, "#!/bin/sh\necho bgscan\n", "v2-iran", "geoip-v2")
}

// makeZipWith builds a release zip whose contents can be pinned for
// update/merge tests.
func makeZipWith(t *testing.T, binContent, ipsContent, assetsContent string) []byte {
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

	if _, err := zw.CreateHeader(header("bgscan-linux-64/", 0o755)); err != nil {
		t.Fatal(err)
	}
	write("bgscan-linux-64/bgscan", 0o755, binContent)
	write("bgscan-linux-64/settings/config.toml", 0o644, "enabled = true\n")
	if ipsContent != "" {
		write("bgscan-linux-64/ips/iran.csv", 0o644, ipsContent)
	}
	if assetsContent != "" {
		write("bgscan-linux-64/assets/geoip.dat", 0o644, assetsContent)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func installTarget(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "bgscan")
}

func linuxAMD64() platform.Info {
	return platform.Info{OS: platform.Linux, Arch: platform.AMD64}
}

func contains(s []string, sub string) bool {
	for _, v := range s {
		if strings.Contains(v, sub) {
			return true
		}
	}
	return false
}

func TestInstall_FreshInstall(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader(""))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	binPath := filepath.Join(installDir, "bgscan")
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("installed binary not executable: %v", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(installDir, "settings", "config.toml")); err != nil {
		t.Errorf("installed settings missing: %v", err)
	}

	api, dl := f.hits()
	if !contains(api, "/repos/"+Repository+"/releases/latest") {
		t.Errorf("expected latest API hit, got %v", api)
	}
	if !contains(dl, "/"+Repository+"/releases/latest/download/checksum.txt") {
		t.Errorf("expected latest checksum hit, got %v", dl)
	}
}

func TestInstall_ExplicitVersionUsesTag(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), linuxAMD64(), "v1.0", installTarget(t), strings.NewReader(""))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	api, dl := f.hits()
	if !contains(api, "/repos/"+Repository+"/releases/tags/v1.0") {
		t.Errorf("expected tagged API hit, got %v", api)
	}
	if !contains(dl, "/"+Repository+"/releases/download/v1.0/checksum.txt") {
		t.Errorf("expected tagged checksum hit, got %v", dl)
	}
}

func TestInstall_ExistingInstallBackup(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)
	parent := filepath.Dir(installDir)

	if err := os.MkdirAll(filepath.Join(installDir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(installDir, "old-marker.txt")
	if err := os.WriteFile(marker, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Option 3: back up existing installation, then install the new version.
	err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("3\n"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	backup := filepath.Join(parent, "bgscan_bck_20260829_191530")
	if _, err := os.Stat(filepath.Join(backup, "old-marker.txt")); err != nil {
		t.Errorf("old installation not preserved in backup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "bgscan")); err != nil {
		t.Errorf("new installation missing: %v", err)
	}
}

func TestInstall_ExistingInstallRemove(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "old-marker.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Option 2: clean install — existing installation is removed.
	err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("2\n"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(installDir, "old-marker.txt")); !os.IsNotExist(err) {
		t.Errorf("old installation should have been removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "bgscan")); err != nil {
		t.Errorf("new installation missing: %v", err)
	}
}

func TestInstall_ExistingInstallCancel(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "old-marker.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Option 4: cancel.
	err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("4\n"))
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected ErrCancelled, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "old-marker.txt")); err != nil {
		t.Errorf("existing installation should be untouched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "bgscan")); !os.IsNotExist(err) {
		t.Errorf("no new installation should exist: %v", err)
	}
}

// TestInstall_ExistingInstallUpdateInPlace exercises option 1: update in
// place, preserving user-added ips/assets files.
func TestInstall_ExistingInstallUpdateInPlace(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	mustWrite(t, filepath.Join(installDir, "bgscan"), "old-bin")
	mustWrite(t, filepath.Join(installDir, "ips", "iran.csv"), "v1-iran")
	mustWrite(t, filepath.Join(installDir, "ips", "user-ips.txt"), "mine-ips")
	mustWrite(t, filepath.Join(installDir, "assets", "user-assets.txt"), "mine-assets")

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("1\n"))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Binary and release files refreshed.
	assertFileEquals(t, filepath.Join(installDir, "bgscan"), "#!/bin/sh\necho bgscan\n")
	assertFileEquals(t, filepath.Join(installDir, "ips", "iran.csv"), "v2-iran")
	// User files preserved, no backup created.
	assertFileEquals(t, filepath.Join(installDir, "ips", "user-ips.txt"), "mine-ips")
	assertFileEquals(t, filepath.Join(installDir, "assets", "user-assets.txt"), "mine-assets")
	if _, err := os.Stat(filepath.Join(filepath.Dir(installDir), "bgscan_bck_20260829_191530")); !os.IsNotExist(err) {
		t.Errorf("option 1 should not create a backup: %v", err)
	}
}

func TestInstall_UnsupportedPlatform(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, nil)
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), platform.Info{OS: platform.UnknownOS, Arch: platform.UnknownArch}, "latest", installTarget(t), strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "unsupported platform") {
		t.Fatalf("expected unsupported platform error, got %v", err)
	}
}

func TestInstall_MissingReleaseAsset(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, func(f *fixture) {
		f.assets = []string{"bgscan-windows-64.zip"} // hide linux assets
	})
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installTarget(t), strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "failed to resolve release asset") {
		t.Fatalf("expected resolve failure, got %v", err)
	}
}

func TestInstall_DownloadFailure(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, func(f *fixture) { f.status = http.StatusInternalServerError })
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installTarget(t), strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "failed to download release") {
		t.Fatalf("expected download failure, got %v", err)
	}
}

func TestInstall_ChecksumFailure(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, func(f *fixture) { f.wrongSum = true })
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installTarget(t), strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "failed to verify checksum") {
		t.Fatalf("expected checksum failure, got %v", err)
	}
}

func TestInstall_ChecksumUnavailableSkipsVerification(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, func(f *fixture) { f.noSum = true })
	inst, _, out := newTestInstaller(t, f)

	if err := inst.Install(context.Background(), linuxAMD64(), "latest", installTarget(t), strings.NewReader("")); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !strings.Contains(out.String(), "checksum unavailable") {
		t.Errorf("expected a checksum-unavailable notice, got output:\n%s", out.String())
	}
}

func TestInstall_ArchiveExtractionFailure(t *testing.T) {
	zipData := makeZip(t)
	f := newFixture(t, zipData, func(f *fixture) { f.badZip = true })
	inst, _, _ := newTestInstaller(t, f)

	err := inst.Install(context.Background(), linuxAMD64(), "latest", installTarget(t), strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "failed to extract archive") {
		t.Fatalf("expected extraction failure, got %v", err)
	}
}

func TestUniqueBackupName_Timestamped(t *testing.T) {
	inst := &Installer{now: func() time.Time { return time.Date(2026, 8, 29, 19, 15, 30, 0, time.UTC) }}
	parent := t.TempDir()
	installDir := filepath.Join(parent, "bgscan")

	got := inst.uniqueBackupName(installDir)
	want := filepath.Join(parent, "bgscan_bck_20260829_191530")
	if got != want {
		t.Errorf("backup = %q, want %q", got, want)
	}
}

func TestUniqueBackupName_CollisionSafe(t *testing.T) {
	inst := &Installer{now: func() time.Time { return time.Date(2026, 8, 29, 19, 15, 30, 0, time.UTC) }}
	parent := t.TempDir()
	installDir := filepath.Join(parent, "bgscan")

	// Simulate an existing backup with the exact timestamped name.
	if err := os.MkdirAll(filepath.Join(parent, "bgscan_bck_20260829_191530"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := inst.uniqueBackupName(installDir)
	want := filepath.Join(parent, "bgscan_bck_20260829_191530_1")
	if got != want {
		t.Errorf("collision backup = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(parent, "bgscan_bck_20260829_191530")); err != nil {
		t.Errorf("existing backup must not be removed: %v", err)
	}
}
