# TODOs

## Implement Getting of Advertised References
Implement getting of advertised references from the server, so that we can implement updating
of references in on-chain repos, and not just pushing new ones.

## Implement Updating of On-Chain Repos
Implement updating of on-chain repos so that you can update a reference with new commits.

### Authentication
We should probably use the auth module in order to implement account handling.

### On-Chain Data Modeling
We need to find out how the Git data (as contained in `ReferenceUpdateRequest`s) can be stored
on the blockchain.

1. Push a reference to another local repo, analyzing go-git's behaviour in applying the
   `ReferenceUpdateRequest` and unpacking the packfile.
2. Having deduced go-git's application of `ReferenceUpdateRequest`s in the filesystem, determine
   how to achieve the same on the blockchain.
