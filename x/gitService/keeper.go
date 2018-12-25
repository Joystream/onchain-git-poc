package gitService

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var reRepoURI = regexp.MustCompile("([^/]+)/([^/]+)")

// Keeper maintains the link to data storage and exposes getter/setter methods for the various
// parts of the state machine
type Keeper struct {
	gitStoreKey sdk.StoreKey

	cdc *codec.Codec // The wire codec for binary encoding/decoding.
}

// NewKeeper creates new instances of the gitService Keeper
func NewKeeper(gitStoreKey sdk.StoreKey, cdc *codec.Codec) Keeper {
	return Keeper{
		gitStoreKey: gitStoreKey,
		cdc:         cdc,
	}
}

// ListRefs lists refs for a repository
func (k Keeper) ListRefs(ctx sdk.Context, owner string, repo string) []string {
	// store := ctx.KVStore(k.gitStoreKey)
	// return string(store.Get([]byte(url)))
	return []string{}
}

// GetAdvertisedReferences gets advertised references for a repository
func (k Keeper) GetAdvertisedReferences(ctx sdk.Context, owner string, repo string) *packp.AdvRefs {
	fmt.Fprintf(os.Stderr, "Keeper getting advertised references for repo %s/%s\n", owner, repo)
	// TODO: Determine if repo exists or not
	return packp.NewAdvRefs()
}

// UpdateReferences updates a Git reference
func (k Keeper) UpdateReferences(ctx sdk.Context, msg MsgUpdateReferences) sdk.Error {
	m := reRepoURI.FindStringSubmatch(msg.URI)
	if m == nil {
		fmt.Fprintf(os.Stderr, "Invalid repo URI: '%s'\n", msg.URI)
		return sdk.ErrUnknownRequest(fmt.Sprintf("Invalid repo URI: '%s'", msg.URI))
	}

	fmt.Fprintf(os.Stderr, "Keeper updating references in repo '%s'\n", msg.URI)
	// TODO: Verify that user is authorized to write to repo
	store := ctx.KVStore(k.gitStoreKey)
	if !store.Has([]byte(fmt.Sprintf("%s/HEAD", msg.URI))) {
		if err := initializeRepo(store, msg); err != nil {
			return sdk.ErrInternal(err.Error())
		}
	}

	if err := writePackfile(store, msg); err != nil {
		return sdk.ErrInternal(err.Error())
	}

	if err := updateReferences(store, msg); err != nil {
		return sdk.ErrInternal(err.Error())
	}

	return nil
}

func initializeRepo(store sdk.KVStore, msg MsgUpdateReferences) error {
	fmt.Fprintf(os.Stderr, "Keeper - store doesn't have repo '%s', initializing it\n", msg.URI)
	store.Set([]byte(fmt.Sprintf("%s/HEAD", msg.URI)), []byte("ref: refs/heads/master\n"))
	store.Set([]byte(fmt.Sprintf("%s/config", msg.URI)), []byte(`[core]
	repositoryformatversion = 0
	bare = true
`))
	return nil
}

func referenceExists(store sdk.KVStore, refPath string) bool {
	fmt.Fprintf(os.Stderr, "Checking if reference exists: '%s'\n", refPath)
	if !store.Has([]byte(refPath)) {
		fmt.Fprintf(os.Stderr, "Reference doesn't exist: '%s'\n", refPath)
		return false
	}

	fmt.Fprintf(os.Stderr, "Reference exists: '%s'\n", refPath)
	return true
}

func writeReference(store sdk.KVStore, refPath string, cmd *UpdateReferenceCommand) {
	ref := plumbing.NewHashReference(cmd.Name, cmd.New)
	var content string
	switch ref.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", ref.Target())
	case plumbing.HashReference:
		content = fmt.Sprintln(ref.Hash().String())
	}
	fmt.Fprintf(os.Stderr, "Writing reference '%s': '%s'\n", refPath, content)
	store.Set([]byte(refPath), []byte(content))
}

// updateReferences updates references in a repository
func updateReferences(store sdk.KVStore, msg MsgUpdateReferences) error {
	errUpdateReference := errors.New("Failed to update reference")

	fmt.Fprintf(os.Stderr, "Updating references\n")
	for _, cmd := range msg.Commands {
		if !strings.HasPrefix(cmd.Name.String(), "refs/") {
			panic(fmt.Sprintf("Reference doesn't start with refs/: '%s'", cmd.Name))
		}
		refPath := fmt.Sprintf("%s/%s", msg.URI, cmd.Name)
		exists := referenceExists(store, refPath)
		switch cmd.Action() {
		case CreateAction:
			if exists {
				fmt.Fprintf(os.Stderr, "Can't create reference '%s' as it already exists\n", refPath)
				return errUpdateReference
			}

			fmt.Fprintf(os.Stderr, "Creating reference '%s' pointing to hash '%s'\n", refPath,
				cmd.New)
			writeReference(store, refPath, cmd)
		case packp.Delete:
			if !exists {
				fmt.Fprintf(os.Stderr, "Can't delete reference '%s' as it doesn't exist\n", refPath)
				return errUpdateReference
			}

			fmt.Fprintf(os.Stderr, "Deleting reference '%s'\n", refPath)
			store.Delete([]byte(refPath))
		case packp.Update:
			if !exists {
				fmt.Fprintf(os.Stderr, "Can't update reference '%s' as it doesn't exist\n", refPath)
				return errUpdateReference
			}

			fmt.Fprintf(os.Stderr, "Updating reference '%s' to point to hash '%s'\n", refPath,
				cmd.New)
			writeReference(store, refPath, cmd)
		}
	}

	return nil
}
