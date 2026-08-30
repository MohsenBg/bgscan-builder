package main

import (
	"context"
	"errors"
	"os"

	"bgscan-builder/internal/downloader"
	"bgscan-builder/internal/installer"
	"bgscan-builder/internal/platform"
	"bgscan-builder/internal/ui"
)

// Install runs the native bgscan installer for the current host platform.
func Install(ctx context.Context, u *ui.UI, cfg Config) error {
	dl := downloader.New(downloader.WithProgressSink(u.ProgressSink()))
	inst := installer.New(u, dl)

	err := inst.Install(ctx, platform.Detect(), cfg.Version, cfg.InstallDir, os.Stdin)
	if errors.Is(err, installer.ErrCancelled) {
		return nil
	}
	return installerError(u, err)
}

// Update refreshes an existing bgscan installation in place, merging the
// release binary plus ips/assets while preserving user-added files.
func Update(ctx context.Context, u *ui.UI, cfg Config) error {
	dl := downloader.New(downloader.WithProgressSink(u.ProgressSink()))
	inst := installer.New(u, dl)

	err := inst.Update(ctx, platform.Detect(), cfg.Version, cfg.InstallDir)
	return installerError(u, err)
}

// installerError renders a structured failure panel when available.
func installerError(u *ui.UI, err error) error {
	if err == nil {
		return nil
	}
	var stageErr *installer.StageError
	if errors.As(err, &stageErr) {
		u.FailPanel(stageErr.Op(), stageErr.Reason(), stageErr.Note())
		return err
	}
	u.Fail(err.Error())
	return err
}
