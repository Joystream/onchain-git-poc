# TODOs

## Push Reference to Uninitialized Repo on Blockchain
1. User: Invoke client's push-refs command.
2. Client: Query server for advertised references in blockchain repo.
3. Server: Return empty list of advertised references since repo doesn't exist.
4. Client: Compute reference updates that should take place, taking into account advertised
   references obtained from server, and packfile containing commit history.
5. Client: Send `MsgUpdateReferences`, computed during previous step, to server including
   command to add reference, and also packfile containing commit history.
6. Server: Decode packfile and store on blockchain.
7. Server: Add reference on blockchain, pointing to correct commit.

### Authentication
We should probably use the auth module in order to implement account handling.

### On-Chain Data Modeling
We need to find out how the Git data (as contained in `ReferenceUpdateRequest`s) can be stored
on the blockchain.

1. Push a reference to another local repo, analyzing go-git's behaviour in applying the
   `ReferenceUpdateRequest` and unpacking the packfile.
2. Having deduced go-git's application of `ReferenceUpdateRequest`s in the filesystem, determine
   how to achieve the same on the blockchain.
