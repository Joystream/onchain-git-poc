# onchain-git
A [Git remote helper](https://git-scm.com/docs/git-remote-helpers) and underlying
[Cosmos/Tendermint](https://cosmos.network/developers) app for synchronizing with remote
repositories on our Cosmos/Tendermint based blockchain.

## Structure
### cmd/git-remote-joystream
Git remote helper, not functional as of now, but should be trivial to wire up against
`gitservicecli` which does the real work.

### cmd/gitservicecli
Cosmos/Tendermint client app that mainly supports pushing references to a repository on
the blockchain. It's implemented as a standard Tendermint app, so it will function by sending
messages to corresponding servers (`gitserviced`).

### cmd/gitserviced
Cosmos/Tendermint server app that supports the following:

* Listing of references (required by the Git remote helper)
* Querying of advertised references (required in conjunction with pushing of references)
* Pushing of references
* Removal of repositories

A server instance will respond to queries (for reference listing or advertised references)
and messages to push references to repositories or remove repositories.

#### Pushing references
A message for pushing of references will cause updates to references in the corresponding
repository to be stored in a blockchain transaction. This information will basically consist
of a list of commands to update the references and a packfile containing Git data
(commits/trees/blobs).

When a server processes such a message, it will also update the corresponding repository in
app storage. If the repository isn't already there it will be initialized.

### cmd/gogitclient
This is a test application to study the behaviour of go-git when it comes to serving pushing
of updates. It's basically a simplified Git client that supports the `push` command and will
use our replacement for [`git-receive-pack`](https://git-scm.com/docs/git-receive-pack),
`gogit-receive-pack` when using the `debugf` protocol, which basically is for pushing to
a remote in the local filesystem but using our own back-end (`gogit-receive-pack`).

### cmd/gogit-receive-pack
This is a re-implementation of `git-receive-pack` using the `go-git` library that's instrumented
through debug logging, so that we can understand how go-git serves pushing of updates.

## References
* https://git-scm.com/docs/git-remote-helpers
* https://keybase.io/blog/encrypted-git-for-everyone
* https://github.com/src-d/go-git
* https://cosmos.network/developers
