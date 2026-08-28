package compiler

import "bgscan-builder/internal/platform"

// Compiler defines methods for compiling bgscan and preparing build workspaces.
type Compiler interface {
	Build(target platform.Info, dest, projectDir, ndkDir string) error
	PrepareProjectFiles(srcProjectDir, destRootDir string) error
	PrepareDevProjectFiles(projectDir string) error
	CopyAssets(srcProjectDir, destRootDir string) error
}

// New returns a new Compiler implementation.
func New() Compiler {
	return &compiler{}
}
