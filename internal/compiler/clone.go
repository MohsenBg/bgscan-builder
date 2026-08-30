package compiler

import (
	"io"

	"github.com/go-git/go-git/v6"
)

// CloneProject clones a fresh copy of the bgscaner repository into destDir.
func CloneProject(destDir string) error {
	_, err := git.PlainClone(destDir, &git.CloneOptions{
		URL:               "https://github.com/MohsenBg/bgscaner.git",
		Progress:          io.Discard,
		SingleBranch:      true,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Depth:             1,
	})
	return err
}
