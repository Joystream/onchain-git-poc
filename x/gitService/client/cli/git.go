package cli

import (
	"fmt"
	"os"

	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
)

type gitConfigStorer struct {
	*filesystem.Storage
	cfg    *gogitcfg.Config
	isStored bool
}

func newGitConfigStorer() (*gitConfigStorer, error) {
	// TODO: Implement to-blockchain filesystem
	storage := filesystem.NewStorage(osfs.New("/tmp/gitservice/.git"), cache.NewObjectLRUDefault())
	cfg, err := storage.Config()
	if err != nil {
		return nil, err
	}
	// To figure out if this config has been written already, check if
	// it differs from the zero Core value (probably because the
	// IsBare bit is flipped).
	fmt.Fprintf(os.Stderr, "Initialized remote Git filesystem storage at /tmp/gitservice/.git\n")
	return &gitConfigStorer{
		storage,
		cfg,
		cfg.Core != gogitcfg.Config{}.Core,
	}, nil
}
