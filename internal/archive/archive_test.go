package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func createTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}
	return path
}

func TestZipCompressor_RoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	targetDir := t.TempDir()

	createTestFile(t, srcDir, "a.txt", "hello")
	createTestFile(t, srcDir, "b.txt", "world")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	createTestFile(t, filepath.Join(srcDir, "sub"), "c.txt", "nested")

	z := &ZipCompressor{}

	archivePath, err := z.Compress(srcDir, targetDir)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if filepath.Ext(archivePath) != ".zip" {
		t.Fatalf("expected .zip extension, got %s", archivePath)
	}

	extractDir := filepath.Join(t.TempDir(), "extracted")
	_, err = z.Decompress(archivePath, extractDir)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}

	for _, tc := range []struct {
		file string
		want string
	}{
		{"a.txt", "hello"},
		{"b.txt", "world"},
		{"sub/c.txt", "nested"},
	} {
		got, err := os.ReadFile(filepath.Join(extractDir, tc.file))
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s = %q, want %q", tc.file, string(got), tc.want)
		}
	}
}

func TestTarArchiver_RoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	targetDir := t.TempDir()

	createTestFile(t, srcDir, "x.txt", "alpha")
	createTestFile(t, srcDir, "y.txt", "beta")

	ta := &TarArchiver{}

	archivePath, err := ta.Compress(srcDir, targetDir)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if filepath.Ext(archivePath) != ".tar" {
		t.Fatalf("expected .tar extension, got %s", archivePath)
	}

	extractDir := filepath.Join(t.TempDir(), "extracted")
	_, err = ta.Decompress(archivePath, extractDir)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}

	for _, tc := range []struct {
		file string
		want string
	}{
		{"x.txt", "alpha"},
		{"y.txt", "beta"},
	} {
		got, err := os.ReadFile(filepath.Join(extractDir, tc.file))
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s = %q, want %q", tc.file, string(got), tc.want)
		}
	}
}

func TestCreateArchiver_UnsupportedFormat(t *testing.T) {
	_, err := CreateArchiver(ArchiveFormat(99))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestArchiveFormat_String(t *testing.T) {
	tests := []struct {
		f    ArchiveFormat
		want string
	}{
		{ArchiveZIP, "zip"},
		{ArchiveTAR, "tar"},
		{ArchiveFormat(99), "ArchiveFormat(99)"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("ArchiveFormat(%d).String() = %q, want %q", int(tt.f), got, tt.want)
		}
	}
}

func TestZipCompressor_Format(t *testing.T) {
	z := &ZipCompressor{}
	if got := z.Format(); got != "zip" {
		t.Errorf("Format() = %q, want %q", got, "zip")
	}
}

func TestTarArchiver_Format(t *testing.T) {
	ta := &TarArchiver{}
	if got := ta.Format(); got != "tar" {
		t.Errorf("Format() = %q, want %q", got, "tar")
	}
}

func TestZip_RejectsPathTraversal(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"parent", "../escape.txt"},
		{"grandparent", "../../escape.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			w, err := zw.Create(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("evil")); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			archivePath := filepath.Join(dir, "evil.zip")
			if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			escapePath := filepath.Join(filepath.Dir(dir), "escape.txt")

			z := &ZipCompressor{}
			if _, err := z.Decompress(archivePath, dir); err == nil {
				t.Fatal("expected path traversal rejection, got nil")
			}
			if _, err := os.Stat(escapePath); !os.IsNotExist(err) {
				t.Errorf("traversed file must not exist: %v", err)
			}
		})
	}
}

func TestTar_RejectsPathTraversal(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"parent", "../escape.txt"},
		{"grandparent", "../../escape.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			if err := tw.WriteHeader(&tar.Header{
				Name: tt.path,
				Mode: 0o600,
				Size: int64(len("evil")),
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte("evil")); err != nil {
				t.Fatal(err)
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			archivePath := filepath.Join(dir, "evil.tar")
			if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			escapePath := filepath.Join(filepath.Dir(dir), "escape.txt")

			ta := &TarArchiver{}
			if _, err := ta.Decompress(archivePath, dir); err == nil {
				t.Fatal("expected path traversal rejection, got nil")
			}
			if _, err := os.Stat(escapePath); !os.IsNotExist(err) {
				t.Errorf("traversed file must not exist: %v", err)
			}
		})
	}
}
