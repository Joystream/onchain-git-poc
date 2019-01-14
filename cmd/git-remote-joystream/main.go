package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var reJoystreamURL = regexp.MustCompile("joystream://(.+)/(.+)/(.+)")

func handlePushBatch(args [][]string, repo repository) error {
	log.Debug().Msgf("Handling push batch for repo %v: %v", repo, args)
	return nil
	// repo, fs, err := r.initRepoIfNeeded(ctx, gitCmdPush)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// canPushAll, kbfsRepoEmpty, err := r.canPushAll(ctx, repo, args)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// localGit := osfs.New(r.gitDir)
	// localStorer, err := filesystem.NewStorage(localGit)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// refspecs := make(map[gogitcfg.RefSpec]bool, len(args))
	// for _, push := range args {
	// 	// `canPushAll` already validates the push reference.
	// 	refspec := gogitcfg.RefSpec(push[0])
	// 	refspecs[refspec] = true
	// }
	//
	// // Get all commits associated with the refs. This must happen before the
	// // push for us to be able to calculate the difference.
	// commits, err = r.parentCommitsForRef(ctx, localStorer,
	// 	repo.Storer, refspecs)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// var results map[string]error
	// // Ignore pushAll for commit collection, for now.
	// if canPushAll {
	// 	err = r.pushAll(ctx, fs)
	// 	// All refs in the batch get the same error.
	// 	results = make(map[string]error, len(args))
	// 	for _, push := range args {
	// 		// `canPushAll` already validates the push reference.
	// 		dst := dstNameFromRefString(push[0]).String()
	// 		results[dst] = err
	// 	}
	// } else {
	// 	results, err = r.pushSome(ctx, repo, fs, args, kbfsRepoEmpty)
	// }
	// if err != nil {
	// 	return nil, err
	// }
	//
	// err = r.waitForJournal(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// r.log.CDebugf(ctx, "Done waiting for journal")
	//
	// for d, e := range results {
	// 	result := ""
	// 	if e == nil {
	// 		result = fmt.Sprintf("ok %s", d)
	// 	} else {
	// 		result = fmt.Sprintf("error %s %s", d, e.Error())
	// 	}
	// 	_, err = r.output.Write([]byte(result + "\n"))
	// }
	//
	// // Remove any errored commits so that we don't send an update
	// // message about them.
	// for refspec := range refspecs {
	// 	dst := refspec.Dst("")
	// 	if results[dst.String()] != nil {
	// 		r.log.CDebugf(
	// 			ctx, "Removing commit result for errored push on refspec %s",
	// 			refspec)
	// 		delete(commits, dst)
	// 	}
	// }
	//
	// if len(commits) > 0 {
	// 	err = libgit.UpdateRepoMD(ctx, r.config, r.h, fs,
	// 		keybase1.GitPushType_DEFAULT, "", commits)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	//
	// err = r.checkGC(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// _, err = r.output.Write([]byte("\n"))
	// if err != nil {
	// 	return nil, err
	// }
	// return commits, nil
}

func handleList(repo repository, command []string) error {
	log.Debug().Msgf("Listing refs in %v - command: %v", repo, command)
	if len(command) == 1 && command[0] == "for-push" {
		log.Debug().Msgf("Treating for-push the same as a regular list")
	} else if len(command) > 0 {
		return fmt.Errorf("Bad list request: %v", command)
	}

	// TODO: Ask gitservicecli to query for refs in repo
	// TODO: Print refs got from gitservicecli

	fmt.Printf("\n")
	return nil
}

type repository struct {
	chainID string
	owner   string
	name    string
}

func (r repository) String() string {
	return fmt.Sprintf("%v/%v/%v", r.chainID, r.owner, r.name)
}

func cmdRoot(_ *cobra.Command, args []string) error {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	var url string
	if len(args) == 1 {
		url = args[0]
	} else {
		url = args[1]
	}
	var m []string
	if m = reJoystreamURL.FindStringSubmatch(url); m == nil {
		return fmt.Errorf("URL on invalid format: '%v'", url)
	}
	repo := repository{chainID: m[1], owner: m[2], name: m[3]}

	log.Debug().Msgf("Starting, repo: %v/%v/%v", repo.chainID, repo.owner, repo.name)

	var pushBatch [][]string
	reader := bufio.NewReader(os.Stdin)
	// Read commands from stdin until closed
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Debug().Msgf("Ending due to closed input")
			break
		}

		command := strings.TrimSpace(line)
		commandParts := strings.Fields(command)
		log.Debug().Msgf("Received command '%v'", command)
		if len(commandParts) == 0 {
			log.Debug().Msgf("Received a blank line, command terminated")
			if len(pushBatch) > 0 {
				log.Debug().Msgf("Processing push batch")
				if err := handlePushBatch(pushBatch, repo); err != nil {
					return err
				}

				pushBatch = nil
			}
		} else {
			var err error
			switch commandParts[0] {
			case "capabilities":
				fmt.Printf("push\n\n")
			case "list":
				handleList(repo, commandParts[1:])
			case "push":
				log.Debug().Msgf("Pushing - args: %v, %v", args[0], args[1])
				pushBatch = append(pushBatch, commandParts[1:])
				log.Debug().Msgf("Push batch: %v", pushBatch)
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	cobra.EnableCommandSorting = false

	rootCmd := &cobra.Command{
		Use:   "git-remote-joystream repository [URL]",
		Short: "Git remote helper for joystream blockchain",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  cmdRoot,
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Msgf("Unrecoverable error: %s", err)
	}
}
