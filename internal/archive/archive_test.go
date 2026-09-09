package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "test.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTar(t *testing.T, files map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "test.tar")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func checkFiles(t *testing.T, dir string, want map[string]string) {
	t.Helper()

	for name, content := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != content {
			t.Errorf("%s = %q, want %q", name, string(got), content)
		}
	}
}

func TestExtractZip(t *testing.T) {
	files := map[string]string{
		"a.txt":     "hello",
		"b.txt":     "world",
		"sub/c.txt": "nested",
	}

	archivePath := writeZip(t, files)

	extractDir := filepath.Join(t.TempDir(), "extracted")
	if _, err := ExtractZip(archivePath, extractDir); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	checkFiles(t, extractDir, files)
}

func TestExtractTar(t *testing.T) {
	files := map[string]string{
		"x.txt": "alpha",
		"y.txt": "beta",
	}

	archivePath := writeTar(t, files)

	extractDir := filepath.Join(t.TempDir(), "extracted")
	if _, err := ExtractTar(archivePath, extractDir); err != nil {
		t.Fatalf("ExtractTar: %v", err)
	}
	checkFiles(t, extractDir, files)
}

func TestExtract_DispatchesByExtension(t *testing.T) {
	zipFiles := map[string]string{"a.txt": "hello"}
	tarFiles := map[string]string{"b.txt": "world"}

	zipPath := writeZip(t, zipFiles)
	tarPath := writeTar(t, tarFiles)

	zipDir := filepath.Join(t.TempDir(), "zip-out")
	if _, err := Extract(zipPath, zipDir); err != nil {
		t.Fatalf("Extract(zip): %v", err)
	}
	checkFiles(t, zipDir, zipFiles)

	tarDir := filepath.Join(t.TempDir(), "tar-out")
	if _, err := Extract(tarPath, tarDir); err != nil {
		t.Fatalf("Extract(tar): %v", err)
	}
	checkFiles(t, tarDir, tarFiles)
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
			dir := t.TempDir()
			archivePath := filepath.Join(dir, "evil.zip")
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
			if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			escapePath := filepath.Join(filepath.Dir(dir), "escape.txt")

			if _, err := ExtractZip(archivePath, dir); err == nil {
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

			if _, err := ExtractTar(archivePath, dir); err == nil {
				t.Fatal("expected path traversal rejection, got nil")
			}
			if _, err := os.Stat(escapePath); !os.IsNotExist(err) {
				t.Errorf("traversed file must not exist: %v", err)
			}
		})
	}
}
