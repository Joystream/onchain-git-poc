# Requirements for On-Chain Git POC
Requirements for on-chain Git proof of concept, as use cases.

## Push to Uninitialized Repository
Given that the user is in a local repository
When the user pushes their master branch to a Joystream repository that doesn't exist
Then the corresponding repository should be created on the Joystream blockchain
And the commit history of the local master branch should be written to the remote repository
And a master branch should be created in the remote repository referencing the same commit
as the local one

## Push to Initialized Repository One Commit Behind
Given that the user is in a local repository
When the user pushes their master branch to a Joystream repository that already exists
and has a master branch one commit behind the local one
Then the master branch of the aforementioned remote repository should be brought up to date with
the local one

## Fetch from Remote Repository One Commit Ahead
Given that the user is in a local repository
And has checked out the master branch
When the user pulls the master branch of a Joystream repository which is one commit ahead of the
local one
Then the master branch of the local repository should be brought up to date with the remote one
