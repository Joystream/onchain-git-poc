package gitService

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewHandler returns a handler for "gitService" type messages.
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgPush:
			return handleMsgPush(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("Unrecognized gitService Msg type: %v", msg.Type())
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func handleMsgPush(ctx sdk.Context, keeper Keeper, msg MsgPush) sdk.Result {
	if err := keeper.PushRefs(ctx, msg.Repository, msg.URL, msg.PushBatches); err != nil {
		return sdk.Result{
			Code: err.Code(),
			Data: []byte(err.Error()),
		}
	}

	return sdk.Result{}
}
