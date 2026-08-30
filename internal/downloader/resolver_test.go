package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"bgscan-builder/internal/platform"
)

// releaseAssets mirrors the asset names published by MohsenBg/bgscan.
var releaseAssets = []string{
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

const (
	testRepo     = "MohsenBg/bgscan"
	testBinary   = "bgscan"
	testTag      = "v2.9.1"
	testChecksum = "0f4d0e1f06d080feb12f32a4a9087d2163e6c2d19e19bbd5d94a4bf5b57f2c35  bgscan-linux-64.zip"
	testHash     = "0f4d0e1f06d080feb12f32a4a9087d2163e6c2d19e19bbd5d94a4bf5b57f2c35"
)

// apiFixture is an in-process stand-in for the GitHub API and release
// download endpoints.
type apiFixture struct {
	tag      string
	assets   []string
	checksum string
	hasSum   bool
	mu       sync.Mutex
	paths    []string
}

// apiHits returns a copy of the recorded request paths.
func (f *apiFixture) apiHits() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

func newAPI(t *testing.T, configure func(*apiFixture)) (*apiFixture, *httptest.Server) {
	t.Helper()
	f := &apiFixture{
		tag:      testTag,
		assets:   append([]string(nil), releaseAssets...),
		checksum: testChecksum,
		hasSum:   true,
	}
	if configure != nil {
		configure(f)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		f.mu.Unlock()

		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/MohsenBg/bgscan/releases/"):
			_, _ = fmt.Fprintf(w, `{"tag_name": %q, "assets": [`, f.tag)
			for i, name := range f.assets {
				if i > 0 {
					_, _ = fmt.Fprint(w, ",")
				}
				_, _ = fmt.Fprintf(w, `{"name": %q, "browser_download_url": %q}`, name, srv.URL+"/assets/"+name)
			}
			_, _ = fmt.Fprint(w, "]}")
		case strings.Contains(r.URL.Path, "checksum.txt"):
			if !f.hasSum {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = fmt.Fprint(w, f.checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func testClient(srv *httptest.Server) Downloader {
	return New(
		WithHTTPClient(srv.Client()),
		WithAPIBaseURL(srv.URL),
		WithDownloadBaseURL(srv.URL),
	)
}

func TestResolveReleaseAsset_PlatformMapping(t *testing.T) {
	cases := []struct {
		info platform.Info
		want string
	}{
		{platform.Info{OS: platform.Linux, Arch: platform.ARM64}, "bgscan-linux-arm64.zip"},
		{platform.Info{OS: platform.Linux, Arch: platform.ARM32}, "bgscan-linux-arm32-v7a.zip"},
		{platform.Info{OS: platform.Linux, Arch: platform.AMD64}, "bgscan-linux-64.zip"},
		{platform.Info{OS: platform.Linux, Arch: platform.AMD32}, "bgscan-linux-32.zip"},
		{platform.Info{OS: platform.MacOS, Arch: platform.ARM64}, "bgscan-macos-arm64.zip"},
		{platform.Info{OS: platform.MacOS, Arch: platform.AMD64}, "bgscan-macos-64.zip"},
		{platform.Info{OS: platform.Android, Arch: platform.ARM64}, "bgscan-android-arm64-v8a.zip"},
		{platform.Info{OS: platform.Android, Arch: platform.ARM32}, "bgscan-android-armeabi-v7a.zip"},
		{platform.Info{OS: platform.Android, Arch: platform.AMD64}, "bgscan-android-x86_64.zip"},
		{platform.Info{OS: platform.Android, Arch: platform.AMD32}, "bgscan-android-x86.zip"},
	}

	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	for _, tc := range cases {
		t.Run(tc.info.String(), func(t *testing.T) {
			asset, err := dl.ResolveReleaseAsset(context.Background(), tc.info, testRepo, testBinary, "latest")
			if err != nil {
				t.Fatalf("ResolveReleaseAsset: %v", err)
			}
			if asset.Version != testTag {
				t.Errorf("version = %q, want %q", asset.Version, testTag)
			}
			if asset.Name != tc.want {
				t.Errorf("asset = %q, want %q", asset.Name, tc.want)
			}
		})
	}

	for _, hit := range f.apiHits() {
		if strings.Contains(hit, "/releases/tags/") {
			t.Errorf("unexpected tagged release request for latest resolution: %s", hit)
		}
	}
}

func TestResolveReleaseAsset_EmptyVersionUsesLatest(t *testing.T) {
	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	_, err := dl.ResolveReleaseAsset(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, testRepo, testBinary, "")
	if err != nil {
		t.Fatalf("ResolveReleaseAsset: %v", err)
	}
	assertHit(t, f, "/repos/MohsenBg/bgscan/releases/latest")
}

func TestResolveReleaseAsset_LatestKeywordUsesLatest(t *testing.T) {
	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	_, err := dl.ResolveReleaseAsset(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, testRepo, testBinary, "latest")
	if err != nil {
		t.Fatalf("ResolveReleaseAsset: %v", err)
	}
	assertHit(t, f, "/repos/MohsenBg/bgscan/releases/latest")
}

func TestResolveReleaseAsset_ExplicitVersionUsesTag(t *testing.T) {
	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	_, err := dl.ResolveReleaseAsset(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, testRepo, testBinary, "v1.0")
	if err != nil {
		t.Fatalf("ResolveReleaseAsset: %v", err)
	}
	assertHit(t, f, "/repos/MohsenBg/bgscan/releases/tags/v1.0")
}

func TestResolveReleaseAsset_MissingAsset(t *testing.T) {
	_, srv := newAPI(t, func(f *apiFixture) {
		f.assets = []string{"bgscan-windows-64.zip"} // hide linux assets
	})
	dl := testClient(srv)

	_, err := dl.ResolveReleaseAsset(context.Background(), platform.Info{OS: platform.Linux, Arch: platform.AMD64}, testRepo, testBinary, "latest")
	if err == nil || !strings.Contains(err.Error(), "no matching asset") {
		t.Fatalf("expected no-matching-asset error, got %v", err)
	}
}

func TestResolveReleaseAsset_UnsupportedPlatform(t *testing.T) {
	_, srv := newAPI(t, nil)
	dl := testClient(srv)

	_, err := dl.ResolveReleaseAsset(context.Background(), platform.Info{OS: platform.UnknownOS, Arch: platform.UnknownArch}, testRepo, testBinary, "latest")
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestFetchChecksum_Latest(t *testing.T) {
	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	got, err := dl.FetchChecksum(context.Background(), testRepo, "bgscan-linux-64.zip", "latest")
	if err != nil {
		t.Fatalf("FetchChecksum: %v", err)
	}
	if got != testHash {
		t.Errorf("hash = %q, want %q", got, testHash)
	}
	assertHit(t, f, "/MohsenBg/bgscan/releases/latest/download/checksum.txt")
}

func TestFetchChecksum_ExplicitVersion(t *testing.T) {
	f, srv := newAPI(t, nil)
	dl := testClient(srv)

	if _, err := dl.FetchChecksum(context.Background(), testRepo, "bgscan-linux-64.zip", "v1.0"); err != nil {
		t.Fatalf("FetchChecksum: %v", err)
	}
	assertHit(t, f, "/MohsenBg/bgscan/releases/download/v1.0/checksum.txt")
}

func TestFetchChecksum_Unavailable(t *testing.T) {
	f, srv := newAPI(t, func(f *apiFixture) { f.hasSum = false })
	dl := testClient(srv)

	_, err := dl.FetchChecksum(context.Background(), testRepo, "bgscan-linux-64.zip", "latest")
	if !errors.Is(err, ErrChecksumUnavailable) {
		t.Fatalf("expected ErrChecksumUnavailable, got %v", err)
	}
	_ = f
}

func assertHit(t *testing.T, f *apiFixture, want string) {
	t.Helper()
	for _, p := range f.apiHits() {
		if p == want {
			return
		}
	}
	t.Errorf("server did not receive %q (got %v)", want, f.apiHits())
}
