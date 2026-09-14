package downloader

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFileChecksum_Valid(t *testing.T) {
	dir := t.TempDir()
	content := []byte("test content for checksum")
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	expected := fmt.Sprintf("%x", h)

	c := &client{}
	if err := c.VerifyFileChecksum(path, expected); err != nil {
		t.Fatalf("VerifyFileChecksum: %v", err)
	}
}

func TestVerifyFileChecksum_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	c := &client{}
	err := c.VerifyFileChecksum(path, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for invalid checksum")
	}
}

func TestVerifyFileChecksum_MissingFile(t *testing.T) {
	c := &client{}
	err := c.VerifyFileChecksum("/nonexistent/file", "abc")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFilenameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path/to/file.zip", "file.zip"},
		{"https://example.com/file.zip", "file.zip"},
		{"https://example.com/", "file"},
		{"https://example.com", "file"},
		{"::not a url::", "file"},
	}
	for _, tt := range tests {
		if got := filenameFromURL(tt.url); got != tt.want {
			t.Errorf("filenameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestNew_ReturnsNonNil(t *testing.T) {
	d := New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestMatchTokens(t *testing.T) {
	tests := []struct {
		text   string
		tokens []string
		want   bool
	}{
		{"tool-linux-amd64.zip", []string{"amd64"}, true},
		{"tool-linux-arm64.zip", []string{"amd64"}, false},
		{"tool-armv7-linux.zip", []string{"armv7", "arm32"}, true},
		{"tool-armv5-linux.zip", []string{"armv5", "armv6"}, false},
	}
	for _, tt := range tests {
		if got := matchTokens(tt.text, tt.tokens); got != tt.want {
			t.Errorf("matchTokens(%q, %v) = %v, want %v", tt.text, tt.tokens, got, tt.want)
		}
	}
}
