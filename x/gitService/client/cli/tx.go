package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cosmos/cosmos-sdk/client/context"
	// "github.com/cosmos/cosmos-sdk/client/utils"
	"github.com/cosmos/cosmos-sdk/codec"
	// "github.com/joystream/onchain-git-poc/x/gitService"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing"
	gogitobj "gopkg.in/src-d/go-git.v4/plumbing/object"
	gogitstor "gopkg.in/src-d/go-git.v4/plumbing/storer"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
)

const (
	maxCommitsToVisitPerRef = 20
)

type refData struct {
	IsDelete bool
	Commits  []*gogitobj.Commit
}

func getParentCommitsForRef(refSpec gogitcfg.RefSpec, localStorage *filesystem.Storage,
		/*, remoteStorer gogitstor.Storer*/) (*refData, error) {
	var rd *refData
	if refSpec.IsDelete() {
		rd = &refData{
			IsDelete: true,
		}
		return rd, nil
	}

	refName := plumbing.ReferenceName(refSpec.Src())
	fmt.Fprintf(os.Stderr, "Resolving reference %v in local repo\n", refName)
	resolved, err := gogitstor.ResolveReference(localStorage, refName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving ref %s\n", refName)
	}
	if resolved != nil {
		refName = resolved.Name()
		fmt.Fprintf(os.Stderr, "Resolved local reference to %v\n", refName)
	}

	fmt.Fprintf(os.Stderr, "Getting local reference %v\n", refName)
	ref, err := localStorage.Reference(refName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting reference %v: %+v", refName, err)
		return nil, err
	}
	hash := ref.Hash()

	// Get the HEAD commit for the ref from the local repository.
	commit, err := gogitobj.GetCommit(localStorage, hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commit for hash %s (%s): %+v\n",
			string(hash[:]), refName, err)
		return nil, err
	}

	// Iterate through the commits backward, until we experience any of the
	// following:
	// 1. Find a commit that the remote knows about,
	// 2. Reach our maximum number of commits to check,
	// 3. Run out of commits.
	haves := make(map[plumbing.Hash]bool)
	walker := gogitobj.NewCommitPreorderIter(commit, haves, nil)
	toVisit := maxCommitsToVisitPerRef
	rd = &refData{
		IsDelete: refSpec.IsDelete(),
		Commits:  make([]*gogitobj.Commit, 0, maxCommitsToVisitPerRef),
	}
	err = walker.ForEach(func(c *gogitobj.Commit) error {
		haves[c.Hash] = true
		toVisit--
		// If toVisit starts out at 0 (indicating there is no
		// max), then it will be negative here and we won't stop
		// early.
		if toVisit == 0 {
			// Append a sentinel value to communicate that there would be
			// more commits.
		rd.Commits = append(rd.Commits, nil)
			return gogitstor.ErrStop
		}
		// TODO: Stop if object (as represented by hash) exists in remote repo
		// hasEncodedObjectErr := remoteStorer.HasEncodedObject(c.Hash)
		// if hasEncodedObjectErr == nil {
		// 	return gogitstor.ErrStop
		// }
		rd.Commits = append(rd.Commits, c)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// return commitsByRef, nil

	return rd, nil
}

func pushRef(uri string, ref string, txBldr authtxb.TxBuilder, cliCtx context.CLIContext,
		account sdk.AccAddress) error {
	fmt.Fprintf(os.Stderr, "Reading ref %v from local Git repo\n", ref)

	// TODO: Support getting repo dir from user
	localRepoPath, err := filepath.Abs(".git")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Using local Git repo at %v\n", localRepoPath)
	localStorage := filesystem.NewStorage(osfs.New(localRepoPath), cache.NewObjectLRUDefault())

	// Get all commits associated with the refs. This must happen before the
	// push for us to be able to calculate the difference.
	refSpec := gogitcfg.RefSpec(ref)
	rd, err := getParentCommitsForRef(refSpec, localStorage)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Found %v commit(s) for destination reference %v\n",
		len(rd.Commits), refSpec.Dst(""))

	// var results map[string]error
	// Ignore pushAll for commit collection, for now.
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
	// err = pushSome(ctx, repo, fs, args, kbfsRepoEmpty)
	// }
	// if err != nil {
	// 	return nil, err
	// }

	// err = r.waitForJournal(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// r.log.CDebugf(ctx, "Done waiting for journal")

	// for d, e := range results {
	// 	result := ""
	// 	if e == nil {
	// 		result = fmt.Sprintf("ok %s", d)
	// 	} else {
	// 		result = fmt.Sprintf("error %s %s", d, e.Error())
	// 	}
	// 	_, err = r.output.Write([]byte(result + "\n"))
	// }

	// Remove any errored commits so that we don't send an update
	// message about them.
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

	// msg := gitService.NewMsgPushRef(uri, ref, account)
	// if err := msg.ValidateBasic(); err != nil {
	// 	return err
	// }
	//
	// cliCtx.PrintResponse = true
	// if err := utils.CompleteAndBroadcastTxCli(txBldr, cliCtx, []sdk.Msg{msg}); err != nil {
	// 	return err
	// }

	return nil
}

// GetCmdPushRefs is the CLI command for pushing Git refs to the blockchain
func GetCmdPushRefs(cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "push-refs URI ref...",
		Short: "Push Git refs to a certain URI on the blockchain",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Executing CmdPushRefs\n")
			cliCtx := context.NewCLIContext().WithCodec(cdc).WithAccountDecoder(cdc)
			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}

			account, err := cliCtx.GetFromAddress()
			if err != nil {
				return err
			}

			uri := args[0]
			txBldr := authtxb.NewTxBuilderFromCLI().WithCodec(cdc)
			for _, ref := range args[1:] {
				if err := pushRef(uri, ref, txBldr, cliCtx, account); err != nil {
					return err
				}
			}

			return nil
		},
	}
}
