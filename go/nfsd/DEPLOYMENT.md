<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Deploying nfsd

One nfsd serves each tree. It owns its sessions, open state and local
staging. Client identities, incarnations and leases are kept in TernFS, so
they outlive the process and its staging disk. A second daemon on the same
tree is not supported.

These are the current implementation's guarantees and limitations.
[Recovery](RECOVERY.md) describes the recovery paths and operator steps.

## Data guarantees

| Event | What it establishes |
| --- | --- |
| Successful create OPEN | The pathname contains an empty published file. Initial size and later writes can still be private. |
| Unstable WRITE | The server accepted bytes into local staging. The client must retain them until made stable. |
| Stable WRITE or COMMIT | Checkpoints staging data and metadata on the local disk. After a restart, COMMIT also syncs the client's recovered writers of the handle. It does not publish to TernFS. |
| Last write CLOSE or downgrade to READ | Publishes a changed, linked staging version synchronously, or answers `NFS4ERR_DELAY` while the backend does not answer, and the client retries. A clean writer leaves the published file unchanged; an unlinked writer discards its private version. |
| Fresh lookup after publication | Finds the replacement inode. Already held handles and client caches need not select that version. |

Writers on one client share staging for the same file within one daemon.
Other clients have independent versions. Concurrent writers do not merge
edits: the last successful publication wins the pathname. Do not use this
as a distributed append or locking service.

Published contents have TernFS's configured durability and retention.
Unpublished contents depend on the staging disk. Even after fsync, a new
file can still appear empty to another client and an edited file can still
show its previous published contents.

