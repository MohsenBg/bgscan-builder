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

func TestGetFilename_FromURL(t *testing.T) {
	name, err := getFilename("https://example.com/path/to/file.zip", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if name != "file.zip" {
		t.Errorf("got %q, want %q", name, "file.zip")
	}
}

func TestGetFilename_FromDestPath(t *testing.T) {
	name, err := getFilename("https://example.com/file.zip", filepath.Join(t.TempDir(), "output.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if name != "output.bin" {
		t.Errorf("got %q, want %q", name, "output.bin")
	}
}

func TestGetFilename_DirDestination(t *testing.T) {
	dir := t.TempDir()
	name, err := getFilename("https://example.com/file.zip", dir+string(os.PathSeparator))
	if err != nil {
		t.Fatal(err)
	}
	if name != "file.zip" {
		t.Errorf("got %q, want %q", name, "file.zip")
	}
}

func TestResolveFilenameConflict_NoConflict(t *testing.T) {
	dir := t.TempDir()
	name, err := resolveFilenameConflict(dir, "newfile.txt")
	if err != nil {
		t.Fatal(err)
	}
	if name != "newfile.txt" {
		t.Errorf("got %q, want %q", name, "newfile.txt")
	}
}

func TestResolveFilenameConflict_WithConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	name, err := resolveFilenameConflict(dir, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if name != "file_1.txt" {
		t.Errorf("got %q, want %q", name, "file_1.txt")
	}
}

func TestResolveFilenameConflict_MultipleConflicts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file_1.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	name, err := resolveFilenameConflict(dir, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if name != "file_2.txt" {
		t.Errorf("got %q, want %q", name, "file_2.txt")
	}
}

func TestResolveFilenameConflict_EmptyDir(t *testing.T) {
	name, err := resolveFilenameConflict("/nonexistent-dir", "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if name != "file.txt" {
		t.Errorf("got %q, want %q", name, "file.txt")
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
		{"xray-linux-amd64.zip", []string{"amd64"}, true},
		{"xray-linux-arm64.zip", []string{"amd64"}, false},
		{"xray-armv7-linux.zip", []string{"armv7", "arm32"}, true},
		{"xray-armv5-linux.zip", []string{"armv5", "armv6"}, false},
	}
	for _, tt := range tests {
		if got := matchTokens(tt.text, tt.tokens); got != tt.want {
			t.Errorf("matchTokens(%q, %v) = %v, want %v", tt.text, tt.tokens, got, tt.want)
		}
	}
}
