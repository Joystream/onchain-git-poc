// Go-git implementation of git-receive-pack
//
// git-receive-pack is a command used by Git servers to implement the receive-pack protocol,
// i.e. handling pushing of references (which the server is to receive). We implement our own
// so that we can analyze go-git's behaviour (through instrumentation) and see how it functions
// when we push data to it (via our gogitclient command).
package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/file"
)

func cmdRoot(_ *cobra.Command, args []string) error {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	logf, err := os.OpenFile("/tmp/gitservice/receive-pack.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE,
		0755)
	if err != nil {
		return err
	}
	defer logf.Close()
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logf})

	dpath := args[0]

	log.Debug().Msgf("Receiving into '%s', calling file.ServeReceivePack", dpath)
	if err := file.ServeReceivePack(dpath); err != nil {
		log.Debug().Msgf("file.ServeReceivePack failed: %s", err)
		return err
	}
	log.Debug().Msgf("file.ServeReceivePack succeeded")

	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:          "gogit-receive-pack",
		Short:        "Go-git version of git-receive-pack",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         cmdRoot,
	}
	if err := rootCmd.Execute(); err != nil {
		log.Info().Msgf("Failure: %s", err)
		log.Fatal().Err(err).Msg("Unrecoverable error")
	}

	log.Info().Msg("Success")
}
