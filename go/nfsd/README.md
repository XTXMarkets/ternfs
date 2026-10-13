<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd

`nfsd` presents TernFS through NFSv4.1 (RFC 8881). It translates NFS
filehandles and operations to the smaller TernFS interface, stages writable
files locally, and implements the session, client and open state required by
the NFS protocol.

nfsd implements minor version 1 only. A COMPOUND with minor version 0 gets
`NFS4ERR_MINOR_VERS_MISMATCH`. `-allow-minor-version-2` also accepts minor
version 2, served as 4.1 with every 4.2-only operation returning
`NFS4ERR_NOTSUPP`. It exists for libnfs 8, which offers sessions only as 4.2.
Linux clients must mount with `vers=4.1`; with the flag set, `vers=4` or
`vers=4.2` would negotiate 4.2.

## Request handling

Each TCP connection has one reader and up to 64 admitted requests.
`-max-in-flight-per-connection` sets that limit; it must be positive.
The reader reserves capacity before reading a complete RPC frame.
COMPOUNDs run concurrently, with operations inside each compound executed
in order. Replies share one writer lock and may arrive out of request order;
the RPC XID identifies each reply. Excess requests wait in socket buffers.

Session slots order and deduplicate requests; see [Sessions](#sessions).
Pathname locks serialize namespace changes. A per-file open lock, taken
after the pathname lock, serializes OPEN, CLOSE and OPEN_DOWNGRADE of one
file, and per-file locks protect staging data.

The read timeout is five minutes from the start of each admitted frame
read. The separate write timeout is five minutes for each reply once it
acquires the writer lock. Read EOF or timeout stops admission and lets
accepted requests finish, including after a TCP half-close. A write failure
closes the socket and wakes the reader. Backend operations already in
flight continue; a stuck backend can therefore retain a handler after its
client disconnects.

Identical requests reusing an in-flight XID on the same connection are
dropped; the original request sends one reply. Reusing that XID with
different request bytes closes the connection. This guard lasts through
the reply write. It does not cover reconnects; completed replies are cached
by session slot instead.

The admission limit bounds request count, not bytes or connections. One
request is limited to 2 MiB, but a compound can contain up to 128 one-MiB
READs. At the default concurrency this can retain roughly 8 GiB of reply
payload on one connection, plus allocation overhead. Ordinary clients
send much smaller compounds. Monitor RSS and tune the request limit for
the workload; there is no aggregate byte budget.

## Sessions

Every COMPOUND other than a lone EXCHANGE_ID, CREATE_SESSION,
DESTROY_SESSION, DESTROY_CLIENTID or BIND_CONN_TO_SESSION starts with
SEQUENCE.

CREATE_SESSION negotiates a slot table of up to 64 slots. Requests on
different slots run concurrently. Each slot records the sequence ID of its
last request and, when the client sets `sa_cachethis`, that request's reply.
A cached reply is at most 64 KiB. A retransmission on the same slot and
sequence ID returns the cached reply. While the original is still running it
gets `NFS4ERR_DELAY`; if its reply was not cached it gets
`NFS4ERR_RETRY_UNCACHED_REP`. The cache is process-local and is lost on
restart.

The negotiated limits are enforced. CREATE_SESSION returns `NFS4ERR_TOOSMALL`
for channel attributes too small to carry a SEQUENCE. SEQUENCE returns
`NFS4ERR_TOO_MANY_OPS` or `NFS4ERR_REQ_TOO_BIG` for a compound beyond the
session's limits. With `sa_cachethis` set, a session whose negotiated cache
size cannot hold even a minimal reply gets `NFS4ERR_REP_TOO_BIG_TO_CACHE`; a
reply larger than the cache size is simply not cached. A reply larger than
the session's maximum is replaced by `NFS4ERR_REP_TOO_BIG`; the operations
may already have taken effect.

Any connection may carry requests for any session; a connection is
associated with a session by CREATE_SESSION, BIND_CONN_TO_SESSION or its
first SEQUENCE for it, up to 64 sessions per connection.

SEQUENCE does no backend I/O. It takes a hold on the client's logical lease,
a use of the session, the connection's association and a reply reservation,
then checks the client's durable lease from memory. Until the request
commits nothing changes, and a failed admission releases what it took. If
this process has not yet published or verified the client's durable lease,
SEQUENCE returns `NFS4ERR_DELAY` and asks the lease refresher to read it.
CREATE_SESSION replies only once the client's first lease is published, or
after at most two seconds, so a mount pays that publication's latency there
instead; libnfs treats a DELAY at mount as a failure. The first SEQUENCE
after an idle period can still get DELAY while the lease is read again. See
[Leases](#leases).

Sessions are held in memory and end with the nfsd process; `PERSIST` is
never granted. A session ID contains the issuing process's epoch and the
client incarnation. After an nfsd restart, SEQUENCE and BIND_CONN_TO_SESSION for a session from an
earlier process return `NFS4ERR_BADSESSION`, as for any session this process
destroyed or never issued. The client then registers again: CREATE_SESSION
with its old clientid gets `NFS4ERR_STALE_CLIENTID`, and EXCHANGE_ID gives a
new clientid for the same durable client while its lease is live. A writer
open is recovered either with its old stateid or, if the client reopens it,
by a writable CLAIM_FH OPEN of the handle it holds; `CLAIM_PREVIOUS` gets
`NFS4ERR_NO_GRACE`.

A request admitted by SEQUENCE renews the client's logical lease when its
reply is written. If this process has decided the client expired, or the
clientid has been replaced, SEQUENCE returns `NFS4ERR_BADSESSION`, and while
expiry is being decided it returns `NFS4ERR_DELAY`. The client then has to
establish a new clientid with EXCHANGE_ID.

### Backchannel

A session that sets `CREATE_SESSION4_FLAG_CONN_BACK_CHAN` on a connection
that can carry callbacks is granted a backchannel there. The connection's
back association is reserved with the session, before the creation slot,
and also on the connection that replays a cached CREATE_SESSION or retries
one held for confirmation; a connection full of back associations gets
`NFS4ERR_RESOURCE` before anything is created. A connection carries the
callbacks of at most 64 sessions, and retired sessions' associations are
pruned.

BIND_CONN_TO_SESSION returns the direction RFC 8881 section 18.34.3
requires. An existing association that satisfies the request is returned
unchanged; a request that would change it gets `NFS4ERR_INVAL`, as does a
direction other than the four defined ones. The back direction needs a
session that asked for a backchannel. The connection's existing
association, the decision, both quotas and the new association are taken in
one step, so concurrent BINDs are decided one after the other and
`NFS4ERR_RESOURCE` changes nothing. BACKCHANNEL_CTL
replaces the callback program and credential. The callback credential is
the first `AUTH_NONE` or `AUTH_SYS` entry the client offers, sent verbatim;
a list offering only `RPCSEC_GSS` gets `NFS4ERR_NOENT` and an empty one
`NFS4ERR_INVAL`, both before the creation slot, and only when a backchannel
is requested. `SET_SSV` returns `NFS4ERR_INVAL`, since only `SP4_NONE` is
offered.

Callbacks use ONC RPC version 2, the session's callback program, and
program version 1, the `NFS_CB` version in the NFSv4.1 XDR, which Linux
sends and accepts (RFC 8881 section 18.36.3's prose says 4). nfsd sends two.
CB_NULL probes every new back connection, after the reply that bound it. A
validated reply marks the path up. Until then, for at most the 10-second
probe timeout, the path counts as up for the status flags only; once the
timeout passes, `SEQ4_STATUS_CB_PATH_DOWN_SESSION` and
`SEQ4_STATUS_CB_PATH_DOWN` are raised. The timed-out CB_NULL stays
outstanding until a reply or a disconnect, or until the connection's last
committed back association goes; a reservation still pending does not keep
it, and the next backchannel committed on the connection is probed afresh.

CB_COMPOUND, minor version 1, with CB_SEQUENCE on slot 0 and CB_RECALL_SLOT,
is sent once per episode when the reply cache is three quarters full and a
session is charged more than its share. Such a session is given half its
slots as SEQUENCE's target highest slot until the cache is below half full;
sessions without a usable backchannel (none requested, no back slot, or
limits too small for the call) see only the target. A callback goes only to
a connection whose path is up, one at a time per session, with a 10-second
timeout and server-wide XIDs. An unanswered callback is never resent while
its connection is up: after a disconnect it goes out with the same slot and
sequence on another back connection that is up, or waits for a BIND. A
late, validated reply marks a timed-out path up again. The frame actually
sent is checked against the negotiated back limits each time; if the
credential has since grown past them, the recall is dropped unsent.

A reply is validated whole before anything in it is used. A valid
CB_SEQUENCE OK consumes the sequence and renews the client's logical lease;
a CB_RECALL_SLOT error after it is the client declining the recall, recorded
and not retried. A `NFS4ERR_DELAY` from CB_SEQUENCE is retried with the same
sequence up to three times. Any other error, or a malformed, truncated or
inconsistent reply, sets `SEQ4_STATUS_BACKCHANNEL_FAULT` and stops callbacks
for that session, without renewing. Replies with unknown XIDs are dropped
and counted. Clients that never ask for a backchannel see no backchannel
status flags.

The implementation is in [`session.go`](session.go),
[`backchannel.go`](backchannel.go) and
[`backchannel_sender.go`](backchannel_sender.go).

## State overview

The design is constrained by two TernFS properties:

- A file is immutable once it has been linked into a directory.
- Several nfsd processes may serve the same namespace, and any process may
  restart.

The first property means that nfsd implements mutable NFS files with
copy-on-write replacement. New and replacement files are staged locally while
open for writing, then published as immutable TernFS inodes on `CLOSE`. The
second property means that client registration and lease decisions must be
visible to every nfsd.

## Namespace and filehandles

The [`TernVFS`](internal/vfs/vfs.go) interface is the boundary between the
NFS protocol implementation and TernFS. The production implementation in
[`internal/vfs/ternvfs.go`](internal/vfs/ternvfs.go) maps namespace
operations, reads and file construction to TernFS requests. Tests can use a local implementation of the
same interface.

TernFS assigns every file and directory an inode ID. nfsd uses the eight-byte
encoding of that ID as the NFS filehandle and reports it as `FH4_PERSISTENT`.
The inode type is part of the ID, so many operation and attribute checks do
not require another namespace lookup.

Ownership and mode attributes are synthetic. A filehandle names the published
inode the client looked up. A write OPEN returns the inode published at the
name as its filehandle; for a new name this is the empty placeholder. The
writer's staging is found by the stateid, not by the handle. FILEID and
FILEHANDLE always describe the handle itself.

Operations which carry an open stateid use that open's version: the writer's
staging, or the version a read open saw at OPEN. Operations without a
stateid (GETATTR, VERIFY, COMMIT and READ with a special stateid) use the
client's view of the handle: its writer if it has one, else for one lease
the version its own CLOSE published, else the version its most recent read
open saw, else the inode itself. That alias is
process-local and is kept while the client still has the handle open. After
it lapses, or after an nfsd restart, the handle names its original immutable
version again. A fresh pathname lookup after publication finds the
replacement inode. Old versions remain readable until the directory's
configured snapshot retention and garbage collection remove them (commonly a
week or longer).

OPEN with `CLAIM_FH` opens the file the handle names. A read open needs no
name. A write publishes at the file's name, which TernFS cannot derive from
the inode, so nfsd keeps a bounded in-memory table of where LOOKUP and OPEN
last saw each file. If the handle is no longer at its remembered name, or
this nfsd never saw it, the OPEN returns `NFS4ERR_STALE` and the client
opens by name instead.

The `/.nfs` directory is reserved for nfsd state and hidden from NFS clients.

## Writes

TernFS cannot modify the contents of a linked file. nfsd presents mutable
files by constructing a replacement inode and atomically publishing it over
the current directory entry after all writes are complete.

Opening a new or existing file for write allocates a transient TernFS inode
for the staged version. For a new pathname, OPEN also publishes a separate
empty inode before replying: LOOKUP and READDIR immediately see a regular
file of size zero, and the creator's filehandle is that empty inode.
The creator's open and local staging are prepared before this publication.

Staging creation holds an inode-keyed lock while creating the local data file
and saving its sidecar. The store-wide map lock only covers lookup and
publication of a complete entry. Removal and quarantine use the same inode
lock through filesystem cleanup, so independent creations can overlap
without cleanup deleting a later creation's files.
Further write opens of the same file by the same client join its existing
staging. Writers on other clients start independent staging from the
currently published version.
nfsd creates these files on its local host:

```
<staging directory>/
    <inode id>.staging
    <inode id>.meta
```

The staging files are named by the transient inode. `.staging` holds
a sparse overlay over the immutable version observed at OPEN, which is the
empty published inode for a newly created file. Exact dirty ranges are
authoritative locally; clean ranges are read from that base until hydrated.
Writers on one client share one staging version per file, because they share
that client's page cache and the client cannot keep them apart. Each client's
writer has independent contents, size and timestamps; readers on other
clients opening the pathname see the currently published version. Initial
CREATE attributes, including size, apply privately until the creator closes,
even for a read-only CREATE open; its share access still prohibits WRITE.

The first data or size mutation starts bounded low-priority hydration of clean
base ranges. Foreground reads take priority, fetched chunks are installed only
where the range is still clean, and fully overwritten or truncated ranges are
not fetched.

The `.meta` sidecar records the target, construction cookie, owning open, base
inode and size, logical size, committed dirty ranges, open-owner identity,
writer attributes and the EXCLUSIVE4 verifier. Stable writes, `COMMIT`, and size changes sync data before
atomically checkpointing this metadata. The checkpoint file is synced before
rename, and its directory is synced afterwards. A failed checkpoint remains
pending for the next retry. The writer's change attribute advances on mutations
and remains stable across GETATTR, VERIFY and recovery.

On the last write `CLOSE` or `OPEN_DOWNGRADE` to read access for a staged
version, nfsd cancels speculative hydration, fetches remaining clean ranges
with bounded parallel foreground reads, streams the complete staging file,
and links the transient inode over the target. Earlier write CLOSEs of a
shared version only hand ownership of the sidecar to a remaining writer.
Publication happens inside the CLOSE; there is no background close. This
publishes the complete version atomically. Concurrent writers on different
clients do not merge their edits: the last successful publishing CLOSE wins
the pathname, on the same nfsd or across hosts. Earlier versions and
outstanding writers retain their own contents. A write-open with no data,
size or explicit timestamp changes scraps its transient without replacing
the file. The initial empty version remains if the creator closes without
changes or abandons its staging. New-file creation therefore adds one empty
inode and publication operation.

A staging file belongs to one nfsd host. Persistent client and lease state can
invalidate an open across the fleet, but it does not make the staged data or
the process-local open state movable to another nfsd. Multiple writable opens
may target the same name. REMOVE and RENAME update their publication
targets as described under
[Namespace changes with staged writers](#namespace-changes-with-staged-writers).
The lookup-and-publish sequence for GUARDED and exclusive creates is
serialized only within one nfsd.

EXCLUSIVE4 and EXCLUSIVE4_1 retries by the same client and open-owner
reuse their staged
writer when the verifier matches and the pathname still names its original
empty inode. Other exclusive creates at that name return `NFS4ERR_EXIST`.
The verifier is retained in staging until CLOSE; it does not reserve the
name across hosts.

`fsync` and `COMMIT` preserve unpublished data on the staging disk; they do
not publish to TernFS or replicate the staging data. The durability guarantee
is therefore per host. Committed staging data survives an in-place restart.
Writers interrupted during a namespace operation may be quarantined and
require manual recovery. The client resends unstable writes after seeing the
new write verifier.

Moving a server address to a different host without its staging disk leaves
the new host with the client's identity but none of its staged bytes. It
cannot publish those bytes at CLOSE. Pin each nfsd instance's address to its
host. If the host must be replaced, attach its staging disk to the replacement
before moving the address. Provision the disk for complete replacement files.
Even a small edit can require reading and republishing the whole base at
CLOSE. Monitor staging capacity, hydration traffic and CLOSE latency.

Timestamp-only CLOSE updates the current published inode without reading or
republishing its contents. This preserves data published by a concurrent writer.
After a data publication commits, failure to restore the writer's timestamps is
logged and CLOSE succeeds; the data is already visible and cannot be rolled back.

The current sidecar format is NFS8. It contains the data checkpoint and two
namespace flags, `Unlinked` and `Guarded`. Decoding requires the exact layout
and rejects unknown flag bits. A sidecar with an older `NFSn` format, such as
NFS5 from the NFSv4.0 server on main or NFS6 and NFS7 from earlier
branches, makes
startup fail rather than being quarantined. Drain active writes, or remove
the staging directory, before upgrading to a binary that cannot read the
staging format on disk.

The implementation is in [`staging.go`](staging.go),
[`staging_lifecycle.go`](staging_lifecycle.go) and [`ops_open.go`](ops_open.go).

## Namespace changes with staged writers

REMOVE and RENAME update staged writers so that a later CLOSE does not
recreate a removed file or publish a renamed file under its old name. These
updates apply only to writers on the nfsd handling the request.

### REMOVE and RENAME

Each writer is either linked or unlinked:

- A linked writer publishes to the directory and name recorded in its sidecar
  on CLOSE.
- An unlinked writer has no publication target. Its open stays usable for
  READ and WRITE, but CLOSE discards its private version.

A retired writer has lost its active open state. nfsd retains its staging
data for recovery but releases its pathname reservation and file descriptors.
An unlinked writer's staging is discarded when its lease expires. If an
already retired writer becomes unlinked, its staging is quarantined because
it can no longer be reclaimed.

A successful REMOVE unlinks every writer at the name. RENAME handles source
and destination writers as follows:

| Source staging | Destination staging | Result |
| --- | --- | --- |
| Active or retired, same directory | None | Source writers follow the new name. |
| Active or retired, same directory | Present | Destination writers become unlinked; source writers follow the new name. |
| Active, another directory | Any | `NFS4ERR_FILE_OPEN`; nothing is submitted. |
| Only retired, another directory | Any | Source and destination writers become unlinked. |
| None | Present | Destination writers become unlinked. |

The cross-directory restriction exists because an active writer's transient
inode was constructed on the source directory's shard and can only be linked
there. Identical source and destination names are a no-op.

An operation through another nfsd does not update these writers. Their later
CLOSE still publishes at the recorded target. Across hosts, the last
successful publication wins the pathname, as described under [Writes](#writes).

### Failed operations

The TernFS client retransmits requests over UDP. A rejection may describe one
copy of a request while another copy applied. After submitting a namespace
operation, nfsd handles the reply as follows:

| Reply | Affected writers | NFS status |
| --- | --- | --- |
| Success | Updated as described above. | `NFS4_OK` |
| REMOVE: entry gone (`EDGE_NOT_FOUND`, `MISMATCHING_CREATION_TIME`) | Unlinked, so they cannot publish at the removed entry's name. | `NFS4ERR_NOENT` |
| Any other error, with staged writers | Marked as failed. | `NFS4ERR_IO` |
| Any other error, without staged writers | None. | Mapped backend error |

The unknown outcomes include `TIMEOUT`, a locked edge, a malformed or lost
reply, and `EDGE_NOT_FOUND` on RENAME. These fail the affected writers even
when the request was actually rejected. A request which arrives late can
still apply; failing the writers does not cancel it.

READ, WRITE, COMMIT and SETATTR through a failed writer return
`NFS4ERR_IO`. Operations which carry a stateid validate it before checking for
failure. CLOSE returns `NFS4ERR_EXPIRED` and removes the open marker. Other
writers belonging to the same client remain usable.

The failed writer's staging data and sidecar move under `quarantine/` for
manual recovery. Looking up the current pathname cannot establish whether
this operation applied or someone else changed the name, so nfsd cannot
automatically choose a publication target.

A retry after an applied first attempt returns `NFS4ERR_NOENT`, so `rm` or
`mv` can report an error even though the operation happened.

### Restart and quarantine

The backend operation and the sidecar update cannot be made atomic. Before
submitting an operation, nfsd sets the `Guarded` flag in each affected writer's
sidecar. It clears the flag when recording the result. A guard left on disk
means startup must quarantine that writer, even if the crash happened before
the backend request was sent.

At startup, nfsd quarantines sidecars which are guarded, or both unlinked and
retired, before truncating staging data or registering recovered writers.
If an entry cannot be recovered, nfsd logs the error, skips the entry and
continues startup. This includes unreadable sidecars, unrecognized formats,
truncate failures and failed quarantine moves. An older `NFSn` sidecar
format is the exception: startup fails, as described under [Writes](#writes).

A quarantined writer's `.staging` and `.meta` files are kept together in a
directory under `quarantine/`. A partial move leaves each file where it
reached and requires manual recovery. To list quarantined entries, run:

```sh
nfsd inspect -registry <addr> -staging <dir>
```

A staging file is a sparse overlay. The unwritten ranges of an unlinked or
quarantined file remain in its immutable base inode. Those ranges are
available only while the directory's snapshot retention preserves that inode.
Keeping the local staging files does not extend that retention period.

### Implementation

Pathname locks serialize REMOVE and RENAME with CLOSE, reclaim and cleanup
within one nfsd. The guard is used only during startup recovery; the running
process relies on these locks.

1. Lock the pathnames, retire inactive writers, and record the IDs of the
   affected writers and their intended publication targets.
2. Look up the source entry, its inode and edge creation time. For RENAME,
   look up the destination as well. A file over a directory or a directory
   over a file returns `NFS4ERR_EXIST`. A failure here returns the error
   without changing any writer's publication target.
3. Write `Guarded` for every affected writer. If a save fails, clear the
   guards already attempted and return `NFS4ERR_IO` without calling the
   backend. If clearing a guard cannot be checkpointed, the writer remains
   usable, but a restart before checkpoint repair will quarantine it.
4. Submit the operation with the source entry's identity, so it cannot apply
   to a later file at the same name.
5. Update the writers according to the backend reply. For a known outcome,
   save the final namespace state and clear the guard.

The local test backend does not retransmit. Its syscall rejections establish
that the operation did not apply, so nfsd leaves the writers linked and clears
their guards.

Guard and final checkpoints update only the namespace fields of the current
sidecar under the file lock. This preserves any data checkpoint advanced by
a concurrent COMMIT. If saving the final namespace state fails, the writer
remains usable with its updated in-memory state. The checkpoint is marked
dirty so the next `Sync` retries the save. After a crash, a final checkpoint
supplies the updated namespace state; a surviving guard causes quarantine.

A failed writer stays in memory as a tombstone until the process exits,
outside the pathname index. The tombstone outlives CLOSE because COMMIT
carries no stateid: without it, nfsd could acknowledge failed writes with the
current write verifier. Restarting changes the verifier, so the tombstone
need not be persisted. The failure check in `lookupDurableOpen` runs before
sidecar recovery and client-wide expiry, keeping the failure confined to
that writer.

Before moving files into quarantine, nfsd syncs the destination directory.
It syncs both source and destination directories after the moves. A partial
move is logged; nfsd does not delete files to finish it.

## Open state

Each nfsd process keeps its detailed NFS open state in memory. This includes
the association between a stateid, its client and open-owner, its file and
its access mode. Open-owners have no sequence numbers; session slots order
and deduplicate requests.

The persistent store holds a smaller record of active opens and leases. The
process-local state supplies file affinity. The shared state allows a client
reboot confirmed on one nfsd to invalidate old state on the others.

An open-owner has one stateid per file. A repeated OPEN by that owner
returns the same stateid with its generation incremented, its share access
ORed in, and its `share_access` and `share_deny` values recorded. A stateid
with seqid 0 means the current generation, and the current stateid set by
OPEN, CLOSE and OPEN_DOWNGRADE can be used in place of an explicit one. A
write OPEN of a file the owner has open for read upgrades that open in
place, keeping its stateid.

`OPEN_DOWNGRADE` takes READ, WRITE or BOTH, ignoring the delegation-want
bits, and a deny value. The target must be a subset of the open's access
and deny and equal the union of the values recorded inside it (RFC 8881
section 18.18.3), otherwise `NFS4ERR_INVAL`: an owner that opened READ and
then WRITE may narrow to either, a single BOTH open may not. Narrowing BOTH
to READ publishes the open's writer exactly as CLOSE would and keeps the
stateid as a read open of the published version. Narrowing BOTH to WRITE
drops READ and keeps the writer. Every success advances the seqid. A writer
recovered after a restart gets only the subset rule, and its deny is taken
to be NONE. Its first downgrade installs it as a live open under the same
stateid, with the narrowed grant and the next seqid; downgraded to READ, it
publishes first. If the same owner already has a live open of the file, the
downgrade gets `NFS4ERR_DELAY` before any effect, and both opens are kept.
READ with a write-only open is allowed.

A stateid is valid only under a session of the client incarnation that
owns it. Another client's stateid, live, recovered or expired, gets
`NFS4ERR_BAD_STATEID` from every operation, a timestamp-only SETATTR
included, with no other effect; only the owner sees `NFS4ERR_EXPIRED`. A
timestamp-only SETATTR with an unknown stateid, or the zero stateid, sets
the times as this client sees the file, as before. FREE_STATEID forgets the caller's expired
stateid, returns `NFS4ERR_LOCKS_HELD` for a live one and
`NFS4ERR_BAD_STATEID` for an unknown or foreign one. TEST_STATEID uses the
same rules.

A read open by a client that also has a writer on the file reads the
writer's staging, so the client sees its own writes. A writer CLOSE that
publishes nothing, or only times, points those readers back at the
published file. If publication fails, because hydration failed or LinkFile
returned an error, the readers of the staging get `NFS4ERR_IO`, never the
old bytes. After a publishing CLOSE they read what was published. A failed
publication with no other reader keeps the baseline policy: the staged bytes
are discarded, not quarantined; later operations on the failed stateid can
get `NFS4ERR_EXPIRED` rather than `NFS4ERR_IO`, special-stateid reads can
get a backend error, and COMMIT is promised no failed tombstone. They affect
only that open: the client's other opens and staging are untouched.

OPEN, CLOSE and OPEN_DOWNGRADE of one file are serialized across all clients
on one nfsd. An OPEN after a publishing CLOSE therefore sees what it
published. Opens of different files run in parallel.

The implementation is in [`open_state.go`](open_state.go) and
[`view.go`](view.go).

## Client identities and incarnations

An NFS client supplies an opaque identity and an eight-byte verifier in
`EXCHANGE_ID`. The identity says which client this is. A changed verifier
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
hash of the opaque `EXCHANGE_ID` identity.

`i.<random>/` is an incarnation directory. Its TernFS inode ID is the NFS
clientid for that incarnation.

`confirmed` is a symlink to the current incarnation.

`pending` is a symlink to the incarnation awaiting its first
`CREATE_SESSION`.

`client` is a JSON record containing the opaque `EXCHANGE_ID` identity, the
client verifier, the confirmation verifier generated by nfsd and the
normalized RPC principal. The server does not use the stored
identity. It is there so the inspector can identify the client behind an
identity hash.

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

`EXCHANGE_ID` registers the identity and `CREATE_SESSION` confirms it. For a
confirmed identity, `EXCHANGE_ID` with the same verifier updates the
existing incarnation. A different verifier represents a new incarnation and
produces a new clientid. The first `CREATE_SESSION` for a clientid confirms
the incarnation; later ones only create sessions. `EXCHANGE_ID` with
`EXCHGID4_FLAG_UPD_CONFIRMED_REC_A` answers from the confirmed record without
changing it, since nfsd has no per-client properties for an update to change.

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
It carries no address. A client which holds a session but no open state can
therefore be replaced by another principal. RFC 8881 section 18.35.5 treats
a live lease with state as in use; this implementation counts only open
state, so an idle identity cannot remain pinned indefinitely by
unauthenticated AUTH_SYS data.

## Leases

A client has two leases here. The logical lease is this process's: it lives
in memory, per incarnation, and is renewed whenever a request admitted by
SEQUENCE, or a successful callback, ends. It stays held while a request is
active and for one lease time after the last one. While it is held this
process never expires the client. The durable lease is the lease file this
process keeps in the client store, which other nfsd processes read.

Durable lease files are written only by a refresher, one worker per
incarnation that needs it. SEQUENCE, OPEN and stateid operations read its
record from memory and ask it for work. The worker publishes while the
logical lease is held, or once after CREATE_SESSION, rewriting the lease
when less than 112.5 seconds of it remain, with a new expiry 135 seconds
ahead: about every 22.5 seconds while a client is active. Each write is a
new uniquely named temporary file linked into the incarnation directory and
renamed over the lease through its exact edge. An unsettled rename blocks
later writes until it is fenced. A temporary file whose LINK might have
applied is kept as an obligation, with its credit, until its edge is found
and removed; nothing is released on elapsed time or absence, and such a file
is never published as the lease. A record has at most four such files and
the process at most four per record credit; at the cap refresh pauses.
Failures back off from 5.6 to 22.5 seconds. The worker also reads the
`expired` marker and the confirmed pointer: a marker or a replaced client
ends the incarnation's sessions here. A record holds one of a bounded number
of record credits until nothing uses it: no session, reservation, request
or quiet period, and no pending bootstrap, verify, attempt or temp
obligation. The lease sweeper, or the release of the last reservation,
then has the record's worker free it at once, without a store call and
without moving any deadline; a record still referenced, needed or holding
an obligation is left as it is.

OPEN requires a published lease that has not expired, and otherwise returns
`NFS4ERR_DELAY` while the worker reads it. It then creates an open marker;
CLOSE removes it.

The worker's passes are scheduled, not driven by requests: a request only
notes that the lease is needed. Maintenance (settling temps, refresh,
pruning and the terminal steps) runs at its own deadline, which a failed
pass's backoff bounds from below; bootstrap, a need arriving while the
worker is idle, or a stale or fleet-expired state not yet acted on makes it
due, but never before the backoff. A verify only reads, at least a second
after the previous pass, and neither runs nor moves maintenance, except
that one finding the client stale or fleet-expired goes on to the terminal
steps when no backoff applies. Removing
other processes' dead lease slots is done only in an authorized pass, while
this process's lease is live, and under the same protection as other
destructive client-store cleanup, as is removing this process's own lease
once the client is stale or fleet-expired. No lease observed and no
expired marker means registration is unproven, not a live lease: it admits
nothing and releases nothing. If bootstrap publication keeps failing, its
bounded record and temp obligations remain until a late LINK is settled or
the process restarts. The store's own `Renew`, used without a server, runs
the worker's pass inline; with a server it only reads the worker's record.

An operation whose open turns out inactive gets `NFS4ERR_EXPIRED`, and only
an open whose own marker is gone loses its entry; local expiry of the client
is the lease gate's decision. When the gate expires a client, it retires
the client's sessions, cancelling their callbacks and returning their
credits, whether or not the expired marker could be written. A
CREATE_SESSION for an incarnation this process has expired gets
`NFS4ERR_STALE_CLIENTID`; its cached CREATE_SESSION result still replays.
This holds from the moment the terminal steps start, before their store
calls.

Separate lease files are needed because several nfsd processes may handle
requests for the same client. A client's durable lease is live while any
nfsd has a live lease file for it.

Marker creation, marker removal and explicit renewal share a per-identity
read lock. Registration, confirmation, expiry decisions and collection take
exclusive ownership. Independent markers can therefore overlap while client
replacement still waits for marker work to finish. A waiting exclusive
holder blocks new readers, so a sweep can temporarily delay new OPENs for
that identity.

A per-incarnation lease lock serializes the refresher's cycles. It is
released before marker I/O. A separate per-marker lock covers the durable
mutation and its local map update.

Every stateid operation checks the confirmed pointer, which costs one lookup
but detects a replaced clientid immediately, and the `expired` marker. It
skips lease scans while this process's published lease is in the future.
READ and WRITE with a real stateid therefore cost two shard lookups. The first
OPEN handled by one nfsd also has to resolve the durable client record; later
OPENs reuse the cached identity and confirmed-pointer location.

These updates leave garbage behind in TernFS. A same-directory rename
soft-unlinks the temporary edge and turns the replaced edge into a snapshot
edge, so each lease renewal leaves two snapshot edges and one dead file inode.
Each OPEN and CLOSE marker pair leaves one snapshot edge and one dead inode.
Renewals are bounded to about one per 22.5 seconds for each nfsd and active
client, but markers scale with open activity. The shard garbage collector removes this
under the snapshot policy in effect for `/.nfs`, so that tree should not keep
the retention used for user data. A policy which deletes snapshots immediately
is appropriate, for example:

```
terncli set-dir-info -id <inode of /.nfs> -tag SNAPSHOT \
    -body '{"DeleteAfterVersions": 0}'
```

Client-store failures without a more specific NFS status return
`NFS4ERR_DELAY`, allowing the client to retry without discarding local open
state.

If no lease is live, its open markers no longer represent active state. If the
confirmed pointer names a different incarnation, the clientid is stale and its
stateids are expired.

Lease and confirmation slots use a random name per nfsd process. While its
own lease is live, the refresher removes other processes' slots which have
been expired for a full additional lease, allowing for clock skew while
preventing slots from accumulating across server restarts. A process removes
its own lease only after the client has an `expired` marker. Only one path
writes that marker: the expiry decision, which needs the logical lease to be
unheld, observes the durable lease, and gives way to a renewal or callback
reply that arrives meanwhile. An expired clientid therefore cannot renew or
create new open state even after all of its process slots have been removed.

OPEN markers are create-if-absent records. A repeated OPEN whose stateid
already owns a marker reuses the existing TernFS inode.

A client whose last request was at time t is therefore expired here by about
t + 3 × lease on a healthy backend (the quiet period, the durable lease
written during it, then a sweep), and other processes see its durable lease
until up to t + 2.5 × lease.

Each nfsd checks its local clients every half lease period. A client whose
logical lease is not held and with no live fleet lease loses its
process-local open state and local open markers.
Its linked staging is retired, preserving acknowledged bytes without reserving
the pathname or holding open file descriptors. Unlinked staging is discarded;
a writer already retired when unlinked is quarantined. Linked retired staging
is retained until recovered and closed or explicitly removed by an
administrator; provision and monitor disk space accordingly.
The persistent client store is the authority for replaced clientids; the
process-local open store does not keep a separate revoked set. After lease
expiry the client's next SEQUENCE returns `NFS4ERR_BADSESSION`, and the
client has to register again with EXCHANGE_ID.

Open-owners are not retained after their last open closes. A retransmitted
CLOSE is answered from the session slot's reply cache; a closed stateid
presented again gets `NFS4ERR_BAD_STATEID`.

## Restart recovery

The persistent records do not reconstruct general process-local open state
after an nfsd restart. A client normally establishes new state.

Sessions do not survive a restart, as described under
[Sessions](#sessions), and stateids are process-local. A client which sends
one nfsd's stateid to a different nfsd, or to the same nfsd after a restart,
gets `NFS4ERR_BAD_STATEID`, not `NFS4ERR_STALE_STATEID`. The Linux client
recovers by registering again and opening the file again: SEQUENCE gets
`NFS4ERR_BADSESSION`, CREATE_SESSION with the old clientid
`NFS4ERR_STALE_CLIENTID`, EXCHANGE_ID gives a new clientid for the same
incarnation, `CLAIM_PREVIOUS` gets `NFS4ERR_NO_GRACE`, and the open is
reestablished with CLAIM_FH.

A write open is the exception because its local staging and sidecar files may
still hold unpublished data. On startup nfsd discovers these files and checks
whether they can be recovered, as described under
[Restart and quarantine](#restart-and-quarantine). An eligible sidecar contains
the state needed to complete the pending `CLOSE` or rebind the staging to a
replacement `OPEN` for the same clientid. Because writers on one client share
staging, any open-owner of that clientid may reclaim it. Multiple writers
recover independently, including when another writer has published a newer
version in the meantime. Replacements recover only checkpointed dirty ranges,
including new files backed by their empty published inode.

A client reconnecting after lease expiry receives a new clientid. Sidecars
also persist a recovery key derived from its stable client identity, boot
verifier and RPC principal. Matching that key and the original open owner allows
the same client boot to reclaim its staged data after expiry,
including across an nfsd restart or collection of the old incarnation.
A changed boot verifier, different principal or different client cannot adopt
that staging. Ambiguous recovery returns EXPIRED instead of silently opening a
fresh version. Sidecars without a recovery key support same-clientid restart
recovery only. Recovery requires returning to the host holding the staging disk.
The immutable base and transient inode must also remain available under TernFS's
retention policies; local staging retention does not extend those policies.

After one lease period of startup grace, the periodic sweep checks all local
staging, including writes created since startup. Staging for an expired or
stale clientid is retired if linked and discarded if unlinked. A confirmed
client with no lease slot is retained because the first OPEN creates staging
before it writes the slot. The startup grace gives a client time to reclaim a
recovered write after a server outage.

## Incarnation collection

Replacing a registration leaves an incarnation directory which may still be
used by a confirmation running on another nfsd. It cannot be removed inline
with `EXCHANGE_ID`.

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
`update` records do not expire; the next update of the confirmed record
replaces them.

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

## Inspecting client state

`nfsd inspect` reads the client store and explains its current state. It uses
the same record definitions and lease rules as the server, but does not
modify the store.

```
nfsd inspect -registry <addr> [-staging <dir>] [-json]
nfsd inspect -registry <addr> -identity "Linux NFSv4.1 host"
nfsd inspect -registry <addr> -identity-hash <hash>
nfsd inspect -registry <addr> -clientid 0x200000000000000e
nfsd inspect -registry <addr> -stateid 09cb5ee40000000000000001
```

With no filter, the command reports every identity. At most one filter may be
specified. `-clientid` and `-stateid` take the values shown to a client or in
a packet capture. The clientid is the incarnation directory inode. The
stateid is the 12-byte `other` field. `-json` emits the same report in a
machine-readable form.

For each identity, the report shows the stored identity string and its
`confirmed` and `pending` pointers. It then shows every incarnation and its
roles: `confirmed`, `pending`, `reboot-target` or `unreachable`. The summary
and each incarnation use distinct lifecycle states:

- `ACTIVE` is confirmed and has a live lease.
- `EXPIRED` had a lease which elapsed and cannot renew.
- `UNLEASED` is confirmed but has never recorded a lease; it can still
  acquire its first lease.
- `PENDING` is waiting for its first `CREATE_SESSION`.
- `REPLACED` is retained temporarily as a reboot target.
- `UNREACHABLE` has no persistent pointer keeping it live.

Each incarnation includes its client and update records, decoded principal,
lease and confirming slots, expiry marker, GC deadline,
open markers and temporary files. Lease slots name the nfsd which wrote them.
The first four bytes of an open stateid identify the nfsd process which issued
it. An open marker is `STALE` when its incarnation is not active.

Expired slots and temporary files are marked `collectable` once they are old
enough for collection. An expired lease slot is only removed when a daemon
next scans that client; the lease sweeper does not walk the entire shared
store. The inspector does not remove anything. It also does not create the
`expired` or `gc` records which a server scan may add. The report is a
read-only view of the state at that point in time.

Open markers do not say which file they belong to. That association lives in
the process-local open state and, for write opens, in the staging sidecars on
the nfsd host. `-staging` reads those sidecars and joins them to open markers
by stateid. This adds the target directory, file name and staged size to the
report. It also reports the sidecar format version, construction cookie,
recovery key, owning client and open owner, access mode, base inode and size,
checkpointed logical size and dirty ranges, writer attributes and the
EXCLUSIVE4 verifier when present, plus the `unlinked` and `guarded` namespace
flags. An entry is `RETIRED` when its lease expired and the acknowledged data
is being held for the client to reclaim. The staging summary includes logical
and allocated bytes. Uncheckpointed writes and in-memory hydration progress
are not visible.

The reported sidecar version is the on-disk metadata layout, printed as `v8`.
It is not an NFS protocol version.

The inspector reads the staging directory as `.staging` and `.meta` pairs named
with the file inode as sixteen hex digits, which is how recovery looks them up.
It reports missing or malformed files, metadata temporaries and any name which
does not match that spelling. Quarantined files are listed separately; see
[Restart and quarantine](#restart-and-quarantine) for the reasons an entry
can be quarantined. A missing staging path is an error rather than an empty
report. The staging directory is local, so this option must run on the nfsd
host which owns it.

Sidecars without an open marker are shown under their client. In an unfiltered
report, sidecars belonging to no client in the store are listed once at the end.
A filtered report has not read the other identities, so it cannot tell an orphan
from another client's file: it counts those sidecars instead of listing them, and
says so. Rerun without a filter to see them.

### Exit status

The command is usable as a monitoring check without parsing its output:

| status | meaning |
| --- | --- |
| 0 | the report was produced and found no problems |
| 1 | the report could not be produced |
| 2 | usage error |
| 3 | the report was produced and found problems |

A problem is a defect: an unreadable or missing record, a dangling `confirmed`,
`pending` or `reboot` pointer, an unrecognized entry in an incarnation
directory, a broken or unpaired staging file, or a quarantined checkpoint. The
count appears as `problems` in the summary line, again at the end of the text
report, and as `summary.problems` in the JSON.

Expired leases, stale open markers, unreachable incarnations, collectable slots
and temporaries, and retired staging entries are **not** problems. They are the
states the lifecycle is meant to pass through, and a check which fired on them
would fire constantly.

A filter which selects nothing is also not a problem: a stale clientid, an
unregistered identity, a stateid with no marker, or the `directory` inode
pasted into `-clientid` by mistake. Those are answers to the question asked, so
they are reported as a `note` and exit 0. Notes are collected under `notes` in
the JSON.

Because a filtered report only reads the selected clients, its exit status
covers those clients and the layout of the staging directory; defects inside
sidecars belonging to other clients are neither shown nor counted. Run without
a filter to check the whole store.

At startup each nfsd logs its `nfsd_id`, which names its `lease.` and
`confirming.` slots, and its `stateid_epoch`. Grep the fleet's logs for these
values to find the process which holds a lease slot or issued a stateid.

## Profile and limitations

nfsd serves a deliberately small NFSv4.1 profile over the same TernFS
client store, staging and synchronous CLOSE as the NFSv4.0 server it
replaced, with the same storage limitations. All new v4.1 state lives in
process memory.

- **Volatile sessions.** Sessions, slots and the reply cache end with the
  process. A request completed before a restart is not run again; the
  client registers again as described under
  [Restart recovery](#restart-recovery).
- **Startup latency.** CREATE_SESSION waits, for up to two seconds, for
  the client's first durable lease publication. If it has not landed by
  then, the first SEQUENCE gets `NFS4ERR_DELAY` until it does. After an idle
  period the first SEQUENCE may get `NFS4ERR_DELAY` while the lease is read.
- **Wire clientids.** A clientid is the process's epoch, the second after
  it bound its listening socket, followed by a counter. It names a record in
  that process's memory only and is never stored. A clientid from another
  process or epoch gets `NFS4ERR_STALE_CLIENTID`.
- **Endpoint ownership and clocks.** Each nfsd is its own server owner. A
  successful bind is taken as exclusive ownership of the endpoint. The wall
  clock must not run backwards across the processes that serve one
  endpoint, and a failover must stop the old owner first.
- **Unresolved confirmation.** While a CREATE_SESSION's confirmation of a
  rebooted client has an outcome this process cannot yet determine, both
  incarnations are protected: their expiry, staging retirement and
  collection are deferred. Until the sweeper or a retry settles it, the
  retried CREATE_SESSION, SEQUENCE, BIND and CREATE_SESSION on the old
  incarnation, and a new EXCHANGE_ID for the identity get `NFS4ERR_DELAY`,
  and DESTROY_CLIENTID gets `NFS4ERR_CLIENTID_BUSY`. Lease expiry does not
  settle it.
- **Logical and durable leases.** This process never expires a client while
  its logical lease is held, but another process cannot be quiesced from
  here: if this process's store writes fail long enough, another process may
  mark the client expired after its last real durable publication.
  Callbacks alone keep a client live in this process.
- **Persistent LINK failure.** If LINK keeps failing with unknown outcomes,
  a client's durable refresh stops at its cap until a late LINK is found and
  removed, or the process restarts; SEQUENCE then gets `NFS4ERR_DELAY` once
  the published lease expires.
- **Locks and share reservations.** LOCK, LOCKT and LOCKU return
  `NFS4ERR_NOTSUPP`, although RFC 8881 table 16 marks them required.
  `share_deny` is recorded for OPEN_DOWNGRADE but not enforced.
- **Deferred handle defects.** Content reversal through an old inode
  handle, including through CLAIM_FH, is not fixed: a client may read
  newer content through a handle it obtained earlier. A healthy cold handle
  this process never named gets `NFS4ERR_STALE` for a writable CLAIM_FH
  unless it names a recoverable staged writer. Read opens are not recovered
  after a restart.

## Not implemented

Share reservations are not enforced. nfsd records the share access mode of an
open so it can reject a `WRITE` on a read open, but it ignores `share_deny`.
TernFS does not hold deny reservations, and process-local reservations would
not be visible across the nfsd fleet.

Byte-range locking, delegations, pNFS, named attributes, hard links and
special files are also not implemented. The backchannel carries only
CB_NULL and CB_RECALL_SLOT. There is no grace
period: `RECLAIM_COMPLETE` is accepted, and there is nothing to reclaim.
OPEN never sets `OPEN4_RESULT_PRESERVE_UNLINKED`, so the Linux client still
silly-renames a busy file instead of removing it. nfsd implements NFSv4.1
only.

The session reply cache is process-local and holds only replies the client
asked to cache. A retransmission of a request which completed before a
restart gets `NFS4ERR_BADSESSION` and does not run again. If the client
resends it on a new session, it is a new request there.

## Testing

TernFS is not a general-purpose POSIX filesystem. Tests must distinguish
between:

- behavior which the server supports;
- operations which the server must reject with the correct NFS status; and
- NFS features which are outside the intended TernFS contract.

The libnfs and pynfs tests use the same process helpers.
Protocol unit tests and the kernel-client VM tests have separate runners.

The raw-protocol `test-cluster` target uses `ss` to check the fixed registry
UDP port `127.0.0.1:55556` before starting. It fails if another
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
writes, sessions, client and open state, and expected errors for
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

### Client integration tests

The client harnesses live under [`test`](test) with their own
[`Makefile`](test/Makefile); run the targets below from `go/nfsd/test`. Every
libnfs and pynfs case runs with separate server and client processes. Targets
ending in `-cluster` start a temporary `ternrun` backend; the others require
an existing backend's registry address:

```sh
make test-libnfs-cluster
make test-pynfs-cluster
make test-libnfs TEST_ARGS='-registry HOST:PORT'
make test-pynfs TEST_ARGS='-registry HOST:PORT'
```

Use `TEST_ARGS='-binaries-dir /path/to/binaries'` with a cluster target to
reuse prebuilt backend binaries. Otherwise `ternrun` builds them. Add
`-nfsd /path/to/nfsd` to test a particular nfsd binary instead of building it.
The registry address is the bincode endpoint. Both modes start their own
nfsd processes on the test host, with `-allow-minor-version-2` so that libnfs
can connect. Relative paths are resolved from `go/nfsd/test`.

```sh
make test-libnfs TEST_ARGS='-registry HOST:PORT' \
    GO_TEST_FLAGS=-short
make test-libnfs TEST_ARGS='-registry HOST:PORT' \
    GO_TEST_FLAGS='-run TestLibnfs/recovery'
make test-libnfs TEST_ARGS='-registry HOST:PORT' \
    GO_TEST_FLAGS='-run TestLibnfs/ReadFile'
```

The Go tests live in [`test/libnfs`](test/libnfs); the pynfs runner is
[`test/pynfs/main.go`](test/pynfs/main.go). To skip timed cases, use
`GO_TEST_FLAGS=-short` for libnfs or add `-short` to `TEST_ARGS` for pynfs.
The NFSv4.1 pynfs suite has no `timed` flag, so the runner's `-short` excludes
its lease-waiting cases by test code.
Failed runs retain logs and test data;
`-artifacts-dir DIR` selects their local parent directory. Pynfs should run
serially per existing filesystem because some cases reuse client identities.

The tests need Linux, Go and the selected client dependencies, with no kernel
mount or root requirement. Make builds pinned libnfs 8.0.0 under `test/.deps`;
this needs CMake and a C/C++ toolchain. libnfs 8 offers sessions only as
NFSv4.2, which is why the harness passes `-allow-minor-version-2`. A release
tarball saved as `test/.deps/libnfs-libnfs-8.0.0.tar.gz` is unpacked if
present; otherwise Make clones the tag, which needs Git. These suites are not
yet run in CI.

### pynfs protocol tests

[pynfs](https://github.com/linux-nfs/pynfs) is the Linux NFS project's
protocol test suite. `test/Makefile` pins release `pynfs-0.5` and uses its
NFSv4.1 server tests in the `nfs4.1` tree, run with `--minorversion=1`.
`make build-pynfs` runs pynfs's top-level `setup.py build`, which generates
the XDR, RPC and NFSv4.1 modules in place.

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

The pynfs runner uses the shared process harness and runs Python against its
nfsd subprocess with `--maketree --rundeps`. It reads JSON results so protocol
failures fail the command. Pynfs itself otherwise exits successfully when
individual tests fail. A failed run retains its test tree.

The Make target writes the complete console output to `test/pynfs.out` while also
displaying it. Set `PYNFS_OUTPUT` to use another path. The output file is not
ignored by git, so completed runs remain visible during review.

TernFS intentionally does not implement several POSIX and NFS features covered
by pynfs. The harness uses two mechanisms to exclude those tests:

* The default `PYNFS_TESTS` value, `all noblock nochar nofifo nosocket
  nodeleg`, uses pynfs flag selectors to exclude block, character, FIFO and
  socket special files and delegations.
* The [`pynfs_unsupported.txt`](test/pynfs/pynfs_unsupported.txt) manifest lists
  individual NFSv4.1 cases requiring unsupported locking, share reservations,
  hard links, directory-over-directory rename, anonymous-stateid writes after
  CLOSE, a grace period, or immediate staleness of a replaced client record.
  The 4.1 suite has no broad lock or link flags, so these are listed by code.

Select flags or individual pynfs test codes with `PYNFS_TESTS`. The manifest
exclusions are appended after `PYNFS_TESTS`, so set `PYNFS_SKIP_FILE=` to
bypass this manifest.

Set additional runner options with `PYNFS_ARGS`. The default Go test timeout is
one hour because pynfs includes lease-expiry cases which deliberately sleep
for a lease period or longer. Override it with `PYNFS_TIMEOUT`:

```sh
make test-pynfs-cluster PYNFS_TESTS='sequence create_session'
make test-pynfs-cluster PYNFS_TESTS='SEQ1 OPEN2' PYNFS_ARGS='--showtraffic'
make test-pynfs-cluster PYNFS_TESTS='all noblock nochar nofifo nosocket nodeleg' TEST_ARGS=-short
make test-pynfs-cluster PYNFS_TESTS='all nodeleg' PYNFS_SKIP_FILE=
make test-pynfs-cluster PYNFS_TIMEOUT=2h
```

The lease-waiting cases excluded by `-short` are listed in
[`test/pynfs/main.go`](test/pynfs/main.go); each waits for at least one
90-second lease. The harness runs Python unbuffered and reports tests taking at least one second, sorted by
duration, after pynfs writes its results.

Use `make clean-pynfs` to remove the source and generated files. This suite is
not currently run in CI.

### Functional workflow tests

[`test/functional`](test/functional) contains numbered, independently
executable workflow tests for an existing mounted filesystem. From
`go/nfsd/test`:

```sh
make test-functional FUNCTIONAL_TEST_ROOT=/mnt/qa-nfs/test
```

The [functional test catalog](test/functional/README.md) describes each case.
Every test creates its own directory and preserves it on failure. A single
case can be run directly or under `strace`:

```sh
strace -ff -o /tmp/0005.trace \
  ./functional/0005-vim-edit.sh /mnt/qa-nfs/test
```

The same cases can run through the Linux kernel NFS client without a host
mount. `test-functional-cluster` starts a temporary `ternrun` cluster and an
nfsd through the shared harness, then boots a small qemu guest which mounts
that nfsd's export with `vers=4.1` and runs the suite:

```sh
make test-functional-cluster TEST_ARGS='-binaries-dir /path/to/binaries'
make test-functional-cluster FUNCTIONAL_TESTS=0005,0009
```

The guest is a locally installed Debian kernel with a busybox initramfs
([`test/internal/kernel/prepare-image.sh`](test/internal/kernel/prepare-image.sh)). It shares
this host's root filesystem over virtio-9p, so the functional scripts and the
tools they call run unchanged, and reaches nfsd on the host loopback through
qemu user networking, so no root, KVM, tap or FUSE access is needed. It runs
under TCG when `/dev/kvm` is absent; a full run then takes a few minutes.
[`test/internal/kernel/main.go`](test/internal/kernel/main.go) is the driver and accepts the
same `-registry`, `-binaries-dir`, `-nfsd` and `-artifacts-dir` arguments as
the libnfs and pynfs targets; [`guest-init.sh`](test/internal/kernel/guest-init.sh) is
PID 1 in the guest. Debian needs `qemu-system-x86 busybox-static cpio kmod
linux-image-amd64 nfs-common`; `KERNEL_VERSION` selects among installed
kernels. Each guest uses its own hostname as its NFSv4 client identity, since
nfsd instances on one TernFS share the durable client store.

With `-restart-mid-write` the driver runs one restart check instead of the
functional cases. The guest
([`restart-writer.py`](test/internal/kernel/restart-writer.py)) holds a file
open for writing and fsyncs part of it. The driver then kills nfsd and starts
it again on the same address, staging directory and export. The guest writes
the rest and closes, and the driver reads the published file back through
the backend.

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
NFS client and runs `terntests -nfs -filter nfs`. It still mounts with
`vers=4.0`, which this nfsd rejects; it needs `vers=4.1` before this path
works again.

That filter selects three tests:

- [`nfs mounted fs`](../terntests/terntests.go) runs the existing `fsTest`
  workload through its `posixHarness`, with the NFS mount as its root. In short
  mode this uses 10 directories and 500 files.
- [`nfs mutations`](../terntests/nfsmutate.go) is the NFS-specific mutation
  suite. It checks create/write/readback, out-of-order writes, rename, delete,
  timestamp updates, existing-file overwrite/truncate, and coherent
  fsync/fstat/append behavior, immediate empty-file visibility, private
  concurrent writers and retained readers. Its private-writers step
  (`nfsPrivateWriters`) opens two writers through one mount and assumes
  per-open isolation. Writers on one client now share a staged version, so
  that step will need updating to use two clients.
- [`nfs lease recovery`](../terntests/nfsoutage.go) disconnects the mount through
  a TCP proxy for 100 seconds, exceeding the 90-second NFS lease. It then
  appends and publishes both a new file and an existing-file replacement,
  checking that all data acknowledged before the outage survives.

Each workload starts its own nfsd and mount, then unmounts and stops nfsd
before cleanup. Cleanup deletes the whole TernFS namespace, including `/.nfs`;
a running server cannot retain client-state directory IDs across that deletion.
Mutation sub-step progress is printed to the CI log.

This is the only CI test path using a kernel NFS client.
