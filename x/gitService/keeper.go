package gitService

import (
	"github.com/cosmos/cosmos-sdk/codec"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper maintains the link to data storage and exposes getter/setter methods for the various
// parts of the state machine
type Keeper struct {
	gitStoreKey  sdk.StoreKey

	cdc *codec.Codec // The wire codec for binary encoding/decoding.
}

// ListRefs - Lists refs for a repository
func (k Keeper) ListRefs(ctx sdk.Context, repository string, url string) []string {
	// store := ctx.KVStore(k.gitStoreKey)
	// return string(store.Get([]byte(url)))
	return []string{}
}

// PushRefs - push a set of batches of refs
func (k Keeper) PushRefs(ctx sdk.Context, repository string, url string, pushBatches [][]string) sdk.Error {
	// store := ctx.KVStore(k.gitStoreKey)
	// TODO: Store refs
	return nil
}

// NewKeeper creates new instances of the gitService Keeper
func NewKeeper(gitStoreKey sdk.StoreKey, cdc *codec.Codec) Keeper {
	return Keeper{
		gitStoreKey:  gitStoreKey,
		cdc:            cdc,
	}
}
