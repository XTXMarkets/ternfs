<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Deploying nfsd

An nfsd process owns its sessions, open state and local staging. The TernFS
namespace and client registration store are shared. Sharing that store
coordinates client lifetimes; it does not share staged bytes or make the
processes interchangeable.

These are the current implementation's guarantees and limitations.
[Recovery](RECOVERY.md) describes the recovery paths and operator steps.

## Data guarantees

| Event | What it establishes |
| --- | --- |
| Successful create OPEN | The pathname contains an empty published file. Initial size and later writes can still be private. |
| Unstable WRITE | The server accepted bytes into local staging. The client must retain them until made stable. |
| Stable WRITE or COMMIT | Normally checkpoints staging data and metadata on the local disk. It does not publish to TernFS. The recovered-COMMIT defect below is an exception. |
| Last write CLOSE or downgrade to READ | Publishes a changed, linked staging version synchronously. A clean writer leaves the published file unchanged; an unlinked writer discards its private version. |
| Fresh lookup after publication | Finds the replacement inode. Already held handles and client caches need not select that version. |

Writers on one client share staging for the same file within one daemon.
Other clients have independent versions. Concurrent writers do not merge
edits: the last successful publication wins the pathname. This also applies
across daemons. Do not use this as a distributed append or locking service.

Published contents have TernFS's configured durability and retention.
Unpublished contents depend on the staging disk. Even after fsync, a new
file can still appear empty to another client and an edited file can still
show its previous published contents.

