package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	cosmosContext "github.com/cosmos/cosmos-sdk/client/context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	gogit "gopkg.in/src-d/go-git.v4"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
	"gopkg.in/src-d/go-git.v4/plumbing/revlist"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	gogitstor "gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

var (
	errAlreadyUpToDate       = errors.New("already up-to-date")
	errDeleteRefNotSupported = errors.New("server does not support delete-refs")
	errForceNeeded           = errors.New("some refs were not updated")
)

// DummyAuth is a preliminary authentication method
type DummyAuth struct {
	Username, Password string
}

// Name is name of the auth
func (a *DummyAuth) Name() string {
	return "http-basic-auth"
}

func (a *DummyAuth) String() string {
	masked := "*******"
	if a.Password == "" {
		masked = "<empty>"
	}

	return fmt.Sprintf("%s - %s:%s", a.Name(), a.Username, masked)
}

func checkFastForwardUpdate(repo *gogit.Repository, remoteRefs storer.ReferenceStorer,
	cmd *packp.Command) error {
	if cmd.Old == plumbing.ZeroHash {
		_, err := remoteRefs.Reference(cmd.Name)
		if err == plumbing.ErrReferenceNotFound {
			return nil
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("non-fast-forward update: %s", cmd.Name.String())
	}

	ff, err := isFastForward(repo, cmd.Old, cmd.New)
	if err != nil {
		return err
	}

	if !ff {
		return fmt.Errorf("non-fast-forward update: %s", cmd.Name.String())
	}

	return nil
}

func isFastForward(repo *gogit.Repository, old, new plumbing.Hash) (bool, error) {
	c, err := object.GetCommit(repo.Storer, new)
	if err != nil {
		return false, err
	}

	found := false
	iter := object.NewCommitPreorderIter(c, nil, nil)
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash != old {
			return nil
		}

		found = true
		return storer.ErrStop
	})
	return found, err
}

func resolveLocalRef(refSpec gogitcfg.RefSpec, repo *gogit.Repository) (
	*plumbing.ReferenceName, error) {
	refName := plumbing.ReferenceName(refSpec.Src())
	log.Debug().Msgf("Resolving reference '%v' in local repo", refName)
	resolved, err := gogitstor.ResolveReference(repo.Storer, refName)
	if err != nil {
		log.Debug().Msgf("Error resolving ref '%s'", refName)
		return nil, err
	}
	if resolved != nil {
		refName = resolved.Name()
		log.Debug().Msgf("Resolved local reference to '%v'", refName)
	}

	return &refName, nil
}

func getReferences(repo *gogit.Repository) ([]*plumbing.Reference, error) {
	var refs []*plumbing.Reference
	iter, err := repo.References()
	if err != nil {
		return nil, err
	}

	for {
		ref, err := iter.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

// getHashesToPush determines hashes that should be pushed to remote
func getHashesToPush(req *packp.ReferenceUpdateRequest, repo *gogit.Repository,
	remoteRefs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	hashes := make([]plumbing.Hash, 0, len(req.Commands))
	for _, cmd := range req.Commands {
		if cmd.New == plumbing.ZeroHash {
			continue
		}

		hashes = append(hashes, cmd.New)
	}

	haveHashes, err := referencesToHashes(remoteRefs)
	if err != nil {
		return nil, err
	}

	stop, err := repo.Storer.Shallow()
	if err != nil {
		return nil, err
	}

	// if we have shallow we should include this as part of the objects that
	// we are aware of
	haveHashes = append(haveHashes, stop...)

	hashesToPush, err := revlist.Objects(repo.Storer, hashes, haveHashes)
	return hashesToPush, err
}

// referencesToHashes returns hashes corresponding to references
func referencesToHashes(refs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	iter, err := refs.IterReferences()
	if err != nil {
		return nil, err
	}

	hashes := make([]plumbing.Hash, 0)
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		hashes = append(hashes, ref.Hash())
		return nil
	})
	if err != nil {
		return nil, err
	}

	return hashes, nil
}

