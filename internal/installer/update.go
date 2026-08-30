package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"bgscan-builder/internal/platform"
)

// Update refreshes an existing installation without overwriting user files:
// the bgscan binary and the release-provided ips/assets are replaced, while
// files the user added to ips/assets are preserved. When no installation
// exists, a fresh copy is installed instead.
func (in *Installer) Update(ctx context.Context, target platform.Info, version, installDir string) error {
	asset, err := in.detectAndResolve(ctx, target, version)
	if err != nil {
		return err
	}

	existing := pathIsDir(installDir)
	if existing {
		in.ui.Notice("Existing installation found — updating in place")
	} else {
		in.ui.Notice("No existing installation detected — installing a fresh copy")
	}

	tmpDir, topDir, err := in.fetchRelease(ctx, asset, version, installDir, "Updating")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if !existing {
		if err := os.Rename(topDir, installDir); err != nil {
			return in.stageErr("Installation", "install release", err)
		}
	} else {
		if err := in.applyUpdate(topDir, installDir); err != nil {
			return err
		}
	}

	if err := in.makeExecutables(installDir); err != nil {
		return in.stageErr("Installation", "finalise permissions", err)
	}
	in.ui.Success("Update complete")

	verb := "is up to date"
	if !existing {
		verb = "installed"
	}
	in.successPanel(fmt.Sprintf("%s %s %s", in.Binary, asset.Version, verb), installDir, target)
	return nil
}

// applyUpdate merges the freshly extracted release into an existing
// installation, touching only the binary and the ips/assets trees.
func (in *Installer) applyUpdate(topDir, installDir string) error {
	// Replace the bgscan binary.
	srcBin := filepath.Join(topDir, in.Binary)
	if info, err := os.Stat(srcBin); err == nil && !info.IsDir() {
		if err := replaceFile(srcBin, filepath.Join(installDir, in.Binary)); err != nil {
			return in.stageErr("Installation", "replace the bgscan binary", err)
		}
	}

	// Merge release-provided ips and assets, keeping user-added files.
	for _, name := range userDataDirs {
		src := filepath.Join(topDir, name)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return in.stageErr("Installation", "stat "+name, err)
		}
		if err := syncTree(src, filepath.Join(installDir, name)); err != nil {
			return in.stageErr("Installation", "update "+name, err)
		}
		in.ui.Muted("  " + name + " updated")
	}
	return nil
}

func pathIsDir(p string) bool {
	info, err := os.Lstat(p)
	return err == nil && info.IsDir()
}
