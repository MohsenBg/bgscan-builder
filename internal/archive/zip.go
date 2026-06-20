package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ZipCompressor provides ZIP archive creation.
type ZipCompressor struct{}

// NewZipArchiver returns a new ZipCompressor.
func NewZipArchiver() Archiver {
	return &ZipCompressor{}
}

func (z *ZipCompressor) Format() string {
	return ArchiveZIP.String()
}

// Compress creates a ZIP archive from source inside targetDir and
// returns the path to the generated archive.
func (z *ZipCompressor) Compress(source string, targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	tempZipPath := filepath.Join(targetDir, filepath.Base(source)+".tmp.zip")
	if err := z.createZip(source, tempZipPath); err != nil {
		return "", err
	}

	finalZipName := filepath.Base(source) + ".zip"
	finalPath := filepath.Join(targetDir, finalZipName)

	if err := os.Rename(tempZipPath, finalPath); err != nil {
		return "", fmt.Errorf("failed to finalize archive: %w", err)
	}

	return finalPath, nil
}

// createZip walks the source path and writes its contents into a ZIP archive.
func (z *ZipCompressor) createZip(source, targetPath string) error {
	zipFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			header.Name = relPath + "/"
		} else {
			header.Name = relPath
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// UnCompress extracts a ZIP file (sourceZip) into targetDir.
// It recreates all directories and files. Returns the output directory path.
func (z *ZipCompressor) Decompress(sourceZip string, targetDir string) (string, error) {
	r, err := zip.OpenReader(sourceZip)
	if err != nil {
		return "", fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
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
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return "", err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", err
		}

		_, err = io.Copy(outFile, rc)

		// Clean up
		outFile.Close()
		rc.Close()

		if err != nil {
			return "", err
		}
	}

	return targetDir, nil
}