// pushToBlockchain sends a message to the server to update a set of references
func pushToBlockChain(ctx context.Context, uri string, refSpecs []gogitcfg.RefSpec,
	repo *gogit.Repository, cliCtx cosmosContext.CLIContext,
	txBldr authtxb.TxBuilder, author sdk.AccAddress, moduleName string) error {
	// TODO: Verify that URL is of joystream protocol
	log.Debug().Msgf("Pushing '%s' to blockchain at '%s'", refSpecs[0], uri)
	c, err := newJoystreamClient(uri, cliCtx, txBldr, author, moduleName)
	if err != nil {
		log.Debug().Msgf("Failed to create client for URL '%s'", uri)
		return err
	}

	// Start a session for uploading data to the endpoint
	log.Debug().Msgf("Starting session")
	session, err := c.NewReceivePackSession(c.ep, &DummyAuth{})
	if err != nil {
		log.Debug().Msgf("Failed opening session for URL '%s'", uri)
		return err
	}
	defer ioutil.CheckClose(session, &err)

	advRefs, err := session.AdvertisedReferences()
	if err != nil {
		return err
	}
	remoteRefs, err := advRefs.AllReferences()
	if err != nil {
		return err
	}

	allDelete := true
	for _, refSpec := range refSpecs {
		isDelete := refSpec.IsDelete()
		if !isDelete {
			allDelete = false
		}
		if !allDelete {
			break
		}
	}

	localRefs, err := getReferences(repo)
	if err != nil {
		return err
	}

	localRefStrings := make([]string, 0, len(localRefs))
	for _, ref := range localRefs {
		localRefStrings = append(localRefStrings, ref.Name().String())
	}
	log.Debug().Msgf("Got local references: %v", strings.Join(localRefStrings, ", "))
	req := packp.NewReferenceUpdateRequest()
	if err := computeRefUpdateCmds(refSpecs, localRefs, remoteRefs, repo, req); err != nil {
		return err
	}
	if len(req.Commands) == 0 {
		log.Debug().Msgf("Remote is already up to date")
		return errAlreadyUpToDate
	}

	var hashesToPush []plumbing.Hash
	// Avoid the expensive revlist operation if we're only doing deletes.
	if !allDelete {
		hashesToPush, err = getHashesToPush(req, repo, remoteRefs)
		if err != nil {
			return err
		}
	}

	reportStatus, err := pushHashes(ctx, session, repo, uri, req, hashesToPush, advRefs)
	if err != nil {
		return err
	}

	if err := reportStatus.Error(); err != nil {
		return err
	}

	return nil
}

