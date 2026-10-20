<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd design

TernFS file contents are immutable once linked into a directory. nfsd
implements writes by constructing a replacement, staging changes locally,
and publishing on CLOSE. It also maintains NFS sessions and open state.

This document explains the implementation. [Deployment](DEPLOYMENT.md)
defines its operational guarantees and known defects;
[Recovery](RECOVERY.md) describes what happens when state is lost.

## State ownership

| State | Owner and lifetime |
| --- | --- |
| Namespace and published file contents | TernFS; subject to its policies. |
| Client identity, incarnation, lease slots and open markers | Shared TernFS client store under `/.nfs`. |
| Staged bytes and checkpointed writer metadata | One daemon's staging disk; retained across compatible restarts. |
| Sessions, their slot reply caches and namespace intents | One daemon's staging disk (`sessions/`), written without fsync; a restart on the same host restores eligible sessions. |
| Publication aliases | One daemon's staging disk (`aliases/published.jsonl`); rewritten at each sweep and reloaded at startup. |
| Wire clientids and logical leases | One daemon process. A restored session rebinds to its incarnation, whose lease is published again at startup. |
| Detailed opens and pathname hints | One daemon process, with writer recovery from sidecars; read opens are reopened by the client. |

Shared client records let one daemon detect a reboot or expiry established
elsewhere. Local staging does not replace that authority. Conversely, the
shared records contain neither staged bytes nor enough information to
reconstruct general opens. Every process advertises its own server owner;
this is client-lifetime coordination, not a shared NFS session service.

## Filehandles and client views

[TernVFS](internal/vfs/vfs.go) separates the protocol from the backend.
[RemoteTernVFS](internal/vfs/remote_tern_vfs.go) sends native TernFS
requests; the [local backend](internal/vfs/local_tern_vfs.go) supports
protocol tests.

A filehandle is an eight-byte TernFS inode ID. The server reports
`FH4_PERSISTENT`. A new-file OPEN returns the empty published placeholder's
handle; an existing-file OPEN returns the published inode's handle. The
transient staging inode is separate. FILEID and FILEHANDLE describe the
handle, even when I/O uses staging or another published version.

An open records both its handle inode and its data inode. Real-stateid
operations use the open's data inode. Operations without an open stateid,
including GETATTR, VERIFY, COMMIT and special-stateid READ, use local
bookkeeping. The client's view selects its writer, then a recent publication
alias, then its latest read open, then the handle inode itself. COMMIT has
a separate writer index, including failed-writer tombstones. After a
restart it also syncs the client's recovered writers of the handle, without
adopting them as live opens.

