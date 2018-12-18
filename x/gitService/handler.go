package gitService

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewHandler returns a handler for "gitService" type messages.
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgPushRef:
			return handleMsgPushRef(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("Unrecognized gitService Msg type: %v", msg.Type())
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func handleMsgPushRef(ctx sdk.Context, keeper Keeper, msg MsgPushRef) sdk.Result {
	if err := keeper.PushRef(ctx, msg.URI, msg.Ref, msg.Owner); err != nil {
		return sdk.Result{
			Code: err.Code(),
			Data: []byte(err.Error()),
		}
	}

	return sdk.Result{}
}
