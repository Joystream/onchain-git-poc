package cli

import (
	stdContext "context"
	encJson "encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"
	// "github.com/cosmos/cosmos-sdk/client/utils"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"gopkg.in/src-d/go-billy.v4/osfs"
	gogit "gopkg.in/src-d/go-git.v4"
	gogitcfg "gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	gogitobj "gopkg.in/src-d/go-git.v4/plumbing/object"
	gogitstor "gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

const (
	maxCommitsToVisitPerRef = 20
	localRepoRemoteName     = "local"
)

type refData struct {
	IsDelete bool
	Commits  []*gogitobj.Commit
}

func realPush(ctx stdContext.Context, refSpec gogitcfg.RefSpec, repo *gogit.Repository,
	localRepoPath string, localStorage *filesystem.Storage) error {

	return nil
}

func getParentCommitsForRef(refSpec gogitcfg.RefSpec, repo *gogit.Repository,

/*, remoteStorer gogitstor.Storer*/) (*refData, error) {
	var rd *refData
	if refSpec.IsDelete() {
		rd = &refData{
			IsDelete: true,
		}
		return rd, nil
	}

	refName, err := resolveLocalRef(refSpec, repo)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Getting local reference %v\n", refName)
	ref, err := repo.Storer.Reference(*refName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting reference %v: %+v", refName, err)
		return nil, err
	}
	hash := ref.Hash()

	// Get the HEAD commit for the ref from the local repository.
	commit, err := gogitobj.GetCommit(repo.Storer, hash)
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

func pushRefs(ctx stdContext.Context, uri string, refs []string, txBldr authtxb.TxBuilder,
	cliCtx context.CLIContext, author sdk.AccAddress, advRefs *packp.AdvRefs) error {
	fmt.Fprintf(os.Stderr, "Pushing refs %v from local to blockchain repo '%s'\n", refs, uri)

	// TODO: Support getting repo dir from user
	localRepoPath, err := filepath.Abs(".git")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Using local Git repo at %v\n", localRepoPath)
	localStorage := filesystem.NewStorage(osfs.New(localRepoPath), cache.NewObjectLRUDefault())
	repo, err := gogit.Open(localStorage, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open local repo: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "Opened local repo successfully\n")

	// Get all commits associated with the refs. This must happen before the
	// push for us to be able to calculate the difference.
	refSpecs := make([]gogitcfg.RefSpec, 0, len(refs))
	for _, ref := range refs {
		refSpec := gogitcfg.RefSpec(ref)
		if err = refSpec.Validate(); err != nil {
			return err
		}

		refSpecs = append(refSpecs, refSpec)
	}

	err = pushToBlockChain(ctx, uri, refSpecs, repo, advRefs, cliCtx, txBldr, author)
	if err != nil {
		return err
	}

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
func GetCmdPushRefs(moduleName string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "push-refs repo ref...",
		Short: "Push Git refs to a certain repository on the blockchain",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr, "Executing CmdPushRefs\n")
			cliCtx := context.NewCLIContext().WithCodec(cdc).WithAccountDecoder(cdc)
			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}

			author, err := cliCtx.GetFromAddress()
			if err != nil {
				return err
			}

			repo := args[0]

			fmt.Fprintf(os.Stderr, "Querying for advertised references in repository '%s'\n", repo)
			res, err := cliCtx.QueryWithData(fmt.Sprintf("custom/%s/advertisedReferences/%s",
				moduleName, repo), nil)
			if err != nil {
				return err
			}
			var advRefs *packp.AdvRefs
			if err := encJson.Unmarshal(res, &advRefs); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Got advertised references from server: %v\n", advRefs.References)

			txBldr := authtxb.NewTxBuilderFromCLI().WithCodec(cdc)
			ctx := stdContext.Background()
			if err := pushRefs(ctx, repo, args[1:], txBldr, cliCtx, author, advRefs); err != nil {
				return err
			}

			return nil
		},
	}
}
