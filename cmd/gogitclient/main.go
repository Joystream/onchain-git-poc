// Git client that can push references to a go-git server
package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/client"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/file"
)

func init() {
	client.InstallProtocol("debugf", file.NewClient(
		"git-upload-pack",
		"gogit-receive-pack",
	))
}

func main() {
	cmdPush := &cobra.Command{
		Use:          "push <remote> <refspec>...",
		Long:         "Push a set of ref-specs to a remote",
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

			log.Debug().Msgf("Pushing ref-specs to '%s': %v", args[0], args[1:])
			r, err := git.PlainOpen(".")
			if err != nil {
				return err
			}

			remote, err := r.Remote(args[0])
			if err != nil {
				return err
			}

			refSpecs := make([]config.RefSpec, 0, len(args[1:]))
			for _, r := range args[1:] {
				refSpecs = append(refSpecs, config.RefSpec(r))
			}

			if err := remote.Push(&git.PushOptions{
				RefSpecs: refSpecs,
			}); err != nil {
				return err
			}

			return nil
		},
	}

	rootCmd := &cobra.Command{
		Use:   "gogitclient <command>",
		Short: "Go-git client",
	}
	rootCmd.AddCommand(cmdPush)
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msgf("Unrecoverable error: %s", err)
	}
}