An old handle identifies an immutable inode, but the issuing daemon may
temporarily map it to the client's newer version. That mapping is lost on
restart and is absent on other daemons. This causes the
[old-handle defect](#old-handles-can-return-older-content).

## Deployment modes

| Mode | Intended use | Main limit |
| --- | --- | --- |
| [One writable server](#one-writable-server) | Clients pinned to one daemon and its staging disk. | Staged data has no second copy in TernFS. |
| [Independent writable servers](#independent-writable-servers) | Separate mounts serve one namespace through different daemons. | Mutable views and namespace bookkeeping are local. |
| [Read-only servers](#read-only-servers) | Read published data without write staging. | Sessions still require affinity; the mode does not block timestamp updates. |
| [Replacement with the staging disk](#replacement-with-the-staging-disk) | Recover a stopped server on another host. | Existing opens recover only in the supported cases. |
| [Replacement without the staging disk](#replacement-without-the-staging-disk) | Restore access to published data. | Unpublished stable data is unavailable. |
| [One mount distributed across servers](#one-mount-distributed-across-servers) | Not supported. | Sessions and stateids are not shared. |

### One writable server

**Topology.** Direct each mount, including all its TCP connections, to one
nfsd. Give that daemon a persistent staging directory.

**Guarantees.** Same-client opens share a mutable view within the daemon.
Publication is synchronous. REMOVE and RENAME update that daemon's staged
writers so a later CLOSE follows the local namespace change.

**Failure behaviour.** Restarting with the same disk restores sessions and
their cached replies, so clients continue on the same session and
retransmissions get their original replies; recovers eligible checkpointed
writers through their old stateids; and restores publication aliases. Read
opens are reopened by the client. Disk loss removes unpublished contents
and every session.

**Known issues.** The backend-timeout issue below applies. A surviving
disk is not a guarantee that every descriptor resumes. There are no
enforced share reservations or byte-range locks.

### Independent writable servers

**Topology.** Several daemons serve one TernFS namespace, each with its own
address and staging directory. A client may mount more than one, but each
mount must stay with its chosen daemon.

**Guarantees.** The published namespace is shared. A fresh lookup can see
another daemon's completed publication. Client reboot and lease decisions
are coordinated through the shared client store.

**Failure behaviour.** Surviving daemons continue serving published files.
They cannot take over another daemon's unpublished writes without its
staging disk. Reads through another daemon do not see a client's private
writes, even if it registered the same client identity there.

**Known issues.** Namespace bookkeeping does not cross daemons. If A has a
writer for `x` and B removes `x`, A's later CLOSE can recreate `x`. If B
renames it to `y`, A can publish at the old name. GUARDED and exclusive
create checks are also serialized only locally. All single-server recovery
defects remain. A backend outage can prevent A from renewing its durable
lease long enough for B to expire the client, even while A has active work.

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

### Replacement with the staging disk

**Topology.** Stop or fence the old daemon, attach its staging disk to the
replacement, and use the same TernFS backend and compatible sidecar format.
To preserve existing mounts' destination, arrange takeover of the address
they already use.

**Guarantees.** Eligible staged writers have the local data and sidecars
needed for recovery. This has the same recovery scope as an in-place restart.

**Failure behaviour.** The new daemon has new sessions, clientids and
in-memory state. Recovery still depends on the client identity and boot,
the held handle, the namespace, and retained base/transient inodes.

**Known issues.** All restart defects below apply. A movable disk does not
make active/active service safe: never run two daemons against the same
staging directory. Endpoint ownership and clocks also matter; see
[host replacement](RECOVERY.md#replace-a-host).

### Replacement without the staging disk

**Topology.** Start a fresh daemon against the same TernFS backend with a
new staging directory. Direct new mounts to it, or separately arrange
address takeover for existing mounts.

**Guarantees.** Published data remains accessible under TernFS's normal
retention rules. New clients can open files and begin new writes.

**Failure behaviour.** The replacement lacks the old daemon's unpublished
bytes, including bytes acknowledged stable. The pathname may contain an
empty placeholder or the earlier published version. Existing write
descriptors cannot rely on resuming.

**Known issues.** A changed write verifier lets clients detect loss of
unstable writes. It does not require them to retain or resend stable data.
If the old disk is recovered later, its staging needs reconciliation with
any intervening publications; do not blindly bring both daemons online.

### One mount distributed across servers

**Topology.** A load balancer, proxy or client sends one mount's connections
to independent nfsd processes.

**Guarantees.** None for a shared NFS session. Multiple connections work
when they all reach the same daemon.

**Failure behaviour.** A session issued by A is unknown to B and gets
`NFS4ERR_BADSESSION`. Registering separately with B does not transfer A's
opens, staged data or handle aliases.

**Known issues.** Each process advertises a distinct server owner. There is
no session trunking across daemons, transparent NFS migration, or replicated
reply cache. Per-connection stickiness is insufficient if one mount has
connections to several daemons. This remains unsupported for read-only
mounts.

## Known recovery defects

### A backend timeout is treated as a failure

The go client gives up on a shard request after 10 s, on the CDC after
60 s and on a block service after 5 min. A CLOSE whose publication hits
that limit discards its staging, scraps the transient file if the shard
is back by then, and answers `NFS4ERR_IO`. The Linux client discards a
CLOSE's status, so `close(2)` returns 0 and the application believes its
save was published while the name holds the previous version or the empty
placeholder. A shard restart lasting longer than about 7 s produces this;
a 7 s one is latency only. Tracked as `work-kd49`.

Any mutation whose outcome cannot be established, which includes every
backend timeout, also retires the client's session. The client re-creates
it, and while the daemon confirms the new session its OPEN, WRITE and
CLOSE get `NFS4ERR_DELAY`, which the Linux client answers with a backoff
of up to 15 s for asynchronous operations. If this happens while the shard
is still down, every application on the mount stalls for the rest of the
outage. Tracked as `work-49b9`.

### Old handles name their original version

A client reads its newly published contents through the handle it still
holds while the daemon keeps a publication alias for it: while the handle
is open and for one lease after publication. The alias survives a restart.
After it lapses, or through another daemon, the handle names its original
immutable inode again, so a read through a very old handle can return
older bytes. The replacement is always available through a fresh pathname
lookup. A writable OPEN by handle can get `NFS4ERR_STALE` when the daemon
has neither a remembered current pathname nor eligible staging; the Linux
client then looks the name up again.

### Lease expiry leaves retired staging

A writer whose client's lease expires is retired with its data in place,
including one that acknowledged nothing, and is kept until recovered or
removed by an administrator (`work-yhrt`). An `EXCLUSIVE4` create retried
by the same client boot after its lease expired gets `NFS4ERR_EXIST`
instead of adopting its retired staging (`work-obgt`).

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
| `eggsfs_nfsd_ops_count` | `op`, `status` | Operations completed, by NFS operation and status. Includes SEQUENCE. |
| `eggsfs_nfsd_op_latency_bucket_count` | `op`, `le` | Operation service time inside nfsd, bounds 0.5 ms to 60 s. |
| `eggsfs_nfsd_op_latency_count`, `_sum_nanos` | `op` | For mean latency. |
| `eggsfs_nfsd_io_ops`, `_bytes` | `direction` | READ and WRITE operations and payload bytes. |
| `eggsfs_nfsd_compounds_count` | | RPC compounds handled. |
| `eggsfs_nfsd_connections_open` | | Open TCP connections (gauge). |
| `eggsfs_nfsd_connections_accepted`, `_closed_by_client`, `_closed_by_server`, `_idle_timeouts`, `_read_errors`, `_parse_errors` | | How connections began and ended. A server close is a reply write failure, an unanswerable request or a conflicting XID. |
| `eggsfs_nfsd_admission_in_flight`, `_max_in_flight` | | Requests admitted and not yet answered, and the sum of per-connection limits (gauges). |
| `eggsfs_nfsd_admission_work_bytes`, `_work_limit`, `_work_replies`, `_work_waiters` | | Reply-work reservations against `-reply-work-budget-mib`, and requests waiting for it (gauges). |
| `eggsfs_nfsd_admission_work_waits`, `_work_wait_nanos` | | Waits for reply work begun, and time spent waiting. |
| `eggsfs_nfsd_admission_cache_bytes`, `_cache_limit`, `_sessions`, `_records`, `_groups`, `_active_requests` | | Reply cache and session store figures (gauges). |
| `eggsfs_nfsd_state_opens`, `_aliases` | | Live open stateids and publication aliases (gauges). |
| `eggsfs_nfsd_state_publishes` | | Files published by CLOSE or OPEN_DOWNGRADE. |
| `eggsfs_nfsd_staging_files`, `_bytes`, `_retired`, `_unlinked`, `_hydrations_in_flight` | | The staging store (gauges). |
| `eggsfs_nfsd_replay_records`, `_write_failures`, `_record_bytes`, `_intents` | | The persistent reply cache: slot and intent records written, writes that failed, reply bytes recorded, intents. |
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

nfsd serves a read-only JSON API about its own state on `-status-addr`,
off by default. `GET /` lists every `/v1` resource, its query filters and
the meaning of every state word the API uses.

```sh
nfsd -registry <addr> -staging <dir> -status-addr 127.0.0.1:9049
curl -s http://127.0.0.1:9049/ | jq .
```

| Path | Content |
| --- | --- |
| `/v1/status` | The process identifiers, limits, reply-cache and request accounting, hydration and client GC activity. |
| `/v1/lookup?id=` | Resolves one typed id. |
| `/v1/clients` | Wire clientids this process issued, each bound to a durable incarnation, with the CREATE_SESSION replay slot. Filters `wire`, `incarnation`, `owner`. |
| `/v1/incarnations` | Per incarnation: the session group and lease gate, the lease refresher record, the cached store location, confirmation protection and destruction holds. Filter `id`. |
| `/v1/sessions` | Each session's channel limits, SEQUENCE status flags, backchannel sender and connections, whether it has a file and was restored, and slots with their restored and intent state. Filters `id`, `incarnation`, `slots=0`. |
| `/v1/opens` | Open stateids with file, data inode, owner and access; expired tombstones; the client views behind a handle. Filters `stateid`, `incarnation`, `file`. |
| `/v1/confirmations` | Client confirmations in progress or unresolved, with the evidence of their writes, destruction holds and purge hints. |
| `/v1/staging` | The staging store in memory: each writer's target, owner, size, dirty ranges and flags, and the failed tombstones. Filters `file`, `incarnation`. |
| `/v1/connections` | Live client connections: requests in flight, the sessions each carries in either direction, and its callback path state. Filter `session`. |
| `/v1/store` | The durable client store report, with `staging=1` to join the sidecars. Filters `identity`, `identity-hash`, `incarnation`, `stateid`. |
| `/debug/pprof/` | Go profiling, served beside the API; `goroutine?debug=2` is the first thing to take during a stall. |

Every `/v1` response carries `now`, `nfsd_id`, `stateid_epoch`,
`wire_epoch` and `mode`, the identities of the process. Ids are typed
`kind:hex` strings, as the log lines spell them.

## Operating requirements

Use a persistent staging disk and size it for complete replacement files.
Small edits can require reading the entire immutable base and uploading a
whole replacement on CLOSE. Monitor free space, retired and quarantined
staging, hydration traffic, CLOSE latency and backend errors. Linked retired
staging has no automatic age-based deletion.

Bound request concurrency with `-max-in-flight-per-connection` if needed.
Its default is 64; it bounds request count, not bytes or total connections.
Reply bytes are bounded across the process by `-reply-work-budget-mib`,
8 GiB by default, charged per request from its operations' reply bounds; a
request that does not fit waits for replies to drain rather than being
refused. A dashboard shows the waits as `admission_work_waits`.

Give `/.nfs` a snapshot policy appropriate for disposable client-store
history. Lease and marker churn otherwise accumulates snapshot edges and
dead inodes. For example, after identifying the inode of `/.nfs`:

```sh
terncli set-dir-info -id <inode-of-/.nfs> -tag SNAPSHOT \
    -body '{"DeleteAfterVersions": 0}'
```

Do not apply that recommendation to user data indiscriminately: staging
recovery can need retained base versions. Keep `/.nfs` itself while serving
the filesystem. Even a read-only daemon uses its current records.

Ownership and modes are synthetic. AUTH_SYS principal comparison is an
identity-collision guard, not an authentication or permission boundary.
Choose network access accordingly.

For upgrades, drain and close writers, then account for remaining active,
retired and quarantined staging. The current sidecar format is NFS8.
Deleting staging to bypass a format incompatibility discards unpublished
data. Follow [upgrade recovery](RECOVERY.md#upgrade-the-server).
