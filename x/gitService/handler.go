package gitService

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/rs/zerolog/log"
)

// NewHandler returns a handler for "gitService" type messages.
func NewHandler(keeper Keeper) sdk.Handler {
	return func(ctx sdk.Context, msg sdk.Msg) sdk.Result {
		switch msg := msg.(type) {
		case MsgUpdateReferences:
			return handleMsgUpdateReferences(ctx, keeper, msg)
		case MsgRemoveRepository:
			return handleMsgRemoveRepository(ctx, keeper, msg)
		default:
			errMsg := fmt.Sprintf("Unrecognized gitService Msg type: %v", msg.Type())
			return sdk.ErrUnknownRequest(errMsg).Result()
		}
	}
}

func handleMsgUpdateReferences(ctx sdk.Context, keeper Keeper, msg MsgUpdateReferences) sdk.Result {
	log.Debug().Msgf("Handling MsgUpdateReferences - author: '%s', repo: '%s'",
		msg.Author, msg.URI)
	if err := keeper.UpdateReferences(ctx, msg); err != nil {
		return sdk.Result{
			Code: err.Code(),
			Data: []byte(err.Error()),
		}
	}

	return sdk.Result{}
}

func handleMsgRemoveRepository(ctx sdk.Context, keeper Keeper, msg MsgRemoveRepository) sdk.Result {
	log.Debug().Msgf("Handling MsgRemoveRepo - author: '%s', repo: '%s'",
		msg.Author, msg.URI)
	if err := keeper.RemoveRepository(ctx, msg); err != nil {
		return sdk.Result{
			Code: err.Code(),
			Data: []byte(err.Error()),
		}
	}

	return sdk.Result{}
}
