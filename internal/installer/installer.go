// Package installer implements a native Go replacement for the shell-based
// bgscan installer. It reuses the platform detection, release resolver,
// downloader, checksum verification, archive extraction, and terminal UI
// components from the rest of the codebase.
package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bgscan-builder/internal/archive"
	"bgscan-builder/internal/downloader"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

const (
	// Repository is the GitHub owner/repo that publishes bgscan releases.
	Repository = "MohsenBg/bgscan"
	// BinaryName is the release asset prefix and the installed binary name.
	BinaryName = "bgscan"
)

// userDataDirs are the release directories that may carry user-added files and
// must therefore be merged rather than replaced during an update.
var userDataDirs = []string{"ips", "assets", "settings"}

// ErrCancelled is returned when the user aborts an existing-installation
// decision, leaving the previous installation untouched.
var ErrCancelled = errors.New("installation cancelled")

// StageError identifies which installer stage failed, what failed, and
// whether any on-disk installation state was modified before the failure.
type StageError struct {
	Stage    string
	Action   string
	err      error
	Modified bool
	Backup   string
}

// Error implements the error interface, keeping the "failed to <action>:" shape.
func (e *StageError) Error() string {
	if e.err == nil {
		return "failed to " + e.Action
	}
	return "failed to " + e.Action + ": " + e.err.Error()
}

// Unwrap exposes the underlying cause for errors.Is/As chains.
func (e *StageError) Unwrap() error { return e.err }

// Op is the "Could not …" headline for the failure panel.
func (e *StageError) Op() string { return "Could not " + e.Action }

// Reason is the short underlying cause.
func (e *StageError) Reason() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

// Note summarises what happened to any previous installation.
func (e *StageError) Note() string {
	switch {
	case e.Backup != "":
		return fmt.Sprintf("Your previous installation is preserved at %s.", e.Backup)
	case e.Modified:
		return "Your previous installation was removed, but the new install did not complete."
	default:
		return "Nothing was changed. Your existing installation is untouched."
	}
}

// provisionMode decides how an extracted release is placed into installDir.
type provisionMode int

const (
	provFresh  provisionMode = iota // no prior installation, atomic rename
	provUpdate                      // merge in place, keep user ips/assets
	provRemove                      // clean install: wipe first, then fresh
	provBackup                      // timestamped backup, then fresh
)

// Installer drives the install/update pipelines using shared platform,
// downloader, archive, and UI abstractions.
type Installer struct {
	ui     *ui.UI
	dl     downloader.Downloader
	Repo   string
	Binary string
	now    func() time.Time

	modified   bool
	backupPath string
	provMode   provisionMode
}

// New returns a fully wired Installer. The downloader is provided by the
// caller (e.g. configured with a progress sink).
func New(u *ui.UI, dl downloader.Downloader) *Installer {
	return &Installer{
		ui:     u,
		dl:     dl,
		Repo:   Repository,
		Binary: BinaryName,
		now:    time.Now,
	}
}

// Install detects the host platform, resolves the latest released bgscan
// asset, verifies and downloads it, and installs it into installDir.
// When installDir already exists, the user is prompted interactively (via
// input) for update, clean install, backup, or cancellation.
func (in *Installer) Install(ctx context.Context, target platform.Info, version, installDir string, input io.Reader) error {
	asset, err := in.detectAndResolve(ctx, target, version)
	if err != nil {
		return err
	}

	// Handle an existing installation before any download side effects.
	if pathExists(installDir) {
		if err := in.reconcileExistingInstall(installDir, input); err != nil {
			return err
		}
	}

	tmpDir, topDir, err := in.fetchRelease(ctx, asset, version, installDir, "Installing")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	switch in.provMode {
	case provUpdate:
		// Merge the new binary and ips/assets into the existing installation,
		// preserving any files the user added to ips/assets.
		if err := in.applyUpdate(topDir, installDir); err != nil {
			return err
		}
	default:
		// provFresh, provRemove and provBackup all leave installDir vacant.
		if err := os.Rename(topDir, installDir); err != nil {
			return in.stageErr("Installation", "install release", err)
		}
	}

	if err := in.makeExecutables(installDir); err != nil {
		return in.stageErr("Installation", "finalise permissions", err)
	}
	in.ui.Success("Installation complete")

	in.successPanel(fmt.Sprintf("%s %s installed successfully", in.Binary, asset.Version), installDir, target)
	return nil
}

