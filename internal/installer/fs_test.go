package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncTree_OverwritesReleaseFilesButKeepsUserFiles(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")

	mustWrite(t, filepath.Join(src, "ips", "iran.csv"), "v2-iran")
	mustWrite(t, filepath.Join(src, "ips", "ospf.csv"), "new-file")

	mustWrite(t, filepath.Join(dst, "ips", "iran.csv"), "v1-iran")
	mustWrite(t, filepath.Join(dst, "ips", "user-ips.txt"), "mine")

	if err := syncTree(src, dst); err != nil {
		t.Fatalf("syncTree: %v", err)
	}

	assertFileEquals(t, filepath.Join(dst, "ips", "iran.csv"), "v2-iran")
	assertFileEquals(t, filepath.Join(dst, "ips", "ospf.csv"), "new-file")
	assertFileEquals(t, filepath.Join(dst, "ips", "user-ips.txt"), "mine")
}

func TestSyncTree_MissingSourceNoOp(t *testing.T) {
	dst := t.TempDir()
	if err := syncTree(filepath.Join(t.TempDir(), "nope"), dst); err != nil {
		t.Fatalf("syncTree on missing source should be a no-op: %v", err)
	}
}

func TestReplaceFile_OverwritesDestination(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "new-bin")
	dst := filepath.Join(root, "dst", "bgscan")

	mustWrite(t, src, "new")
	mustWrite(t, dst, "old")

	if err := replaceFile(src, dst); err != nil {
		t.Fatalf("replaceFile: %v", err)
	}
	assertFileEquals(t, dst, "new")
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should be consumed: %v", err)
	}
}

func TestCopyFile_PreservesMode(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "bin")
	dst := filepath.Join(root, "out", "bin")
	mustWrite(t, src, "x")
	if err := os.Chmod(src, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("copied file lost execute permission: %v", info.Mode())
	}
}
