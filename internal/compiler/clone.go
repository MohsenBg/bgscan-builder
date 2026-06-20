package compiler

import (
	"os"

	"github.com/go-git/go-git/v6"
)

// CloneProject checkouts a fresh instance of the core bgscaner target repository
// into the provided temporary scratch workspace directory path.
func CloneProject(destDir string) error {
	_, err := git.PlainClone(destDir, &git.CloneOptions{
		URL:      "https://github.com/MohsenBg/bgscaner.git",
		Progress: os.Stdout,
	})
	return err
}