A publication alias remains while that client has the handle open and for
at least one lease after publication. It is recorded, without fsync, in
`aliases/published.jsonl` under the staging directory with an expiry. Each
sweep rewrites the log, giving an alias whose handle is still open a lease
from then. Startup reloads only aliases whose recorded expiry is still in
the future. A short process restart therefore does not reverse what a
client reads through a handle it still holds; after a longer outage the
alias can have expired, even for a handle the client still has open.
Without aliases, a client that had just saved read the previous version,
or the empty placeholder of a new file, until it looked the name up again.
After the alias lapses the handle names its original immutable version; a
fresh pathname lookup finds the replacement. See
[old handles](DEPLOYMENT.md#old-handles-name-their-original-version).

A read `CLAIM_FH` OPEN needs only the inode. A writable one needs a
publication target. A bounded table of 65,536 pathname hints remembers
where LOOKUP and OPEN saw files; each hint is checked against the current
directory entry. A cold miss can use eligible recovered staging, otherwise
it returns `NFS4ERR_STALE`. See [view.go](internal/server/view.go),
[view_store.go](internal/openstate/view_store.go) and
[staging_lifecycle.go](internal/server/staging_lifecycle.go).

Ownership and modes are synthetic. The `/.nfs` namespace is hidden from
LOOKUP and READDIR. PUTFH rejects internal objects whose ancestry is known
locally; after restart, a guessed file inode can escape that check because
TernFS cannot recover its parent from the inode. Client-store records
contain identity and expiry metadata, not authentication secrets.

## Writing and publication

A writable OPEN constructs a transient inode on the target directory's
shard. Creating a pathname also publishes a separate empty inode before
replying. Further writers of that file on the same client and daemon join
the staging; other clients build independent versions.

The staging directory contains pairs named by the transient inode:

```text
<inode>.staging
<inode>.meta
```

The data file is a sparse overlay over the immutable base seen at OPEN.
Dirty ranges select local bytes; clean ranges read from the base. The first
data or size change starts bounded background hydration. Foreground reads
take priority, and hydration installs only ranges still clean.

The sidecar records the publication target, construction cookie, owning
incarnation and stateid, open-owner, recovery key, base inode and size,
checkpointed logical size and dirty ranges, attributes, exclusive-create
verifier and lifecycle flags. Its current format is NFS8; decoding requires
the exact layout and known flag bits.

Stable WRITE, COMMIT and size changes sync data before checkpointing
metadata. The metadata temporary is synced, renamed and followed by a
directory sync. Failed checkpoints remain pending for retry. Namespace
checkpoints update only namespace fields, preserving the current durable
data checkpoint.

The last write CLOSE, or downgrade to READ, finishes hydration, uploads the
whole replacement and links it at the target. There is no background CLOSE.
Earlier closes hand sidecar ownership to another writer of the shared
staging. A clean writer scraps its transient without publication; an
unlinked writer discards its private version.

Timestamp-only publication updates the current published inode without
replacing its contents. After data publication succeeds, failure to restore
timestamps is logged and CLOSE still succeeds because the data is already
visible. Hydration and LinkFile errors have the destructive behaviour
documented under [failed CLOSE](DEPLOYMENT.md#a-backend-timeout-is-treated-as-a-failure).

Initial CREATE attributes can be staged even for a read-only create OPEN;
the share access still prohibits WRITE. EXCLUSIVE4 and EXCLUSIVE4_1 retries
reuse staging for a matching client, owner and verifier while the name
still contains the original placeholder. The verifier remains in staging
until CLOSE; it is not a reservation across hosts.

The implementation is in [internal/staging](internal/staging/store.go),
[hydration.go](internal/server/hydration.go) and
[ops_open.go](internal/server/ops_open.go).

## Namespace operations

A linked writer publishes to its recorded directory and name. An unlinked
writer remains usable for I/O but has no publication target. A retired
writer has lost active protocol state and retains linked data for recovery.

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
source shard. A retired writer becoming unlinked is quarantined.

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

If recording a known outcome fails, the running writer keeps its updated
target and retries the checkpoint on Sync. A surviving disk guard causes
quarantine at startup. The guard has no role in ordering live requests;
the pathname locks provide that ordering.

Failed-writer tombstones remain after CLOSE so COMMIT, which has no stateid,
cannot acknowledge a failed writer as healthy. They are process-local;
restart changes the write verifier. The implementation is in
[staging_lifecycle.go](internal/server/staging_lifecycle.go) and
[ops_namespace.go](internal/server/ops_namespace.go).

## Open state

[internal/openstate](internal/openstate/store.go) holds the association
between stateid, client incarnation, open-owner, handle, data inode and
access grants. An owner has
one stateid per file. Repeated OPEN advances its generation and combines
access; upgrading a read open to write keeps its stateid.

NFSv4.1 session slots handle ordering and retransmissions; open-owner
sequence numbers are not used. A stateid seqid of zero selects its current
generation. The compound's current stateid can substitute for an explicit
one after OPEN, CLOSE or OPEN_DOWNGRADE.

OPEN_DOWNGRADE requires a subset of access and deny equal to the union of
recorded grants. Recovered writers lack the grant history and use the
subset rule, with deny NONE. A downgrade to READ publishes first and keeps
a live read open. Recovery which would collide with a live open of the
same owner gets DELAY before changing either open.

An open is usable only under its owning incarnation's session and with
active durable state. Foreign stateids get BAD_STATEID; the owner can see
EXPIRED. TEST_STATEID follows those rules. FREE_STATEID removes an expired
stateid, reports LOCKS_HELD for a live one and BAD_STATEID for an unknown or
foreign one. A retransmitted CLOSE uses the session reply cache; a closed
stateid presented as a new operation is invalid.

Readers of a client's staging see that client's writes. A clean or
timestamp-only writer CLOSE returns those readers to published data.
Successful data publication leaves them on the published replacement.
Publication failure gives those readers IO rather than silently selecting
old data. General read state does not survive restart.

Share-deny is recorded for downgrade validation but not enforced.
Byte-range locking and delegations are not implemented. OPEN does not set
`OPEN4_RESULT_PRESERVE_UNLINKED`, so Linux normally handles unlink of a busy
file through a `.nfs...` silly rename.

## Client identities and registration

EXCHANGE_ID supplies an opaque client identity and boot verifier. The
shared store hashes the identity to select a stable parent directory. A
verifier-defined lifetime is a durable *incarnation*, identified internally
by its directory inode.

The wire clientid is separate: a process epoch and counter indexing an
in-memory record. The epoch is chosen after binding the endpoint, in a
later whole second. The counter never wraps. Restarts therefore require
fresh wire clientids, under the endpoint and clock assumptions in
[Recovery](RECOVERY.md#replace-a-host). See
[wire_clientid.go](internal/server/wire_clientid.go).

EXCHANGE_ID selects or creates an incarnation. Its first CREATE_SESSION
confirms it; subsequent calls create additional sessions. An unchanged
verifier can reuse a live confirmed incarnation. A changed verifier creates
a replacement. EXCHANGE_ID's update-confirmed flag reads the existing
record without changing it.

Confirmation protects the pending incarnation with a temporary claim. A
replacement becomes confirmed before the old incarnation's opens and
leases are purged. A reboot pointer makes interrupted purge retryable.
While a confirmation outcome is unresolved, the local server protects both
incarnations from expiry and destructive cleanup, delays dependent
requests and rejects client destruction as busy. Elapsed time alone does
not settle an unknown outcome.

Principal comparison uses credential flavour and normalized AUTH_SYS uid,
gid and groups, ignoring timestamp and machine name. It is a collision
guard. CLID_INUSE requires a different principal with live open state;
sessions without opens do not pin an identity indefinitely.

## Persistent client store

```text
/.nfs/clients/<identity-hash>/
    confirmed
    pending
    i.<random>/
        client
        update
        reboot
        o.<stateid>
        lease.<nfsd-id>
        expired
        confirming.<nfsd-id>
        gc
```

| Record | Purpose |
| --- | --- |
| `confirmed`, `pending` | Symlinks selecting confirmed and pending incarnations. |
| `client`, `update` | Identity, boot and confirmation verifiers, and normalized principal; `update` awaits confirmation. |
| `reboot` | Previous incarnation whose state still needs purging. |
| `o.<stateid>` | Empty active-open marker; detailed open state is local. |
| `lease.<nfsd-id>` | One process's durable expiry time. |
| `expired` | Prevents revival of an incarnation after fleet expiry and slot cleanup. |
| `confirming.<nfsd-id>` | Temporary confirmation protection with an expiry. |
| `gc` | Earliest collection time for an unreachable incarnation. |

Immutable records are replaced through temporary files or symlinks and
atomic rename. `pending` may continue to name the confirmed incarnation:
lookup-then-remove could delete another daemon's newer pointer.

Registration schedules asynchronous collection. Confirmed, pending,
reboot-referenced and actively confirming incarnations are retained.
Unreachable incarnations must age for one lease and be rechecked; each
pass removes at most eight. Failures are retried without failing the client
operation. Old `t.*` temporaries become eligible after a lease.

The identity directory and current confirmed incarnation remain until that
identity registers again; there is no whole-store sweep of abandoned
identities. Per-identity locks, per-marker locks and confirmation/destruction
holds order local mutations. Those locks are not distributed pathname locks.
See [internal/clientstore](internal/clientstore/store.go),
[confirm_attempt.go](internal/clientstore/confirm_attempt.go) and
[confirmation.go](internal/server/confirmation.go).

## Leases

The advertised lease is 90 seconds. Each incarnation has a process-local
logical lease, held by active admitted requests and for one lease after
their completion. Successful callbacks also renew it. That process does
not expire the client while the logical lease is held.

A refresher publishes a separate durable lease slot for that process. The
fleet lease is live while any slot is live. It publishes 135 seconds ahead,
refreshing when less than 112.5 seconds remain: roughly every 22.5 seconds
while active. Failures back off from about 5.6 to 22.5 seconds.

SEQUENCE reads the refresher's cached proof and performs no backend I/O.
An unproven or expired proof gives DELAY and requests refresh work.
CREATE_SESSION waits up to two seconds for initial publication. OPEN also
needs a live published lease before creating its marker. Stateid operations
check the confirmed pointer and expired marker; a live local durable slot
avoids a full lease scan.

Lease writes use uniquely named temporary files and exact-edge rename.
An unsettled rename blocks subsequent writes until fenced. A temporary
whose LINK may have applied remains an obligation until its edge can be
found and removed. Absence or elapsed time does not release it. Each record
has at most four such obligations; hitting the bound pauses refresh.
Records themselves are bounded and held while sessions, work or obligations
need them. A backend outage can therefore turn into DELAY rather than
unbounded allocation.

The expiry gate observes durable state only when local holds permit it,
and lets a concurrent renewal win before submitting an expired marker.
Expiry retires local sessions even if writing that marker fails. Another
daemon can still expire a client whose durable publication lapsed; a local
logical lease cannot prevent that fleet decision.

The sweep runs every 45 seconds. With a healthy backend, local expiry can
take roughly three lease periods after the last request; other daemons can
observe a live durable slot for up to roughly 2.5 periods. These are normal
scheduling bounds, not failure-recovery deadlines.

Dead process slots are pruned after an additional lease interval while the
pruning process has a live slot. Expired markers prevent slot removal from
reviving an old incarnation. Data disposition on expiry is described in
[Recovery](RECOVERY.md#lease-expiry). See
[lease_gate.go](internal/server/lease_gate.go) and
[lease_refresher.go](internal/clientstore/lease_refresher.go).

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

Namespace locks precede per-file open locks. Staging has its own file locks;
the store map lock covers indexing, not file I/O. These locks keep OPEN,
CLOSE, downgrade and namespace changes ordered within a process without
serializing unrelated files.

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
