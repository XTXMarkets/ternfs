<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# NFS Server

`nfsd` presents TernFS through NFSv4.0. It translates NFS filehandles and
operations to the smaller TernFS interface, stages new files locally while
they are writable, and implements the client and open state required by the
NFS protocol.

The design is constrained by two TernFS properties:

* A file is immutable once it has been linked into a directory.
* An nfsd process may disappear without taking the shared TernFS namespace
  with it.

The first property makes a writable open local to one nfsd until its first
`CLOSE`. The second means that client registration and lease decisions must
be visible to every nfsd.

## Namespace and filehandles

The [`TernVFS`](../go/nfsd/vfs.go) interface is the boundary between the NFS
protocol implementation and TernFS. The production implementation in
[`ternvfs.go`](../go/nfsd/ternvfs.go) maps lookups, directory changes, reads
and file construction to TernFS requests. Tests can use an in-memory or local
filesystem implementation of the same interface.

TernFS assigns every file and directory an inode ID. nfsd uses the eight-byte
encoding of that ID as the NFS filehandle. The inode type is part of the ID, so
many operation and attribute checks do not require another namespace lookup.

Ownership and mode attributes are synthetic. File size and timestamps come
from TernFS for linked files and from the local staging file while a new file
is still open for writing.

The `/.nfs` directory is reserved for nfsd state. LOOKUP and READDIR hide it
from NFS clients.

## Writes

TernFS cannot modify the contents of a linked file. nfsd therefore only
allows write access when `OPEN(CREATE)` constructs a new file. A write open on
an existing file fails. Replacing a file is a remove-and-create operation.
Without a staging directory, nfsd rejects namespace mutations and write
opens.

Creating a writable file has two parts. TernFS first allocates a transient
inode and construction cookie, but does not link the inode into the target
directory. nfsd then creates these files on its local host:

```
<staging directory>/
    <inode id>.staging
    <inode id>.meta
```

The NFS filehandle names the transient TernFS inode. The `.staging` file holds
the current contents. `WRITE` can replace any range, `SETATTR(SIZE)` can grow
or truncate it, `READ` reads it, and `GETATTR` reports its current size. The
file remains fully editable until `CLOSE` starts publication. `COMMIT` flushes
the staging file to local storage but does not link it into TernFS.

The `.meta` sidecar is a binary record containing the target directory inode
ID, target name, TernFS construction cookie, twelve-byte NFS state ID and
owning client ID. It gives nfsd enough information to finish `CLOSE` after a
process restart.

On `CLOSE`, nfsd streams the staging file into the transient TernFS inode and
links that inode into the target directory. It then removes the local staging
file and sidecar. If the link succeeds, the file is immutable from then on and
can only be opened for reading. A replayed successful `CLOSE` checks the
linked inode rather than writing it again.

A staging file belongs to one nfsd host. Persistent client and lease state can
invalidate an open across the fleet, but it does not make the staged data or
the process-local open state movable to another nfsd.

The implementation is in
[`staging.go`](../go/nfsd/staging.go) and
[`ops.go`](../go/nfsd/ops.go).

## Open state

Each nfsd process keeps its NFS open state in memory. An open owner is the
pair of a client ID and the opaque owner value supplied in `OPEN`. The server
stores the next owner sequence ID and enough information to replay the most
recent `OPEN`.

An NFSv4 stateid has a four-byte generation followed by a twelve-byte opaque
state ID. nfsd allocates the state ID when it opens a file. The local record
associates it with the client, open owner, file inode ID and read or write
access. It also records whether the open is confirmed or closed and the
sequence IDs needed to replay `OPEN_CONFIRM` and `CLOSE`.

The generation changes when the open is confirmed or closed. A request using
an earlier generation receives `NFS4ERR_OLD_STATEID`. Unknown, future or
mismatched stateids receive the corresponding bad or stale stateid error.
The all-zero and all-one stateids retain their protocol-defined anonymous
meaning and do not identify a local open.

`READ`, `WRITE`, `SETATTR(SIZE)`, `OPEN_CONFIRM` and `CLOSE` first validate the
local record. They then check the shared open marker and lease described
below. The local record supplies operation ordering and file affinity; the
shared records allow a client reboot confirmed on one nfsd to invalidate old
state on another.

The implementation is in
[`open_state.go`](../go/nfsd/open_state.go).

## Client identities and incarnations

An NFS client supplies an opaque identity and an eight-byte verifier in
`SETCLIENTID`. The identity says which client this is. A changed verifier
means that the client has rebooted and is starting a new lifetime.

The persistent store calls one such lifetime an incarnation. An incarnation
is represented by an `i.<random>` directory in TernFS. The directory itself
has a TernFS inode ID, and that numeric inode ID is the NFS clientid returned
to the client. A new incarnation directory therefore produces a new clientid.
The inode is a directory because the records and open markers belonging to
that client lifetime are stored below it.

The opaque client identity is not stored directly. nfsd hashes it with
SHA-256 and uses the hexadecimal hash as the stable parent directory for all
incarnations of that identity.

## Persistent client state

Client state has the following shape:

```
/.nfs/clients/
    <client identity hash>/
        confirmed
        pending
        i.<random>/
            client
            update
            reboot
            o.<state ID>
            lease.<nfsd ID>
            confirming.<nfsd ID>
            gc
```

The names have the following meanings. Not every file is present at the same
time.

`<client identity hash>/` is a directory. Its name is the hexadecimal SHA-256
hash of the opaque `SETCLIENTID` identity.

