package gitService

import (
	encJson "encoding/json"
	"fmt"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

// NewQuerier is the module level router for state queries
func NewQuerier(keeper Keeper) sdk.Querier {
	return func(ctx sdk.Context, path []string, req abci.RequestQuery) (res []byte, err sdk.Error) {
		root := path[0]
		switch root {
		case "listRefs":
			return queryListRefs(ctx, path[1:], req, keeper)
		case "advertisedReferences":
			return queryAdvertisedReferences(ctx, path[1:], req, keeper)
		default:
			return nil, sdk.ErrUnknownRequest(
				fmt.Sprintf("Unknown gitService query endpoint: '%s'", root))
		}
	}
}

// nolint: unparam
func queryListRefs(ctx sdk.Context, path []string, req abci.RequestQuery, keeper Keeper) (
	[]byte, sdk.Error) {
	fmt.Fprintf(os.Stderr, "queryListRefs: %v\n", path)
	refs := keeper.ListRefs(ctx, path[0], path[1])
	bytes, err := encJson.Marshal(refs)
	if err != nil {
		return nil, sdk.ErrInternal(err.Error())
	}

	return bytes, nil
}

func queryAdvertisedReferences(ctx sdk.Context, path []string, req abci.RequestQuery, keeper Keeper) (
	[]byte, sdk.Error) {
	fmt.Fprintf(os.Stderr, "Querying for advertised references\n")
	advRefs := keeper.GetAdvertisedReferences(ctx, path[0], path[1])
	bytes, err := encJson.Marshal(advRefs)
	if err != nil {
		return nil, sdk.ErrInternal(err.Error())
	}

	return bytes, nil
}
