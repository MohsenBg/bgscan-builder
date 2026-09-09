package archive

import (
	"path/filepath"
	"strings"
)

// Extract unpacks archivePath into targetDir, picking the extractor from
// the file extension (.zip vs anything tar-based).
func Extract(archivePath, targetDir string) (string, error) {
	if strings.EqualFold(filepath.Ext(archivePath), ".zip") {
		return ExtractZip(archivePath, targetDir)
	}
	return ExtractTar(archivePath, targetDir)
}
