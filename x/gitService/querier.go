package gitService

import (
	encJson "encoding/json"
	"fmt"

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
		default:
			return nil, sdk.ErrUnknownRequest(
				fmt.Sprintf("unknown gitService query endpoint: '%v'", root))
		}
	}
}

// nolint: unparam
func queryListRefs(ctx sdk.Context, path []string, req abci.RequestQuery, keeper Keeper) ([]byte, sdk.Error) {
	refs := keeper.ListRefs(ctx, path[0], path[1])
	bytes, err := encJson.Marshal(refs)
	if err != nil {
		return nil, sdk.ErrInternal(err.Error())
	}

	return bytes, nil
}
