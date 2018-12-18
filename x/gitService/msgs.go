package gitService

import (
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MsgPushRef defines the Push message
type MsgPushRef struct {
	URI  string
	Ref string
	Owner sdk.AccAddress
}

// NewMsgPushRef is the constructor function for MsgPushRef
func NewMsgPushRef(uri string, ref string, owner sdk.AccAddress) MsgPushRef {
	return MsgPushRef{
		URI:  uri,
		Ref:  ref,
		Owner: owner,
	}
}

// Route implements Msg.
func (msg MsgPushRef) Route() string { return "gitService" }

// Type implements Msg.
func (msg MsgPushRef) Type() string { return "push" }

// ValidateBasic Implements Msg.
func (msg MsgPushRef) ValidateBasic() sdk.Error {
	if msg.Owner.Empty() {
		return sdk.ErrInvalidAddress(msg.Owner.String())
	}
	if len(msg.URI) == 0 {
		return sdk.ErrUnknownRequest("URI cannot be empty")
	}
	if len(msg.Ref) == 0 {
		return sdk.ErrUnknownRequest("Ref cannot be empty")
	}
	if msg.Owner == nil {
		return sdk.ErrUnknownRequest("Owner cannot be empty")
	}

	return nil
}

// GetSignBytes Implements Msg.
func (msg MsgPushRef) GetSignBytes() []byte {
	b, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	return sdk.MustSortJSON(b)
}

// GetSigners Implements Msg.
func (msg MsgPushRef) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Owner,}
}
