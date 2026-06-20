package archive

import "fmt"

// ArchiveFormat represents supported archive formats.
type ArchiveFormat int

const (
	ArchiveZIP ArchiveFormat = iota
	ArchiveTAR
)

func (f ArchiveFormat) String() string {
	switch f {
	case ArchiveZIP:
		return "zip"
	case ArchiveTAR:
		return "tar"
	default:
		return fmt.Sprintf("ArchiveFormat(%d)", f)
	}
}

// Archiver defines methods for compressing and decompressing archives.
type Archiver interface {
	// Compress archives the given source path into the target directory.
	// It returns the path to the created archive.
	Compress(sourcePath, targetDir string) (string, error)

	// Decompress extracts the given archive into the target directory.
	// It returns the extraction directory path.
	Decompress(archivePath, targetDir string) (string, error)

	Format() string
}
