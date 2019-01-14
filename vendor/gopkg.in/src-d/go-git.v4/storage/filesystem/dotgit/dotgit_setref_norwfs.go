// +build norwfs

package dotgit

import (
	"fmt"
	"os"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// There are some filesystems that don't support opening files in RDWD mode.
// In these filesystems the standard SetRef function can not be used as it
// reads the reference file to check that it's not modified before updating it.
//
// This version of the function writes the reference without extra checks
// making it compatible with these simple filesystems. This is usually not
// a problem as they should be accessed by only one process at a time.
func (d *DotGit) setRef(fileName, content string, old *plumbing.Reference) error {
	logger.Debug().Msgf("Setting reference, filename: '%s', content: '%s'\n", fileName, content)
	_, err := d.fs.Stat(fileName)
	if err == nil && old != nil {
		fRead, err := d.fs.Open(fileName)
		if err != nil {
			return err
		}

		ref, err := d.readReferenceFrom(fRead, old.Name().String())
		fRead.Close()

		if err != nil {
			return err
		}

		if ref.Hash() != old.Hash() {
			return fmt.Errorf("reference has changed concurrently")
		}
	}

	logger.Debug().Msgf("Creating file '%s' and writing content to it\n", fileName)
	f, err := d.fs.Create(fileName)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.Write([]byte(content))
	return err
}
