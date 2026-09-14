package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGoVersionSupported(t *testing.T) {
	tests := []struct {
		installed string
		min       string
		want      bool
	}{
		{"1.26.3", "1.26.3", true},
		{"1.26.4", "1.26.3", true},
		{"1.27.0", "1.26.3", true},
		{"1.26.2", "1.26.3", false},
		{"1.25.9", "1.26.3", false},
		{"2.0.0", "1.26.3", true},
	}
	for _, tt := range tests {
		if got := isGoVersionSupported(tt.installed, tt.min); got != tt.want {
			t.Errorf("isGoVersionSupported(%q, %q) = %v, want %v", tt.installed, tt.min, got, tt.want)
		}
	}
}

func TestCheckGoMod_Valid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module bgscan\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := checkGoMod(dir, "bgscan"); err != nil {
		t.Fatalf("checkGoMod: %v", err)
	}
}

func TestCheckGoMod_WrongModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module other\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := checkGoMod(dir, "bgscan")
	if err == nil {
		t.Fatal("expected error for wrong module name")
	}
}

func TestCheckGoMod_MissingFile(t *testing.T) {
	err := checkGoMod("/nonexistent", "bgscan")
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}

func TestCheckGoMod_NoModuleDecl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("go 1.26\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := checkGoMod(dir, "bgscan")
	if err == nil {
		t.Fatal("expected error for missing module declaration")
	}
}

func TestGetNDKPath_FromParam(t *testing.T) {
	dir := t.TempDir()
	path, err := GetNDKPath(dir)
	if err != nil {
		t.Fatalf("GetNDKPath: %v", err)
	}
	if path != dir {
		t.Errorf("got %q, want %q", path, dir)
	}
}

func TestGetNDKPath_FromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NDK_DIR", dir)

	path, err := GetNDKPath("")
	if err != nil {
		t.Fatalf("GetNDKPath: %v", err)
	}
	if path != dir {
		t.Errorf("got %q, want %q", path, dir)
	}
}

func TestGetNDKPath_Empty(t *testing.T) {
	t.Setenv("NDK_DIR", "")
	_, err := GetNDKPath("")
	if err == nil {
		t.Fatal("expected error for empty NDK path")
	}
}

func TestGetNDKPath_Nonexistent(t *testing.T) {
	_, err := GetNDKPath("/nonexistent/ndk")
	if err == nil {
		t.Fatal("expected error for nonexistent NDK path")
	}
}

func TestPrepareProjectFiles_CopiesIps(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "ips"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "ips", "default.txt"), []byte("ip data"), 0644); err != nil {
		t.Fatal(err)
	}

	c := &compiler{}
	if err := c.prepareProjectFiles(srcDir, destDir); err != nil {
		t.Fatalf("prepareProjectFiles: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "ips", "default.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(got) != "ip data" {
		t.Errorf("got %q, want %q", string(got), "ip data")
	}
}

func TestCopyAssets(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "assets", "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "assets", "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "assets", "sub", "nested.txt"), []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	c := &compiler{}
	if err := c.copyAssets(srcDir, destDir); err != nil {
		t.Fatalf("copyAssets: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "assets", "file.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "content" {
		t.Errorf("got %q, want %q", string(got), "content")
	}

	got, err = os.ReadFile(filepath.Join(destDir, "assets", "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("read nested file: %v", err)
	}
	if string(got) != "deep" {
		t.Errorf("got %q, want %q", string(got), "deep")
	}
}

func TestCopyAssets_NoAssetsDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	c := &compiler{}
	if err := c.copyAssets(srcDir, destDir); err != nil {
		t.Fatalf("copyAssets should not fail for missing assets dir: %v", err)
	}
}

func TestNew_ReturnsNonNil(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}
