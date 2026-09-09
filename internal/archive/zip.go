package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip extracts a ZIP file into targetDir, recreating all
// directories and files. It returns the output directory path.
func ExtractZip(sourceZip string, targetDir string) (string, error) {
	r, err := zip.OpenReader(sourceZip)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file: %w", err)
	}
	defer func() { _ = r.Close() }()

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		// Prevent ZipSlip vulnerability (avoid absolute or traversal paths)
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return "", err
			}
			continue
		}

		// Create parent dirs if missing
		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return "", err
		}

		if err := extractZipFile(f, fpath); err != nil {
			return "", err
		}
	}

	return targetDir, nil
}

func extractZipFile(f *zip.File, fpath string) error {
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		_ = outFile.Close()
		return err
	}

	_, copyErr := io.Copy(outFile, rc)

	_ = outFile.Close()
	_ = rc.Close()

	return copyErr
}