// pushHashes pushes a set of hashes from a local repository to a remote one
func pushHashes(ctx context.Context, sess transport.ReceivePackSession, repo *gogit.Repository,
	uri string, req *packp.ReferenceUpdateRequest, hashes []plumbing.Hash,
	advRefs *packp.AdvRefs) (*packp.ReportStatus, error) {
	rd, wr := io.Pipe()
	req.Packfile = rd
	config, err := repo.Storer.Config()
	if err != nil {
		return nil, err
	}

	done := make(chan error)
	go func() {
		useRefDeltas := !advRefs.Capabilities.Supports(capability.OFSDelta)
		encoder := packfile.NewEncoder(wr, repo.Storer, useRefDeltas)
		log.Debug().Msgf("Encoding packfile to writer")
		// Write encoded packfile into writing end of pipe, the Joystream client will in turn
		// read this data from the reading end of the pipe and close it. After the reading end of
		// the pipe is closed, the encoding finishes.
		//
		// The packfile gets sent to Joystream servers that decode it and store the corresponding
		// Git data to the blockchain. For example all objects (corresponding to commits?) that get
		// pushed get stored on the chain, and also references (which correspond to f.ex. branch
		// heads).
		//
		// When queried for advertised refs, Joystream servers will respond by returning a mapping
		// of reference names to commit hashes. This way the client will know which references
		// need updating/adding/deleting.
		if _, err = encoder.Encode(hashes, config.Pack.Window); err != nil {
			log.Debug().Msgf("Packfile encoding failed: %v", err)
			done <- wr.CloseWithError(err)
			return
		}

		log.Debug().Msgf("Packfile encoding succeeded, closing writer")
		done <- wr.Close()
	}()

	// Write reference update request to remote
	reportStatus, err := sess.ReceivePack(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wait on done to be written to, which happens after encoder.Encode returns
	log.Debug().Msgf("Waiting for packfile encoding to finish")
	if err := <-done; err != nil {
		log.Debug().Msgf("Packfile encoding finished with an error: %v", err)
		return nil, err
	}

	log.Debug().Msgf("Packfile writing finished successfully")
	return reportStatus, nil
}

// computeRefUpdateCmds determines reference update commands
func computeRefUpdateCmds(refSpecs []gogitcfg.RefSpec, localRefs []*plumbing.Reference,
	remoteRefs storer.ReferenceStorer, repo *gogit.Repository,
	req *packp.ReferenceUpdateRequest) error {
	name2LocalRef := make(map[string]*plumbing.Reference)
	for _, ref := range localRefs {
		name2LocalRef[ref.Name().String()] = ref
	}

	log.Debug().Msgf("Determining remote references to update")
	req.Commands = make([]*packp.Command, 0)
	for _, refSpec := range refSpecs {
		log.Debug().Msgf("Handling RefSpec '%v'", refSpec)
		if refSpec.IsDelete() {
			log.Debug().Msgf("It's a deletion")
			if err := deleteReferences(refSpec, remoteRefs, req); err != nil {
				return err
			}
		} else {
			log.Debug().Msgf("It's not a deletion")
			// If it is not a wilcard refspec we can search directly for the reference
			// in the references map
			if !refSpec.IsWildcard() {
				refName, err := resolveLocalRef(refSpec, repo)
				if err != nil {
					return err
				}

				localRef, ok := name2LocalRef[refName.String()]
				if !ok {
					log.Debug().Msgf("Couldn't find local ref corresponding to RefSpec %s",
						refSpec.Src())
					continue
				}

				if err := addReference(refSpec, remoteRefs, localRef, req, repo); err != nil {
					return err
				}
			} else {
				for _, localRef := range localRefs {
					if refSpec.Match(localRef.Name()) {
						if err := addReference(refSpec, remoteRefs, localRef, req, repo); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// addReference adds a command for adding or updating a reference to a ReferenceUpdateRequest,
// if required conditions are met
func addReference(refSpec gogitcfg.RefSpec, remoteRefs storer.ReferenceStorer,
	localRef *plumbing.Reference, req *packp.ReferenceUpdateRequest, repo *gogit.Repository) error {
	log.Debug().Msgf("Determining whether to add a command to ReferenceUpdateRequest")
	if localRef.Type() != plumbing.HashReference {
		return nil
	}

	refSpecSrcName, err := resolveLocalRef(refSpec, repo)
	if err != nil {
		return nil
	}
	if *refSpecSrcName != localRef.Name() {
		return errors.Errorf("RefSpec and local ref don't match (%s != %s)", refSpec.Src(),
			localRef.Name())
	}

	cmd := &packp.Command{
		Name: refSpec.Dst(localRef.Name()),
		Old:  plumbing.ZeroHash,
		New:  localRef.Hash(),
	}

	remoteRef, err := remoteRefs.Reference(cmd.Name)
	if err == nil {
		if remoteRef.Type() != plumbing.HashReference {
			// TODO: check actual git behavior here
			return nil
		}

		cmd.Old = remoteRef.Hash()
	} else if err != plumbing.ErrReferenceNotFound {
		return err
	}

	if cmd.Old == cmd.New {
		return nil
	}

	if cmd.Old == plumbing.ZeroHash {
		log.Debug().Msgf("Adding reference to remote %s -> %s", cmd.Name, cmd.New)
	} else {
		log.Debug().Msgf("Updating reference in remote %s -> %s", cmd.Name, cmd.New)
	}

	if !refSpec.IsForceUpdate() {
		log.Debug().Msgf("Not in force mode - verifying update is a fast forward")
		if err := checkFastForwardUpdate(repo, remoteRefs, cmd); err != nil {
			return err
		}
	}

	if cmd.Old == plumbing.ZeroHash {
		log.Debug().Msgf("Adding command for adding reference '%s' -> '%s'", cmd.Name, cmd.New)
	} else {
		log.Debug().Msgf("Adding command for updating reference '%s'; old: '%s', new: '%s'",
			cmd.Name, cmd.Old, cmd.New)
	}
	req.Commands = append(req.Commands, cmd)

	return nil
}

// deleteReferences adds commands for deleting remote references corresponding to a refspec
func deleteReferences(refSpec gogitcfg.RefSpec, remoteRefs storer.ReferenceStorer,
	req *packp.ReferenceUpdateRequest) error {
	iter, err := remoteRefs.IterReferences()
	if err != nil {
		return err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		if refSpec.Dst("") != ref.Name() {
			return nil
		}

		log.Debug().Msgf("Adding command to delete reference '%s'", ref.Name())
		cmd := &packp.Command{
			Name: ref.Name(),
			Old:  ref.Hash(),
			New:  plumbing.ZeroHash,
		}
		req.Commands = append(req.Commands, cmd)
		return nil
	})

	return err
}