An old handle identifies an immutable inode, but the issuing daemon may
temporarily map it to the client's newer version. A process restart on
the same staging disk keeps that mapping only if its recorded expiry has
not passed; a failover with an empty staging directory loses it. This causes the
[old-handle defect](#old-handles-name-their-original-version).

## Deployment modes

| Mode | Intended use | Main limit |
| --- | --- | --- |
| [One writable server](#one-writable-server) | Each tree served by one daemon and its staging disk. | Staged data has no second copy in TernFS. |
| [Read-only servers](#read-only-servers) | Read published data without write staging. | Sessions still require affinity; the mode does not block timestamp updates. |
| [Failover with the staging directory](#failover-with-the-staging-directory) | Move a tree's daemon to another host, copying the stopped daemon's staging. The preferred failover. | The copy time is outage time. |
| [Failover with an empty staging directory](#failover-with-an-empty-staging-directory) | Move a tree's daemon when the old staging cannot be read. | Unpublished writes of files open for writing are lost, mostly silently. |

Several daemons serving one tree, or one mount spread across daemons, are
not supported. Sessions, stateids, staged writers and namespace bookkeeping
are local to one daemon.

### One writable server

**Topology.** Direct each mount, including all its TCP connections, to one
nfsd. Give that daemon a persistent staging directory.

**Client mounts.** Mount with `vers=4.1,proto=tcp,hard`. A client that
reopens files without walking their path must also mount with
`lookupcache=none`. Samba is one: it opens a file `O_PATH` by name and
reopens it for writing through `/proc/self/fd`. A writable open by
filehandle needs nfsd to know the file's current name, and nfsd records
names only for files a client has looked up or opened since it started. A
handle the client learned from READDIR, kept across its own save, or held
across a restart can therefore be refused with `NFS4ERR_STALE`. An
ordinary open recovers by looking the name up again; a reopen through
`/proc/self/fd` cannot, and Samba reports access denied. With
`lookupcache=none` every open walks the path, so the name is fresh. It
costs one LOOKUP per open.

**Guarantees.** Same-client opens share a mutable view within the daemon.
Publication is synchronous. REMOVE and RENAME update that daemon's staged
writers so a later CLOSE follows the local namespace change.

**Failure behaviour.** Restarting the process with the same disk can restore
sessions and their cached replies, so clients continue on the same session
and retransmissions get their original replies. Restoration is conditional;
see [session restoration](internal/server/README.md#session-restoration). A
restart also recovers eligible checkpointed writers through their old
stateids, and restores publication aliases whose recorded expiry has not
passed. Read opens are reopened by the client. Session files and aliases are
written without fsync, so a host crash can lose them. Disk loss removes
unpublished contents and every session.

**Known issues.** The backend-timeout and rejected-publication issues below
apply. A surviving disk is not a guarantee that every descriptor resumes.
There are no enforced share reservations or byte-range locks.

### Read-only servers

**Topology.** Omit `-staging` and use read-only client mounts, for example
`-o vers=4.1,proto=tcp,hard,ro`. Each mount still belongs to one daemon.

**Guarantees.** Published files can be read without local write staging.
The daemon rejects writable opens and create/remove/rename mutations.

**Failure behaviour.** A replacement can serve the published data without
the old host's disk. Existing sessions and opens still need recovery, and
DNS changes alone do not redirect existing mounts.

**Known issues.** The server's read-only mode is incomplete: timestamp-only
SETATTR can still change backend timestamps. A client `ro` mount prevents
ordinary writes through that mount; it does not enforce policy on other
clients. The daemon also writes its client registrations, leases and open
markers under `/.nfs`. Read-only service is therefore not a backend with no
writes. It adds no coherence or failover guarantee for a concurrently
changing namespace.

### Failover with the staging directory

**Topology.** The operator fences the old daemon, copies its stopped
staging directory to the new host, moves the tree's address and starts nfsd
there with the same binary and flags. The procedure is in
[Replace a host](RECOVERY.md#replace-a-host).

**Guarantees.** The same recovery scope as a restart in place: the copied
session files, sidecars and aliases are what the new daemon recovers from.
Measured with a Linux 6.1 client in a sandbox, killing the old daemon and
starting the new one from an rsync copy of its staging: the client carried
on with its session without registering again, every descriptor open for
writing kept its data, including data acknowledged by fsync and a 1 GiB
CLOSE in flight at the kill, and reads of just-saved files were correct.

**Failure behaviour.** Clients stall on their hard mounts from the moment
the old daemon stops until the new one answers, so the copy time is outage
time. If the session files are missing or unusable on the new host, the
mount still survives: the client registers again and staged writers are
adopted from their sidecars, but a CLOSE in flight at the failover returns
0 to the application and its data stays unpublished on the new host's
staging disk. A copy taken while the old daemon still served loses
whatever that daemon acknowledged after the copy, silently; in the sandbox
an fsync-acknowledged write left a hole of zeros.

**Known issues.** All restart defects below apply. Never run two daemons
against the same staging directory or tree. Staging left behind by a lost
CLOSE or a bad copy is carried into every later copy; resolve it after each
failover.

### Failover with an empty staging directory

**Topology.** As above, but the new daemon starts with an empty staging
directory, because the old one cannot be read. Keep the old disk untouched
if it may come back: it holds the only copy of what is lost.

**Guarantees.** Published data remains accessible. Clients stay mounted.
Measured with a Linux 6.1 client in a sandbox, five failovers: the client
saw its session refused, registered again, had `CLAIM_PREVIOUS` refused
with `NFS4ERR_NO_GRACE` and reopened its files; new work succeeded 0.6 to
3 s after the new daemon listened. Files closed before the failover, and
read-only opens, were unaffected.

**Failure behaviour.** Files open for writing at the failover lose their
unpublished writes, mostly without an error. The new daemon refuses their
writable reopens with `NFS4ERR_STALE`, having neither a name nor staging
for them. The application then gets `EBADF` on its next write, while
`fsync` and `close` return 0; data an earlier fsync acknowledged is lost
too, and the server holds the previous version of an existing file or an
empty file for a new one. A CLOSE in flight returns 0 and its data is
lost. Reads of files saved just before the failover returned the previous
version for 10 to 20 s, because the client's aliases went with the old
staging directory. Once a write is acknowledged as stable, NFS has no way
to tell the client it was lost, so this is inherent to the procedure.

**Known issues.** The new write verifier tells clients to resend unstable
writes they still hold; it does not bring back stable ones.

## Known defects

### A backend timeout fails writers at a REMOVE or RENAME

nfsd uses the Go client's default timeout settings: an overall limit of
10 s on a shard request, 60 s on a CDC request and 5 min on a block-service
request. These are configured limits on one request's retries, not
guarantees about how long a CLOSE or an outage takes. Time a metadata
request spends in the client's send queue comes on top, a block-service
request sets a fresh socket deadline for each attempt, and one CLOSE makes
many requests.

CLOSE and OPEN_DOWNGRADE do not treat a timeout as a failure. A publication
or hydration the backend does not answer gets `NFS4ERR_DELAY` and keeps the
open and the staging, and the Linux client retries, so `close(2)` blocks for
about the outage, as on a hard mount; see
[Publication](internal/server/README.md#publication). A client that does
not retry DELAY, such as libnfs, gets an error from its close instead.

REMOVE and RENAME do. A timeout there, like any error except a REMOVE whose
edge is already gone, is an unknown outcome. The staged writers at the
names are failed and quarantined, and the operation gets `NFS4ERR_IO` even
if it applied; with no staged writer it gets the mapped backend error. The
session is kept. Deciding these outcomes by looking the names up is not
done yet.

### A rejected publication is not reported to the application

When the backend rejects a publication, CLOSE returns the backend error
mapped to an NFS status. Any error without a more specific mapping is
`NFS4ERR_IO`; a missing name is `NFS4ERR_NOENT` and a permission error
`NFS4ERR_ACCESS`. What happens to the staging depends on the step that
failed:

- If the upload fails, or LinkFile is rejected and resolving the
  publication candidate does not show that another copy of the request
  linked it, the staging is discarded and its transient scrapped. A first
  LinkFile answered not-exist is not a rejection: the CLOSE retries once
  through a fresh transient. If other
  opens still read that staging, it is quarantined instead: its files are
  kept, and those readers get `NFS4ERR_IO`.
- If a timestamp-only CLOSE gets an error other than a timeout looking up
  the name or setting the times, the writer ends as after a clean CLOSE. A
  failed SetTime may still have applied.

The Linux client discards a CLOSE's status, so `close(2)` returns 0 and the
application believes its save was published while the name holds the
previous version or the empty placeholder. An upload error that is neither
a timeout nor a missing file, such as a block service refusing writes after
the Go client's retries, takes this path too.

### A CLOSE retried after an hour can publish again

A retried CLOSE decides that its earlier link did not apply when `ScrapFile`
of the candidate succeeds. A candidate that did link, was then replaced at
the name by another save, and was collected by snapshot GC after terngc's
`-collect-directories-min-edge-age` (one hour by default) is a transient
again. A CLOSE still retrying that late publishes its bytes again, over the
newer save.

### Old handles name their original version

A client reads its newly published contents through the handle it still
holds while the daemon keeps a publication alias for it: while the handle
is open and for one lease after publication. The alias survives a restart
while its recorded expiry, renewed at each sweep for an open handle, is
still in the future. After it lapses, including during a long outage or
after a failover with an empty staging directory, the handle names its
original immutable inode again, so a read through a very old handle can return older bytes. The
replacement is always available through a fresh pathname lookup. A
writable OPEN by handle can get `NFS4ERR_STALE` when the daemon has
neither a remembered current pathname nor eligible staging. An ordinary
open on Linux then looks the name up again; a reopen without a path walk
cannot, which is why such clients mount with `lookupcache=none`.

### Lease expiry leaves retired staging

A writer whose client's lease expires is retired with its data in place,
including one that acknowledged nothing, and is kept until recovered or
removed by an administrator. An `EXCLUSIVE4` create retried
by the same client boot after its lease expired gets `NFS4ERR_EXIST`
instead of adopting its retired staging.

## Operating requirements

### Staging disk

Use a persistent staging disk and size it for complete replacement files.
Small edits can require reading the entire immutable base and uploading a
whole replacement on CLOSE. Monitor free space, retired and quarantined
staging, hydration traffic, CLOSE latency and backend errors. Linked retired
staging has no automatic age-based deletion.

### Request limits

Bound request concurrency with `-max-in-flight-per-connection` if needed.
Its default is 64; it bounds request count per connection, not bytes,
connections or total server RSS. Monitor RSS and tune it for the workload.
Reply memory is bounded across the process by `-reply-work-budget-mib`,
8 GiB by default. Each request reserves eight times its reply capacity plus
a fixed allowance before it runs, so a 1 MiB READ reserves about 8.5 MiB; see
[reply work](internal/server/README.md#reply-work). A request that
does not fit waits for replies to drain rather than being refused. A request
larger than the whole budget is refused and its connection closed. A
dashboard shows the waits as `admission_work_waits`.

### Client store snapshot policy

Give `/.nfs` a snapshot policy appropriate for disposable client-store
history. Lease and marker churn otherwise accumulates snapshot edges and
dead inodes. For example, with the inode of `/.nfs` in `NFS_DIR_ID`:

```sh
terncli set-dir-info -id "$NFS_DIR_ID" -tag SNAPSHOT \
    -body '{"DeleteAfterVersions": 0}'
```

Do not apply that recommendation to user data indiscriminately: staging
recovery can need retained base versions. Keep `/.nfs` itself while serving
the filesystem. Even a read-only daemon uses its current records.

### Network access

Ownership and modes are synthetic. AUTH_SYS principal comparison is an
identity-collision guard, not an authentication or permission boundary.
Choose network access accordingly. The diagnostics API has no
authentication either; see [Diagnostics](#diagnostics).

### Upgrades

For upgrades, drain and close writers, then account for remaining active,
retired and quarantined staging. The current sidecar format is NFS9.
Deleting staging to bypass a format incompatibility discards unpublished
data. Follow [upgrade recovery](RECOVERY.md#upgrade-the-server).

## Metrics

With `-influx-db-origin`, `-influx-db-org` and `-influx-db-bucket`, nfsd
pushes its metrics every `-metrics-interval` (10 s) as InfluxDB line
protocol, the way the other TernFS Go services do, so VictoriaMetrics
stores each field as `<measurement>_<field>`. `-filesystem` is required and
becomes the `filesystem` label on every series; `instance` is the short
host name and NFS port unless `-instance` says otherwise, so several nfsd
processes on one host stay apart. The `Eggs NFS` dashboard reads these.

Counters are cumulative from process start. Latency histograms have a
`le` label with the bound in seconds and a final `+Inf`, for
`histogram_quantile`.

| Series | Labels | Meaning |
|---|---|---|
| `eggsfs_nfsd_ops_count` | `op`, `status` | Operation results, by NFS operation and status, once each, including SEQUENCE and operations refused before they ran. An undefined opcode counts as `OP_ILLEGAL`, as on the wire. A replayed reply counts only its SEQUENCE. |
| `eggsfs_nfsd_op_latency_bucket_count` | `op`, `le` | Operation service time inside nfsd, bounds 0.5 ms to 60 s. |
| `eggsfs_nfsd_op_latency_count`, `_sum_nanos` | `op` | For mean latency. |
| `eggsfs_nfsd_io_ops`, `_bytes` | `direction` | READ and WRITE operations and payload bytes. |
| `eggsfs_nfsd_compounds_count` | | RPC compounds handled. |
| `eggsfs_nfsd_connections_open` | | Tracked connection handlers, including connections draining requests (gauge). |
| `eggsfs_nfsd_connections_accepted`, `_closed_by_client`, `_closed_by_server`, `_idle_timeouts`, `_read_errors`, `_parse_errors` | | How connections began and ended. Each ended connection is counted once, by the first cause seen, when its handler returns, so the causes sum to the accepted connections that are no longer open. A client close is EOF or a reset; a server close is a reply write failure, a panic, a request refused or dropped without a reply, or a conflicting XID. |
| `eggsfs_nfsd_admission_in_flight`, `_max_in_flight` | | Requests admitted and not yet answered, and the sum of per-connection limits (gauges). |
| `eggsfs_nfsd_admission_work_bytes`, `_work_limit`, `_work_replies`, `_work_waiters` | | Reply-work reservations against `-reply-work-budget-mib`, and requests waiting for it (gauges). |
| `eggsfs_nfsd_admission_work_waits`, `_work_wait_nanos` | | Waits for reply work begun, and time spent waiting. |
| `eggsfs_nfsd_admission_cache_bytes`, `_cache_limit`, `_sessions`, `_records`, `_groups`, `_active_requests` | | Reply cache and session store figures (gauges). |
| `eggsfs_nfsd_state_opens`, `_aliases` | | Live open stateids and publication aliases (gauges). |
| `eggsfs_nfsd_state_publishes` | | Files published by CLOSE or OPEN_DOWNGRADE. |
| `eggsfs_nfsd_state_publish_delays` | | CLOSEs and OPEN_DOWNGRADEs answered `NFS4ERR_DELAY` because the backend did not answer a step of the publication, its hydration, scrap, lookup or SetTime, refused the link on a locked name, a candidate checkpoint failed, or a WRITE landed while it ran; the client retries them. |
| `eggsfs_nfsd_staging_files`, `_bytes`, `_retired`, `_unlinked`, `_busy`, `_hydrations_in_flight` | | The staging store (gauges). Files are read without waiting on a file busy with disk I/O: a busy file counts in `files` and `busy` but not in `bytes`, `retired` or `unlinked`, so those are lower bounds while `busy` is above zero. `bytes` is the files' live size, including UNSTABLE writes not yet committed. |
| `eggsfs_nfsd_replay_records`, `_write_failures`, `_record_bytes`, `_intents` | | The persistent reply cache: slot and intent records written, record writes and session file creations that failed, reply bytes recorded, intents. |
| `eggsfs_nfsd_replay_session_files` | | Sessions with a file on the staging disk (gauge). |
| `eggsfs_nfsd_replay_sessions_restored`, `_served`, `_decided_applied`, `_undecidable` | | Sessions restored at startup; retransmissions after a restart answered from a restored reply, run again with their intent found applied, and undecidable (`NFS4ERR_DEADSESSION`). |
| `eggsfs_nfsd_backend_count`, `_attempts` | `service`, `kind` | Requests to shards and the CDC by kind, and attempts including resends. |
| `eggsfs_nfsd_backend_latency_bucket_count` | `service`, `kind`, `le` | Backend round-trip time, bounds 0.1 ms to 10 s. |
| `eggsfs_nfsd_process_heap_alloc_bytes`, `_sys_bytes`, `_goroutines`, `_gc_pause_nanos`, `_gc_count`, `_uptime_seconds` | | Go runtime figures. |

The two signals that have mattered so far are `status` other than
`NFS4_OK` on SEQUENCE or WRITE, since a Linux client sleeps 15 s on a
session-level DELAY, and `closed_by_server`, since every server-side close
costs a client that reconnect and backoff. `OP_CREATE_SESSION` while no
client is mounting means a session was retired.

## Diagnostics

`-status-addr` serves a read-only JSON API about the process's own state,
off by default. `-store-only` serves the same API without the NFS server,
reading the client store and staging sidecars without changing them. The
[diagnostics API reference](inspect/README.md) describes the resources,
typed ids and the store report.

```sh
nfsd -registry registry.example:55556 -staging /var/lib/nfsd \
    -status-addr 127.0.0.1:9049
curl -s http://127.0.0.1:9049/ | jq .
```

The API has no authentication. Bind it to a loopback or management address.
During a stall, take `/debug/pprof/goroutine?debug=2` first.
