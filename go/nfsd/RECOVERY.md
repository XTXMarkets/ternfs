<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Recovering nfsd

Recovery depends on three separate stores: published data and client records
in TernFS, staging on the nfsd disk, and protocol state in process memory.
Only the first two survive a process crash. Session files and publication
aliases on the staging disk let a restarted daemon rebuild part of the
third. The
[deployment modes and known defects](DEPLOYMENT.md) define the limits of
the procedures here.

## What survives a failure

| Event | Published data | Unpublished staging | Client protocol state |
| --- | --- | --- | --- |
| Connection loss, daemon still running | Unchanged. | Remains on the daemon. | Sessions can be reused while live; lease expiry can retire opens. |
| Daemon crash, staging disk retained | Remains in TernFS. | Eligible checkpoints can be recovered. | Eligible sessions, their cached replies and namespace intents, and publication aliases, are restored from the disk; see [sessions](DESIGN.md#requests-and-sessions). Read opens are not: their stateids get `BAD_STATEID` and the client reopens. |
| Host replacement with its staging disk | Remains in TernFS. | Same recovery scope as a daemon restart. | The new daemon restores from the same files, under the same conditions. Records written without fsync may be missing after a host crash. Clients must reach it at the old address to use them. |
| Staging disk lost or unavailable | Remains in TernFS. | Unavailable, including acknowledged stable bytes. | Shared client records do not reconstruct the missing data. |
| Client reboot | Remains in TernFS. | Linked old staging may be retained, but the new client boot cannot adopt it automatically. | Confirmation of a new incarnation invalidates the old one across daemons. |

No restart publishes all leftover staging automatically. The client must
recover an eligible writer and close it. A successful mount alone proves
neither that old descriptors recovered nor that pending writes were published.

## Client recovery

A reconnect to the same running daemon reuses its session and cached
replies. A reconnect after a restart can too, when the new daemon rebuilt
the session from its file on the staging disk. That needs a usable file
that recorded a request, an incarnation that is still the confirmed one,
and room for the session in the new daemon; see
[sessions](DESIGN.md#requests-and-sessions). Before it listens, the new
daemon starts publishing rebuilt incarnations' leases and waits up to two
seconds for each; if one is not published by then, it logs a warning and
starts anyway.
The Linux client, which learns nothing of the restart, retransmits every
request that was in flight with the same slot and sequence number and goes
on using its stateids. A retransmission of a completed request is answered
from the recorded reply.

A slot holding an intent but no reply belongs to a request the earlier
process was running when it died. The request runs again, and the
operation the intent names is decided against TernFS first:

| Intent | Decision |
| --- | --- |
| RENAME | Source edge still in place: not applied, so it runs now. Destination holds the source inode and the source edge is gone: applied, so it reports success without running again. Anything else: `NFS4ERR_DEADSESSION`. |
| REMOVE | Edge still in place: not applied, so it runs now. Edge gone or replaced: applied. |
| CREATE | Always `NFS4ERR_DEADSESSION`: nothing shows whether it made what is, or is not, at the name. |

An operation on other names or directories than its intent's, any mutating
operation before it in the compound, and every mutating operation when the
intent cannot be read also get `NFS4ERR_DEADSESSION`. A backend lookup
error while deciding returns that error, mapped to NFS. After
`NFS4ERR_DEADSESSION` the client re-establishes its session.

While a session's file is on disk, each of its compounds may run at most
one mutating operation. Before a second one of any kind runs, OPEN, WRITE
and CLOSE included, the file is removed and the session goes on in memory
only, so a retransmission of that request after a restart gets
`NFS4ERR_BADSESSION`. A restored slot accepts a later sequence number than
it recorded once, since the record of a later request may not have been
written.

A session with no usable file is unknown to the new daemon, and so is one
retired after the restart because its incarnation expired: SEQUENCE and
BIND_CONN_TO_SESSION receive `NFS4ERR_BADSESSION`, CREATE_SESSION with the
old wire clientid `NFS4ERR_STALE_CLIENTID`, and EXCHANGE_ID and
CREATE_SESSION establish new wire state, reusing the durable incarnation
when it is still eligible.

There is no NFS reclaim grace period. OPEN with `CLAIM_PREVIOUS` receives
`NFS4ERR_NO_GRACE`; RECLAIM_COMPLETE is accepted without reconstructing
opens. Clients may then reopen by handle or pathname. Whether a particular
application descriptor resumes depends on that recovery path.

Read stateids receive `NFS4ERR_BAD_STATEID`: read opens leave no durable
marker, and the Linux client reopens the handle with CLAIM_FH. A writer
whose sidecar still proves ownership continues with its old stateid. READ,
WRITE and SETATTR with that stateid adopt the recovered staging as a live
open. When adoption is refused, because the staging is read-only or the
owner already has a live open of the file, the request runs on the state
recorded in the sidecar. OPEN_DOWNGRADE also adopts, but does not fall back
to the sidecar: if the owner already has a live open of the file, it gets
`NFS4ERR_DELAY` before any publication, and both opens are kept. CLOSE
closes the writer through its sidecar. COMMIT carries no stateid: it syncs
the client's recovered writers of the handle without adopting them. A new
read OPEN selects the held inode or this daemon's current view of it,
including a restored publication alias.

The write verifier changes at every daemon restart. Clients can use it to
detect loss of unstable writes. Stable writes may already have left the
client's retransmission buffers.

An operation submitted on a new session, after a session the client lost,
is a new request: a RENAME or REMOVE the old daemon applied without
answering gets `NFS4ERR_NOENT` on that retry. On a restored session the
same retry is answered from the slot, which is what the session files are
for.

## Writer recovery

There are three paths, with different eligibility rules:

| Path | Conditions |
| --- | --- |
| Continue with the old stateid | Sidecar matches the stateid and client incarnation, and the client presents the staging inode or the sidecar's base inode as its handle; its durable open marker and lease remain active. For a file the earlier process created, the base is the empty placeholder, which stays the client's handle after the name is renamed or removed. The stateid alone does not find the staging. |
| Reopen by pathname | A unique eligible staging entry matches the target and access. Within the same incarnation, another open-owner can adopt shared staging. |
| Writable `CLAIM_FH` on a cold daemon | A unique healthy, linked writer matches the client and recorded owner, and its recorded pathname still names the held handle. |

If the old incarnation expired, recovery into a new incarnation additionally
requires the sidecar's recovery key and original open-owner. The key
identifies the stable client identity, boot verifier and normalized RPC
principal. A rebooted client, another principal or another client cannot
adopt those bytes. The key survives collection of the old incarnation.

Recovery by pathname can retain an older writer's private version even
after another writer published. Cold recovery by handle cannot do that
when the name no longer contains the held inode. An unlinked or quarantined
writer is also excluded from cold `CLAIM_FH` recovery. No candidate gives
`NFS4ERR_STALE`; ambiguous candidates give `NFS4ERR_EXPIRED`. These responses
do not guarantee that the client can repair an existing descriptor by
looking up a name.

The base and transient TernFS inodes must still exist. Retaining staging
does not extend TernFS snapshot or transient-file retention. Also account
for the [CLOSE defect](DEPLOYMENT.md#a-backend-timeout-is-treated-as-a-failure)
before relying on recovered writes. A CLOSE whose hydration of the base
fails, or whose LinkFile fails without the file turning out to be
published already, discards its staging. If other opens were reading that
staging, it is quarantined instead, and those readers get `NFS4ERR_IO`. A
timestamp-only CLOSE whose lookup or SetTime fails returns the error
without discarding it.

## Lease expiry

The advertised lease is 90 seconds. This is not a deadline at which every
idle writer disappears: the daemon has a logical lease and the fleet has
durable lease slots, with a periodic sweep. See
[Two leases](DESIGN.md#two-leases).
Client-store errors generally produce `NFS4ERR_DELAY` and defer recovery or
expiry while the outcome is unresolved.

On expiry, linked staging is retired. Its acknowledged data remains on
disk, but it releases file descriptors and its pathname reservation.
It stays until recovered and closed or explicitly removed. Unlinked staging
is discarded on expiry. Staging already retired when it becomes unlinked
is quarantined.

For the first 90 seconds after startup, the staging sweep does not retire
recovered writers. This is a local cleanup delay, not NFS grace-period
reclaim. Linked retired data can remain recoverable after this interval.

## Startup and quarantine

Startup reads each `.staging`/`.meta` pair. The current sidecar format is
NFS8. Recovery restores checkpointed size and dirty ranges; bytes outside
the checkpoint cannot be assumed committed.

Interrupted REMOVE/RENAME operations leave a `Guarded` sidecar. Startup
quarantines it without guessing whether the backend operation applied.
Unreadable or malformed metadata, and sidecars both unlinked and retired,
are also excluded and moved to quarantine where possible. Individual
recovery failures are logged and startup continues. A recognized older
`NFSn` format instead fails startup.

The running daemon also quarantines writers when a submitted namespace
operation has an unknown outcome. READ, WRITE, COMMIT and SETATTR through
a failed writer report `NFS4ERR_IO`; CLOSE reports `NFS4ERR_EXPIRED`.
Other writers of the client remain usable.

Quarantine retains the data and sidecar beneath `quarantine/`. A failed
move can leave the pair split between directories. Keep both parts. The
mechanism is in [staging](internal/staging/README.md#startup-recovery-and-quarantine).
A CLOSE whose hydration or LinkFile fails quarantines its staging only
when other opens read it; otherwise the staging is discarded. A
timestamp-only CLOSE that fails keeps its staging; see
[Writer recovery](#writer-recovery) and
[failed CLOSE](DEPLOYMENT.md#a-backend-timeout-is-treated-as-a-failure).

## Inspect stored state

The [diagnostics API](inspect/README.md) reads the durable client store and
the staging sidecars without changing them. On a running daemon, use its
`-status-addr`. When the daemon will not start, or should not be disturbed:

1. On the host which owns the staging disk, start a
   [store-only](inspect/README.md#store-only-mode) process against the same
   backend. It never opens the staging store, so startup recovery does not
   quarantine or truncate anything. It runs in the foreground until
   stopped, so run the following steps from another terminal.

   ```sh
   nfsd -store-only -registry registry.example:55556 -staging /var/lib/nfsd \
       -status-addr 127.0.0.1:9049
   ```

2. Save a whole-store report with the sidecars joined, then read it from
   the saved file:

   ```sh
   curl -sf -o store-report.json 'http://127.0.0.1:9049/v1/store?staging=1'
   jq .report.summary store-report.json
   ```

3. Narrow it to the affected client with one
   [selector](inspect/README.md#selectors), for example:

   ```sh
   curl -s 'http://127.0.0.1:9049/v1/store?staging=1&identity=Linux%20NFSv4.1%20host' | jq .report
   curl -s 'http://127.0.0.1:9049/v1/store?staging=1&stateid=stateid:09cb5ee40000000000000001' | jq .report
   ```

   An `incarnation` selector takes the durable incarnation directory inode,
   not the wire clientid a client sees. Only the process that issued a wire
   clientid can resolve it, with [lookup](inspect/README.md#lookup).

4. Read `summary.problems` and find each problem where it was recorded; see
   [problems and notes](inspect/README.md#problems-and-notes). Quarantined
   files count as problems. Retired, unlinked and guarded staging entries do
   not, so account for them separately: a count of zero does not mean all
   staged writes were published or recovered.

The report shows checkpointed state only. In-memory writes, hydration
progress and session state are absent; on a running daemon, `/v1/staging`
and `/v1/sessions` show them. A report taken while the daemon runs is an
observation, not an atomic backup. Lease slots in the report name the
`nfsd_id` that wrote them, and a stateid's first four bytes are the issuing
process's `stateid_epoch`. Match these against startup logs or a serving
daemon's [process identities](inspect/README.md#process-identities) to find
that daemon; a store-only process has none of its own.

## Restart on the same host

1. Establish that the old process has stopped. For a planned restart,
   quiesce applications and close writers first.
2. Preserve the staging directory and sidecars. After an abnormal failure,
   retain a consistent copy before attempting recovery if the data needs
   manual investigation.
3. Start a compatible binary against the same TernFS backend, staging
   directory and listening address.
4. Inspect startup exclusions and quarantine. Check affected applications
   and the published contents after their recovery and CLOSE.

A failed CLOSE may already have discarded the data. Repeatedly restarting
does not restore missing staging or make quarantined targets unambiguous.

## Replace a host

1. Fence the old daemon so it cannot accept requests or later publish old
   staging. Do this before giving a replacement its disk or address.
2. If the staging disk survives, preserve and attach it with its sidecars.
   Use the same backend and a compatible binary. If it does not survive,
   treat unpublished data as unavailable.
3. Arrange the client destination separately from recovering the data.
   Existing Linux NFS mounts normally retain the resolved IP address.
   Changing DNS affects new resolution; it does not move those mounts.
   A hard mount can keep waiting on the dead address.
4. For existing mounts, provide the address they already use. Otherwise
   plan application shutdown and fresh mounts. Mounting elsewhere does not
   move descriptors held on the old mount.
5. Check recovered writers, quarantine and published contents as for an
   in-place restart.

The server derives wire-clientid epochs from wall time after binding its
socket. Clocks must not go backwards across successive owners of one
endpoint. A successful bind on another host is not proof that the old owner
was fenced. There is no automatic disk transfer, address takeover or
fencing service in nfsd.

## Recover retained data manually

Stop mutations to the affected staging before copying or editing it.
Preserve the data, sidecar, logs, target identity and any quarantine
directories. Do not clear `Guarded` or rebind a sidecar simply because its
old pathname currently exists: that does not establish which operation
applied or who now owns the name.

A staging file is a sparse overlay. Its clean ranges may exist only in the
immutable base inode named by the sidecar. Copying `.staging` alone can
produce an incomplete file. Reconstruction needs the checkpointed size and
dirty ranges, the base where required, and an explicit decision about where
to publish the recovered contents. Do this before the required TernFS
inodes are collected.

The diagnostics API is read-only; it does not reconstruct or publish files.
There is no general automatic repair command for quarantined staging.

## Upgrade the server

Drain writes and close their descriptors before stopping the old binary.
Inspect remaining staging, including retired and quarantined entries.
Preserve or recover unpublished data before changing formats.

A binary which cannot read the existing sidecar format may fail startup.
The current format is NFS8; older NFS5, NFS6 and NFS7 sidecars are not
automatically converted. Use a compatible binary to finish or recover them.
Remove staging only after its data has been accounted for. It can contain
bytes already acknowledged by fsync.
