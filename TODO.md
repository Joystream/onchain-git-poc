# TODOs

## Write Pushed Git Data to Cosmos Store
1. helper: Receive request to list refs from Git
2. helper: Send request to gitservicecli to query refs in repo, chain ID being the first component
  of the URL
3. cli: Get refs in remote repo - create repo if it doesn't exist
4. cli: Write refs to terminal
5. helper: Read refs from cli output
6. helper: Write refs to terminal
7. helper: Receive request to push refs to remote
8. helper: Send request to gitservicecli to write refs to repo, chain ID being the first URL
  component
9. cli: Determine which references to update/add/delete on the blockchain
10. cli: Encode changes as a ReferenceUpdateRequest, including a packfile
11. cli: Send ReferenceUpdateRequest to server
12. server: Receive ReferenceUpdateRequest as encoded byte stream and store contained changes to blockchain
  12.1 server: In `ReceivePack` (plumbing/transport/server/server.go), decode ReferenceUpdateRequest from received byte stream
  12.2 server: Verify that capabilities are compatible with those in request
  12.3 server: Write packfile contents to storage
    12.3.1 server: Call `s.PackfileWriter()` if `Storer` object has this method, as for example `ObjectStorage` does.
    12.3.2 server: Use `io.Copy` to copy packfile to writer obtained by calling `s.PackfileWriter`.
  12.4 server: Update references in storage according to request (add/update/delete)

### Authentication
We should probably use the auth module in order to implement account handling.

### On-Chain Data Modeling
We need to find out how the Git data (as contained in `ReferenceUpdateRequest`s) can be stored
on the blockchain.

1. Push a reference to another local repo, analyzing go-git's behaviour in applying the
   `ReferenceUpdateRequest` and unpacking the packfile.
2. Having deduced go-git's application of `ReferenceUpdateRequest`s in the filesystem, determine
   how to achieve the same on the blockchain.