`i.<random>/` is an incarnation directory. Its TernFS inode ID is the NFS
clientid for that incarnation.

`confirmed` is a JSON pointer file in the identity directory. It contains
`{"client_id": N}`, where `N` is the inode ID of the confirmed incarnation
directory.

`pending` has the same format. It points to the incarnation which the most
recent successful `SETCLIENTID` expects `SETCLIENTID_CONFIRM` to confirm.

`client` is a JSON record in an incarnation directory. Its fields are
`verifier`, `confirm`, `principal_flavor`, `principal_body`, `netid` and
`addr`. They hold the eight-byte client verifier supplied to `SETCLIENTID`,
the eight-byte confirmation verifier generated by nfsd, the RPC credential
flavor and normalized credential body, and the callback network ID and
address. JSON encodes the byte fields as base64 strings.

`update` has the same format as `client`. It holds a callback and principal
update which has been returned by `SETCLIENTID` but not yet confirmed.

`reboot` is another `{"client_id": N}` pointer. It is stored in a newly
confirmed incarnation while nfsd removes the leases and open markers
belonging to the incarnation identified by `N`.

`o.<state ID>` is an empty file. Its name contains the hexadecimal encoding of
the twelve-byte opaque state ID allocated by `OPEN`. It records that this
incarnation still owns that open. The stateid generation, open owner and
read/write mode remain in the process-local open-state record.

`lease.<nfsd ID>` is a JSON record containing
`{"expires_unix_nano": N}`. Each nfsd instance has a random ID and writes its
own lease file. The client lease is live while any of these expiry times is
in the future.

`confirming.<nfsd ID>` has the same expiry-record format. It is a temporary
claim that prevents collection of an incarnation while that nfsd is
confirming it.

`gc` is a JSON record containing `{"collect_after_unix_nano": N}`. It records
the earliest time at which an unreachable incarnation may be collected.

TernFS files are immutable. Updating a pointer or JSON record creates a
temporary `t.<random>` file and renames it over the public name. Readers see a
complete old or new record rather than a partly written update.

The implementation is in
[`clients.go`](../go/nfsd/clients.go).

## Client registration

`SETCLIENTID` with the same identity and verifier is a callback update. It
keeps the confirmed incarnation, writes an `update` record and points
`pending` at the confirmed incarnation. Confirmation replaces the `client`
record without changing the clientid.

A different verifier represents a new incarnation. Replacing an unconfirmed
registration also creates a new incarnation. In both cases `SETCLIENTID`
creates an incarnation directory and points `pending` at it.

`SETCLIENTID_CONFIRM` writes a `confirming.<nfsd ID>` claim before changing
shared state, then checks that `pending` still points to the incarnation. The
claim prevents another nfsd from collecting the directory if a newer
`SETCLIENTID` replaces `pending` during confirmation.

When confirmation replaces an existing incarnation, the new incarnation
records the old clientid in `reboot`. It becomes `confirmed` before the old
incarnation's leases and open markers are removed. Every nfsd therefore sees
the old clientid as stale before cleanup starts. The `reboot` pointer keeps
cleanup retryable if removing the old state fails.

The stored RPC principal includes the credential flavor and credential body.
For `AUTH_SYS`, the changing timestamp at the start of the credential is not
part of the principal. A different principal cannot claim an identity with
live open state; `SETCLIENTID` returns `NFS4ERR_CLID_INUSE` and the callback
address of the current owner.

## Leases

`OPEN` creates the empty open marker and renews the lease file belonging to
the local nfsd. `RENEW` updates that lease after checking that the clientid is
still confirmed. `CLOSE` removes the open marker.

Separate lease files are needed because several nfsd processes may handle
requests for the same client. One process cannot overwrite a later expiry
written by another process. Readers inspect all lease files and use the
latest live expiry.

If no lease is live, open markers no longer represent active state and may be
removed. If the confirmed pointer names a different incarnation, the clientid
and all its stateids are stale regardless of their lease expiry.

## Restart recovery

The persistent records do not reconstruct general process-local open state
after an nfsd restart. A client normally establishes new state.

A write open is the exception because its local `.staging` and `.meta` files
may still contain data which has not been linked into TernFS. On startup nfsd
discovers those files. A matching `CLOSE` can use the sidecar's state ID,
client ID, target name and construction cookie to link the data and remove the
persistent open marker.

Sidecars written before the client ID field was added remain readable. Their
client ID is zero, so there is no persistent open marker to remove.

## Incarnation collection

Replacing a registration leaves an incarnation directory which may still be
used by a confirmation running on another nfsd. It cannot be removed inline
with `SETCLIENTID`.

Successful registration and confirmation schedule collection for that client
identity. Collection treats the confirmed, pending and in-progress reboot
incarnations as roots. A live `confirming.<nfsd ID>` record also protects an
incarnation.

An incarnation which is no longer reachable is first given a `gc` record. The
record ages the observation that the incarnation is unreachable for one lease
period. A later collection pass checks the roots and confirmation claims again
before removing it.

Collection is bounded and asynchronous. Failure is logged rather than
returned to the client operation. A later registration schedules another
attempt. Reclamation is therefore not required for the correctness of client
registration or reboot handling.

## Unsupported state

TernFS does not retain or enforce NFS share-deny reservations. Its write path
uses last-closer-wins semantics, and process-local reservation state would not
be consistent across the nfsd fleet.

Locking, delegations and grace-period state are also not implemented.
`CLAIM_PREVIOUS` returns `NFS4ERR_NO_GRACE`.
