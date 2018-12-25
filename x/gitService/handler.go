package gitService

import (
	"fmt"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewHandler returns a handler for "gitService" type messages.
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgUpdateReferences:
			return handleMsgUpdateReferences(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("Unrecognized gitService Msg type: %v", msg.Type())
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func handleMsgUpdateReferences(ctx sdk.Context, keeper Keeper, msg MsgUpdateReferences) sdk.Result {
	fmt.Fprintf(os.Stderr, "Handling MsgUpdateReferences - author: '%s', repo: '%s'\n",
		msg.Author, msg.URI)
	if err := keeper.UpdateReferences(ctx, msg); err != nil {
		return sdk.Result{
			Code: err.Code(),
			Data: []byte(err.Error()),
		}
	}

	return sdk.Result{}
}
