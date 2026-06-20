package archive

import "fmt"

// createArchiver initializes an archiver based on the given ArchiveFormat.
func CreateArchiver(archiveFormat ArchiveFormat) (Archiver, error) {
	switch archiveFormat {
	case ArchiveZIP:
		return NewZipArchiver(), nil
	case ArchiveTAR:
		return NewTarArchiver(), nil
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", archiveFormat.String())
	}
}
