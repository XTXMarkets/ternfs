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
2. Several nfsd processes may serve the same namespace, and any of them may
   restart. Client lifetimes and lease decisions must therefore be visible
   to every process. Everything else stays local to the process or its
   staging disk.

## Packages

| Package | Responsibility | Reference |
| --- | --- | --- |
| [internal/server](internal/server/server.go) | RPC and connections, sessions and slots, compounds and operations, the logical lease, confirmation, the staging lifecycle and the backchannel. | This document. |
| [internal/clientstore](internal/clientstore/store.go) | The shared client records under `/.nfs`: identities, incarnations, confirmation, open markers, durable leases and collection. | [README](internal/clientstore/README.md) |
| [internal/openstate](internal/openstate/store.go) | Process-local stateids, opens, tombstones and client views. | [README](internal/openstate/README.md) |
| [internal/staging](internal/staging/store.go) | Staged data files and sidecars on the local disk, checkpoints, startup recovery and quarantine. | [README](internal/staging/README.md) |
| [internal/sessionstore](internal/sessionstore/store.go) | The session file format on the staging disk. | [Requests and sessions](#requests-and-sessions) |
| [internal/vfs](internal/vfs/vfs.go) | The boundary between NFS and TernFS. | [Backend boundary](#backend-boundary) |
| [inspect](inspect/types.go) | The diagnostics API types and handlers. | [README](inspect/README.md) |
| [internal/nfs4](internal/nfs4/nfs.go), [internal/byterange](internal/byterange/set.go), [internal/keyedlock](internal/keyedlock/locker.go) | NFSv4.1 XDR types, byte-range sets and keyed locks. | Source. |

## State ownership

| State | Owner and lifetime |
| --- | --- |
| Namespace and published file contents | TernFS; subject to its policies. |
| Client identity, incarnation, lease slots and open markers | Shared TernFS client store under `/.nfs`; see [clientstore](internal/clientstore/README.md). |
| Staged bytes and checkpointed writer metadata | One daemon's staging disk; retained across compatible restarts; see [staging](internal/staging/README.md). |
| Sessions, their slot reply caches and namespace intents | One daemon's staging disk (`sessions/`), written without fsync; a restart on the same host restores eligible sessions. |
| Publication aliases | One daemon's staging disk (`aliases/published.jsonl`); rewritten at each sweep and reloaded at startup. |
| Wire clientids and logical leases | One daemon process. A restored session rebinds to its incarnation, whose lease is published again at startup. |
| Detailed opens and pathname hints | One daemon process, with writer recovery from sidecars; read opens are reopened by the client; see [openstate](internal/openstate/README.md). |

Shared client records let one daemon detect a reboot or expiry established
elsewhere. Local staging does not replace that authority. Conversely, the
shared records contain neither staged bytes nor enough information to
reconstruct general opens. Every process advertises its own server owner;
this is client-lifetime coordination, not a shared NFS session service.

## Filehandles and client views

A filehandle is an eight-byte TernFS inode ID. The server reports
`FH4_PERSISTENT`. A new-file OPEN returns the empty published placeholder's
handle; an existing-file OPEN returns the published inode's handle. The
transient staging inode is separate. FILEID and FILEHANDLE describe the
handle, even when I/O uses staging or another published version.

An open records both its handle inode and its data inode. Real-stateid
operations use the open's data inode. GETATTR, VERIFY and special-stateid
READ use the client's view of the handle instead: its writer, then a recent
publication alias, then its latest open, then the handle itself. COMMIT has
no stateid either, but does not use the view. It syncs the handle, every
staging the client wrote through it, from a separate writer index, and
after a restart the client's recovered writers of the handle. The view,
the writer index, the alias lifetime
and what readers see when a writer closes are in
[openstate](internal/openstate/README.md#client-views). Stateids, owners
and downgrades are there too.

The server records each publication alias, without fsync, in
`aliases/published.jsonl` under the staging directory with an expiry. Each
sweep rewrites the log, giving an alias whose handle is still open a lease
from then. Startup reloads only aliases whose recorded expiry is still in
the future. A short process restart therefore does not reverse what a
client reads through a handle it still holds; after a longer outage the
alias can have expired, even for a handle the client still has open.
Without aliases, a client that had just saved read the previous version,
or the empty placeholder of a new file, until it looked the name up again.
See [old handles](DEPLOYMENT.md#old-handles-name-their-original-version)
and [alias_log.go](internal/server/alias_log.go).

A read `CLAIM_FH` OPEN needs only the inode. A writable one needs a
publication target. A bounded table of 65,536 pathname hints remembers
where LOOKUP and OPEN saw files; each hint is checked against the current
directory entry. A cold miss can use eligible recovered staging, otherwise
it returns `NFS4ERR_STALE`. See [view.go](internal/server/view.go) and
[staging_lifecycle.go](internal/server/staging_lifecycle.go).

Ownership and modes are synthetic. The `/.nfs` namespace is hidden from
LOOKUP and READDIR. PUTFH rejects internal objects whose ancestry is known
locally; after restart, a guessed file inode can escape that check because
TernFS cannot recover its parent from the inode. Client-store records
contain identity and expiry metadata, not authentication secrets.

## Writes and publication

A writable OPEN constructs a transient inode on the target directory's
shard. Creating a pathname also publishes a separate empty inode before
replying. Further writers of that file on the same client and daemon join
the staging; other clients build independent versions.

The staged file is a sparse overlay over the immutable base seen at OPEN.
Its data file and sidecar, named by the transient inode, are described in
[staging](internal/staging/README.md). The first data or size change starts
background hydration of the clean ranges. Across the process at most two
background base reads run at once, and each gives foreground reads up to
10 ms to go first.

Stable WRITE, COMMIT and size changes sync data before checkpointing the
sidecar. Namespace checkpoints update only namespace fields, preserving the
current durable data checkpoint. See
[Checkpoints](internal/staging/README.md#checkpoints).

The last write CLOSE, or downgrade to READ, finishes hydration, uploads the
whole replacement and links it at the target. There is no background CLOSE.
Earlier closes hand sidecar ownership to another writer of the shared
staging. A clean writer scraps its transient without publication; an
unlinked writer discards its private version.

Timestamp-only publication updates the current published inode without
replacing its contents; if that lookup or SetTime fails, CLOSE returns the
error and the staging is kept. After data publication succeeds, failure to
restore timestamps is logged and CLOSE still succeeds because the data is
already visible. A hydration or LinkFile failure discards the staging,
unless other opens read it; then it is quarantined and those readers get
`NFS4ERR_IO`.
The client-visible consequences are under
[failed CLOSE](DEPLOYMENT.md#a-backend-timeout-is-treated-as-a-failure).

Initial CREATE attributes can be staged even for a read-only create OPEN;
the share access still prohibits WRITE. EXCLUSIVE4 and EXCLUSIVE4_1 retries
reuse staging for a matching client, owner and verifier while the name
still contains the original placeholder. The verifier remains in staging
until CLOSE; it is not a reservation across hosts.

The implementation is in [hydration.go](internal/server/hydration.go) and
[ops_open.go](internal/server/ops_open.go).

## Writer lifecycle

A staged writer has two independent properties, and can leave the normal
path in two ways:

| Property or state | Meaning | Changed when |
| --- | --- | --- |
| Linked or unlinked | A linked writer publishes to its recorded directory and name on CLOSE. An unlinked one is still usable for I/O, but has no publication target, and CLOSE discards it. | OPEN creates a linked writer; REMOVE or RENAME can detach it or move its target. |
| Active or retired | Any writer not marked `Retired` in its sidecar. It need not have an open in this process: after a restart, staging is loaded before any request adopts it. A retired writer keeps its data for recovery, without a pathname reservation or an open descriptor. | The lease sweep retires the writers of an expired client. Before a namespace change, a non-retired writer at the name is checked: it counts as active while its durable open marker is active, or a confirmation protects its client, and is otherwise retired; a check that fails stops the change. Recovery rebinds a writer and clears `Retired`. |
| Failed | Excluded for the process lifetime, with its files moved to quarantine. | A namespace operation has an unknown outcome, or hydration or LinkFile fails during CLOSE while other opens read the staging. |
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
therefore treated conservatively. The local test backend can classify
syscall rejections as known non-application and leave writers linked.

Failed-writer tombstones remain after CLOSE so COMMIT, which has no stateid,
cannot acknowledge a failed writer as healthy. They are process-local;
restart changes the write verifier.

### Mutation protocol

Pathname locks serialize namespace changes with OPEN, CLOSE, recovery and
cleanup within one daemon. Before submitting a mutation, the server:

1. Retires inactive writers and captures the affected IDs and intended
   targets under the pathname locks.
2. Looks up the source edge and destination as needed.
3. Checkpoints `Guarded` on affected writers.
4. Submits the operation using the source edge identity.
5. Records the outcome and clears guards when the outcome is known.

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

EXCHANGE_ID supplies an opaque client identity and boot verifier. The
shared store hashes the identity to select a stable parent directory. A
verifier-defined lifetime is a durable *incarnation*, identified internally
by its directory inode. An unchanged verifier can reuse a live confirmed
incarnation; a changed verifier creates a pending replacement, which its
first CREATE_SESSION confirms. Later CREATE_SESSIONs create additional
sessions. EXCHANGE_ID's update-confirmed flag reads the existing record
without changing it.

Principal comparison uses credential flavour and normalized AUTH_SYS uid,
gid and groups, ignoring timestamp and machine name. It is a collision
guard. `NFS4ERR_CLID_INUSE` requires a different principal with live,
marked open state; sessions without opens do not pin an identity
indefinitely. The records and rules are in
[clientstore](internal/clientstore/README.md#registration).

### Wire clientids

The wire clientid is separate: a process epoch and counter indexing an
in-memory record. The epoch is chosen after binding the endpoint, in a
later whole second. The counter never wraps. Restarts therefore require
fresh wire clientids, under the endpoint and clock assumptions in
[Recovery](RECOVERY.md#replace-a-host). See
[wire_clientid.go](internal/server/wire_clientid.go).

### Two leases

The advertised lease is 90 seconds. Each incarnation has two leases here.

- The *logical lease* is this process's, in memory. It is held while any
  admitted request holds it, and for a quiet period of one lease after the
  last hold is released. A request's hold is released when its worker
  finishes: after the compound has executed and its reply has been written,
  or skipped because the connection had closed. Not when it arrives, and
  not when its connection fails: a request still executing keeps its hold
  until it returns. A validated callback reply also renews it. This process
  does not expire a client while its logical lease is held.
- The *durable lease* is this process's `lease.<nfsd-id>` slot in the
  client store, written only by the
  [refresher](internal/clientstore/README.md#refresher): once at
  [bootstrap](internal/clientstore/README.md#bootstrap), after
  CREATE_SESSION or a session restore, and then while the logical lease is
  needed. It is published 135 seconds ahead and refreshed when
  less than 112.5 seconds remain: about every 22.5 seconds while the client
  is active. Failures back off from about 5.6 to 22.5 seconds. The fleet
  lease is live while any process's slot is live.

SEQUENCE reads the refresher's cached proof and performs no backend I/O.
An unproven or expired proof gives `NFS4ERR_DELAY` and requests refresh
work. CREATE_SESSION waits up to two seconds for the first publication.
OPEN also needs a live published lease before creating its marker. Stateid
operations check the confirmed pointer and expired marker; a live local
durable slot avoids reading every slot.

The server's sweep runs every 45 seconds. A client is expired here only
when its logical lease is not held, no confirmation protects it, and it has
durable slots but none is live. A client with no slot yet is not expired;
its first OPEN may still be registering its lease. Only that decision
writes the `expired` marker, and a renewal or hold that arrives while it is
checking wins.
Expiry retires local sessions even if writing the marker fails. Another
daemon can still expire a client whose durable publication lapsed; a local
logical lease cannot prevent that fleet decision.

Nominal timings follow from these constants. Suppose the hold of a
client's last request here is released at time t, its bootstrap
publication is done, and afterwards nothing renews it: no further requests
or callbacks, no confirmation protection and no other process publishing a
lease for it; and the backend and scheduler make ordinary progress. Then:

- the quiet period ends at t + 90 s, and with it the durable refreshes;
- this process's slot therefore expires by about t + 225 s (90 + 135), and
  other daemons can see it live until then;
- the next sweep after that expires the client here, by about t + 270 s.

These are derived estimates, not deadlines the code enforces. A slow
backend, a request still executing, confirmation protection or another
daemon's lease lengthens them.

Another process's dead slot becomes eligible for pruning one further lease,
90 seconds, after it expired. That is not a promise of removal: a slot is
removed only by a refresher pass for a lease that is needed or
bootstrapping, while this process's own published lease is live; see
[slot pruning](internal/clientstore/README.md#expired-marker-and-slot-pruning).
Expired markers prevent slot removal from reviving an old incarnation. Data
disposition on expiry is described in [Recovery](RECOVERY.md#lease-expiry).
See [lease_gate.go](internal/server/lease_gate.go) and
[client_group.go](internal/server/client_group.go).

### Client reboot

Confirmation of a replacement incarnation is in two parts. The client
store makes its writes and decides each outcome from evidence; see
[Confirmation](internal/clientstore/README.md#confirmation). The server
acts on each result once. Before the writes, it closes admission for the
old incarnation in this process, so its new requests wait, and protects
both incarnations.

A replacement becomes confirmed before the old incarnation's opens and
leases are purged. A reboot pointer makes an interrupted purge retryable.
While a confirmation outcome is unresolved, the local server protects both
incarnations from expiry and destructive cleanup, delays dependent
requests and rejects client destruction as busy. Elapsed time alone does
not settle an unknown outcome. See
[confirmation.go](internal/server/confirmation.go).

## Requests and sessions

Each TCP connection has one reader and a default limit of 64 admitted
requests. Capacity is reserved before reading a complete frame. Compounds
run concurrently; operations within each compound run in order. Replies
share a writer lock and use RPC XIDs to match requests.

Read and reply-write timeouts are independently five minutes. A clean EOF
may be a TCP half-close, so accepted requests drain and send their replies
before the socket closes. A read timeout, reset, other read failure or an
RPC that does not parse closes the socket first and wakes requests waiting
for reply work. A write failure also closes the socket. Backend operations
already executing finish their effects but send no reply on the closed
socket. Identical in-flight XIDs on one connection are dropped, while
conflicting reuse closes it. This guard lasts through the reply write.

Each admitted request reserves reply work before it executes: eight times
the sum of its reply capacity and a 65 KiB allowance, plus a preparation
allowance for READDIR. A 1 MiB READ reserves about 8.5 MiB. Reservations
count against `-reply-work-budget-mib`. A request that does not fit waits,
keeping its admission slot, until work is released or its connection
closes. A request larger than the whole budget is refused and its
connection closed. RPC messages are limited to 2 MiB; allowing 128
operations in a compound does not multiply the session's response limit.

Except for the permitted standalone session-management operations, every
compound starts with SEQUENCE. A session negotiates up to 64 slots. Each
slot orders requests and, when requested, caches a reply up to 64 KiB.
Retransmissions return the cached reply, DELAY while work is active, or
RETRY_UNCACHED_REP if the reply was not cached.

Negotiated request, operation and response limits are enforced. With
`sa_cachethis` set, the reply must also fit the negotiated cache limit; it
is not silently served uncached. Each operation's reply bound is checked
before dispatch: an operation that would exceed the remaining response or
cache budget gets `NFS4ERR_REP_TOO_BIG` or `NFS4ERR_REP_TOO_BIG_TO_CACHE`
without executing. Earlier operations in the compound may already have
taken effect. A read-only operation with a variable result can also be
replaced by a size error after its result is built. A mutation that
exceeds its fixed reply bound after executing retires the session and
closes the connection instead. Any connection to the issuing daemon can
carry a session, with at most 64 session associations per connection.

Each session has a file under `sessions/` in the staging directory holding
its identity, its negotiated channel attributes and its slot table: for
each slot the last request's sequence number, a digest of the request, its
effective user and its reply, the whole reply when the client asked for
caching and the SEQUENCE-only prefix otherwise. The record is written
before the reply goes to the socket, by a `pwrite` without fsync. It
survives the death of the process, not necessarily a host crash.

At startup the daemon rebuilds a session only when its file is readable
and of the current format, it recorded at least one request, and its
incarnation is still the confirmed one. The session must also get a lease
record, reply-cache space for its recorded replies, a session record and a
client group; otherwise it is dropped and its file removed. A
retransmission to a rebuilt session gets its original reply (RFC 8881
§2.10.6.5). Before the daemon listens, it starts publishing rebuilt
incarnations' leases and waits up to two seconds for them; a lease not
published by then is logged, and startup continues. A session with no
usable file is unknown to the new process and gets `NFS4ERR_BADSESSION`,
as does one retired later because its incarnation expired. `PERSIST` is
still not granted: the Linux client behaves the same either way, except
that it would switch exclusive creates to GUARDED4 and lose their verifier.

The file does not record the callback sequence, nor a callback program or
credential changed by BACKCHANNEL_CTL. It is therefore removed before a
session's first CB_COMPOUND and before an accepted BACKCHANNEL_CTL; CB_NULL
probes leave it alone. A session without its file goes on in memory and is
not restored by a later restart. If the file cannot be removed, the
callback is not sent or BACKCHANNEL_CTL gets `NFS4ERR_SERVERFAULT`, and the
session is retired.

RENAME, REMOVE and CREATE write an intent to the slot before their effect,
so a request the earlier process was running when it died is decided
against TernFS rather than run blind; see
[client recovery](RECOVERY.md#client-recovery). The file can protect only
one effect per request. Before a compound's second mutating operation of
any kind, OPEN, WRITE and CLOSE included, the file is removed and the
session goes on in memory only. A retransmission of that request after a
restart gets `NFS4ERR_BADSESSION`.

A mutation whose backend outcome cannot be established, every backend
timeout included, retires the client's session as fatal so that no cached
reply can assert an outcome the daemon does not know. The client then
re-creates its session; see the
[known issue](DEPLOYMENT.md#a-backend-timeout-is-treated-as-a-failure).

## Backchannel

CREATE_SESSION can grant a backchannel; BIND_CONN_TO_SESSION associates
further connections within the negotiated direction and quotas. Callback
credentials support AUTH_NONE and AUTH_SYS. Only SP4_NONE state protection
is offered.

CB_NULL probes a new back connection after the binding reply. Its timeout
is ten seconds; until then the path provisionally counts as up for status
flags. CB_RECALL_SLOT, sent in a CB_COMPOUND, asks clients to reduce slot
use under reply-cache pressure. The first CB_COMPOUND of a session removes
its session file, as described above. Clients without a usable backchannel
see the target highest slot in SEQUENCE instead.

Callbacks use RPC version 2, callback program version 1, and NFS minor
version 1. Only one recall is outstanding per session. An unanswered call
is not resent on the same live connection; after disconnect it can retry
on another connection with the same slot and sequence.

Replies are validated before use. Successful CB_SEQUENCE renews the logical
lease; a later recall error does not undo that renewal. DELAY is retried
at most three times. Other invalid replies mark a backchannel fault without
renewing. Clients which never request a backchannel see no backchannel
status flags. There are no delegation callbacks.

See [server.go](internal/server/server.go),
[ops_session.go](internal/server/ops_session.go),
[session_persist.go](internal/server/session_persist.go),
[replay_decide.go](internal/server/replay_decide.go),
[backchannel.go](internal/server/backchannel.go) and
[backchannel_sender.go](internal/server/backchannel_sender.go).

## Lock order

Locks across packages are taken in this order:

1. Pathname locks, which serialize namespace changes, writable OPEN,
   CLOSE, recovery and cleanup of a target.
2. The openstate file lock, by filehandle, which serializes OPEN, CLOSE and
   OPEN_DOWNGRADE of one file; see
   [openstate](internal/openstate/README.md#locking).
3. Staging locks: the per-inode lock for creation and cleanup, and the
   per-file lock held across that file's disk I/O. The store map lock
   covers indexing only; see [staging](internal/staging/README.md#locking).

These keep OPEN, CLOSE, downgrade and namespace changes ordered within a
process without serializing unrelated files. The client store has its own
locks; see [clientstore](internal/clientstore/README.md#locks). The server's
destruction holds keep destructive client-store work away from
incarnations a confirmation protects. None of these locks is shared
between processes. Diagnostics try, rather than wait for, locks held across
I/O; see [snapshot consistency](inspect/README.md#snapshot-consistency).

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
