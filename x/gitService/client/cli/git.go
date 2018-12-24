package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joystream/onchain-git-poc/x/gitService/client/cli/storage"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-billy.v4/osfs"
	gogit "gopkg.in/src-d/go-git.v4"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"
	"gopkg.in/src-d/go-git.v4/plumbing/revlist"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	gogitstor "gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/utils/ioutil"
)

var (
	errAlreadyUpToDate       = errors.New("already up-to-date")
	errDeleteRefNotSupported = errors.New("server does not support delete-refs")
	errForceNeeded           = errors.New("some refs were not updated")
)

func init() {
	transport := newJoystreamTransport()
	client.InstallProtocol("joystream", &transport)
}

type gitConfigStorer struct {
	*storage.Storage
	cfg      *gogitcfg.Config
	isStored bool
}

func newGitConfigStorer() (*gitConfigStorer, error) {
	// TODO: Implement to-blockchain filesystem
	storage := storage.NewStorage(osfs.New("/tmp/gitservice/.git"), cache.NewObjectLRUDefault())
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

func newClient(url string) (transport.Transport, *transport.Endpoint, error) {
	ep, err := transport.NewEndpoint(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create endpoint for URL '%s'\n", url)
		return nil, nil, err
	}

	fmt.Fprintf(os.Stderr, "Opening client for endpoint '%s'\n", ep.String())
	c, err := client.NewClient(ep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client for URL '%s'\n", url)
		return nil, nil, err
	}

	return c, ep, err
}

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

func newReferenceUpdateRequest(refSpecs []gogitcfg.RefSpec, localRefs []*plumbing.Reference,
	remoteRefs storer.ReferenceStorer, ar *packp.AdvRefs, repo *gogit.Repository) (
	*packp.ReferenceUpdateRequest, error) {
	req := packp.NewReferenceUpdateRequestFromCapabilities(ar.Capabilities)

	// if o.Progress != nil {
	// 	req.Progress = o.Progress
	// 	if ar.Capabilities.Supports(capability.Sideband64k) {
	// 		req.Capabilities.Set(capability.Sideband64k)
	// 	} else if ar.Capabilities.Supports(capability.Sideband) {
	// 		req.Capabilities.Set(capability.Sideband)
	// 	}
	// }

	if err := addReferencesToUpdate(refSpecs, localRefs, remoteRefs, req, repo); err != nil {
		return nil, err
	}

	return req, nil
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

func addReference(refSpec gogitcfg.RefSpec, remoteRefs storer.ReferenceStorer,
	localRef *plumbing.Reference, req *packp.ReferenceUpdateRequest,
	repo *gogit.Repository) error {
	fmt.Fprintf(os.Stderr, "Determining whether to add a command to ReferenceUpdateRequest\n")
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
			//TODO: check actual git behavior here
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
		fmt.Fprintf(os.Stderr, "Adding reference to remote %s -> %s\n", cmd.Name, cmd.New)
	} else {
		fmt.Fprintf(os.Stderr, "Updating reference in remote %s -> %s\n", cmd.Name, cmd.New)
	}

	if !refSpec.IsForceUpdate() {
		fmt.Fprintf(os.Stderr, "Not in force mode - verifying update is a fast forward\n")
		if err := checkFastForwardUpdate(repo, remoteRefs, cmd); err != nil {
			return err
		}
	}

	if cmd.Old == plumbing.ZeroHash {
		fmt.Fprintf(os.Stderr, "Adding command for adding reference '%s' -> '%s'\n", cmd.Name, cmd.New)
	} else {
		fmt.Fprintf(os.Stderr, "Adding command for updating reference '%s'; old: '%s', new: '%s'\n",
			cmd.Name, cmd.Old, cmd.New)
	}
	req.Commands = append(req.Commands, cmd)
	return nil
}

func resolveLocalRef(refSpec gogitcfg.RefSpec, repo *gogit.Repository) (
	*plumbing.ReferenceName, error) {
	refName := plumbing.ReferenceName(refSpec.Src())
	fmt.Fprintf(os.Stderr, "Resolving reference '%v' in local repo\n", refName)
	resolved, err := gogitstor.ResolveReference(repo.Storer, refName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving ref '%s'\n", refName)
		return nil, err
	}
	if resolved != nil {
		refName = resolved.Name()
		fmt.Fprintf(os.Stderr, "Resolved local reference to '%v'\n", refName)
	}

	return &refName, nil
}

// addReferencesToUpdate adds references for deletion/adding/updating to a ReferenceUpdateRequest
func addReferencesToUpdate(refSpecs []gogitcfg.RefSpec, localRefs []*plumbing.Reference,
	remoteRefs storer.ReferenceStorer, req *packp.ReferenceUpdateRequest,
	repo *gogit.Repository) error {
	// This references dictionary will be used to search references by name.
	refsDict := make(map[string]*plumbing.Reference)
	for _, ref := range localRefs {
		refsDict[ref.Name().String()] = ref
	}

	fmt.Fprintf(os.Stderr, "Determining remote references to update\n")
	for _, refSpec := range refSpecs {
		fmt.Fprintf(os.Stderr, "Handling RefSpec '%v'\n", refSpec)
		if refSpec.IsDelete() {
			fmt.Fprintf(os.Stderr, "It's a deletion\n")
			if err := deleteReferences(refSpec, remoteRefs, req); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(os.Stderr, "It's not a deletion\n")
			// If it is not a wilcard refspec we can search directly for the reference
			// in the references map
			if !refSpec.IsWildcard() {
				refName, error := resolveLocalRef(refSpec, repo)
				if error != nil {
					return error
				}

				localRef, ok := refsDict[refName.String()]
				if !ok {
					fmt.Fprintf(os.Stderr, "Couldn't find local ref corresponding to RefSpec %s\n",
						refSpec.Src())
					return nil
				}

				if err := addReference(refSpec, remoteRefs, localRef, req, repo); err != nil {
					return err
				}
			} else {
				for _, localRef := range localRefs {
					if refSpec.Match(localRef.Name()) {
						err := addReference(refSpec, remoteRefs, localRef, req, repo)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func deleteReferences(refSpec gogitcfg.RefSpec, remoteRefs storer.ReferenceStorer,
	req *packp.ReferenceUpdateRequest) error {
	iter, err := remoteRefs.IterReferences()
	if err != nil {
		return err
	}

	return iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		if refSpec.Dst("") != ref.Name() {
			return nil
		}

		fmt.Fprintf(os.Stderr, "Adding command to delete reference '%s'\n", ref.Name())
		cmd := &packp.Command{
			Name: ref.Name(),
			Old:  ref.Hash(),
			New:  plumbing.ZeroHash,
		}
		req.Commands = append(req.Commands, cmd)
		return nil
	})
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
func getHashesToPush(commands []*packp.Command, repo *gogit.Repository,
	remoteRefs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	var hashes []plumbing.Hash
	for _, cmd := range commands {
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

// getReferencesToHashes returns hashes corresponding to references
func referencesToHashes(refs storer.ReferenceStorer) ([]plumbing.Hash, error) {
	iter, err := refs.IterReferences()
	if err != nil {
		return nil, err
	}

	var hs []plumbing.Hash
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		hs = append(hs, ref.Hash())
		return nil
	})
	if err != nil {
		return nil, err
	}

	return hs, nil
}

// pushToRemote pushes a set of refs to a repository signified by a certain URL
func pushToBlockChain(ctx context.Context, url string, refSpecs []gogitcfg.RefSpec,
	repo *gogit.Repository) error {
	// TODO: Verify that URL is of joystream protocol
	// TODO: Implement transport for joystream protocol
	fmt.Fprintf(os.Stderr, "Pushing '%s' to blockchain at '%s'\n", refSpecs[0], url)
	c, ep, err := newClient(fmt.Sprintf("joystream://%s", url))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open client for URL '%s'\n", url)
		return err
	}

	// Start a session for uploading data to the endpoint
	fmt.Fprintf(os.Stderr, "Starting session\n")
	session, err := c.NewReceivePackSession(ep, &DummyAuth{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed opening session for URL '%s'\n", url)
		return err
	}

	defer ioutil.CheckClose(session, &err)

	// Get remote references
	advRefs, err := session.AdvertisedReferences()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed getting advertised refs\n")
		return err
	}

	remoteRefs, err := advRefs.AllReferences()
	if err != nil {
		return err
	}

	isDelete := false
	allDelete := true
	for _, refSpec := range refSpecs {
		if refSpec.IsDelete() {
			isDelete = true
		} else {
			allDelete = false
		}
		if isDelete && !allDelete {
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
	fmt.Fprintf(os.Stderr, "Got local references: %v\n", strings.Join(localRefStrings, ", "))
	req, err := newReferenceUpdateRequest(refSpecs, localRefs, remoteRefs, advRefs, repo)
	if err != nil {
		return err
	}

	if len(req.Commands) == 0 {
		fmt.Fprintf(os.Stderr, "Remote is already up to date\n")
		return errAlreadyUpToDate
	}

	var hashesToPush []plumbing.Hash
	// Avoid the expensive revlist operation if we're only doing deletes.
	if !allDelete {
		hashesToPush, err = getHashesToPush(req.Commands, repo, remoteRefs)
		if err != nil {
			return err
		}
	}

	reportStatus, err := pushHashes(ctx, session, repo, req, hashesToPush, advRefs)
	if err != nil {
		return err
	}

	err = reportStatus.Error()
	return err
}

// pushHashes pushes a set of hashes from a local repository to a remote one
func pushHashes(ctx context.Context, sess transport.ReceivePackSession, repo *gogit.Repository,
	req *packp.ReferenceUpdateRequest, hashes []plumbing.Hash, advRefs *packp.AdvRefs) (
	*packp.ReportStatus, error) {
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
		fmt.Fprintf(os.Stderr, "Encoding packfile to writer\n")
		// Write encoded packfile into writing end of pipe, the Joystream transport will in turn
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
			fmt.Fprintf(os.Stderr, "Packfile encoding failed: %v\n", err)
			done <- wr.CloseWithError(err)
			return
		}

		fmt.Fprintf(os.Stderr, "Packfile encoding succeeded, closing writer\n")
		done <- wr.Close()
	}()

	// Write reference update request to remote
	reportStatus, err := sess.ReceivePack(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wait on done to be written to, which happens after encoder.Encode returns
	fmt.Fprintf(os.Stderr, "Waiting for packfile encoding to finish\n")
	if err := <-done; err != nil {
		fmt.Fprintf(os.Stderr, "Packfile encoding finished with an error: %v\n", err)
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Packfile writing finished successfully\n")
	return reportStatus, nil
}
