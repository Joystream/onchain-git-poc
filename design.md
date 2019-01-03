# Design Document
This document describes the technical design of the Joystream Git to blockchain bridge.

The Joystream Git to blockchain bridge is a [Cosmos](https://github.com/cosmos/cosmos-sdk) app,
consisting of a server and a client, both written in the Go language. The client is
responsible for funneling _query_ and _transaction_ type requests to the server. The former type
of request is for asking the server for information, i.e. getting state and not changing it,
whereas the latter type is in order to change blockchain state.

A good example of a transaction type request is pushing a local Git branch; the client
has to compute the set of changes needed to be done on the server side and encode them
(as a set of add/update/delete commands and a
[packfile](https://git-scm.com/book/en/v2/Git-Internals-Packfiles) of Git objects) in
a `MsgUpdateReferences` message to broadcast to nodes. In response to such a message,
nodes (i.e. servers) should update the corresponding repository, in app state, with the
incoming packfile and also according to the reference update commands (i.e. add/update/delete).

Our Cosmos application is named _GitService_. In order to integrate with the `git` command line
client, we have a third component, which is the
[Git remote helper](https://git-scm.com/docs/git-remote-helpers) `git-remote-joystream`. Simply
told, this is a command line tool that Git will invoke when interacting with remotes using
the `joystream` protocol. When invoked, it will in turn invoke the GitService client
according to the arguments it has been given by git.

The GitService client and server use the [go-git](https://github.com/src-d/go-git/) library
for implementing Git functionality.

## GitService Client
The GitService client, `gitservicecli`, is a command line tool offering the following sub-commands:

### list
The `list` sub-command asks the server to list references within a certain repository. This
is used by the Git remote helper when it receives a `list` command from Git.

### push-refs
The `push-refs` sub-command computes a set of commands to add, update or delete references as well
as a [packfile](https://git-scm.com/book/en/v2/Git-Internals-Packfiles) containing Git objects
(e.g. commits, trees...) to be shared with the remote. This data gets encoded in a
`MsgUpdateReferences` message that in turn gets broadcasted to nodes (i.e. servers) via
blockchain transaction.

#### The MsgUpdateReferences Format
The MsgUpdateReferences message type contains the following fields:

* URI - the unique identifier of the repository.
* Author - the account address of the person pushing the changes.
* Commands - a set of UpdateReferenceCommands, which each instruct how to add, update or delete
  a reference.
* Shallow - a set of shallow references (not sure yet what this entails)
* Packfile - The [packfile](https://git-scm.com/book/en/v2/Git-Internals-Packfiles) containing the
  Git objects to update the remote with.

#### Computing of Changes
The `push-refs` sub-command computes the updates to send to the server (as encoded in the
`MsgUpdateReferences` message) according to a certain algorithm:

1. Get advertised references from server.
2. Determine references existing in local repository.
3. Given provided "refspecs" (specifications from Git on references to add/update/delete),
   produce commands for adding, updating and deleting references on the server.
4. Determine hashes corresponding to references that need to be pushed to the server, as they
   don't already exist in the remote repository.
5. Encode packfile corresponding to hashes to be pushed, in background.
6. Make MsgUpdateReferences mesage containing repository URI, commands for adding/updating/deleting
   references, a shallow reference (TODO: find out purpose), author and packfile.
7. Broadcast MsgUpdateReferences message for server nodes to process.

## GitService Server
The GitService server, `gitserviced`, is a Cosmos/Tendermint node that offers a set of query routes
and handles a set of messages.

The server has two query routes, `listRefs` and `advertisedReferences`. The former lists
the names of all Git references stored for a repository, whereas the latter queries so-called
advertised references from a Git repository. Both uses Git repository data stored in the
Cosmos MultiStore. An `advertisedReferences` response will mainly provide the references
contained in the repository, along with corresponding hashes. This route will be used
by the client for example to find out what data it needs to push to the server.

The server currently handles one message type, `MsgUpdateReferences`, which the client sends
in order to push a set of references from a local Git repository to a repository on the blockchain.
As described before, the message will contain a set of commands to add, update or delete
references in the repository as well as a packfile containing Git objects.

In response to a `MsgUpdateReferences` message, the server will write the packfile along with
a generated index of it to the repository in the KVStore. It will also write/delete references
accordingly.

### Handling of MsgUpdateReferences Messages
When receiving a MsgUpdateReferences message, the server will

## Git Remote Helper
The Git remote helper, `git-remote-joystream`, implements the
[Git remote helper](https://git-scm.com/docs/git-remote-helpers) protocol, i.e. it accepts
a URL command line argument and optionally a repository argument, and receives commands on
[standard input](https://en.wikipedia.org/wiki/Standard_streams#Standard_input_(stdin)).
Only the URL argument is used, to determine the repository on the blockchain.

The helper will read lines of command input from standard input, as provided by `git`.
The supported commands are:

* capabilities
* list
* push

In response to the push command, which refers to a set of references, it will invoke the
GitService client with the `tx gitService push-refs` sub-command along with the repository
URL and references as arguments.
