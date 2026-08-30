package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdate_FreshInstall(t *testing.T) {
	f := newFixture(t, makeZip(t), nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	if err := inst.Update(context.Background(), linuxAMD64(), "latest", installDir); err != nil {
		t.Fatalf("Update: %v", err)
	}

	for _, rel := range []string{"bgscan", "ips/iran.csv", "assets/geoip.dat", "settings/config.toml"} {
		if _, err := os.Stat(filepath.Join(installDir, rel)); err != nil {
			t.Errorf("fresh-updated install missing %s: %v", rel, err)
		}
	}
}

func TestUpdate_ExistingUpdatesBinaryAndPreservesUserFiles(t *testing.T) {
	f := newFixture(t, makeZip(t), nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	// Simulate a previous installation with release files plus user additions.
	mustWrite(t, filepath.Join(installDir, "bgscan"), "old-bin")
	mustWrite(t, filepath.Join(installDir, "ips", "iran.csv"), "v1-iran")
	mustWrite(t, filepath.Join(installDir, "ips", "user-ips.txt"), "mine-ips")
	mustWrite(t, filepath.Join(installDir, "assets", "geoip.dat"), "geoip-v1")
	mustWrite(t, filepath.Join(installDir, "assets", "user-assets.txt"), "mine-assets")

	if err := inst.Update(context.Background(), linuxAMD64(), "latest", installDir); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertFileEquals(t, filepath.Join(installDir, "bgscan"), "#!/bin/sh\necho bgscan\n")
	assertFileEquals(t, filepath.Join(installDir, "ips", "iran.csv"), "v2-iran")
	assertFileEquals(t, filepath.Join(installDir, "ips", "user-ips.txt"), "mine-ips")
	assertFileEquals(t, filepath.Join(installDir, "assets", "geoip.dat"), "geoip-v2")
	assertFileEquals(t, filepath.Join(installDir, "assets", "user-assets.txt"), "mine-assets")
}

func TestInstallChoice3_Backup(t *testing.T) {
	f := newFixture(t, makeZip(t), nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	// Previous installation with user customisations.
	mustWrite(t, filepath.Join(installDir, "ips", "iran.csv"), "v1-iran")
	mustWrite(t, filepath.Join(installDir, "ips", "user-ips.txt"), "mine-ips")
	mustWrite(t, filepath.Join(installDir, "assets", "geoip.dat"), "geoip-v1")
	mustWrite(t, filepath.Join(installDir, "assets", "user-assets.txt"), "mine-assets")
	mustWrite(t, filepath.Join(installDir, "top-marker.txt"), "keep")

	if err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("3\n")); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The complete previous installation is preserved in a timestamped backup.
	backup := filepath.Join(filepath.Dir(installDir), "bgscan_bck_20260829_191530")
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	assertFileEquals(t, filepath.Join(backup, "ips", "iran.csv"), "v1-iran")
	assertFileEquals(t, filepath.Join(backup, "ips", "user-ips.txt"), "mine-ips")
	assertFileEquals(t, filepath.Join(backup, "assets", "user-assets.txt"), "mine-assets")
	assertFileEquals(t, filepath.Join(backup, "top-marker.txt"), "keep")

	// The new install gets the fresh release files.
	assertFileEquals(t, filepath.Join(installDir, "ips", "iran.csv"), "v2-iran")
	assertFileEquals(t, filepath.Join(installDir, "assets", "geoip.dat"), "geoip-v2")
}

func TestInstallChoice3_NoUserDirs(t *testing.T) {
	f := newFixture(t, makeZip(t), nil)
	inst, _, _ := newTestInstaller(t, f)
	installDir := installTarget(t)

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(installDir, "top-marker.txt"), "keep")

	if err := inst.Install(context.Background(), linuxAMD64(), "latest", installDir, strings.NewReader("3\n")); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(installDir, "bgscan")); err != nil {
		t.Errorf("new installation missing: %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, string(got), want)
	}
}
