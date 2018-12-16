package client

import (
	"github.com/cosmos/cosmos-sdk/client"
	gitServiceCmd "github.com/joystream/onchain-git-poc/x/gitService/client/cli"
	"github.com/spf13/cobra"
	amino "github.com/tendermint/go-amino"
)

// ModuleClient exports all client functionality from this module
type ModuleClient struct {
	storeKey string
	cdc      *amino.Codec
}

// NewModuleClient is the ModuleClient constructor
func NewModuleClient(storeKey string, cdc *amino.Codec) ModuleClient {
	return ModuleClient{storeKey, cdc}
}

// GetQueryCmd returns the cli query commands for this module
func (mc ModuleClient) GetQueryCmd() *cobra.Command {
	// Group gov queries under a subcommand
	govQueryCmd := &cobra.Command{
		Use:   "gitService",
		Short: "GitService query commands",
	}

	govQueryCmd.AddCommand(client.GetCommands(
		gitServiceCmd.GetCmdListRefs(mc.storeKey, mc.cdc),
	)...)

	return govQueryCmd
}

// GetTxCmd returns the cli transaction commands for this module
func (mc ModuleClient) GetTxCmd() *cobra.Command {
	// Group gov queries under a subcommand
	govTxCmd := &cobra.Command{
		Use:   "gitService",
		Short: "GitService transaction commands",
	}

	return govTxCmd
}
