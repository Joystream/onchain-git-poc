package cli

import (
	stdContext "context"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/utils"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/joystream/onchain-git-poc/x/gitService"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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

	log.Debug().Msgf("Getting local reference %v", refName)
	ref, err := repo.Storer.Reference(*refName)
	if err != nil {
		log.Debug().Msgf("Error getting reference %v: %+v", refName, err)
		return nil, err
	}
	hash := ref.Hash()

	// Get the HEAD commit for the ref from the local repository.
	commit, err := gogitobj.GetCommit(repo.Storer, hash)
	if err != nil {
		log.Debug().Msgf("Error getting commit for hash %s (%s): %+v\n",
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
	cliCtx context.CLIContext, author sdk.AccAddress, moduleName string) error {
	log.Debug().Msgf("Pushing refs %v from local to blockchain repo '%s'", refs, uri)

	// TODO: Support getting repo dir from user
	localRepoPath, err := filepath.Abs(".git")
	if err != nil {
		return err
	}
	log.Debug().Msgf("Using local Git repo at %v", localRepoPath)
	localStorage := filesystem.NewStorage(osfs.New(localRepoPath), cache.NewObjectLRUDefault())
	repo, err := gogit.Open(localStorage, nil)
	if err != nil {
		log.Debug().Msgf("Failed to open local repo: %v", err)
		return err
	}
	log.Debug().Msgf("Opened local repo successfully")

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

	err = pushToBlockChain(ctx, uri, refSpecs, repo, cliCtx, txBldr, author, moduleName)
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
			log.Debug().Msgf("Executing CmdPushRefs")
			cliCtx := context.NewCLIContext().WithCodec(cdc).WithAccountDecoder(cdc)
			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}

			author, err := cliCtx.GetFromAddress()
			if err != nil {
				return err
			}

			repo := args[0]

			txBldr := authtxb.NewTxBuilderFromCLI().WithCodec(cdc)
			ctx := stdContext.Background()
			if err := pushRefs(ctx, repo, args[1:], txBldr, cliCtx, author, moduleName); err != nil {
				return err
			}

			return nil
		},
	}
}

func removeRepo(ctx stdContext.Context, uri string, txBldr authtxb.TxBuilder,
	cliCtx context.CLIContext, author sdk.AccAddress, moduleName string) error {
	log.Debug().Msgf("Removing repository '%s' from blockchain", uri)
	msg, err := gitService.NewMsgRemoveRepository(uri, author)
	if err != nil {
		log.Debug().Msgf("Joystream client failed to create NewMsgRemoveRepository: %s", err)
		return err
	}
	log.Debug().Msgf("Joystream client sending MsgRemoveRepository to server for repo '%s'",
		msg.URI)

	if err := utils.CompleteAndBroadcastTxCli(txBldr, cliCtx, []sdk.Msg{msg}); err != nil {
		log.Debug().Msgf("Sending MsgRemoveRepository to node failed: %s", err)
		return err
	}

	return nil
}

// GetCmdRemoveRepo is the CLI command for removing a repository on the blockchain
func GetCmdRemoveRepo(moduleName string, cdc *codec.Codec) *cobra.Command {
	return &cobra.Command{
		Use:   "remove-repo repo",
		Short: "Remove a Git repository on the blockchain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug().Msgf("Executing CmdRemoveRepo")
			cliCtx := context.NewCLIContext().WithCodec(cdc).WithAccountDecoder(cdc)
			if err := cliCtx.EnsureAccountExists(); err != nil {
				return err
			}

			author, err := cliCtx.GetFromAddress()
			if err != nil {
				return err
			}

			repo := args[0]

			txBldr := authtxb.NewTxBuilderFromCLI().WithCodec(cdc)
			ctx := stdContext.Background()
			if err := removeRepo(ctx, repo, txBldr, cliCtx, author, moduleName); err != nil {
				return err
			}

			return nil
		},
	}
}
