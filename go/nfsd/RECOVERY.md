<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Recovering nfsd

Recovery depends on three places: published data in TernFS, the staging
directory on the nfsd disk, and protocol state in process memory. The
staging directory is the server: its identity (`server-id`), the client
records (`clients/`), the staged writers, the session files (`sessions/`)
and the publication aliases (`aliases/`). nfsd keeps nothing about clients
in TernFS. A process crash loses only the memory, and the session files
and aliases let a restarted daemon rebuild part of it. The
[deployment modes and known defects](DEPLOYMENT.md) define the limits of
the procedures here.

## What survives a failure

Published data stays in TernFS in every case.

| Event | Sessions | Client records | Staged writers | What clients see |
| --- | --- | --- | --- | --- |
| Connection loss, daemon still running | Kept while live. | Kept. | Kept. | The client reconnects and goes on with its session. Lease expiry can retire its opens if it stays away. |
| Restart in place | Restored from `sessions/` when eligible; see [session restoration](internal/server/README.md#session-restoration). | Loaded. | Recovered from sidecars. | The client goes on with its session, retransmissions get their recorded replies, and writers resume through their old stateids. Read opens get `BAD_STATEID` and the client reopens them. |
| Failover with a copy of the stopped daemon's staging directory | Restored. | Loaded. | Recovered. | The same as a restart in place, since every piece of state is in the copied directory. Measured in a sandbox: nothing lost; see [Deployment](DEPLOYMENT.md#failover-with-the-staging-directory). Clients must reach the new daemon at the old address. |
| Host crash, disk intact | Restored if their files survived: session files are not synced. | Loaded: they are synced. | Recovered: sidecars are synced. | A client whose session file survived goes on with it. One whose file was lost registers again; its record still confirms its incarnation, so its writers can resume through `CLAIM_FH` or their old stateids. This case has not been tested. |
| Wipe: an empty staging directory, after a failover or an upgrade | None. | None. | None; the old disk keeps them if it survives. | A new server. Every client gets `NFS4ERR_BADSESSION`, registers again as a new client and reopens its files. Read-only opens continue. Every file open for writing loses its unpublished writes, mostly silently: a later write gets `EBADF`, and fsync and close return 0. |
| Disk loss | None. | None. | Lost. | The same as a wipe. |
| Client reboot | The old boot's go. | Rewritten for the new boot. | The old boot's are retired; the new boot cannot adopt them. | Confirmation of the new incarnation ends the old one. |

No restart publishes all leftover staging automatically. The client must
recover an eligible writer and close it. A successful mount alone proves
neither that old descriptors recovered nor that pending writes were published.

## Client recovery

A reconnect to the same running daemon reuses its session and cached
replies. A reconnect after a restart can too, when the new daemon rebuilt
the session from its file on the staging disk. That needs a usable file
that recorded a request, an incarnation that is still the confirmed one,
and room for the session in the new daemon; see
[session restoration](internal/server/README.md#session-restoration).
Before it listens, the new daemon finishes any reboot purge or expiry that
a client record shows unfinished, and every confirmed record starts a
fresh lease. The Linux client, which learns nothing of the restart,
retransmits every request that was in flight with the same slot and
sequence number and goes on using its stateids. A retransmission of a
completed request is answered from the recorded reply.

A slot holding an intent but no reply belongs to a request the earlier
process was running when it died. The request runs again, and the
operation the intent names is decided against TernFS first: a RENAME or
REMOVE whose effect is visible reports success without running again, one
whose source is still in place runs now, and anything that cannot be
proved, a pending CREATE included, gets `NFS4ERR_DEADSESSION`. The full
rules are in the
[replay decision](internal/server/README.md#restart-replay-decision).
After `NFS4ERR_DEADSESSION` the client re-establishes its session.

While a session's file is on disk, each of its compounds may run at most
one mutating operation, COMMIT aside. Before a second one of any kind runs,
the file is removed and the session goes on in memory only, so a
retransmission of that request after a restart gets `NFS4ERR_BADSESSION`.
A restored slot accepts a later sequence number than it recorded once,
since the record of a later request may not have been written. That request
runs as a new one: an upgrade OPEN gets a new stateid, and a CLOSE or
OPEN_DOWNGRADE whose staging was removed gets `NFS4ERR_BAD_STATEID`. A
request killed after its effect and before its record runs again when it
is retransmitted.

A session with no usable file is unknown to the new daemon, and so is one
retired after the restart because its incarnation expired: SEQUENCE and
BIND_CONN_TO_SESSION receive `NFS4ERR_BADSESSION`, CREATE_SESSION with the
old wire clientid `NFS4ERR_STALE_CLIENTID`, and EXCHANGE_ID and
CREATE_SESSION establish new wire state. A client whose record survived and
that comes back with the same boot verifier and principal keeps its
confirmed incarnation, and with it its staged writers.

There is no NFS reclaim grace period. OPEN with `CLAIM_PREVIOUS` receives
`NFS4ERR_NO_GRACE`; RECLAIM_COMPLETE is accepted without reconstructing
opens. Clients may then reopen by handle or pathname. Whether a particular
application descriptor resumes depends on that recovery path.

Ordinary read stateids receive `NFS4ERR_BAD_STATEID`: a read open of an
existing file leaves no staging to recover, and the Linux client reopens
the handle with CLAIM_FH. A read-only create is different: it stages its
initial attributes, and its old stateid can still find that staging after
a restart, although a WRITE through it gets `NFS4ERR_OPENMODE`. A writer
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
| Continue with the old stateid | Sidecar matches the stateid and client incarnation, and the client presents the staging inode or the sidecar's base inode as its handle; its incarnation is still confirmed with a live lease. For a file the earlier process created, the base is the empty placeholder, which stays the client's handle after the name is renamed or removed. The stateid alone does not find the staging. |
| Reopen by pathname | A unique eligible staging entry matches the target and access. Within the same incarnation, another open-owner can adopt shared staging. |
| Writable `CLAIM_FH` on a cold daemon | A unique healthy, linked writer matches the client and recorded owner, and its recorded pathname still names the held handle. |

If the old incarnation expired, recovery into a new incarnation additionally
requires the sidecar's recovery key and original open-owner. The key
identifies the stable client identity, boot verifier and normalized RPC
principal. A rebooted client, another principal or another client cannot
adopt those bytes. The key does not depend on the old incarnation's
record, which expiry removes.

Recovery by pathname can retain an older writer's private version even
after another writer published. Cold recovery by handle cannot do that
when the name no longer contains the held inode. An unlinked or quarantined
writer is also excluded from cold `CLAIM_FH` recovery. No candidate gives
`NFS4ERR_STALE`; ambiguous candidates give `NFS4ERR_EXPIRED`. These responses
do not guarantee that the client can repair an existing descriptor by
looking up a name.

The base and transient TernFS inodes must still exist. Retaining staging
does not extend TernFS snapshot or transient-file retention.

A recovered CLOSE resolves the publication candidate its sidecar records,
or the staging's own transient when it records none, as a retried CLOSE
does: if the earlier process's link applied and nothing has been written
since, it completes without publishing again, and otherwise it publishes
through a fresh transient; see
[Publication](internal/server/README.md#publication). Times set with the
zero stateid before the CLOSE comes back go to the recovered staging. Also
account for the
[CLOSE defect](DEPLOYMENT.md#a-rejected-publication-is-not-reported-to-the-application)
before relying on recovered writes. A CLOSE whose publication the backend
rejects discards its staging, unless resolving the candidate shows that the
link applied. If other opens were reading that staging, it is quarantined
instead, and those readers get `NFS4ERR_IO`. A timestamp-only CLOSE whose
lookup or SetTime the backend does not answer gets `NFS4ERR_DELAY` and
keeps the staging.

## Lease expiry

The advertised lease is 90 seconds. This is not a deadline at which every
idle writer disappears: the lease is kept in the daemon's memory and
checked by a periodic sweep, and a client expires about 90 to 135 seconds
after its last request. A restart gives every confirmed client a fresh
lease. See [The lease](DESIGN.md#the-lease). Expiry writes the client's
record expired, retires its sessions and opens, and removes the record. If
the record cannot be written the client stays live, and the next sweep
tries again.

On expiry, linked staging is retired. Its acknowledged data remains on
disk, but it releases file descriptors and its pathname reservation, and
its publication candidate is scrapped, best effort, so that a link the
backend has not answered cannot land later. It stays until recovered and
closed or explicitly removed. Unlinked staging is discarded on expiry.
Staging already retired when it becomes unlinked is quarantined.

For the first 90 seconds after startup, the periodic staging sweep does not
run. Ordinary client expiry, and the check before a namespace change, can
still retire staging in that time. This is a local cleanup delay, not NFS
grace-period reclaim. Linked retired data can remain recoverable after this
interval.

## Startup and quarantine

Startup reads each `.staging`/`.meta` pair. The current sidecar format is
NFS9. Recovery restores checkpointed size and dirty ranges; bytes outside
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
A failed CLOSE has its own rules for what is kept; see
[Writer recovery](#writer-recovery).

Startup removes the temporaries a crash left in `clients/` and finishes a
reboot purge or an expiry that a record shows unfinished. A file there
that nfsd did not write, one that does not decode or whose name does not
match its owner, stops startup with `client records: corrupt`. Move it
aside to start; that client then registers as new.

If syncing `clients/` fails while the daemon serves, it cannot tell whether
the update reached the disk, so it changes no record afterwards:
CREATE_SESSIONs that need a record write get `NFS4ERR_SERVERFAULT`, and no
client expires. The log says `not durable`. Restart the daemon, which
reloads the records from the disk.

## Inspect stored state

On a running daemon, the [diagnostics API](inspect/README.md) on its
`-status-addr` shows the client records (`/v1/clients`), the staged
writers (`/v1/staging`) and the sessions (`/v1/sessions`) without changing
them.

When the daemon will not start, or should not be disturbed, read the
staging directory itself. Starting a daemon on it runs startup recovery,
which can quarantine and truncate staged files, so copy the directory aside
first if it must stay as it is. The client records are small JSON files,
one per client identity:

```sh
jq . /var/lib/nfsd/clients/*
```

Each names the identity's `confirmed` incarnation and boot verifier, and
`purging` or `expired` when a reboot purge or an expiry was interrupted;
the next start finishes either. A staged writer is a `.staging` data file
and a `.meta` sidecar named with its inode as 16 hex digits; sidecars are
binary and are only decoded by a daemon. A stateid's first four bytes are
the issuing process's `stateid_epoch`. Match it against startup logs or a
serving daemon's [process identities](inspect/README.md#process-identities)
to find that daemon.

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

A tree's daemon moves to another host by moving the tree's address. There
are two ways; they differ in what clients lose. What clients see in each is
in [Deployment](DEPLOYMENT.md#failover-with-the-staging-directory).

### Before either

1. The operator fences the old daemon before the address moves: it must
   not serve or publish again once the new one starts, because a late CLOSE
   from it would publish old staging over newer data. Stop and disable its
   unit and make sure the old host cannot come back with the address. nfsd
   has no fencing of its own.
2. The new host runs a binary that reads the old sidecar format, with the
   same `-registry`, `-filesystem` and `-owner` flags, as the same uid, with
   its staging directory at the path its unit uses.

Wire-clientid epochs come from wall time after binding the socket, so
clocks must not go backwards across successive owners of an address; NTP
keeps that true. Existing Linux mounts keep the resolved IP address, so
the address itself must move: a DNS change affects only new mounts.

### With the staging directory (preferred)

Copy the stopped daemon's staging directory to the new host. Copy through
a machine that can reach both hosts if the hosts cannot reach each other:

```sh
# A = old host, B = new host, S = the staging directory, same path on both.
# Optional warm copy while A still serves, to shorten the outage:
rsync -aHS --numeric-ids --rsync-path='sudo rsync' A:$S/ ./staging/
# Fence A: stop and disable it before the final copy.
ssh A 'sudo systemctl disable --now nfsd.service'
# Final copy with A stopped. -c because staging files are rewritten in
# place at the same size, which a size-and-mtime check can miss.
rsync -aHSc --numeric-ids --delete --rsync-path='sudo rsync' A:$S/ ./staging/
rsync -aHS --numeric-ids --delete --rsync-path='sudo rsync' ./staging/ B:$S/
ssh B sync
# Move the address to B, then start nfsd there.
```

Copy the whole directory, `server-id`, `clients/`, `sessions/` and
`aliases/` included: without the session files a CLOSE in flight at the
failover is lost silently. `server-id` makes the new daemon the same
server; nfsd refuses to start on a copy that has staged files, session
files or client records but no `server-id`
([server identity](internal/staging/README.md#server-identity)). Never
start the new daemon from a copy taken while the old one was serving.
`-S` keeps the sparse staging files sparse; `--numeric-ids` keeps
ownership. Afterwards check the new daemon's startup log for restored
sessions, recovered staging and quarantine, and resolve leftover staging
before the next failover.

### With an empty staging directory

Use this when the old staging cannot be read. Start the new daemon with an
empty staging directory owned as the unit expects. It writes a new
`server-id` and is a new server to every client: it has no client records,
so every client registers as new and the principal check starts afresh.
If the old disk may come
back, keep it untouched; its data can be recovered by hand later, before
the base and transient inodes it needs are collected; see
[Recover retained data manually](#recover-retained-data-manually). Tell
the users of files that were open for writing at the failover that their
last changes may be gone: the new daemon's journal names the refused
writable reopens ("writable open by filehandle refused").

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

### From a release with client records in TernFS

Releases before this one kept client records in TernFS under
`/.nfs/clients`, and their staging directories have no `server-id`; their
sidecars and session files name incarnations by TernFS inode. None of it
is converted. The upgrade is a wipe, one reboot recovery per tree:

1. Drain writers: stop or quiesce the applications that write through the
   tree, so no file is open for writing. Check that `/v1/staging` on the
   old daemon shows no live writers.
2. Stop nfsd.
3. Move the old staging directory aside and create an empty one, owned as
   the unit expects. Keep the old one until its contents are accounted
   for. Starting the new binary on it fails: with staged files or session
   files and no `server-id` it looks like a partial copy.
4. Start the new binary. It writes `server-id` and serves as a new server.
5. Clients get `NFS4ERR_BADSESSION`, register again and reopen their
   files. Nothing is lost if step 1 drained every writer.
6. Later, delete `/.nfs/clients` from the tree by hand, through a TernFS
   client: nfsd hides `/.nfs`. This release still hides it from LOOKUP and
   READDIR so the old records are not exposed; a later release stops.

### Sidecar formats

A binary which cannot read the existing sidecar format may fail startup.
The current format is NFS9; older NFS5 to NFS8 sidecars are not
automatically converted. Use a compatible binary to finish or recover them.
Remove staging only after its data has been accounted for. It can contain
bytes already acknowledged by fsync.
