package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TarCompressor handles TAR archive creation and extraction.
type TarArchiver struct{}

// NewTarCompressor creates a new TarCompressor instance.
func NewTarArchiver() Archiver {
	return &TarArchiver{}
}

func (t *TarArchiver) Format() string {
	return ArchiveTAR.String()
}

// Compress creates a .tar archive of the source in targetDir.
// Returns the full path to the final .tar file.
func (t *TarArchiver) Compress(source string, targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	tempTar := filepath.Join(targetDir, filepath.Base(source)+".tmp.tar")
	if err := t.createTar(source, tempTar); err != nil {
		return "", err
	}

	finalTar := filepath.Join(targetDir, filepath.Base(source)+".tar")
	if err := os.Rename(tempTar, finalTar); err != nil {
		return "", fmt.Errorf("failed to finalize tar file: %w", err)
	}

	return finalTar, nil
}

// createTar walks the source and writes all files + dirs into a TAR archive.
func (t *TarArchiver) createTar(source, targetPath string) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// Use linux-like slashes inside tar; required for portability
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

// Decompress extracts a .tar or .tar.gz archive (sourceTar) into targetDir.
// Returns the path to the extracted root directory.
func (t *TarArchiver) Decompress(sourceTar string, targetDir string) (string, error) {
	file, err := os.Open(sourceTar)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer file.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	// Dynamic Reader Layer selection
	var tarReader io.Reader = file

	// If the filename indicates a gzip compression wrapper, pass through a gzip reader
	lowerName := strings.ToLower(sourceTar)
	if strings.Contains(lowerName, ".tar.gz") || strings.Contains(lowerName, ".tgz") || strings.Contains(lowerName, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return "", fmt.Errorf("failed to initialize gzip reader wrapper: %w", err)
		}
		defer gzReader.Close()
		tarReader = gzReader
	}

	tr := tar.NewReader(tarReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // done
		}

		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		targetPath := filepath.Join(targetDir, header.Name)

		// Prevent TarSlip directory traversal security vulnerabilities
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)) {
			return "", fmt.Errorf("security check failure: illegal path traversal tracking detected in tar header name: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return "", err
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return "", fmt.Errorf("failed to create target output file: %w", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", fmt.Errorf("failed to stream untar tracking info contents: %w", err)
			}
			outFile.Close()
		}
	}

	return targetDir, nil
}
