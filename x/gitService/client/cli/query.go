package cli

import (
	"fmt"
	"os"
	encJson "encoding/json"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/spf13/cobra"
)

// GetCmdListRefs lists Git refs
func GetCmdListRefs(queryRoute string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "list URL",
		Short: "List Git refs in repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			fmt.Fprintf(os.Stderr, "Listing refs of repo %v\n", url)

			cliCtx := context.NewCLIContext().WithCodec(cdc)
			res, err := cliCtx.QueryWithData(fmt.Sprintf("custom/%s/listRefs/%s", queryRoute, url), nil)
			if err != nil {
				return err
			}

			var refs []string
			if err := encJson.Unmarshal(res, &refs); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Received refs: %v\n", refs)
			fmt.Printf("\n")

			return nil
		},
	}
}
