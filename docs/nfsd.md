<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# NFS Server

`nfsd` presents TernFS through NFSv4.0. It translates NFS filehandles and
operations to the smaller TernFS interface, stages new files locally while
they are writable, and implements the client and open state required by the
NFS protocol.

## State overview

The design is constrained by two TernFS properties:

* A file is immutable once it has been linked into a directory.
* Several nfsd processes may serve the same namespace, and any process may
  restart.

The first property means that nfsd stages a new file locally until its first
`CLOSE`. The file is fully editable while it is staged and read-only once it
has been published in TernFS. The second property means that client
registration and lease decisions must be visible to every nfsd.

## Namespace and filehandles

The [`TernVFS`](../go/nfsd/vfs.go) interface is the boundary between the NFS
protocol implementation and TernFS. The production implementation in
[`ternvfs.go`](../go/nfsd/ternvfs.go) maps namespace operations, reads and file
construction to TernFS requests. Tests can use a local implementation of the
same interface.

TernFS assigns every file and directory an inode ID. nfsd uses the eight-byte
encoding of that ID as the NFS filehandle. The inode type is part of the ID, so
many operation and attribute checks do not require another namespace lookup.

Ownership and mode attributes are synthetic. File size and timestamps come
from TernFS for linked files and from the local staging file while a new file
is still open for writing.

The `/.nfs` directory is reserved for nfsd state and hidden from NFS clients.

## Writes

TernFS cannot modify the contents of a linked file. nfsd therefore only allows
write access when it creates a new file. Replacing a file is a
remove-and-create operation.

Creating a writable file allocates a transient TernFS inode which is not yet
visible in the target directory. nfsd also creates these files on its local
host:

```
<staging directory>/
    <inode id>.staging
    <inode id>.meta
```

The NFS filehandle names the transient TernFS inode. The `.staging` file holds
the mutable contents. The `.meta` sidecar identifies the target name and
directory, the TernFS construction cookie and the NFS open. It contains
enough information to finish `CLOSE` after an nfsd restart.

On `CLOSE`, nfsd streams the staging file into the transient TernFS inode and
links that inode into the target directory. It then removes the local staging
file and sidecar. Once the link succeeds the file is immutable and can only be
opened for reading.

A staging file belongs to one nfsd host. Persistent client and lease state can
invalidate an open across the fleet, but it does not make the staged data or
the process-local open state movable to another nfsd.

The implementation is in
[`staging.go`](../go/nfsd/staging.go) and
[`ops.go`](../go/nfsd/ops.go).

## Open state

Each nfsd process keeps its detailed NFS open state in memory. This includes
open-owner sequencing, replay information and the association between a
stateid, its client, its file and its access mode.

The persistent store holds a smaller record of active opens and leases. The
process-local state supplies operation ordering and file affinity. The shared
state allows a client reboot confirmed on one nfsd to invalidate old state on
the others.

The implementation is in
[`open_state.go`](../go/nfsd/open_state.go).

## Client identities and incarnations

An NFS client supplies an opaque identity and an eight-byte verifier in
`SETCLIENTID`. The identity says which client this is. A changed verifier
means that the client has rebooted and is starting a new lifetime.

The persistent store calls one such lifetime an incarnation. Each incarnation
has a directory in TernFS. The inode ID of that directory is the NFS clientid,
and the records for that client lifetime are stored below it.

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

`confirmed` is a pointer to the current incarnation.

`pending` points to the incarnation awaiting `SETCLIENTID_CONFIRM`.

`client` is a JSON record containing the client verifier, the confirmation
verifier generated by nfsd, the normalized RPC principal and the callback
address.

`update` has the same contents as `client`. It holds an update which has not
yet been confirmed.

`reboot` points to the incarnation being replaced while nfsd removes its
leases and open markers.

`o.<state ID>` is an empty marker whose name identifies an open owned by the
incarnation. The rest of the stateid and open-owner state remains local to the
nfsd process.

`lease.<nfsd ID>` contains the lease expiry written by one nfsd instance.

`confirming.<nfsd ID>` contains an expiry for a temporary claim which prevents
collection while an nfsd is confirming the incarnation.

`gc` contains the earliest time at which an unreachable incarnation may be
collected.

The physical TernFS files are immutable. Updating a pointer or JSON record
creates a temporary file and renames it over the public name. Readers
therefore see a complete old or new record.

The implementation is in
[`clients.go`](../go/nfsd/clients.go).

## Client registration

For a confirmed identity, `SETCLIENTID` with the same verifier updates the
existing incarnation. A different verifier represents a new incarnation and
produces a new clientid.

Confirmation uses a temporary claim to protect the pending incarnation from
collection. It also verifies that the incarnation is still pending before
making it current.

When confirmation replaces an existing incarnation, the new incarnation
records the old clientid in `reboot`. It becomes `confirmed` before the old
incarnation's leases and open markers are removed. Every nfsd therefore sees
the old clientid as stale before cleanup starts. The `reboot` pointer keeps
cleanup retryable if removing the old state fails.

The stored RPC principal prevents a different client from claiming an identity
which still has live open state.

## Leases

`OPEN` creates an open marker and renews the lease belonging to the local
nfsd. `RENEW` updates that lease and `CLOSE` removes the marker.

Separate lease files are needed because several nfsd processes may handle
requests for the same client. A client's lease is live while any nfsd has a
live lease for it.

If no lease is live, its open markers no longer represent active state. If the
confirmed pointer names a different incarnation, the clientid is stale and its
stateids are expired.

## Restart recovery

The persistent records do not reconstruct general process-local open state
after an nfsd restart. A client normally establishes new state.

A write open is the exception because its local staging and sidecar files may
still hold unpublished data. On startup nfsd discovers these files. The
sidecar contains the state needed to complete the pending `CLOSE`.

## Incarnation collection

Replacing a registration leaves an incarnation directory which may still be
used by a confirmation running on another nfsd. It cannot be removed inline
with `SETCLIENTID`.

Successful registration and confirmation schedule collection for that client
identity. Confirmed, pending and in-progress reboot incarnations are retained.
A live confirmation claim also protects an incarnation.

An incarnation which is no longer reachable is first given a `gc` record. The
record ages the observation that the incarnation is unreachable for one lease
period. A later collection pass checks the roots and confirmation claims again
before removing it.

Collection is bounded and asynchronous. Failure does not affect the client
operation, and a later registration schedules another attempt.

## Not implemented

Share reservations are not enforced. nfsd records the share access mode of an
open so it can reject a `WRITE` on a read open, but it ignores `share_deny`.
TernFS does not hold deny reservations, and process-local reservations would
not be visible across the nfsd fleet.

Byte-range locking, delegations, grace-period reclaim, named attributes, hard
links and special files are also not implemented. nfsd implements NFSv4.0
only.
