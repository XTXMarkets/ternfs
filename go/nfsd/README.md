<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd

`nfsd` presents TernFS through NFSv4.0. It translates NFS filehandles and
operations to the smaller TernFS interface, stages writable files locally,
and implements the client and open state required by the NFS protocol.

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

The [`TernVFS`](vfs.go) interface is the boundary between the NFS protocol
implementation and TernFS. The production implementation in
[`ternvfs.go`](ternvfs.go) maps namespace operations, reads and file
construction to TernFS requests. Tests can use a local implementation of the
same interface.

TernFS assigns every file and directory an inode ID. nfsd uses the eight-byte
encoding of that ID as the NFS filehandle. The inode type is part of the ID, so
many operation and attribute checks do not require another namespace lookup.

Ownership and mode attributes are synthetic. Each write-open receives its own
transient filehandle. Reads and attributes through that handle describe the
writer's private staging file. Published filehandles always describe their
immutable version, including while writers are active and after they close.
A fresh pathname lookup after publication finds the replacement inode.
Old versions remain readable until the directory's configured snapshot
retention and garbage collection remove them (commonly a week or longer).

The `/.nfs` directory is reserved for nfsd state and hidden from NFS clients.

## Writes

TernFS cannot modify the contents of a linked file. nfsd presents mutable
files by constructing a replacement inode and atomically publishing it over
the current directory entry after all writes are complete.

Opening a new or existing file for write allocates a private transient TernFS
inode. For a new pathname, OPEN also publishes a separate empty inode before
replying: LOOKUP and READDIR immediately see a regular file of size zero.
The creator's open and local staging are prepared before this publication.
Other writers start independent staging from the empty published version.
nfsd creates these files on its local host:

```
<staging directory>/
    <inode id>.staging
    <inode id>.meta
```

The creator's filehandle names its private transient inode. `.staging` holds
a sparse overlay over the immutable version observed at OPEN, which is the
empty published inode for a newly created file. Exact dirty ranges are
authoritative locally; clean ranges are read from that base until hydrated.
Each writer has independent contents, size and timestamps; readers opening
the pathname see the currently published version. Initial CREATE attributes,
including size, apply privately until the creator closes, even for a read-only
CREATE session; its share access still prohibits WRITE.

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

On `CLOSE`, nfsd cancels speculative hydration, fetches remaining clean ranges
with bounded parallel foreground reads, streams the complete staging file,
and links the transient inode over the target. This publishes the complete
private version atomically. Concurrent writers do not merge their edits:
the last successful publishing CLOSE wins the pathname, on the same nfsd
or across hosts. Earlier versions and outstanding writers retain their own
contents. A write-open with no data, size or explicit timestamp changes
scraps its transient without replacing the file. The initial empty version
remains if the creator closes without changes or abandons its staging.
New-file creation therefore adds one empty inode and publication operation.

