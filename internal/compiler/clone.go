package compiler

import (
	"io"

	gitclient "github.com/go-git/go-git/v6/plumbing/client"

	"github.com/go-git/go-git/v6"

	"bgscan-builder/internal/netutil"
)

// CloneProject clones a fresh copy of the bgscaner repository into destDir.
// The clone shares the netutil HTTP client so repository fetches resolve DNS
// with the same system-first, public-server fallback used by every other
// download in the builder.
func CloneProject(destDir string) error {
	_, err := git.PlainClone(destDir, &git.CloneOptions{
		URL: "https://github.com/MohsenBg/bgscaner.git",
		ClientOptions: []gitclient.Option{
			gitclient.WithHTTPClient(netutil.DefaultHTTPClient()),
		},
		Progress:          io.Discard,
		SingleBranch:      true,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Depth:             1,
	})
	return err
}
