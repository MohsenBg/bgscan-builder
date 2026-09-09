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

// ExtractTar extracts a .tar or .tar.gz archive into targetDir.
// It returns the path to the extracted root directory.
func ExtractTar(sourceTar string, targetDir string) (string, error) {
	file, err := os.Open(sourceTar)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	tarReader, closeGzip, err := tarStream(file, sourceTar)
	if err != nil {
		return "", err
	}
	if closeGzip != nil {
		defer func() { _ = closeGzip.Close() }()
	}

	tr := tar.NewReader(tarReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		targetPath := filepath.Join(targetDir, header.Name)

		// Prevent TarSlip directory traversal security vulnerabilities
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)) {
			return "", fmt.Errorf("illegal path traversal in tar header name: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return "", err
			}

			if err := writeTarFile(targetPath, header, tr); err != nil {
				return "", err
			}
		}
	}

	return targetDir, nil
}

// tarStream wraps file in a gzip decoder when the filename indicates
// compression. It returns the stream to read tar entries from and the gzip
// reader to close (nil for uncompressed archives).
func tarStream(file *os.File, name string) (io.Reader, *gzip.Reader, error) {
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, ".tar.gz") || strings.Contains(lowerName, ".tgz") || strings.Contains(lowerName, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to initialize gzip reader: %w", err)
		}
		return gzReader, gzReader, nil
	}
	return file, nil, nil
}

func writeTarFile(targetPath string, header *tar.Header, tr *tar.Reader) error {
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(outFile, tr); err != nil {
		return fmt.Errorf("failed to extract file contents: %w", err)
	}
	return nil
}
