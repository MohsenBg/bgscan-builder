package compiler

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PrepareProjectFiles handles staging profiles, lists, and documentation
// components directly into the build workspace root.
func PrepareProjectFiles(srcProjectDir, destRootDir string) error {
	srcSettings := filepath.Join(srcProjectDir, "settings")
	destSettings := filepath.Join(destRootDir, "settings")

	if err := copyAndStripDefaultExtension(srcSettings, destSettings); err != nil {
		return fmt.Errorf("failed to copy main settings: %w", err)
	}

	srcIPs := filepath.Join(srcProjectDir, "ips")
	destIPs := filepath.Join(destRootDir, "ips")

	if _, err := os.Stat(srcIPs); err == nil {
		if err := copyAndStripDefaultExtension(srcIPs, destIPs); err != nil {
			return fmt.Errorf("failed to copy ips settings: %w", err)
		}
	}

	metaFiles := []string{"LICENSE", "README.md"}
	for _, filename := range metaFiles {
		srcMetaPath := filepath.Join(srcProjectDir, filename)

		if _, err := os.Stat(srcMetaPath); os.IsNotExist(err) {
			continue
		}

		destMetaPath := filepath.Join(destRootDir, filename)
		if err := copyFileRaw(srcMetaPath, destMetaPath); err != nil {
			return fmt.Errorf("failed to copy metadata asset %s: %w", filename, err)
		}
	}

	return nil
}

// CopyAssets copies everything recursively inside the cloned assets directory
// directly into the destination directory structure.
func CopyAssets(srcProjectDir, destRootDir string) error {
	srcAssets := filepath.Join(srcProjectDir, "assets")
	destAssets := filepath.Join(destRootDir, "assets")

	if _, err := os.Stat(srcAssets); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(srcAssets, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcAssets, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destAssets, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFileRaw(path, targetPath)
	})
}

func copyAndStripDefaultExtension(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcFile := filepath.Join(srcDir, entry.Name())
		cleanName := strings.TrimSuffix(entry.Name(), ".default")
		destFile := filepath.Join(destDir, cleanName)

		if err := copyFileRaw(srcFile, destFile); err != nil {
			return err
		}
	}
	return nil
}

func copyFileRaw(src, dest string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
