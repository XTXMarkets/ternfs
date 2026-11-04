<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# openstate

Package openstate is the process-local NFSv4.1 open state: stateids, the
opens they name, expired-stateid tombstones, and each client's view of a
handle. Nothing here is durable. A writer's staging sidecar records its
stateid
([staging](../staging/README.md#sidecars)). The server decides when to open,
close, recover and expire; this package records the result.

[store.go](store.go) holds opens and tombstones, [state.go](state.go) the
per-open record and downgrade rule, and [view_store.go](view_store.go) the
client views.

## Stateids

A stateid's 12-byte `other` field is the process's epoch in the first four
bytes and a counter in the last eight. The epoch is random per process and
never 0 or 1. Its seqid is the open's generation: 1 for a new open, and
advanced by every repeated OPEN, OPEN_DOWNGRADE and CLOSE, skipping zero
when it wraps.

A seqid of zero means the current generation. A smaller nonzero seqid gets
`NFS4ERR_OLD_STATEID`; a larger one gets `NFS4ERR_BAD_STATEID`.

`CanRecover` reports a stateid from another epoch that this process has
neither open nor expired. Only those can name a writer an earlier process
left in staging. An open adopted from such staging with seqid zero takes
its generation from the first nonzero seqid it is presented with.

## Owners and files

An owner is an incarnation and its open-owner bytes. Each owner has at most
one open per filehandle. A repeated OPEN by that owner returns the same
stateid with the next generation, adds its share access and records its
share access and deny values. A write OPEN of an owner's read open upgrades
it in place: the stateid is kept and its I/O moves to the new writer's
staging.

An open records two inodes:

- `FileID` is the filehandle the client holds: the published inode at the
  name when it opened.
- `DataID` is what its I/O uses: a writer's staging inode, or for a read
  open the version chosen at OPEN.

Opens by one client of one handle share the client's page cache, so they
share one staging writer. Session slots order and deduplicate
requests, so open-owners have no sequence numbers and no replay cache. An
owner is not retained after its last open closes.

An open whose CLOSE answered `NFS4ERR_DELAY` is marked closing. libnfs
gives such an open up, so it is not treated as a writer still using the
staging: another writer's CLOSE does not hand it the staging
(`OtherWriter`), a write open of the same client that joins the staging
takes its sidecar over, and the CLOSE that ends the staging expires it
(`ExpireWriters`). A repeated OPEN by its owner clears the mark. A
downgrade that answers DELAY does not set it. See
[Publication](../server/README.md#publication).

## Stateid scope

A stateid is valid only for the incarnation that owns it and the handle it
was opened on. Another client's stateid, live or expired, gets
`NFS4ERR_BAD_STATEID` with no other effect. An unknown stateid gets
`NFS4ERR_EXPIRED` if it is a tombstone of the same client, and
`NFS4ERR_BAD_STATEID` otherwise.

Expiry removes an open and leaves a tombstone naming its owner. At most
1024 tombstones are kept; the oldest is dropped first. CLOSE removes an
open without a tombstone, so a closed stateid presented again as a new
request gets `NFS4ERR_BAD_STATEID`. A retransmitted CLOSE is answered from
the session's reply cache instead.

The server builds on these rules:

- It checks the client before using an open: a client record must confirm
  the open's incarnation, and this process must not have expired it.
  Otherwise the open gets `NFS4ERR_EXPIRED`. The check reads only memory;
  see [The lease](../../DESIGN.md#the-lease).
- It also treats another client's recovered staging as foreign, so its
  stateid gets `NFS4ERR_BAD_STATEID`, timestamp-only SETATTR included.
- TEST_STATEID uses these rules without a filehandle, and also accepts a
  stateid that names this client's recovered staging. It reads no durable
  state.
- FREE_STATEID forgets the caller's own tombstone, gets
  `NFS4ERR_LOCKS_HELD` for a live stateid, returns `NFS4ERR_OLD_STATEID`
  unchanged, and gets `NFS4ERR_BAD_STATEID` for anything else.

## Downgrades

OPEN_DOWNGRADE (RFC 8881 section 18.18.3) takes a share access and deny.
The target must be a subset of the open's access and deny, and equal the
union of the values recorded inside it; otherwise `NFS4ERR_INVAL`. An owner
that opened READ and then WRITE may narrow to either. A single BOTH open may
not. An accepted downgrade drops the recorded values outside the target and
advances the generation.

Share deny is recorded only for this check. It is not enforced.

An open recovered from staging has no recorded values, so it gets only the
subset rule, with deny taken as NONE. Narrowing BOTH to READ publishes the
writer, as CLOSE would, and the open continues as a read open of the
published version under the same stateid. Narrowing BOTH to WRITE drops READ
and keeps the writer. A recovered writer that cannot be adopted, because
its owner already has a live open of the file, gets `NFS4ERR_DELAY` before
anything is published; both opens are kept.

## Reader visibility

These rules decide what a read open of a handle sees when the same client
also writes it. The server applies them under the file lock.

- A read OPEN takes the client's [view](#client-views) of the handle. When
  the client has a writer, live or recovered from the staging disk, that is
  the writer's staging, so the client reads its own writes; the read open
  never owns that staging, and its CLOSE leaves it to the writer. The
  source is chosen again once the file lock is
  held, since a writer's CLOSE may have ended that staging meanwhile. A
  failed staging, or a published version that no longer exists, falls back
  to the handle.
- The open that created a file owns its staging even when it is read-only,
  so its CLOSE publishes a size set at CREATE.
- A clean or timestamp-only CLOSE discards the staging and points every
  open whose I/O used it back at its own handle (`Retarget`). A
  timestamp-only CLOSE whose lookup or SetTime the backend does not answer
  gets `NFS4ERR_DELAY` and changes nothing; any other error ends the
  staging the same way.
- A successful data publication points the staging's opens at what it
  published, before the staging goes: the staging inode itself, or the
  fresh transient a retried CLOSE published through.
- When the publication fails and other opens read the staging, it is
  quarantined: its files are kept under `quarantine/` and it stays failed
  for the life of the process. Those readers get `NFS4ERR_IO`, never the old
  bytes or `NFS4ERR_NOENT`. With no other reader the staging is discarded.
- READ takes no lock. A READ that chose the staging just before a CLOSE
  removed it reads what its stateid resolves to then. If an OPEN upgraded
  that source to a writer, READ checks staging again before using the backend.
- A writer downgraded to READ reads the version its writer published.
- A writer whose name was removed keeps its private version while an open
  of its client reads it. A downgrade to READ goes on reading it, and a
  writer's CLOSE hands it to another read open of the client that is not
  closing (`ReaderOf`), bound to that open as read-only in the sidecar, so
  a restart does not bring back a writer; that open's CLOSE discards it. If
  the binding fails, the readers get `NFS4ERR_IO`. The hand-over does not
  survive a restart.

No open leaves a record of its own. A writer's sidecar is what a restart
recovers from.

## Client views

Operations without a usable stateid, such as GETATTR, VERIFY and READ with a
special stateid, use the client's view of a handle. The view is, in order:

1. the staging of the client's most recent write open of the handle, or a
   writer of its own that the restarted process recovered from the staging
   disk and no request has adopted yet;
2. the client's publication alias for the handle;
3. the data inode of the client's most recent open of it;
4. the handle itself.

The recovered writer counts because after a restart the client's in-flight
CLOSE is retransmitted some time after its first requests reach the new
process, and a read open of the name in that window would otherwise see
the version before the edit.

COMMIT has no stateid either. It uses a separate writer index: the staging
each client wrote through each handle. That index lets COMMIT find a
failed writer's tombstone after its CLOSE. It is not a list of the
client's open writers. An entry lives as long as its staging: while the
staging is live, or failed and kept as a tombstone. The server's sweep
drops entries whose staging has gone, and nothing else evicts, so COMMIT
through a handle reports a failed writer for as long as its tombstone
stands. The index is bounded by the staging it names. After a restart
COMMIT also
syncs the client's recovered writers of the handle, without adopting them;
see [Client recovery](../../RECOVERY.md#client-recovery).

## Publication aliases

When a client's CLOSE publishes a replacement over a handle it holds, the
server sets an alias from that client's handle to the replacement. The
handle then keeps showing the replacement, so close-to-open revalidation
sees the client's own write. Other clients are not affected. A later LOOKUP
finds the replacement's own handle.

An alias's expiry is one lease after publication. A sweep drops an expired
alias only when the client has no open of the handle; while the handle is
open the alias stays. A lookup does not check the expiry, so a closed
handle's alias lasts until the first sweep after it expires: 90 to 135 s
after publication, with the 45 s sweep. After the alias is gone the handle
names its original immutable version again.

Aliases are for one thing: a client seeing its own save through the handle
it holds. Other clients do not see them, and they are not a substitute for
looking the name up.

The server records aliases on the staging disk and reloads the unexpired
ones at startup; see the
[alias log](../server/README.md#publication-alias-log). This package holds
only the in-memory aliases.

## Locking

`Store.mu` guards the open and tombstone maps and is never held across
I/O. `LockFile`, keyed by filehandle, serializes OPEN, CLOSE and
OPEN_DOWNGRADE of one file across all clients, so an OPEN after a publishing
CLOSE sees what it published; a SETATTR of times through the client's view
takes it too, and releases it when it returns. A CLOSE, OPEN_DOWNGRADE or
OPEN that ends a compound keeps it until its slot record is written; see
[Locks held until the record](../server/README.md#locks-held-until-the-record).
Opens of different files run in parallel. A caller that needs the file's
pathname lock takes it before `LockFile`. `ViewStore` has its own mutex.
