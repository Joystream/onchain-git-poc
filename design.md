# Design Document

## Pushing to Blockchain

### Fetch into Remote (Alternative A)
We push from a local repo to a blockchain one by adding a remote to the blockchain
repo pointing to the local one, and then fetching from said remote. This causes data from
the local repo to be written to a go-git `Storer` which again writes data to the blockchain.

### Push to Remote (Alternative B)
We push from a local repo to a blockchain one by using go-git's push functionality.

## ReferenceUpdateRequest Format
When pushing to another Git repo, changes are encoded in a so-called `ReferenceUpdateRequest`,
which contains commands to add/update/delete references and a so-called packfile, which contains
file data. In this section, we describe the `ReferenceUpdateRequest` and packfile formats.