// detectAndResolve validates the platform and prints the System and Release
// sections, returning the release asset selected for it.
func (in *Installer) detectAndResolve(ctx context.Context, target platform.Info, version string) (downloader.ReleaseAsset, error) {
	if !isSupported(target) {
		return downloader.ReleaseAsset{}, in.unsupported(target)
	}

	in.ui.Section("System")
	in.ui.Row("OS", target.OS.String())
	in.ui.Row("Architecture", target.Arch.String())
	in.ui.Row("Platform", target.String())
	in.ui.Success("System ready")

	in.ui.Section("Release")
	asset, err := in.dl.ResolveReleaseAsset(ctx, target, in.Repo, in.Binary, version)
	if err != nil {
		return downloader.ReleaseAsset{}, in.stageErr("Release", "resolve release asset", err)
	}
	in.ui.Row("Version", asset.Version)
	in.ui.Row("Asset", asset.Name)
	in.ui.Success("Release found")
	return asset, nil
}

// reconcileExistingInstall offers the user a choice when a previous
// installation occupies installDir.
func (in *Installer) reconcileExistingInstall(installDir string, input io.Reader) error {
	in.ui.Notice("Existing installation found")
	in.ui.Rawln("   " + installDir)
	in.ui.Rawln("")
	in.ui.Muted("What would you like to do?")
	in.ui.Rawln("   [1]  Update installation (keeps your ips/assets/settings)")
	in.ui.Rawln("   [2]  Clean install (removes existing installation)")
	in.ui.Rawln("   [3]  Back up existing installation and install new version")
	in.ui.Rawln("   [4]  Cancel")
	in.ui.Raw("   Choice: ")

	switch readChoice(input) {
	case 1:
		in.provMode = provUpdate
		in.ui.Muted("Updating in place — your ips/assets files will be kept")
	case 2:
		in.provMode = provRemove
		in.ui.Section("Removing old installation")
		if err := os.RemoveAll(installDir); err != nil {
			return in.stageErr("Installation", "remove the old installation", err)
		}
		in.modified = true
		in.ui.Success("Previous installation removed")
	case 3:
		in.provMode = provBackup
		in.ui.Section("Creating backup")
		backup := in.uniqueBackupName(installDir)
		in.ui.Muted("   " + installDir)
		in.ui.Muted("     ↓")
		in.ui.Rawln("   " + backup)
		// Atomic move of the complete previous installation.
		if err := os.Rename(installDir, backup); err != nil {
			return in.stageErr("Installation", "create backup", err)
		}
		in.backupPath = backup
		in.modified = true
		in.ui.Success("Backup created: " + backup)
	default:
		in.ui.Notice("Installation cancelled, nothing changed")
		return ErrCancelled
	}
	return nil
}

// fetchRelease downloads, verifies, and extracts the release asset into a
// temporary directory under the install parent (same filesystem, so the final
// rename is atomic). It returns the temp root and the extracted top directory.
func (in *Installer) fetchRelease(
	ctx context.Context,
	asset downloader.ReleaseAsset,
	version, installDir, extractTitle string,
) (tmpDir, topDir string, err error) {
	parent := filepath.Dir(installDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", in.stageErr("Installation", "prepare the install directory", err)
	}

	in.ui.Section("Download")
	in.ui.Muted("Downloading " + asset.Name)
	tmpDir, err = os.MkdirTemp(parent, ".bgscan-install-*")
	if err != nil {
		return "", "", in.stageErr("Download", "stage the download", err)
	}

	archivePath, err := in.dl.DownloadFile(ctx, asset.URL, tmpDir)
	if err != nil {
		return "", "", in.stageErr("Download", "download release", err)
	}
	in.ui.Success("Download complete")

	in.ui.Section("Verification")
	sum, err := in.dl.FetchChecksum(ctx, in.Repo, asset.Name, version)
	switch {
	case errors.Is(err, downloader.ErrChecksumUnavailable):
		in.ui.Notice("checksum unavailable for this release; verification skipped")
	case err != nil:
		return "", "", in.stageErr("Verification", "fetch checksum", err)
	default:
		if err := in.dl.VerifyFileChecksum(archivePath, sum); err != nil {
			return "", "", in.stageErr("Verification", "verify checksum", err)
		}
		in.ui.Success("Checksum verified")
	}

	in.ui.Section(extractTitle)
	extractDir := filepath.Join(tmpDir, "extracted")
	archiver, err := archive.CreateArchiver(archive.ArchiveZIP)
	if err != nil {
		return "", "", in.stageErr("Installation", "initialise the archive extractor", err)
	}
	if _, err := archiver.Decompress(archivePath, extractDir); err != nil {
		return "", "", in.stageErr("Installation", "extract archive", err)
	}
	in.ui.Success("Archive extracted")

	topDir, err = singleTopDir(extractDir)
	if err != nil {
		return "", "", in.stageErr("Installation", "locate the extracted release", err)
	}
	return tmpDir, topDir, nil
}

