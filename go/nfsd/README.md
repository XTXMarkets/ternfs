<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd

`nfsd` presents TernFS through NFSv4.0. It translates NFS filehandles and
operations to the smaller TernFS interface, stages new files locally while
they are writable, and implements the client and open state required by the
NFS protocol.

## State overview

The design is constrained by two TernFS properties:

- A file is immutable once it has been linked into a directory.
- Several nfsd processes may serve the same namespace, and any process may
  restart.

The first property means that nfsd stages a new file locally until its first
`CLOSE`. The file is fully editable while it is staged and read-only once it
has been published in TernFS. The second property means that client
registration and lease decisions must be visible to every nfsd.

## Namespace and filehandles

The [`TernVFS`](vfs.go) interface is the boundary between the NFS protocol
implementation and TernFS. The production implementation in
[`ternvfs.go`](ternvfs.go) maps namespace operations, reads and file
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

The implementation is in [`staging.go`](staging.go) and [`ops.go`](ops.go).

## Open state

Each nfsd process keeps its detailed NFS open state in memory. This includes
open-owner sequencing, replay information and the association between a
stateid, its client, its file and its access mode.

The persistent store holds a smaller record of active opens and leases. The
process-local state supplies operation ordering and file affinity. The shared
state allows a client reboot confirmed on one nfsd to invalidate old state on
the others.

Repeated read OPENs by one open-owner on the same file share one stateid.
Read-to-write upgrades cannot occur because write access to an existing
immutable file is rejected. `OPEN_DOWNGRADE` is unsupported.

The implementation is in [`open_state.go`](open_state.go).

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
            expired
            confirming.<nfsd ID>
            gc
