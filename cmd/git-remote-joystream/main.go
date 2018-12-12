package main

import (
	"log"
	"bufio"
	"os"
	"strings"
	"fmt"

	"github.com/spf13/cobra"
)

func handlePushBatch(args [][]string) error {
	fmt.Fprintf(os.Stderr, "Handling push batch: %v\n", args)
	fmt.Printf("\n")
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

func handleList(args []string) error {
	if len(args) == 1 && args[0] == "for-push" {
		fmt.Fprintf(os.Stderr, "Treating for-push the same as a regular list\n")
	} else if len(args) > 0 {
		return fmt.Errorf("Bad list request: %v", args)
	}

	// repo, _, err := r.initRepoIfNeeded(ctx, gitCmdList)
	// if err != nil {
	// 	return err
	// }
	//
	// refs, err := repo.References()
	// if err != nil {
	// 	return err
	// }
	//
	// var symRefs []string
	// hashesSeen := false
	// for {
	// 	ref, err := refs.Next()
	// 	if errors.Cause(err) == io.EOF {
	// 		break
	// 	}
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	value := ""
	// 	switch ref.Type() {
	// 	case plumbing.HashReference:
	// 		value = ref.Hash().String()
	// 		hashesSeen = true
	// 	case plumbing.SymbolicReference:
	// 		value = "@" + ref.Target().String()
	// 	default:
	// 		value = "?"
	// 	}
	// 	refStr := value + " " + ref.Name().String() + "\n"
	// 	if ref.Type() == plumbing.SymbolicReference {
	// 		// Don't list any symbolic references until we're sure
	// 		// there's at least one object available.  Otherwise
	// 		// cloning an empty repo will result in an error because
	// 		// the HEAD symbolic ref points to a ref that doesn't
	// 		// exist.
	// 		symRefs = append(symRefs, refStr)
	// 		continue
	// 	}
	// 	r.log.CDebugf(ctx, "Listing ref %s", refStr)
	// 	_, err = r.output.Write([]byte(refStr))
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	//
	// if hashesSeen {
	// 	for _, refStr := range symRefs {
	// 		r.log.CDebugf(ctx, "Listing symbolic ref %s", refStr)
	// 		_, err = r.output.Write([]byte(refStr))
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }
	//
	// err = r.waitForJournal(ctx)
	// if err != nil {
	// 	return err
	// }
	// r.log.CDebugf(ctx, "Done waiting for journal")
	//
	// _, err = r.output.Write([]byte("\n"))
	// return err
	fmt.Printf("\n")
	return nil
}

func cmdRoot(_ *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stderr, "Starting\n")

	var pushBatch [][]string
	reader := bufio.NewReader(os.Stdin)
	// Read commands from stdin until closed
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ending due to closed input\n")
			break
		}

		command := strings.TrimSpace(line)
		commandParts := strings.Fields(command)
		fmt.Fprintf(os.Stderr, "Received command '%v'\n", command)
		if len(commandParts) == 0 {
			fmt.Fprintf(os.Stderr, "Received a blank line, command terminated\n")
			if len(pushBatch) > 0 {
				fmt.Fprintf(os.Stderr, "Processing push batch\n")
				if err := handlePushBatch(pushBatch); err != nil {
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
				handleList(commandParts[1:])
			case "push":
				fmt.Fprintf(os.Stderr, "Pushing - args: %v, %v\n", args[0], args[1])
				pushBatch = append(pushBatch, commandParts[1:])
				fmt.Fprintf(os.Stderr, "Push batch: %v\n", pushBatch)
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
		Args:	 cobra.RangeArgs(1, 2),
		RunE: 	cmdRoot,
	}
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Unrecoverable error: %v\n", err)
	}
}
