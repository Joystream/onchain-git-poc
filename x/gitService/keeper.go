package gitService

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/rs/zerolog/log"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/capability"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var reRepoURI = regexp.MustCompile("^[^/]+/[^/]+$")

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
func (k Keeper) GetAdvertisedReferences(ctx sdk.Context, owner string, repo string) (
	*packp.AdvRefs, error) {
	uri := fmt.Sprintf("%s/%s", owner, repo)
	log.Debug().Msgf("Keeper getting advertised references for repo '%s'", uri)
	ar := packp.NewAdvRefs()
	if err := setSupportedCapabilities(ar.Capabilities); err != nil {
		return nil, err
	}

	store := ctx.KVStore(k.gitStoreKey)
	if err := setReferences(store, ar, uri); err != nil {
		return nil, err
	}
	if err := setHead(store, ar, uri); err != nil {
		return nil, err
	}

	return ar, nil
}

func setSupportedCapabilities(c *capability.List) error {
	if err := c.Set(capability.Agent, capability.DefaultAgent); err != nil {
		return err
	}

	if err := c.Set(capability.OFSDelta); err != nil {
		return err
	}

	if err := c.Set(capability.DeleteRefs); err != nil {
		return err
	}

	return c.Set(capability.ReportStatus)
}

func setReferences(store sdk.KVStore, ar *packp.AdvRefs, uri string) error {
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		// TODO: Define which references should be included
		if strings.HasPrefix(key, fmt.Sprintf("%s/refs/", uri)) {
			refName := key[len(uri)+1:]
			hashBytes := store.Get([]byte(key))
			if hashBytes == nil {
				return fmt.Errorf("Couldn't get hash for reference '%s'", key)
			}
			hash := string(hashBytes)

			log.Debug().Msgf("Keeper adding reference '%s' -> '%s' to advertised references", refName, hash)
			ar.References[refName] = plumbing.NewHash(string(hash))
		}
	}

	return nil
}

func setHead(store sdk.KVStore, ar *packp.AdvRefs, uri string) error {
	headPath := fmt.Sprintf("%s/HEAD", uri)
	log.Debug().Msgf("Keeper determining repository head, path: '%s'", headPath)
	refBytes := store.Get([]byte(headPath))
	if refBytes == nil {
		log.Debug().Msgf("Repository doesn't have head")
		return nil
	}
	refStr := string(refBytes)

	ref := plumbing.NewReferenceFromStrings("HEAD", refStr)
	if ref.Type() == plumbing.SymbolicReference {
		log.Debug().Msgf("Repository head reference is symbolic, target: '%s'", refStr)
		if err := ar.AddReference(ref); err != nil {
			return nil
		}

		// Get target reference
		headPath = fmt.Sprintf("%s/%s", uri, ref.Target())
		log.Debug().Msgf("Keeper getting repository head reference, path: '%s'", headPath)
		refBytes = store.Get([]byte(headPath))
		if refBytes == nil {
			log.Debug().Msgf("Failed to get the contents of head reference at '%s'", headPath)
			return nil
		}
		refStr = string(refBytes)

		ref = plumbing.NewReferenceFromStrings(ref.Target().String(), refStr)
	}

	if ref.Type() != plumbing.HashReference {
		return plumbing.ErrInvalidType
	}

	h := ref.Hash()
	ar.Head = &h
	log.Debug().Msgf("Determined repo head: '%s'", ar.Head)

	return nil
}

// UpdateReferences updates a set of Git references
func (k Keeper) UpdateReferences(ctx sdk.Context, msg MsgUpdateReferences) sdk.Error {
	if !reRepoURI.MatchString(msg.URI) {
		log.Debug().Msgf("Invalid repo URI: '%s'", msg.URI)
		return sdk.ErrUnknownRequest(fmt.Sprintf("Invalid repo URI: '%s'", msg.URI))
	}

	log.Debug().Msgf("Keeper updating references in repo '%s'", msg.URI)
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
	log.Debug().Msgf("Keeper - store doesn't have repo '%s', initializing it", msg.URI)
	store.Set([]byte(fmt.Sprintf("%s/HEAD", msg.URI)), []byte("ref: refs/heads/master"))
	store.Set([]byte(fmt.Sprintf("%s/config", msg.URI)), []byte(`[core]
	repositoryformatversion = 0
	bare = true
`))
	return nil
}

func referenceExists(store sdk.KVStore, refPath string) bool {
	log.Debug().Msgf("Checking if reference exists: '%s'", refPath)
	if !store.Has([]byte(refPath)) {
		log.Debug().Msgf("Reference doesn't exist: '%s'", refPath)
		return false
	}

	log.Debug().Msgf("Reference exists: '%s'", refPath)
	return true
}

func writeReference(store sdk.KVStore, refPath string, cmd *UpdateReferenceCommand) {
	ref := plumbing.NewHashReference(cmd.Name, cmd.New)
	var content string
	switch ref.Type() {
	case plumbing.SymbolicReference:
		content = fmt.Sprintf("ref: %s\n", ref.Target())
	case plumbing.HashReference:
		content = ref.Hash().String()
	}
	log.Debug().Msgf("Writing reference '%s': '%s'", refPath, content)
	store.Set([]byte(refPath), []byte(content))
}

// updateReferences updates references in a repository
func updateReferences(store sdk.KVStore, msg MsgUpdateReferences) error {
	errUpdateReference := errors.New("Failed to update reference")

	log.Debug().Msgf("Updating references")
	for _, cmd := range msg.Commands {
		if !strings.HasPrefix(cmd.Name.String(), "refs/") {
			panic(fmt.Sprintf("Reference doesn't start with refs/: '%s'", cmd.Name))
		}
		refPath := fmt.Sprintf("%s/%s", msg.URI, cmd.Name)
		exists := referenceExists(store, refPath)
		switch cmd.Action() {
		case CreateAction:
			if exists {
				log.Debug().Msgf("Can't create reference '%s' as it already exists", refPath)
				return errUpdateReference
			}

			log.Debug().Msgf("Creating reference '%s' pointing to hash '%s'", refPath,
				cmd.New)
			writeReference(store, refPath, cmd)
		case packp.Delete:
			if !exists {
				log.Debug().Msgf("Can't delete reference '%s' as it doesn't exist", refPath)
				return errUpdateReference
			}

			log.Debug().Msgf("Deleting reference '%s'", refPath)
			store.Delete([]byte(refPath))
		case packp.Update:
			if !exists {
				log.Debug().Msgf("Can't update reference '%s' as it doesn't exist", refPath)
				return errUpdateReference
			}

			log.Debug().Msgf("Updating reference '%s' to point to hash '%s'", refPath,
				cmd.New)
			writeReference(store, refPath, cmd)
		}
	}

	return nil
}

// RemoveRepository deletes a repository
func (k Keeper) RemoveRepository(ctx sdk.Context, msg MsgRemoveRepository) sdk.Error {
	if !reRepoURI.MatchString(msg.URI) {
		log.Debug().Msgf("Invalid repository URI: '%s'", msg.URI)
		return sdk.ErrUnknownRequest(fmt.Sprintf("Invalid repository URI: '%s'", msg.URI))
	}

	log.Debug().Msgf("Keeper removing repository '%s'", msg.URI)
	// TODO: Verify that user is authorized to write to repo
	store := ctx.KVStore(k.gitStoreKey)
	iter := store.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		key := string(iter.Key())
		if strings.HasPrefix(key, fmt.Sprintf("%s/", msg.URI)) {
			log.Debug().Msgf("Keeper removing removing entry '%s' from store", key)
			store.Delete(iter.Key())
		}
	}

	return nil
}