// successPanel renders the final confirmation block.
func (in *Installer) successPanel(header, installDir string, target platform.Info) {
	in.ui.Rawln("")
	in.ui.Divider()
	in.ui.Success(header)
	in.ui.Rawln("")
	in.ui.Row("Location", installDir)
	in.ui.Row("Platform", target.String())
	in.ui.Rawln("")
	in.ui.Muted("   Get started")
	in.ui.Rawln("     cd " + installDir)
	in.ui.Rawln("     ./" + in.Binary)
	in.ui.Rawln("")
	in.ui.Divider()
	in.ui.Rawln("")
}

// uniqueBackupName builds a timestamped backup path (bgscan_bck_YYYYMMDD_HHMMSS)
// that never points to an existing path, appending a suffix if needed.
func (in *Installer) uniqueBackupName(installDir string) string {
	parent := filepath.Dir(installDir)
	base := filepath.Base(installDir)
	ts := in.now().Format("20060102_150405")

	backup := filepath.Join(parent, fmt.Sprintf("%s_bck_%s", base, ts))
	for i := 1; pathExists(backup); i++ {
		backup = filepath.Join(parent, fmt.Sprintf("%s_bck_%s_%d", base, ts, i))
	}
	return backup
}

// stageErr wraps a failure with structured context for the caller's panel.
func (in *Installer) stageErr(stage, action string, err error) error {
	return &StageError{
		Stage:    stage,
		Action:   action,
		err:      err,
		Modified: in.modified,
		Backup:   in.backupPath,
	}
}

func (in *Installer) unsupported(target platform.Info) error {
	return &StageError{
		Stage:  "System",
		Action: "determine a supported platform",
		err: fmt.Errorf(
			"unsupported platform: %s (open an issue: https://github.com/%s/issues)",
			target, in.Repo,
		),
	}
}

// readChoice reads the user's menu selection from input. Any unreadable or
// invalid input resolves to 0 (cancel).
func readChoice(input io.Reader) int {
	if input == nil {
		input = os.Stdin
	}
	scanner := bufio.NewScanner(input)
	if !scanner.Scan() {
		return 0
	}
	choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return 0
	}
	return choice
}

// isSupported restricts installs to platforms the release assets cover:
// Linux, macOS, Android (Termux), and Windows.
func isSupported(info platform.Info) bool {
	fmt.Println(info.OS)
	switch info.OS {
	case platform.Linux, platform.MacOS, platform.Android, platform.Windows:
	default:
		return false
	}
	switch info.Arch {
	case platform.ARM64, platform.ARM32, platform.AMD64, platform.AMD32:
	default:
		return false
	}
	return true
}

// singleTopDir finds the single top-level directory inside an extracted
// archive, mirroring the shell installer's archive structure handling.
func singleTopDir(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			return filepath.Join(extractDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no top-level directory found")
}

// makeExecutables grants execute permissions to installed binaries whose name
// carries the application prefix (e.g. bgscan on POSIX).
func (in *Installer) makeExecutables(installDir string) error {
	return filepath.WalkDir(installDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasPrefix(d.Name(), in.Binary) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.Chmod(path, info.Mode()|0o111)
	})
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
