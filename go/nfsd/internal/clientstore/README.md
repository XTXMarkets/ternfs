<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# clientstore

Package clientstore keeps the NFS client records every nfsd process serving
one TernFS namespace shares: client identities and their incarnations,
confirmation evidence, durable lease slots, open markers and collection.
It lives under `/.nfs/clients` in TernFS. The server owns the protocol
decisions and the process-local state built on these records; see
[Clients and leases](../../DESIGN.md#clients-and-leases).

[store.go](store.go) holds the layout, registration, open markers, lease
observation and collection. [confirm_attempt.go](confirm_attempt.go) is
confirmation, [lease_refresher.go](lease_refresher.go) the durable lease
writer, and [store_report.go](store_report.go) the read-only
[store report](../../inspect/README.md#the-store-report).

## Layout

```text
/.nfs/clients/<identity-hash>/
    confirmed               symlink to i.<hex>
    pending                 symlink to i.<hex>
    t.<hex>                 temporary
    i.<hex>/                one incarnation
        client              JSON record
        update              JSON record
        reboot              symlink to ../i.<hex>
        o.<stateid>         empty open marker
        lease.<nfsd-id>     JSON expiry
        confirming.<nfsd-id> JSON expiry
        expired             empty marker
        gc                  JSON collection time
        t.<hex>             temporary
```

| Record | Meaning |
| --- | --- |
| `<identity-hash>` | SHA-256 of the EXCHANGE_ID client owner, as 64 hex digits. One directory per client identity. |
| `i.<hex>` | One incarnation: a client lifetime set by its boot verifier. 32 random hex digits. Its directory inode is the incarnation's durable id. |
| `confirmed` | The identity's current incarnation. |
| `pending` | The incarnation awaiting its first CREATE_SESSION. It may still name the confirmed one; see [Record updates](#record-updates). |
| `client` | The owner, the client's verifier, the confirmation verifier nfsd issued and the RPC principal. The owner is kept for inspection; the directory name is its hash. |
| `update` | An unconfirmed replacement for `client`, written only by the NFSv4.0 path. |
| `reboot` | The incarnation this one replaced, while its state still needs purging. |
| `o.<stateid>` | An open that needs a durable marker, named by its 12-byte stateid as hex. The detailed open state is process-local. |
| `lease.<nfsd-id>` | One process's durable lease expiry. |
| `confirming.<nfsd-id>` | One process's claim on an incarnation it is confirming, with an expiry. |
| `expired` | The fleet expired this incarnation. It cannot be revived. |
| `gc` | The earliest time an unreachable incarnation may be removed. |

An `nfsd-id` is 32 random hex digits chosen when a process opens the store.
The incarnation's directory inode is what the store and the diagnostics API
call an incarnation. It is not the clientid a client sees: the server
issues process-local wire clientids; see
[Wire clientids](../../DESIGN.md#wire-clientids).

## Record updates

Linked TernFS files are immutable, so a record or pointer is replaced by
creating a uniquely named `t.<hex>` temporary and renaming it over the
name. A reader sees either the old record or the new one. If the rename
fails, the temporary is removed if possible; one left behind is collected
later.

Confirmation writes and lease writes use a stricter form. The temporary's
edge is looked up and pinned, and that exact edge is renamed, so a rename
whose outcome is unknown can be fenced by removing the same edge; see
[Writes and fencing](#writes-and-fencing) and
[Write obligations](#write-obligations).

Markers are create-if-absent: an existing marker is kept, inode and all.

After confirmation, `pending` may continue to name the confirmed
incarnation. Removing it by lookup-then-remove could delete a newer pending
pointer another process installed meanwhile. The next registration
replaces it.

## Registration

`ExchangeID` binds an NFSv4.1 EXCHANGE_ID to an incarnation, under the
identity's exclusive lock:

- If the identity has a confirmed incarnation with a different principal
  that has an open marker and a live lease, the call fails with
  `ClientInUseError`, which the server returns as `NFS4ERR_CLID_INUSE`.
  An identity with sessions but no marked open can be taken over.
- If the confirmed incarnation has the same principal and verifier, and no
  `expired` marker, and either no lease slot or a live one, it is adopted
  without any write. This is how the same client boot registering again,
  after a reconnect or a server restart, keeps its incarnation.
- Otherwise a new incarnation is created with its `client` record, and
  `pending` is pointed at it.

The observation only reads. It publishes no expired marker and removes no
lease slot, even when every lease is dead, and a read error is an error,
never evidence of absence.

`ConfirmedRecord` answers EXCHANGE_ID with `EXCHGID4_FLAG_UPD_CONFIRMED_REC_A`
from the confirmed record, without changing anything.

Principals are compared exactly, flavor and body. The server strips the
caller-supplied AUTH_SYS timestamp and machine name first, so uid, gid and
groups decide. This is a guard against identity collisions, not
authentication.

`StagingRecoveryKey` is a SHA-256 over the identity directory inode, the
client's boot verifier and its principal. Staging sidecars record it, and
it survives collection of the incarnation, so the same client boot can
recover its staged data after its incarnation expires; see
[Writer recovery](../../RECOVERY.md#writer-recovery).

`SetClientID` and `ConfirmClientID` are the NFSv4.0 registration paths.
The NFSv4.1 server does not call them.

## Confirmation

Confirming a pending incarnation N over the identity's confirmed
incarnation P takes two calls. `PrepareConfirmation` checks, under the
shared identity lock, that N is pending with the expected confirmation
verifier, and reads P and the inode of the `confirmed` symlink. It writes
nothing. The server then closes admission for P in this process, so new
requests for it wait, and registers protection for N and P before
`CommitConfirmation`; see
[Client reboot](../../DESIGN.md#client-reboot).

### Writes and fencing

`CommitConfirmation` takes the exclusive identity lock and checks again
that N is pending and that `confirmed` is still P through the same symlink.
If either check fails or cannot be read, nothing is written. Otherwise it
makes up to three writes, stopping at the first that does not plainly
succeed:

| Write | Target | Purpose |
| --- | --- | --- |
| A | `N/confirming.<nfsd-id>` | A claim that keeps N from collection, expiring after one lease. |
| B | `N/reboot -> ../P` | Evidence for purging P afterwards. Skipped without a P. |
| D | `confirmed -> N` | The commit. |

Each write creates a temporary, pins its edge and renames that exact edge.
A rename that returned an error is fenced by removing the same temporary
edge:

- The removal succeeds: the write can never apply.
- The edge is already gone: the write may have applied.
- The removal fails or never returns: the outcome stays unknown.

### Observation table

Once every fence has settled, `confirmed` is read back and the attempt is
classified:

| Read back | Result |
| --- | --- |
| `confirmed` names N | Committed, whether through D or by another process. |
| D never applied, and `confirmed` is still P through the same symlink | Rolled back: P is unchanged. |
| Anything else, including no `confirmed` | Conservative: neither committed nor provably unchanged. |
| A fence or the read-back failed | Unresolved. |

Missing data never proves a rollback. An unresolved attempt keeps all its
evidence. `SettleConfirmation` retries its fences and read-back and never
writes again. A settled result is never recomputed. Elapsed time does not
settle anything.

### Replacement purge

`FinishReplacement` completes a committed attempt. It purges P's open
markers and lease slots only if write B applied or may have, and N's
`reboot` symlink, read through its exact edge, still names P. It then
removes that edge and N's confirming claim. If the purge fails, the
`reboot` symlink stays as evidence.

`ResumeReplacementPurge` finishes such a purge later from durable evidence
alone: `confirmed` names N and N's `reboot` names another incarnation. It
needs no attempt, so it works after a restart or in another process. It
holds the server's destruction holds on N and P through the purge, and a
confirmation in progress defers it with `ErrPurgeProtected`.

These steps order work within one process only. The identity lock is
local, and another process's writes are not atomic with these reads.

## Open markers

The server records an open with `MarkOpen`, which creates `o.<stateid>`,
or `NoteOpen`, which creates nothing. Both require a confirmed incarnation
whose lease record has a published lease in the future. Otherwise they get
`NFS4ERR_DELAY` while the lease is read again, `NFS4ERR_STALE_CLIENTID` for
a replaced incarnation, or `NFS4ERR_EXPIRED` once it is fleet-expired.
Which opens are marked is the server's rule; see
[Reader visibility](../openstate/README.md#reader-visibility).

`HasOpen` reports a marked open active: its marker exists, its incarnation
is still confirmed and its lease is live. The fast path, for an open this
process recorded, checks the `confirmed` pointer and, while this process's
own published lease is in the future, only the `expired` marker. Otherwise
it reads the marker and every lease slot. `HasLocalOpen` makes the same
client checks for an unmarked open. `RemoveOpen` removes a marker.
`MarkerGone` reports a marker definitely absent.

`NFS4ERR_CLID_INUSE` and the store report see only marked opens.

## Durable leases

The two-lease model and its timings are in
[Two leases](../../DESIGN.md#two-leases). This section is the durable half.

### Refresher

Each incarnation that needs one has a `LeaseRefresher` record and at most
one worker goroutine. The worker is the only writer of this process's
`lease.<nfsd-id>`. Everything else reads the record from memory and asks
the worker for work. Each pass holds the incarnation's `LeaseLocks` entry
and:

1. settles any unsettled write and consumes obligations whose edges it can
   find;
2. reads evidence without writing: this process's lease, the `expired`
   marker and the `confirmed` pointer;
3. refreshes and prunes, or runs the terminal steps.

A record is live, stale (`confirmed` names another incarnation) or
fleet-expired (an `expired` marker exists). A stale or fleet-expired record
runs its terminal steps: it removes this process's lease through its edge,
best effort, and has the server retire the incarnation's sessions, both
under a destruction hold. While a confirmation protects the incarnation the
hold is refused, and the steps are retried after a backoff.

The server supplies the need condition, `LeaseNeed`: a request still
holding the logical lease, or a quiet period still running; see
[Two leases](../../DESIGN.md#two-leases). A `Store` used on its own as a library,
without the server's hooks, as the package tests do, has no workers: a pass
runs inline in the caller that needs the lease instead. This is not
`nfsd -store-only`, which opens the store with `OpenReader` and only reads
it for the store report.

### Bootstrap

After CREATE_SESSION commits, and for each incarnation whose sessions are
restored at startup, the server calls `StartLeaseBootstrap`. The record
then publishes once even before any request needs it. Bootstrap is cleared
only by a publication or a terminal state. `AwaitLeasePublished` waits, up
to `BootstrapWait` (two seconds), for a published lease in the future;
CREATE_SESSION uses it so that a client's first SEQUENCE does not get
`NFS4ERR_DELAY`.

### Publication schedule

A refresh is due when the published lease is less than 112.5 seconds
ahead. A publication writes an expiry 135 seconds ahead (`LeaseWriteTerm`),
always later than the previous one. While the lease is needed the worker
passes every 22.5 seconds, or sooner when a refresh falls due, so an active
client is republished about every 22.5 seconds.

A failed pass backs off from 5.625 seconds, doubling to 22.5 seconds. No
settlement or write runs before the backoff deadline, whatever wakes the
worker. A pass before the maintenance deadline is a verify: it only reads,
at least one second after the previous pass. Requests can bring a pass
forward only when it is allowed; they cannot drive passes at request rate
or cut a backoff short. An idle worker waits up to a day and is woken for
work.

### Write obligations

A lease publication makes a uniquely named temporary with `ConstructFile`,
writes and links it with `LinkFile`, pins its edge, and renames that exact
edge over `lease.<nfsd-id>`.

Each temporary is an obligation from before its first backend call until
its edge is consumed: by the rename, by the fence of an unsettled rename,
or by removing the edge found pointing at it. A `LinkFile` that fails has
an unknown outcome and is never retried. An obligation is not released
because its temporary is absent or because time has passed. Settling an
obligation never publishes it as the lease.

A rename that returned an error leaves the record's one unsettled attempt.
No later publication starts until it is fenced.

### Record bounds

At most 4096 records exist (`MaxSessionRecords`). Each record has at most
four open obligations, and the process at most four per record credit.
At either cap refresh pauses until an obligation settles. When the
published lease then lapses, lease users get `NFS4ERR_DELAY` rather than
the process allocating without bound.

A record is freed only when all of these hold: no reservation, no
bootstrap, no unsettled attempt or obligation, no pending verify, no call
in flight, and no need; and either it is live with no sessions here, or it
is terminal and its terminal steps have finished. A finished terminal
record with a reservation or an obligation is still kept. The server's
sweep, or the release of the last reservation, has the worker free an idle
record without a store call.

### Expired marker and slot pruning

With a server, only the server's expiry decision writes `expired`; see
[Two leases](../../DESIGN.md#two-leases). A `Store` used on its own,
without the server's hooks, writes it when it observes a dead lease, and
removes old slots, as part of the same observation. The read-only store
report never does either.

Another process's lease slot becomes eligible for pruning once it has been
expired for one further lease, 90 seconds. Eligibility is not removal. A
slot is removed only by a maintenance pass for a lease that is needed or
bootstrapping, after its refresh succeeds or is found not yet due, while
this process's own published lease is live, under a destruction hold.
Verify passes never prune, so an eligible slot can remain for as long as
no such pass runs. This process's own slot is removed only by the terminal
steps, after a marker. The `expired` marker stops an incarnation whose
slots have all been removed from acquiring a new lease.

## Collection

`CollectStaleForClientResult` collects one identity. It holds the server's
collection hold and the exclusive identity lock. The server runs it in the
background after EXCHANGE_ID and confirmation, and again from its sweep for
identities that asked for a recheck.

1. The roots are `confirmed`, `pending` and the `reboot` targets of those
   two. A rooted incarnation loses any `gc` record.
2. An incarnation with a live confirming claim is kept and rechecked.
3. An unreachable incarnation is first given a `gc` record one lease in
   the future. This ages the observation that it is unreachable, not the
   incarnation.
4. Once that time has passed, the roots are read again. A still
   unreachable incarnation is removed: its entries, its `gc` record, then
   its directory.

Each pass removes at most eight incarnations. Temporaries at least one
lease old are removed from the identity and incarnation directories;
younger ones may belong to a call in flight in another process. Failures
are retried by a later pass and never fail the client's request.

The identity directory and its confirmed incarnation stay until the same
identity registers again. There is no sweep of abandoned identities.

## Locks

| Lock | Scope |
| --- | --- |
| Identity lock | Per identity directory, reader-writer. Registration, confirmation commit and settlement, replacement purge and collection take it exclusively. Open marker creation and removal, confirmation preparation, lease observation and the slow path of `HasOpen` share it. A waiting exclusive holder blocks new shared holders, so a collection can briefly delay new OPENs for that identity. |
| Marker lock | Per open marker. Orders a marker's creation, removal and lookup with its cached entry, while unrelated markers proceed. |
| `LeaseLocks` | Per incarnation, held for a refresher pass. A server's terminal steps take the identity lock under it. |
| `RefreshMu`, then a record's `Mu` | The record map and each record's fields. `Mu` is never held across I/O. |
| Attempt mutex | Serializes settlement and purge of one confirmation attempt, taken before the identity lock. `CommitConfirmation` takes the identity lock first, then the mutex of the attempt it is creating, which nothing else can see yet. |

Destruction holds (`BeginMarker`, `BeginCollect`, `BeginPurge`) are the
server's. They keep marker publication, collection, pruning and purges away
from incarnations a confirmation protects. They are nil in a `Store`
without a server, including the read-only store `nfsd -store-only` opens.

All of these are local to one process. They do not order another nfsd's
writes.

## Garbage in TernFS

Every replacement leaves the old record behind as history, and every
marker removed leaves a dead file. Lease renewals and write-open markers
therefore accumulate snapshot edges and dead inodes under `/.nfs` until the
shard garbage collector removes them under that directory's snapshot
policy. Give `/.nfs` a short policy; see
[Operating requirements](../../DEPLOYMENT.md#operating-requirements).
