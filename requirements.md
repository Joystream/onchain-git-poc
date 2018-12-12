# Requirements for On-Chain Git POC
Requirements for on-chain Git proof of concept, as use cases.

## Add Remote
Given that the user is in a local repository
When the user adds a repository on the blockchain as a remote
Then the remote should be added to the local repository

## Push to Existent Remote
Given that the user is in a local repository
And the local repository has a remote on the blockchain
And the repository exists on the blockchain
When the user pushes their master branch to the remote
Then the master branch of the repository on the blockchain should be in sync with the local one

## Fetch from Remote
