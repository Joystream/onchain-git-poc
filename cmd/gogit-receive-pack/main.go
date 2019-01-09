// Go-git server
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
	logf, err := os.Create("/tmp/gitservice/receive-pack.log")
	if err != nil {
		return err
	}
	defer logf.Close()

	dpath := args[0]

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logf})
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
