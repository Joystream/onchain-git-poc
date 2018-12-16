package gitService

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MsgPush defines the Push message
type MsgPush struct {
	Repository string
	URL  string
	PushBatches [][]string
	Owner sdk.AccAddress
}

// NewMsgPush is the constructor function for MsgPush
func NewMsgPush(repository string, url string, pushBatches [][]string) MsgPush {
	return MsgPush{
		Repository: repository,
		URL:    url,
		PushBatches:  pushBatches,
	}
}

// Route implements Msg.
func (msg MsgPush) Route() string { return "gitService" }

// Type implements Msg.
func (msg MsgPush) Type() string { return "push" }

// ValidateBasic Implements Msg.
func (msg MsgPush) ValidateBasic() sdk.Error {
	if msg.Owner.Empty() {
		return sdk.ErrInvalidAddress(msg.Owner.String())
	}
	if len(msg.URL) == 0 {
		return sdk.ErrUnknownRequest("URL cannot be empty")
	}
	if len(msg.Repository) == 0 {
		return sdk.ErrUnknownRequest("Repository cannot be empty")
	}
	if len(msg.PushBatches) == 0 {
		return sdk.ErrUnknownRequest("PushBatches cannot be empty")
	}

	return nil
}

// GetSignBytes Implements Msg.
func (msg MsgPush) GetSignBytes() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return sdk.MustSortJSON(b)
}

// GetSigners Implements Msg.
func (msg MsgPush) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Owner,}
}