```

The names have the following meanings. Not every file is present at the same
time.

`<client identity hash>/` is a directory. Its name is the hexadecimal SHA-256
hash of the opaque `SETCLIENTID` identity.

`i.<random>/` is an incarnation directory. Its TernFS inode ID is the NFS
clientid for that incarnation.

`confirmed` is a symlink to the current incarnation.

`pending` is a symlink to the incarnation awaiting `SETCLIENTID_CONFIRM`.

`client` is a JSON record containing the client verifier, the confirmation
verifier generated by nfsd, the normalized RPC principal and the callback
address.

`update` has the same contents as `client`. It holds an update which has not
yet been confirmed.

`reboot` is a symlink to the incarnation being replaced while nfsd removes
its leases and open markers.

`o.<state ID>` is an empty marker whose name identifies an open owned by the
incarnation. The rest of the stateid and open-owner state remains local to the
nfsd process.

`lease.<nfsd ID>` contains the lease expiry written by one nfsd instance.

`expired` is an empty marker recording that the fleet lease expired. It
prevents the confirmed clientid from acquiring a new lease after old process
slots have been collected.

`confirming.<nfsd ID>` contains an expiry for a temporary claim which prevents
collection while an nfsd is confirming the incarnation.

`gc` contains the earliest time at which an unreachable incarnation may be
collected.

The physical TernFS files are immutable. Updating a pointer or JSON record
creates a temporary symlink or file and renames it over the public name.
Readers therefore see a complete old or new record.

The implementation is in [`client_store.go`](client_store.go).

## Client registration

For a confirmed identity, `SETCLIENTID` with the same verifier updates the
existing incarnation. A different verifier represents a new incarnation and
produces a new clientid.

Confirmation uses a temporary claim to protect the pending incarnation from
collection. It also verifies that the incarnation is still pending before
making it current.

The `pending` pointer may continue to name the confirmed incarnation after
confirmation. Removing it with a lookup-then-remove sequence could delete a
new pending pointer installed concurrently by another nfsd. The next
registration replaces it atomically.

When confirmation replaces an existing incarnation, the new incarnation
records the old clientid in `reboot`. It becomes `confirmed` before the old
incarnation's leases and open markers are removed. Every nfsd therefore sees
the old clientid as stale before cleanup starts. The `reboot` pointer keeps
cleanup retryable if removing the old state fails.

The stored RPC principal is a collision guard, not an authentication or
authorization control. AUTH_SYS fields are supplied by the caller. The
comparison uses the credential flavor, uid, gid and auxiliary groups and
ignores the changing timestamp and caller-supplied machine name, matching
Linux nfsd.

`NFS4ERR_CLID_INUSE` requires both a different principal and live open state.
A client which only sends `RENEW` can therefore be replaced by another
principal. RFC 7530 section 16.33.5 conditions reuse on the principal alone;
this implementation deliberately requires open state as well so an idle
identity cannot remain pinned indefinitely by unauthenticated AUTH_SYS data.

## Leases

`OPEN` ensures that the local nfsd has a live lease and then creates an open
marker. Successful operations with a stateid also renew that lease when
needed, while `RENEW` updates it explicitly and `CLOSE` removes the marker.

Separate lease files are needed because several nfsd processes may handle
requests for the same client. A client's lease is live while any nfsd has a
live lease for it. Each nfsd caches the expiry of its own lease slot and
rewrites the immutable lease file at most once per half of the lease period
while `OPEN` or stateid operations are active. The cache is retained across
`CLOSE`, so a later `OPEN` can reuse the same lease.

Every stateid operation checks the confirmed pointer, which costs one lookup
but detects a replaced clientid immediately. It skips marker and lease scans
while the local lease cache is fresh. Accepting up to the 45-second renewal
interval before checking the pointer would save that lookup, but would allow
stale client state to remain usable during that interval.

READ and WRITE with a real stateid therefore cost one shard lookup while the
local lease cache is fresh. Lease renewal replaces the local nfsd's slot
through a temporary file and rename. The first OPEN handled by one nfsd also
has to resolve the durable client record; later OPENs reuse the cached identity
and confirmed-pointer location.

Client-store failures without a more specific NFS status return
`NFS4ERR_DELAY`, allowing the client to retry without discarding local open
state.

If no lease is live, its open markers no longer represent active state. If the
confirmed pointer names a different incarnation, the clientid is stale and its
stateids are expired.

Lease and confirmation slots use a random name per nfsd process. Scans remove
slots which have been expired for a full additional lease, allowing for clock
skew while preventing slots from accumulating across server restarts. Before
removing the last lease evidence, nfsd writes the `expired` marker. An expired
clientid therefore cannot renew or create new open state even after all of its
process slots have been removed.

OPEN markers are create-if-absent records. Reopening or replaying an OPEN which
already owns a marker reuses the existing TernFS inode.

Each nfsd checks its local clients once per lease period. A client with no
live fleet lease loses its process-local open state and any local staging
files. Its local open markers are removed at the same time. This bounds
abandoned staging and markers even when the client never contacts the server
again. The persistent client store is the authority for replaced clientids;
the process-local open store does not keep a separate revoked set.

An open-owner with no remaining open state is retained for replay for one
lease after its last operation. It is then removed. A later CLOSE retransmit
gets `NFS4ERR_BAD_STATEID`, which RFC 7530 section 16.18.5 permits after the
server's owner-retention period.

## Restart recovery

The persistent records do not reconstruct general process-local open state
after an nfsd restart. A client normally establishes new state.

Stateids are process-local. A client which sends one nfsd's stateid to a
different nfsd gets `NFS4ERR_BAD_STATEID`, not `NFS4ERR_STALE_STATEID`. The
Linux client recovers by opening the file again by name.

A write open is the exception because its local staging and sidecar files may
still hold unpublished data. On startup nfsd discovers these files. The
sidecar contains the state needed to complete the pending `CLOSE`.

After one lease period of startup grace, the periodic sweep checks all local
staging, including writes created since startup. Staging for an expired or
stale clientid is removed. A confirmed client with no lease slot is retained
because the first OPEN creates staging before it writes the slot. The startup
grace gives a client time to reclaim a recovered write after a server outage.

## Incarnation collection

Replacing a registration leaves an incarnation directory which may still be
used by a confirmation running on another nfsd. It cannot be removed inline
with `SETCLIENTID`.

Successful registration and confirmation schedule collection for that client
identity. Pending and in-progress reboot incarnations are retained. The
confirmed incarnation is retained until a later registration replaces it,
including after its lease expires. A live confirmation claim also protects an
incarnation.

An incarnation which is no longer reachable is first given a `gc` record. The
record ages the observation that the incarnation is unreachable for one lease
period. The next periodic lease sweep schedules another collection pass, which
checks the roots and confirmation claims again before removing it.

Collection is asynchronous and removes at most eight incarnations per pass.
A pass still scans the identity and its incarnation directories. Failure does
not affect the client operation. Failed and incomplete passes are also retried
by a later lease sweep.

Temporary `t.*` files left by a crash become eligible for removal after they
are one lease old. A later registration-triggered collection removes them
from identity and incarnation directories. Younger temporary files may belong
to an operation running on another nfsd and are not touched. Unconfirmed
`update` records do not expire; the next callback update replaces them.

The stable identity directory and its current confirmed incarnation are
retained until the same identity registers again. Distinct one-off client
identities therefore leave one identity directory and one current incarnation
in `/.nfs/clients`.

The client-store namespace is hidden from LOOKUP and READDIR. PUTFH rejects
internal directories and files whose parent is already known to this nfsd.
After a restart, a guessed record, lease or marker inode may be readable by
filehandle because TernFS cannot recover a file's parent from its inode ID.
Those records contain client-supplied identity metadata and expiry times, not
authentication secrets.

## Not implemented

Share reservations are not enforced. nfsd records the share access mode of an
open so it can reject a `WRITE` on a read open, but it ignores `share_deny`.
TernFS does not hold deny reservations, and process-local reservations would
not be visible across the nfsd fleet.

Byte-range locking, delegations, grace-period reclaim, named attributes, hard
links and special files are also not implemented. nfsd implements NFSv4.0
only.

## Testing

TernFS is not a general-purpose POSIX filesystem. Tests must distinguish
between:

- behavior which the server supports;
- operations which the server must reject with the correct NFS status; and
- NFS features which are outside the intended TernFS contract.

There are currently five test paths.

Test targets ending in `-cluster` build and start a temporary TernFS cluster.
Test targets without that suffix do not use a TernFS cluster. For example,
`test-libnfs` runs against the local filesystem backend. Pynfs only has a
cluster-backed mode, so its target is `test-pynfs-cluster`; there is no
`test-pynfs` target.

Cluster-backed targets use `ss` to check the fixed registry UDP port
`127.0.0.1:55556` before starting. They fail with an explicit error if another
test cluster is already using that port. This check does not require root.
Install `ss` (provided by the `iproute2` package on Debian and Ubuntu) before
running these targets.

### Protocol tests

Run the default Go tests from this directory:

```sh
make test
```

[`nfsd_test.go`](nfsd_test.go) starts the server on a local TCP socket with a
local filesystem backend. The tests construct NFS RPC messages directly.
[`client_store_test.go`](client_store_test.go) holds the focused durable client
store and collection tests. Together they cover RPC framing, compound
processing, filehandles, attributes, directory traversal, reads, staged
writes, client and open state, replay behavior, and expected errors for
unsupported operations.

**These tests run as part of the CI functional-test step through `go test ./...`.**

### TernFS-backed protocol tests

[`cluster_test.go`](cluster_test.go) contains tests behind the `ternnfs` build
tag. The intended command is:

```sh
make test-cluster
```

The test `TestMain` builds and starts a temporary TernFS cluster, including the
registry, block services, CDC and metadata shards. Each test starts an NFS
server backed by that cluster and sends NFS RPC messages directly.

This suite exercises the TernFS backend but still uses the test's own RPC
client.

**This test is currently not run in any CI workflow.**

### libnfs tests

[`libnfs_test.go`](libnfs_test.go) uses
[libnfs](https://github.com/sahlberg/libnfs) as an independent NFS client. The
tests are behind the `libnfs` build tag.

Fetch the pinned libnfs source:

```sh
make fetch-libnfs
```

The pinned release is libnfs 5.0.2.

Run the tests:

```sh
make test-libnfs
```

`test-libnfs` builds and installs libnfs under `.deps/libnfs-install` when
needed, then runs the 11 `TestLibnfs_*` cases verbosely against a server with
the local filesystem backend. It disables the Go test cache so that each
invocation exercises the client and server. The downloaded source and
installation are ignored by git. Use `make clean-libnfs` to remove them.
Fetching requires git; building requires CMake and a C compiler.

**This test is currently not run in any CI workflow.**

### pynfs protocol tests

[pynfs](https://github.com/linux-nfs/pynfs) is the Linux NFS project's
protocol test suite. The Makefile pins release `pynfs-0.5` and uses its
NFSv4.0 server tests.

Fetch and build pynfs:

```sh
make fetch-pynfs
make build-pynfs
```

The build requires Python 3, setuptools and PLY. On Debian and Ubuntu, install
the latter two with:

```sh
sudo apt-get install python3-setuptools python3-ply
```

Run the standard pynfs suite:

```sh
make test-pynfs-cluster
```

[`pynfs_test.go`](pynfs_test.go) reuses the `ternnfs` test harness. It starts a
temporary TernFS cluster and `nfsd`, runs pynfs with `--maketree --rundeps`,
and reads pynfs's JSON results so protocol failures fail the Go test. Pynfs
itself otherwise exits successfully when individual tests fail.

The Make target writes the complete console output to `pynfs.out` while also
displaying it. Set `PYNFS_OUTPUT` to use another path. The output file is not
ignored by git, so completed runs remain visible during review.

TernFS intentionally does not implement several POSIX and NFS features covered
by pynfs. The harness uses two mechanisms to exclude those tests:

* The default `PYNFS_TESTS` value uses pynfs flag selectors to exclude broad
  capability classes such as FIFO, socket, GSS and ACL tests.
* The [`pynfs_unsupported.txt`](pynfs_unsupported.txt) manifest lists
  individual locking and hard-link cases. Using the broad `nolock` and
  `nolink` selectors would also hide useful related tests.

Select flags or individual pynfs test codes with `PYNFS_TESTS`. The manifest
exclusions are appended after `PYNFS_TESTS`, so set `PYNFS_SKIP_FILE=` to
bypass this manifest.

Set additional runner options with `PYNFS_ARGS`. The default Go test timeout is
one hour because pynfs includes lease-expiry cases which deliberately sleep
for several minutes. Override it with `PYNFS_TIMEOUT`:

```sh
make test-pynfs-cluster PYNFS_TESTS='putrootfh getattr'
make test-pynfs-cluster PYNFS_TESTS='GETATTR1 GETATTR2' PYNFS_ARGS='--showtraffic'
make test-pynfs-cluster PYNFS_TESTS='all notimed noblock nochar nofifo nosocket nogss noacl nomode000'
make test-pynfs-cluster PYNFS_TESTS='all notimed' PYNFS_SKIP_FILE=
make test-pynfs-cluster PYNFS_TIMEOUT=2h
```

Pynfs tags eight standard cases as `timed`; these exercise lease expiry and
can each wait for 135 or 180 seconds with nfsd's 90-second lease. The harness
runs Python unbuffered and reports tests taking at least one second, sorted by
duration, after pynfs writes its results.

Use `make clean-pynfs` to remove the source and generated files. This suite is
not currently run in CI.

All Go test targets accept additional flags through `GO_TEST_FLAGS`. For
example:

```sh
make test-cluster GO_TEST_FLAGS='-v -run TestTernWriteAndRead'
```

### Linux kernel client integration test

CI runs a separate NFS test in a QEMU Ubuntu VM:

```sh
./ci.py --short --prepare-image <noble-cloud-image> --nfs --leader-only
```

[`ci_nfs.sh`](../../ci_nfs.sh) deploys a built TernFS cluster and `nfsd` into
the VM. [`terntests`](../terntests) then mounts the server using the Linux
NFSv4.0 client and runs `terntests -nfs -filter nfs`.

That filter selects exactly two tests:

- [`nfs mounted fs`](../terntests/terntests.go) runs the existing `fsTest`
  workload through its `posixHarness`, with the NFS mount as its root. In short
  mode this uses 10 directories and 500 files.
- [`nfs mutations`](../terntests/nfsmutate.go) is the NFS-specific mutation
  suite. It checks create/write/readback, out-of-order writes, rename, delete,
  timestamp updates, and rejection of in-place modification.

This is the only current test path using a kernel NFS client.
