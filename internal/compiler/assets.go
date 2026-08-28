package compiler

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PrepareProjectFiles copies project configuration and metadata into a build workspace.
func (c *compiler) PrepareProjectFiles(srcProjectDir, destRootDir string) error {
	if err := copyDefaultFiles(
		filepath.Join(srcProjectDir, "ips"),
		filepath.Join(destRootDir, "ips"),
		false,
	); err != nil {
		return fmt.Errorf("copy ips: %w", err)
	}

	return copyMetadata(srcProjectDir, destRootDir)
}

// PrepareDevProjectFiles creates missing files from *.default templates.
func (c *compiler) PrepareDevProjectFiles(projectDir string) error {
	if err := copyDefaultFiles(
		filepath.Join(projectDir, "ips"),
		filepath.Join(projectDir, "ips"),
		true,
	); err != nil {
		return fmt.Errorf("copy ips: %w", err)
	}

	return nil
}

// CopyAssets copies the assets directory into the destination workspace.
func (c *compiler) CopyAssets(srcProjectDir, destRootDir string) error {
	srcAssets := filepath.Join(srcProjectDir, "assets")
	destAssets := filepath.Join(destRootDir, "assets")

	if _, err := os.Stat(srcAssets); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return filepath.Walk(
		srcAssets,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(srcAssets, path)
			if err != nil {
				return err
			}

			dst := filepath.Join(destAssets, rel)

			if info.IsDir() {
				return os.MkdirAll(dst, info.Mode())
			}

			return copyFile(path, dst)
		},
	)
}

// copyDefaultFiles copies files from srcDir while removing the .default suffix.
// Existing files are preserved when skipExisting is true.
func copyDefaultFiles(srcDir, destDir string, skipExisting bool) error {
	entries, err := os.ReadDir(srcDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(
			destDir,
			strings.TrimSuffix(entry.Name(), ".default"),
		)

		if skipExisting {
			if _, err := os.Stat(dst); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}

		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	return nil
}

// copyMetadata copies common project metadata files.
func copyMetadata(srcDir, destDir string) error {
	for _, name := range []string{
		"LICENSE",
		"README.md",
	} {
		src := filepath.Join(srcDir, name)

		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		}

		if err := copyFile(src, filepath.Join(destDir, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}

	return nil
}

// copyFile copies a file without modification.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