A staging file belongs to one nfsd host. Persistent client and lease state can
invalidate an open across the fleet, but it does not make the staged data or
the process-local open state movable to another nfsd. Multiple writable OPEN
sessions may target the same name. REMOVE and RENAME update their publication
targets as described under
[Namespace changes with staged writers](#namespace-changes-with-staged-writers).
The lookup-and-publish sequence for GUARDED and EXCLUSIVE4 creates is
serialized only within one nfsd.

EXCLUSIVE4 retries by the same client and open-owner reuse their staged
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

The current sidecar format is NFS6. It contains the data checkpoint and two
namespace flags, `Unlinked` and `Guarded`. Decoding requires the exact layout
and rejects unknown flag bits. Earlier formats are quarantined rather than
recovered. Drain active writes before changing to a binary that cannot read
the staging format on disk.

The implementation is in [`staging.go`](staging.go),
[`staging_lifecycle.go`](staging_lifecycle.go) and [`ops.go`](ops.go).

## Namespace changes with staged writers

REMOVE and RENAME update staged writers so that a later CLOSE does not
recreate a removed file or publish a renamed file under its old name. These
updates apply only to writers on the nfsd handling the request.

### REMOVE and RENAME

Each writer is either linked or unlinked:

- A linked writer publishes to the directory and name recorded in its sidecar
  on CLOSE.
- An unlinked writer has no publication target. Its handle stays usable for
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

READ, WRITE, COMMIT and SETATTR on a failed writer's handle return
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
continues startup. This includes unreadable sidecars, unsupported formats,
truncate failures and failed quarantine moves.

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
open-owner sequencing, replay information and the association between a
stateid, its client, its file and its access mode.

The persistent store holds a smaller record of active opens and leases. The
process-local state supplies operation ordering and file affinity. The shared
state allows a client reboot confirmed on one nfsd to invalidate old state on
the others.

Repeated read OPENs by one open-owner on the same file share one stateid.
Mutable write sessions use replacement staging. `OPEN_DOWNGRADE` is
unsupported.

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

`client` is a JSON record containing the opaque `SETCLIENTID` identity, the
client verifier, the confirmation verifier generated by nfsd, the normalized
RPC principal and the callback address. The server does not use the stored
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

These updates leave garbage behind in TernFS. A same-directory rename
soft-unlinks the temporary edge and turns the replaced edge into a snapshot
edge, so each lease renewal leaves two snapshot edges and one dead file inode.
Each OPEN and CLOSE marker pair leaves one snapshot edge and one dead inode.
Renewals are bounded to one per half lease for each nfsd and client, but
markers scale with open activity. The shard garbage collector removes this
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

Lease and confirmation slots use a random name per nfsd process. Scans remove
slots which have been expired for a full additional lease, allowing for clock
skew while preventing slots from accumulating across server restarts. Before
removing the last lease evidence, nfsd writes the `expired` marker. An expired
clientid therefore cannot renew or create new open state even after all of its
process slots have been removed.

OPEN markers are create-if-absent records. Reopening or replaying an OPEN which
already owns a marker reuses the existing TernFS inode.

Each nfsd checks its local clients once per lease period. A client with no
live fleet lease loses its process-local open state and local open markers.
Its linked staging is retired, preserving acknowledged bytes without reserving
the pathname or holding open file descriptors. Unlinked staging is discarded;
a writer already retired when unlinked is quarantined. Linked retired staging
is retained until recovered and closed or explicitly removed by an
administrator; provision and monitor disk space accordingly.
The persistent client store is the authority for replaced clientids; the
process-local open store does not keep a separate revoked set.

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
still hold unpublished data. On startup nfsd discovers these files and checks
whether they can be recovered, as described under
[Restart and quarantine](#restart-and-quarantine). An eligible sidecar contains
the state needed to complete the pending `CLOSE` or rebind the staging to a
replacement `OPEN` for the same client and open-owner. Multiple writers
recover independently, including when another writer has published a newer
version in the meantime. Replacements recover only checkpointed dirty ranges,
including new files backed by their empty published inode.

A client reconnecting after lease expiry receives a new clientid. Sidecars
also persist a recovery key derived from its stable client identity, boot
verifier and RPC principal. Matching that key and the open owner allows the
same client boot to reclaim its private filehandle and data after expiry,
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

## Inspecting client state

`nfsd inspect` reads the client store and explains its current state. It uses
the same record definitions and lease rules as the server, but does not
modify the store.

```
nfsd inspect -registry <addr> [-staging <dir>] [-json]
nfsd inspect -registry <addr> -identity "Linux NFSv4.0 host/10.0.0.1"
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
- `PENDING` is waiting for `SETCLIENTID_CONFIRM`.
- `REPLACED` is retained temporarily as a reboot target.
- `UNREACHABLE` has no persistent pointer keeping it live.

Each incarnation includes its client and update records, decoded principal
and callback address, lease and confirming slots, expiry marker, GC deadline,
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

The reported sidecar version is the on-disk metadata layout, printed as `v6`.
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
nfsd processes on the test host. Relative paths are resolved from `go/nfsd/test`.

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
Failed runs retain logs and test data;
`-artifacts-dir DIR` selects their local parent directory. Pynfs should run
serially per existing filesystem because some cases reuse client identities.

The tests need Linux, Go and the selected client dependencies, with no kernel
mount or root requirement. Make builds pinned libnfs 7.0.2 under `test/.deps`;
this needs Git, CMake and a C/C++ toolchain. These suites are not yet run in CI.

### pynfs protocol tests

[pynfs](https://github.com/linux-nfs/pynfs) is the Linux NFS project's
protocol test suite. `test/Makefile` pins release `pynfs-0.5` and uses its
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

The pynfs runner uses the shared process harness and runs Python against its
nfsd subprocess with `--maketree --rundeps`. It reads JSON results so protocol
failures fail the command. Pynfs itself otherwise exits successfully when
individual tests fail. A failed run retains its test tree.

The Make target writes the complete console output to `test/pynfs.out` while also
displaying it. Set `PYNFS_OUTPUT` to use another path. The output file is not
ignored by git, so completed runs remain visible during review.

TernFS intentionally does not implement several POSIX and NFS features covered
by pynfs. The harness uses two mechanisms to exclude those tests:

* The default `PYNFS_TESTS` value uses pynfs flag selectors to exclude broad
  capability classes such as FIFO, socket, GSS and ACL tests.
* The [`pynfs_unsupported.txt`](test/pynfs/pynfs_unsupported.txt) manifest lists
  individual cases requiring unsupported locking, hard links, share
  reservations, metadata changes or access to another writer's private data.
  Using broad `nolock` and `nolink` selectors would also hide useful related tests.

Select flags or individual pynfs test codes with `PYNFS_TESTS`. The manifest
exclusions are appended after `PYNFS_TESTS`, so set `PYNFS_SKIP_FILE=` to
bypass this manifest.

Mutable-write coverage includes `WRT1`–`WRT4` (unstable/stable writes and
zero-length write timestamps), `WRT8` (read-only open state), and
`WRT11`–`WRT12` (stale/old stateids). `OPEN23b` (READ with write-only access),
`OPEN31` (bad OPEN sequence), `RNM19` (rename a file to itself), and `RPLY11`
(replay of a CLOSE with a bad sequence number) are also enabled. `WRT18`
already checks that successive writes update the change attribute.

Immediate empty-file visibility also enables `OPEN3`, `OPEN24`, `OPEN29`,
`RDDR2`, `RDDR3`, `SATT3d`, `SATT6r` and `SATT11r`.

Set additional runner options with `PYNFS_ARGS`. The default Go test timeout is
one hour because pynfs includes lease-expiry cases which deliberately sleep
for several minutes. Override it with `PYNFS_TIMEOUT`:

```sh
make test-pynfs-cluster PYNFS_TESTS='putrootfh getattr'
make test-pynfs-cluster PYNFS_TESTS='GATT1r GATT2' PYNFS_ARGS='--showtraffic'
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
that nfsd's export and runs the suite:

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

That filter selects three tests:

- [`nfs mounted fs`](../terntests/terntests.go) runs the existing `fsTest`
  workload through its `posixHarness`, with the NFS mount as its root. In short
  mode this uses 10 directories and 500 files.
- [`nfs mutations`](../terntests/nfsmutate.go) is the NFS-specific mutation
  suite. It checks create/write/readback, out-of-order writes, rename, delete,
  timestamp updates, existing-file overwrite/truncate, and coherent
  fsync/fstat/append behavior, immediate empty-file visibility, private
  concurrent writers and retained readers.
- [`nfs lease recovery`](../terntests/nfsoutage.go) disconnects the mount through
  a TCP proxy for 100 seconds, exceeding the 90-second NFS lease. It then
  appends and publishes both a new file and an existing-file replacement,
  checking that all data acknowledged before the outage survives.

Each workload starts its own nfsd and mount, then unmounts and stops nfsd
before cleanup. Cleanup deletes the whole TernFS namespace, including `/.nfs`;
a running server cannot retain client-state directory IDs across that deletion.
Mutation sub-step progress is printed to the CI log.

This is the only current test path using a kernel NFS client.
