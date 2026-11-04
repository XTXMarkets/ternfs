<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd design

TernFS file contents are immutable once linked into a directory. nfsd
implements writes by constructing a replacement, staging changes locally,
and publishing on CLOSE. It also maintains NFS sessions and open state.

This document explains how the packages fit together. Each package's
README describes its own mechanisms. [Deployment](DEPLOYMENT.md) defines
the operational guarantees and known defects; [Recovery](RECOVERY.md)
describes what happens when state is lost.

## Constraints

Two TernFS properties shape the design:

1. A file is immutable once it is linked into a directory. nfsd therefore
   presents mutable NFS files by copy-on-write: a writer's changes are
   staged on the nfsd host and published as a new immutable inode on CLOSE.
2. One nfsd serves a tree at a time. It restarts in place on the same
   staging disk, which is the normal recovery, and its address can move to
   another host, which is failover. Staging stays on that daemon's local
   POSIX disk: nfsd turns TernFS, where a file is visible on close, into
   POSIX, where it is visible on write, and only a local POSIX filesystem
   can hold the bytes in between. Two daemons on one tree are not
   supported and nfsd does not detect them: the operator fences the old
   daemon before its address moves; see
   [Replace a host](RECOVERY.md#replace-a-host).

## Packages

| Package | Responsibility | Reference |
| --- | --- | --- |
| [internal/server](internal/server/server.go) | RPC and connections, sessions and slots, compounds and operations, persistent sessions and replay, the logical lease, confirmation, the staging lifecycle, the alias log and the backchannel. | [README](internal/server/README.md) |
| [internal/clientrecords](internal/clientrecords/records.go) | One fsynced record per confirmed client identity in the staging directory. | Package documentation. |
| [internal/openstate](internal/openstate/store.go) | Process-local stateids, opens, tombstones and client views. | [README](internal/openstate/README.md) |
| [internal/staging](internal/staging/store.go) | Staged data files and sidecars on the local disk, checkpoints, startup recovery and quarantine. | [README](internal/staging/README.md) |
| [internal/sessionstore](internal/sessionstore/store.go) | The session file format on the staging disk. | [Session files on disk](internal/server/README.md#session-files-on-disk) |
| [internal/vfs](internal/vfs/vfs.go) | The boundary between NFS and TernFS. | [Backend boundary](#backend-boundary) |
| [inspect](inspect/types.go) | The diagnostics API types and handlers. | [README](inspect/README.md) |
| [internal/nfs4](internal/nfs4/nfs.go), [internal/byterange](internal/byterange/set.go), [internal/keyedlock](internal/keyedlock/locker.go) | NFSv4.1 XDR types, byte-range sets and keyed locks. | Source. |

## State ownership

The staging directory is the server. It holds the server's identity, its
client records, its sessions and its staged writes. A restart on the same
directory, or on a copy of the whole stopped directory, is the same
server. An empty directory is a new server: every session is gone, every
client registers again, unpublished writes are lost and nothing about old
clients is remembered. There is no middle ground. Startup refuses a
directory that has server state but no `server-id`, since that is a
partial copy. nfsd keeps nothing in TernFS except the data. What clients
see in each case is in
[What survives a failure](RECOVERY.md#what-survives-a-failure).

| State | Lives in | Durability and lifetime |
| --- | --- | --- |
| Namespace and published file contents | TernFS | Subject to its policies. |
| Server identity: the server owner and scope | `server-id` in the staging directory | Written once and synced at the first start on an empty directory; as long as the directory. See [server identity](internal/staging/README.md#server-identity). |
| Client records: identity, principal, confirmed incarnation and boot verifier | `clients/`, one file per identity | Replaced through a synced temporary and a rename. From the first confirmation to expiry or destruction. See [Identities and incarnations](#identities-and-incarnations). |
| Staged bytes and sidecars | The staging directory | Sidecars synced at checkpoints. Until published, discarded or quarantined. See [staging](internal/staging/README.md). |
| Sessions, their slot reply caches and namespace intents | `sessions/` | Written without fsync. A restart restores eligible sessions. See [Sessions across restarts](#sessions-across-restarts). |
| Publication aliases | `aliases/published.jsonl` | Not synced; rewritten at each sweep and reloaded at startup while unexpired. See the [alias log](internal/server/README.md#publication-alias-log). |
| Wire clientids and logical leases | Process memory | The process. A restart gives every confirmed record a fresh lease. |
| Opens, stateids, tombstones, client views, the writer index and name hints | Process memory | The process, with writer recovery from sidecars; read opens are reopened by the client. See [openstate](internal/openstate/README.md). |

Wire clientids belong to one process, so a Linux client whose session was
not restored runs server-reboot recovery whatever the records say. The
records decide what that recovery finds: a client that comes back with the
same boot verifier and principal keeps its confirmed incarnation, and with
it its staged writers, and a client without a record is new.

## Filehandles and client views

A filehandle is an eight-byte TernFS inode ID. The server reports
`FH4_PERSISTENT`. A new-file OPEN returns the empty published placeholder's
handle; an existing-file OPEN returns the published inode's handle. The
transient staging inode is separate. FILEID and FILEHANDLE describe the
handle, even when I/O uses staging or another published version.

An open records both its handle inode and its data inode. Real-stateid
operations use the open's data inode. GETATTR, VERIFY and special-stateid
READ use the client's view of the handle instead: its writer, live or
recovered, then a recent publication alias, then its latest open, then the
handle itself. COMMIT has no stateid either, but does not use the view. It
syncs the handle, every staging the client wrote through it, from a
separate writer index, and after a restart the client's recovered writers
of the handle. The view, the writer index, the alias lifetime and what
readers see when a writer closes are in
[openstate](internal/openstate/README.md#client-views). Stateids, owners
and downgrades are there too.

The server also records each publication alias on the staging disk, so a
short process restart does not reverse what a client reads through a handle
it still holds. After a longer outage the alias can have expired, even for a
handle the client still has open. Without aliases, a client that had just
saved read the previous version, or the empty placeholder of a new file,
until it looked the name up again. See the
[alias log](internal/server/README.md#publication-alias-log) and
[old handles](DEPLOYMENT.md#old-handles-name-their-original-version).

Aliases exist so a client sees its own save through the handle it holds.
They are not seen by other clients and do not replace a fresh lookup.

A read `CLAIM_FH` OPEN needs only the inode. A writable one needs a
publication target. A bounded table of 65,536 pathname hints remembers
where LOOKUP and OPEN saw files; each hint is checked against the current
directory entry. A cold miss can use eligible recovered staging, otherwise
it returns `NFS4ERR_STALE`. See
[Writer recovery mechanics](internal/server/README.md#writer-recovery-mechanics).

The hints exist for one case: finding where to publish when a writable
open names a handle after a write has changed the file's inode. They are
not a map of the namespace. READDIR hands out filehandles without filling
them; a handle held across this process's start, a handle the client
learned from READDIR, and a reference taken before the file was replaced
all miss. Eviction is by first insertion, and a hit does not refresh an
entry. An ordinary open on Linux recovers from the STALE by looking the
name up again. A client that reopens without a path walk cannot, and must
mount with `lookupcache=none`; see
[Client mounts](DEPLOYMENT.md#one-writable-server).

Ownership and modes are synthetic.

## Writes and publication

A writable OPEN constructs a transient inode on the target directory's
shard. Creating a pathname also publishes a separate empty inode before
replying. Further writers of that file on the same client and daemon join
the staging; other clients build independent versions.

The staged file is a sparse overlay over the immutable base seen at OPEN.
Its data file and sidecar, named by the transient inode, are described in
[staging](internal/staging/README.md). The first data or size change starts
background hydration of the clean ranges, which yields to foreground reads;
see [Hydration](internal/server/README.md#hydration).

Stable WRITE, COMMIT and size changes sync data before checkpointing the
sidecar. Namespace checkpoints update only namespace fields, preserving the
current durable data checkpoint. See
[Checkpoints](internal/staging/README.md#checkpoints).

The last write CLOSE, or downgrade to READ, finishes hydration, uploads the
whole replacement and links it at the target. There is no background CLOSE.
Earlier closes hand sidecar ownership to another writer of the shared
staging. A clean writer scraps its transient without publication; an
unlinked writer discards its private version, unless another open of its
client still reads it; see
[Reader visibility](internal/openstate/README.md#reader-visibility).

A publication the backend does not answer in time, as a shard restart
causes (the Go client gives a shard 10 s), is neither a success nor a
failure: the link may have applied, in whole or in part, or not at all. The
CLOSE then keeps the open and the staging and answers `NFS4ERR_DELAY`; the
Linux client retries CLOSE after DELAY with a backoff that reaches 15 s, so
`close(2)` blocks for about the outage, as on a hard mount. There is no
limit to the retries while the backend is silent: the lease still runs, and
a client that goes away leaves the writer to lease expiry as any other. A
link refused because the name is locked, and hydration of the base that the
backend does not answer, also answer DELAY.

TernFS appends spans at a file's current size, so a transient that may hold
some of them is never written again. Each staging therefore has at most one
publication candidate, saved in its sidecar: the transient selected for
publication and its data version. A fresh candidate is not submitted if its
checkpoint fails; CLOSE returns DELAY and keeps the staging. A retried CLOSE,
or one recovered after a restart, resolves the candidate first, from
positive evidence only: a linked candidate with the current data version is
the publication, and otherwise the CLOSE links one fresh transient. The
candidate belongs to the staging, not to an open, so whichever CLOSE or
OPEN_DOWNGRADE ends the staging finishes it. A staging that will not be
published scraps its candidate before it goes, so that a link the backend
has not answered cannot land later. The mechanics are in
[Publication](internal/server/README.md#publication).

Timestamp-only publication updates the current published inode without
replacing its contents. A lookup or SetTime the backend does not answer
gives `NFS4ERR_DELAY` with nothing changed; any other error ends the writer,
as a clean CLOSE does, and is returned. After data publication succeeds,
failure to restore timestamps is logged and CLOSE still succeeds because the
data is already visible. A publication whose upload fails, or that the
backend rejects, discards the staging, unless resolving the candidate shows
that another copy of the request linked it. If other opens read the
staging, it is quarantined instead and those readers get `NFS4ERR_IO`. The
client-visible consequences are under
[failed CLOSE](DEPLOYMENT.md#a-rejected-publication-is-not-reported-to-the-application).

Initial CREATE attributes can be staged even for a read-only create OPEN;
the share access still prohibits WRITE. EXCLUSIVE4 and EXCLUSIVE4_1 retries
reuse staging for a matching client, owner and verifier while the name
still contains the original placeholder. The verifier remains in staging
until CLOSE; it is not a reservation across hosts. See
[Exclusive creates](internal/server/README.md#exclusive-creates).

## Writer lifecycle

A staged writer has two independent properties, and can leave the normal
path in two ways:

| Property or state | Meaning | Changed when |
| --- | --- | --- |
| Linked or unlinked | A linked writer publishes to its recorded directory and name on CLOSE. An unlinked one is still usable for I/O, but has no publication target, and CLOSE discards it unless another open of its client still reads it. | OPEN creates a linked writer; REMOVE or RENAME can detach it or move its target. |
| Active or retired | Any writer not marked `Retired` in its sidecar. It need not have an open in this process: after a restart, staging is loaded before any request adopts it. A retired writer keeps its data for recovery, without a pathname reservation or an open descriptor. | The lease sweep retires the writers of an expired client. Before a namespace change, a non-retired writer at the name is checked: it counts as active while a record confirms its client and its lease has not run out, and is otherwise retired; a check that cannot remove the client's record stops the change. Recovery rebinds a writer and clears `Retired`. |
| Failed | Excluded for the process lifetime, with its files moved to quarantine. | A namespace operation has an unknown outcome, or a CLOSE's publication fails while other opens read the staging. |
| Quarantined at startup | Not loaded; its files are moved to quarantine. | Its sidecar is unreadable, `Guarded`, or both unlinked and retired. |

Recovery is in [Writer recovery](RECOVERY.md#writer-recovery). Linked
retired staging is kept until recovered or removed by an administrator. On
expiry an unlinked writer is discarded, and a retired writer that becomes
unlinked is quarantined. Published and discarded writers leave nothing in
staging.

## Namespace changes with staged writers

### REMOVE and RENAME

REMOVE and RENAME update only the writers on the handling daemon:

| Operation | Local writer changes |
| --- | --- |
| REMOVE | Detach every writer at the name. |
| Same-directory RENAME | Source writers follow the new name; destination writers become unlinked. |
| Cross-directory RENAME with an active source writer | Reject with `NFS4ERR_FILE_OPEN` before submitting the operation. |
| Cross-directory RENAME with only retired source writers | Source and destination writers become unlinked. |
| RENAME with no source writers | Destination writers become unlinked. |

Identical source and destination names are a no-op. An active writer cannot
follow a cross-directory rename because its transient inode belongs to the
source shard. OPEN does not set `OPEN4_RESULT_PRESERVE_UNLINKED`, so Linux
normally handles unlink of a busy file through a `.nfs...` silly rename.

### Unknown backend outcomes

| Backend outcome | Writer treatment | NFS result |
| --- | --- | --- |
| Success | Apply the target changes. | `NFS4_OK` |
| REMOVE reports the source edge gone | Detach writers. | `NFS4ERR_NOENT` |
| Unknown outcome with staged writers | Fail and quarantine those writers. | `NFS4ERR_IO` |
| Error without staged writers | No writer bookkeeping. | Mapped backend error |

The UDP client can submit more than one copy. An error for one copy does
not prove another did not apply. Production namespace rejections are
therefore treated conservatively: every error except a REMOVE whose edge is
gone is an unknown outcome, `TIMEOUT` and `DIRECTORY_NOT_EMPTY` included.
The local test backend can classify syscall rejections as known
non-application and leave writers linked. No outcome ends the session: the
status is the operation's reply, recorded and replayed like any other.

A CLOSE's own publication is different: a backend that gives no answer
fails no writer, and the CLOSE is retried after `NFS4ERR_DELAY`, as
described under [Writes and publication](#writes-and-publication). SETATTR
of times on a file without staging, and an OPEN whose `ConstructFile` gets
no answer, also give `NFS4ERR_DELAY`, since running them again is harmless.

Failed-writer tombstones remain after CLOSE so COMMIT, which has no stateid,
cannot acknowledge a failed writer as healthy. They are process-local;
restart changes the write verifier.

### Mutation protocol

Pathname locks serialize namespace changes with OPEN, CLOSE, recovery and
cleanup within one daemon. Before submitting a mutation, the server:

1. Retires inactive writers and captures the affected IDs and intended
   targets under the pathname locks.
2. Scraps each affected writer's publication candidate, so that a link the
   backend has not answered cannot land at the old name later. A scrap the
   backend does not answer gives `NFS4ERR_DELAY` before anything is saved.
3. Looks up the source edge and destination as needed.
4. Checkpoints `Guarded` on affected writers.
5. Submits the operation using the source edge identity.
6. Records the outcome and clears guards when the outcome is known.

A lookup or guard failure stops before submission. The source edge identity
prevents a delayed mutation from applying to a later file at the same name.
It does not make sidecar updates atomic with the backend.

If recording a known outcome fails, the running writer keeps its updated
target and retries the checkpoint on Sync. A surviving disk guard causes
quarantine at startup. The guard has no role in ordering live requests;
the pathname locks provide that ordering. The implementation is in
[ops_namespace.go](internal/server/ops_namespace.go) and
[staging_lifecycle.go](internal/server/staging_lifecycle.go).

## Clients and leases

### Identities and incarnations

EXCHANGE_ID supplies an opaque client identity and boot verifier. One boot
of a client is an *incarnation*: 64 random bits, chosen when it registers.
A registration is pending, in memory only, until its first CREATE_SESSION
confirms it. Confirmation writes the identity's record in `clients/`,
named by the SHA-256 of the identity: the owner, the principal, the
confirmed incarnation and its boot verifier. Later CREATE_SESSIONs create
additional sessions. A record update is a temporary file, fsync, rename
and fsync of `clients/`. Its outcome is known, and the server acts on it,
or replies, only once it is durable. A record lives from the identity's
first confirmation to its expiry or DESTROY_CLIENTID, and a reboot
rewrites it. Nothing else accumulates, so nothing is collected.

An EXCHANGE_ID with the record's principal and boot verifier adopts the
confirmed incarnation and writes nothing. That is how a client whose
session did not survive a restart keeps its incarnation, and with it its
staged writers; it is still answered as a new registration. A changed
verifier gets a pending incarnation, whose confirmation is a
[client reboot](#client-reboot). EXCHANGE_ID's update-confirmed flag reads
the existing record without changing it.

Principal comparison uses credential flavour and normalized AUTH_SYS uid,
gid and groups, ignoring timestamp and machine name. It is a collision
guard. `NFS4ERR_CLID_INUSE` requires a different principal while this
daemon holds a write open or staged writer of the incarnation and its lease
is live; sessions without write state do not pin an identity indefinitely.
The check comes from the record, so it starts afresh with an empty staging
directory, which is what a new server means. The rules are in
[Registration](internal/server/README.md#registration).

### Wire clientids

The wire clientid is separate: a process epoch and counter indexing an
in-memory record. The epoch is chosen after binding the endpoint, in a
later whole second. The counter never wraps. Restarts therefore require
fresh wire clientids, under the endpoint and clock assumptions in
[Recovery](RECOVERY.md#replace-a-host). See
[wire_clientid.go](internal/server/wire_clientid.go).

### The lease

The advertised lease is 90 seconds. The lease is logical and kept per
incarnation in this process's memory; nothing about it is durable. It is
held while any admitted request holds it, and for a quiet period of one
lease after the last hold is released. A request's hold is released when
its worker finishes: after the compound has executed and its reply has
been written, or skipped because the connection had closed. Not when it
arrives, and not when its connection fails: a request still executing
keeps its hold until it returns. A new session and a validated callback
reply also renew it. This process does not expire a client while its
lease is held.

SEQUENCE does no I/O. Apart from a retransmission whose original is still
running, or a slot whose lock is held at that instant (see
[Slot replay](internal/server/README.md#slot-replay)), it gets
`NFS4ERR_DELAY` only while the client's expiry is being decided, which
removes its local record, or while a reboot's confirmation replaces the
incarnation. OPEN and the stateid
operations check in memory that a record confirms the incarnation and that
this process has not expired it.

The server's sweep runs every 45 seconds. A client is expired when its
lease is not held, no CREATE_SESSION for it is in progress, and the
startup lease does not cover it. Its record is removed first, while new
SEQUENCEs and CREATE_SESSIONs wait; then its sessions, wire record and
opens are retired. A crash in between leaves nothing to finish: no record
confirms the incarnation, so the next start restores none of its sessions
and the sweep retires its staged writers. A removal that fails leaves the
client live, and a later sweep tries again. An incarnation that no record
confirms counts as expired.

A restart gives every confirmed record a fresh lease. Nothing it confirms
expires in the first lease after startup, and a client that never comes
back expires after it, so a client that died while nfsd was down gets one
extra lease.

This process is the only one that decides. That rests on the operator
fencing the old server before a failover (DEPLOYMENT.md): nothing in nfsd
stops a second live daemon, on a copy of the same staging directory, from
deciding for the same clients.

Nominal timings follow from these constants. Suppose the hold of a
client's last request here is released at time t, nothing renews it
afterwards, and the scheduler makes ordinary progress. Then the quiet
period ends at t + 90 s, and the next sweep after that expires the client,
by about t + 135 s. After a restart at time s, a confirmed client that does
not come back expires by about s + 135 s. These are derived estimates, not
deadlines the code enforces; a request still executing lengthens them.

Data disposition on expiry is described in
[Recovery](RECOVERY.md#lease-expiry). See
[Lease gate](internal/server/README.md#lease-gate).

### Client reboot

A changed boot verifier is a reboot. A different principal taking over an
identity whose incarnation holds no write state is handled the same way.
The first CREATE_SESSION of the new incarnation N closes admission for the
old one, P, in this process, so P's new requests wait. It writes the record
confirming N, and only once that is durable purges P here: its sessions
and their files, its wire record and its opens go, and its staged writers
are retired. Then it replies. A crash between the write and the purge
leaves nothing to finish: no record confirms P, so the next start restores
none of its sessions and the sweep retires its staged writers. If the
write fails, the CREATE_SESSION gets `NFS4ERR_SERVERFAULT` and P is
reopened.

P's writers are never resumed: no record confirms P any more, and their
recovery key names the old boot verifier. See
[Client confirmation](internal/server/README.md#client-confirmation).

## Sessions across restarts

NFSv4.1 session slots replace open-owner sequence numbers: a retransmission
on a slot gets the slot's recorded reply instead of running again. That
only holds while the slot still exists. Sessions therefore live in memory
and in one file each under `sessions/`, so that a restart of the process
on the same staging directory, or on a copy of it, can rebuild them. The
mechanisms are in
[Persistent sessions](internal/server/README.md#persistent-sessions).

The design keeps one invariant: no recorded reply asserts an outcome the
daemon does not know.

- A completed request's record is written, without fsync, before its reply
  goes to the socket. A process restart can answer a retransmission from
  it; a host crash may have lost it.
- A CLOSE, OPEN_DOWNGRADE, OPEN, CREATE, REMOVE or RENAME that ends a
  compound keeps the locks it took until its record is written, so no other
  request changes what it acted on before a retransmission could find the
  reply; see
  [Locks held until the record](internal/server/README.md#locks-held-until-the-record).
- RENAME, REMOVE and CREATE of a directory or symlink record an intent
  before their effect. A request the earlier process was running when it
  died is decided against TernFS from that intent, or gets
  `NFS4ERR_DEADSESSION` when nothing can prove the outcome; see the
  [replay decision](internal/server/README.md#restart-replay-decision).
- A file can protect one effect per request, so a compound's second
  mutating operation, COMMIT aside, removes the file first, and that
  session is not restored.
- Anything the file does not record, the first CB_COMPOUND and an accepted
  BACKCHANNEL_CTL, removes the file first. CB_NULL does not.
- A mutation whose backend outcome cannot be established does not end the
  session. Its status, `NFS4ERR_IO` when it affects staged writers, is
  recorded like any other reply and asserts nothing about the outcome; see
  [Unknown backend outcomes](#unknown-backend-outcomes).

A restart rebuilds a session only from a usable file that recorded a
request, for an incarnation a live client record confirms, within the new
process's resource limits; see
[Session restoration](internal/server/README.md#session-restoration).
Sessions never move between daemons. `PERSIST` is not granted.

## Client behaviours nfsd depends on

Several mechanisms exist because of how particular clients behave. This
is the list; each entry says where the handling is described.

| Behaviour | How nfsd handles it |
| --- | --- |
| Linux 4.1 opens a cached dentry by `CLAIM_FH` with the handle it holds, and keeps its inode even when the OPEN reply names another. | Name hints for handles this process named, cold recovery of staged writers after a restart, `NFS4ERR_STALE` otherwise. See [Filehandles and client views](#filehandles-and-client-views). |
| A client reopens a file without a path walk, as smbd does through `/proc/self/fd`. | A deployment rule: such a client mounts with `lookupcache=none`. Without it, a refused reopen is `NFS4ERR_STALE`, which smbd reports as access denied. See [Client mounts](DEPLOYMENT.md#one-writable-server). |
| A client keeps its handle after its own save and revalidates it by GETATTR. | Publication aliases while the handle is open and for a lease after; content reversal after that is a [known defect](DEPLOYMENT.md#old-handles-name-their-original-version). |
| Linux discards a CLOSE's status: `close(2)` returns 0 whatever CLOSE answered. | A CLOSE the backend does not answer gets `NFS4ERR_DELAY` and is retried. A rejected publication is still lost silently; see [Known defects](DEPLOYMENT.md#a-rejected-publication-is-not-reported-to-the-application). |
| Linux retries CLOSE after `NFS4ERR_DELAY`, backing off to 15 s. | The CLOSE DELAY relies on it; see [Writes and publication](#writes-and-publication). |
| Linux re-encodes a retransmission with its current highest slot. | The slot digest leaves out SEQUENCE arguments; see [Slot replay](internal/server/README.md#slot-replay). |
| Linux answers `NFS4ERR_DELAY` on a lone SEQUENCE by waiting its 15 s maximum. | SEQUENCE does no I/O. Apart from the slot cases in [Slot replay](internal/server/README.md#slot-replay), it gets DELAY only while the client's expiry decision writes its local record, or a reboot's confirmation replaces its incarnation; see [The lease](#the-lease). |
| After losing its session to a server restart, Linux registers again with the same owner and boot verifier. | The client record adopts the confirmed incarnation, so its staged writers stay eligible; see [Identities and incarnations](#identities-and-incarnations). |
| Linux 4.10 and later end CLOSE, OPEN_DOWNGRADE, REMOVE and RENAME compounds with that operation. | Locks held until the record; see [Sessions across restarts](#sessions-across-restarts). |
| After a server restart Linux retransmits in-flight requests in no particular order, so a read OPEN can arrive before the CLOSE it follows. | The client's recovered writer is its view of the handle; see [client views](internal/openstate/README.md#client-views). |
| Facing a stale clientid or a new server, Linux runs server-reboot recovery: `CLAIM_PREVIOUS`, then reopens. It did so with a changed server owner in every failover test, and with a changed server scope after a wipe, where it skipped `CLAIM_PREVIOUS` and registered as new. | `CLAIM_PREVIOUS` gets `NFS4ERR_NO_GRACE`; reopens go through writer recovery. An empty staging directory is a new server, with a new owner and no client records, and every client registers as new. This is what clients see after a [failover](RECOVERY.md#replace-a-host). |
| libnfs sends PUTFH, COMMIT, CLOSE in one compound. | The mutation gate does not count COMMIT; see [Sessions across restarts](#sessions-across-restarts). |
| libnfs turns `NFS4ERR_DELAY` into an error. | Not handled: libnfs is a test client. |

## Lock order

Locks across packages are taken in this order:

1. Pathname locks, which serialize namespace changes, writable OPEN,
   CLOSE, recovery and cleanup of a target.
2. The openstate file lock, by filehandle, which serializes OPEN, CLOSE and
   OPEN_DOWNGRADE of one file, and a SETATTR of times through the client's
   view; see [openstate](internal/openstate/README.md#locking).
3. Staging locks: the per-inode lock for creation and cleanup, and the
   per-file lock held across that file's disk I/O. The store map lock
   covers indexing only; see [staging](internal/staging/README.md#locking).

These keep OPEN, CLOSE, downgrade and namespace changes ordered within a
process without serializing unrelated files. The CLOSE, OPEN_DOWNGRADE,
OPEN, CREATE, REMOVE or RENAME that ends a compound releases its pathname
and file locks only once its slot record is written; other operations, and
earlier ones, release theirs when they return. The client records
serialise the transitions of one identity with a lock of their own. None
of these locks is shared between processes. Diagnostics try, rather than wait for, locks held across I/O;
see [snapshot consistency](inspect/README.md#snapshot-consistency).

## Backend boundary

[TernVFS](internal/vfs/vfs.go) separates the protocol from the backend.
[RemoteTernVFS](internal/vfs/remote_tern_vfs.go) sends native TernFS
requests through the Go client, which can retransmit them over UDP. The
[local backend](internal/vfs/local_tern_vfs.go) maps the same interface to
a local directory for protocol tests. Its operations are not retransmitted,
so its rejections establish that nothing applied.

Beyond the usual namespace and data calls, the interface has what the
copy-on-write design needs: constructing a transient file, linking it at a
name with its contents, and scrapping it. It also has edge operations that
look up, rename or remove one exact directory edge, so an operation whose
outcome is unknown can be fenced, and a delayed mutation cannot apply to a
later file at the same name.

RemoteTernVFS keeps two process-local caches. The parent map, filled by
Lookup, Readdir and the calls that create or move a name, serves LOOKUPP.
The reader cache keeps one file reader per inode read, so READs reuse span
metadata; a published file is immutable, so a cached reader cannot be
stale. Neither is a record of the namespace: an entry goes when this
process removes the name, or when the cache is full and it is the least
recently used. Overwriting a name drops nothing; the displaced inode's
entries stay until they are evicted. The parent map holds at most 262,144 entries
and the reader cache 4,096. An evicted reader is created again, and an
evicted directory's parent is read again from the backend.
