<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd diagnostics API

nfsd serves a read-only JSON API about its own state. Most of the
`/v1` resources are snapshots of the running process's memory: live
sessions and slots, wire client records, lease refresher records and the
logical lease gate, opens, confirmations in progress and backchannel
paths. Some of that state also has a copy on disk. Session files under
`sessions/` in the staging directory record each session's slots and its
wire clientid value, and `aliases/` records publication aliases; the
registration record a wire clientid indexes is process-local. `/v1/store`
reads the durable client store in TernFS and, on request, the staging
sidecars. It does not read the session files or the alias log.

This package defines the response types and the HTTP handlers. The server
implements its `Source` interface; this package does not reach into server
internals. The server's snapshots are in
[diagnostics.go](../internal/server/diagnostics.go) and
[diagnostics_snapshots.go](../internal/server/diagnostics_snapshots.go);
the store report is in
[store_report.go](../internal/clientstore/store_report.go).

## Enabling the API

`-status-addr` turns the API on. It is off by default.

```sh
nfsd -registry registry.example:55556 -staging /var/lib/nfsd \
    -status-addr 127.0.0.1:9049
curl -s http://127.0.0.1:9049/ | jq .
```

The API has no authentication. Bind it to a loopback or management address.

Only GET and HEAD are accepted; other methods get 405. Responses are
indented JSON with `Cache-Control: no-store`. Errors are JSON too, with
`error` and `status`:

| Status | Cause |
| --- | --- |
| 400 | A filter or id that does not parse, more than one store selector, or `staging=1` without a staging directory. |
| 404 | No resource at that path. |
| 405 | A method other than GET or HEAD. |
| 503 | A resource that needs a running server, in store-only mode. |
| 500 | Any other failure, such as a backend error while reading the store. |

## Resources

`GET /` is the index. It lists every resource and its filters, the meaning
of every state word the API uses, and a few general notes. Read it first.

| Path | Content | Filters |
| --- | --- | --- |
| `/v1/status` | Process identifiers, limits, reply-cache and request accounting, wire clientid issuer, hydration and client GC activity. | |
| `/v1/lookup` | Resolves one typed id; see [Lookup](#lookup). | `id` (required) |
| `/v1/clients` | Wire clientids this process issued, each bound to a durable incarnation, with the CREATE_SESSION replay slot. | `wire`, `incarnation`, `owner` (hex) |
| `/v1/incarnations` | Per incarnation: session group and lease gate, lease refresher record, cached store location, confirmation protection, destruction holds, sessions and opens. | `id` |
| `/v1/sessions` | Channel limits, SEQUENCE status flags, backchannel sender and connections, whether the session has a file and was restored, and each slot. | `id`, `incarnation`, `slots=0` omits slots |
| `/v1/connections` | Live TCP connections: requests in flight, the sessions each carries in either direction, and callback path state. | `session` |
| `/v1/opens` | Open stateids with file, data inode, owner and access; expired stateid tombstones; the client views behind each handle. | `stateid`, `incarnation`, `file` |
| `/v1/confirmations` | Client confirmations in progress or unresolved, with the evidence of their writes, destruction holds and purge hints. Normally empty. | |
| `/v1/staging` | The staging store in memory: each writer's target, owner, size, dirty and hydrated ranges and flags, and the failed tombstones. | `file`, `incarnation` |
| `/v1/store` | The durable client store report; see [The store report](#the-store-report). | `identity`, `identity-hash`, `incarnation`, `stateid`, `staging=1` |
| `/debug/pprof/` | Go profiling. Not in the index, and not JSON. `goroutine?debug=2` is the first thing to take during a stall. | |

On `/v1/staging`, a `file` filter applies to the failed tombstones too. An
`incarnation` filter does not, since a tombstone records no owner.

## Process identities

Every JSON response, the index and errors included, carries `now`,
`nfsd_id`, `stateid_epoch`, `wire_epoch` and `mode`. The profiling pages are
not JSON and carry none of them.

- `nfsd_id` names this process's `lease.` and `confirming.` slots in the
  client store.
- `stateid_epoch` is the first four bytes of every stateid and session id
  the process issued.
- `wire_epoch` is the high 32 bits of every wire clientid it issued. It is
  zero until the NFS endpoint is bound.
- `mode` is `serving` or `store-only`. A store-only process issues
  nothing, so its `nfsd_id` and `stateid_epoch` are empty strings and its
  `wire_epoch` is zero.

Startup logs contain `nfsd_id` and `stateid_epoch`, and the wire clientid
epoch once the endpoint is bound. The `session created` log line records a
wire clientid, its incarnation and its session id together, in the spellings
the API uses. It is the one place all three appear. Grep a fleet's logs for
these values to find the process which holds a lease slot or issued a
stateid, session or clientid.

## Typed ids

Ids the API routes on are typed `kind:hex` strings, lowercase and of fixed
width:

| Kind | Hex digits | What it is |
| --- | --- | --- |
| `wire-clientid` | 16 | The clientid EXCHANGE_ID returned: epoch in the high 32 bits, counter in the low. The record it indexes is process-local. |
| `incarnation` | 16 | A client incarnation directory inode: the durable client id. |
| `inode` | 16 | Any other inode: a filehandle, a staging data inode, a directory or a base. |
| `stateid` | 24 | The stateid `other` field. The first four bytes are the stateid epoch. |
| `session` | 32 | A session id: stateid epoch, incarnation, counter. |
| `identity-hash` | 64 | SHA-256 of the EXCHANGE_ID owner, naming the identity directory. |

A filter or lookup takes exactly the kind it names. An incarnation is never
read as a wire clientid, though both are eight bytes. A value copied from
any response or from the `session created` log line pastes back unchanged.
Uppercase hex is accepted. A present but empty parameter is parsed and
rejected like any other bad value. A zero `wire-clientid`, `incarnation` or
`inode` is rejected rather than treated as no filter. Values from a packet
capture need the kind written in front of them.

The process identifiers in the envelope, and opaque metadata such as owners,
verifiers and cookies, keep their plain forms. A field whose id was not
observed is an empty string.

## Lookup

The clientid an NFSv4.1 client sees is not the incarnation inode the client
store uses. It is a wire clientid. The client store never holds it, and the
registration record it indexes lives only in the issuing process's memory.
A session file records its value, but `/v1/store` does not read session
files. Only the issuing process can resolve a wire clientid.

```sh
curl -s 'http://127.0.0.1:9049/v1/lookup?id=wire-clientid:4000000100000001' | jq .
```

`/v1/lookup` takes one typed id. The answer gives `epoch` and
`issued_by_this_process` for kinds that carry an epoch, `known` for whether
this process has state for the id, a `note` explaining the result, and
`links` to the resources that describe it. An `identity-hash` is not looked
up; the answer links to its store report.

A stateid lookup also checks recovered staging from earlier daemon epochs,
so `issued_by_this_process=false` does not mean the stateid is unknown
here. If busy staging files prevent a complete check, the answer has
`incomplete=true`, and `known=false` then does not establish that no
staging names the stateid. Retry when the files are no longer busy.

## Snapshot consistency

Each store copies its own state under its own locks, one store at a time,
so counts across resources may disagree slightly. Registry and record locks
are taken normally; they are held only for map and field access.

Three locks are only tried, because a request path tries them or holds them
across I/O: a session slot, a staging file and a confirmation attempt. An
entry whose lock is held at that instant is reported with `locked: true`:

- A locked slot shows only its index.
- A locked confirmation attempt shows only that it is locked.
- A locked staging entry keeps its file, target and hydration activity. Its
  other fields are unavailable, not zero-valued observations.

Trying those locks avoids waiting for their holders. It does not avoid all
interference: a SEQUENCE that arrives while the API holds a slot for its
brief copy sees a busy slot and retries with `NFS4ERR_DELAY`, as it would
for any other holder.

`/v1/staging` reports `incomplete: true` when it contains locked entries.
With an `incarnation` filter, a locked entry whose owner is unknown is kept
as a possible match rather than hiding a client's busy writer. Sessions and
confirmations have no response-level `incomplete` flag; their locked
entries are reported in place. `hydrating` is true until the hydration
worker completes, including after cancellation.

`/v1/store` is different. It reads the backend and, with `staging=1`, the
sidecars on disk, so it takes as long as that I/O does.

## Store-only mode

When nfsd will not start, or a host's store should be read without
disturbing a running server, `-store-only` serves the API without the NFS
listener. It requires `-status-addr` and one of `-registry` or `-root`;
`-staging` is optional and only `/v1/store?staging=1` reads it.

```sh
nfsd -store-only -registry registry.example:55556 -staging /var/lib/nfsd \
    -status-addr 127.0.0.1:9049
```

The process opens the client store read-only and never opens the staging
store, which would quarantine and truncate sidecars during recovery. `mode`
is `store-only`; `nfsd_id` and `stateid_epoch` are empty strings and
`wire_epoch` is zero, since the process issues nothing. The index,
`/v1/store` and profiling work; every resource that describes a running
server returns 503.

## The store report

`/v1/store` reads the client store and explains its current state. It uses
the same record definitions and lease rules as the server but never writes:
it does not create expired markers, remove collectable slots or age GC
candidates. It works in both modes. Every lease and collection judgement in
one report is taken against a single instant, `now`.

```sh
curl -s 'http://127.0.0.1:9049/v1/store' | jq .report.summary
curl -s 'http://127.0.0.1:9049/v1/store?identity=Linux%20NFSv4.1%20host' | jq .report
curl -s 'http://127.0.0.1:9049/v1/store?incarnation=incarnation:200000000000000e' | jq .report
curl -s 'http://127.0.0.1:9049/v1/store?staging=1&stateid=stateid:09cb5ee40000000000000001' | jq .report
curl -s 'http://127.0.0.1:9049/v1/store?staging=1' | jq .report.staging_summary
```

### Selectors

With no selector, the report covers every identity. At most one of
`identity`, `identity-hash`, `incarnation` and `stateid` may be given.
`identity` is the raw EXCHANGE_ID owner string; the others take their typed
spelling. An `incarnation` must be a directory inode, or the request gets
400. A `stateid` selects the incarnations holding its open marker. With
`staging=1` it also selects the incarnation of a staging sidecar that names
it; without `staging=1` the sidecars are not read, so a stateid known only
to retained staging matches nothing. To find retained staging, include
`staging=1`. The report echoes the selector under `filter`.

The clientid an NFSv4.1 client sees is a wire clientid, which only
`/v1/lookup` on the issuing process can resolve.

### Lifecycle states

For each identity, the report shows the stored identity string, its
`confirmed` and `pending` pointers and every incarnation. Each incarnation
has `roles`: any of `confirmed`, `pending` and `reboot-target`, where a
reboot target is the incarnation a confirmed or pending incarnation's
`reboot` pointer names. Its `lease_state` is `live`, `expired` (an
`expired` marker, or only dead lease slots) or `none`. Its `status` is:

| Status | Meaning |
| --- | --- |
| `ACTIVE` | Confirmed, with a live lease. |
| `EXPIRED` | Confirmed, with an expired marker or only dead lease slots. |
| `UNLEASED` | Confirmed, with no lease slot recorded. It can still acquire its first lease. |
| `PENDING` | Named by `pending`, waiting for its first CREATE_SESSION. |
| `REPLACED` | Retained as a reboot target until its state is purged. |
| `UNREACHABLE` | No pointer keeps it. Client collection removes it after it has aged. |

`usable` is true only for `ACTIVE`: the incarnation's stateids are
accepted. Each incarnation also shows its `client` and `update` records,
with the decoded principal; its lease and confirming slots, each naming the
nfsd which wrote it; the expired marker; `gc_after`; its open markers; and
temporary files. An open marker's `active` follows `usable`, and the
summary counts the inactive ones as `stale_open_markers`. A lease or
confirming slot is `collectable` once it has been expired for a full extra
lease; a temporary file is `collectable` once it is a lease old. These say
the record is old enough to be removed, not when it will be.

An ordinary read-only open of an existing file has no open marker; it
appears only in `/v1/opens` on the running process, not in this report.
Opens with a writer or staging, and creates, are marked even when opened
read-only, and a writer downgraded to READ keeps its marker. `/v1/opens`
shows which opens are `marked`.

### Staging join

`staging=1` reads the server's own staging directory: the `-staging`
directory of the serving or store-only process. The query never names a
path. Files are read as `.staging` and `.meta` pairs named with the inode as
16 hex digits, the names recovery looks up. The `quarantine/`, `aliases/`
and `sessions/` subdirectories are expected. `staging_summary` counts
entries, complete pairs, retired entries, data files, sidecars, data bytes
and allocated bytes.

Each sidecar is joined to an open marker by stateid, within its incarnation.
The joined entry carries the target directory and name, staged and
checkpointed size, sidecar version, owning incarnation and open owner,
access mode, base inode and size, dirty ranges, writer attributes, the
recovery key, the EXCLUSIVE4 verifier when present, and the `retired`,
`unlinked` and `guarded` flags. An entry is `retired` when its lease
expired and its acknowledged data is held for the client to reclaim. The
sidecar version is the on-disk layout, such as `v8`, not an NFS version.

A sidecar with no open marker is listed under its incarnation's `staging`.
In an unfiltered report, sidecars belonging to no incarnation in the store
are listed once under the top-level `staging`. A filtered report has not
read the other identities, so it cannot tell an orphan from another
client's file; it counts those sidecars in `staging_summary.other_clients`
instead. Quarantined files are listed under `quarantined`, as paths
relative to the staging directory.

The report shows only checkpointed state. Uncheckpointed writes and
in-memory hydration progress are absent; `/v1/staging` on the running
process shows them.

### Problems and notes

`summary.problems` counts defects anywhere in the report:

- Top-level `problems`: an unreadable quarantine directory, an unexpected
  file, directory or name in the staging directory, or a leftover metadata
  temporary.
- Each `quarantined` file.
- An identity's `problems`: a directory under `clients` whose name is not an
  identity hash, an entry that is not a directory, or an unexpected entry.
- Each `confirmed`, `pending` or `reboot` pointer marked `dangling` or
  carrying a `problem`.
- An incarnation's `problems`, such as an unreadable or missing client
  record, lease or GC record, and its `unknown` entries.
- A staging entry's `problems`: a missing half of a pair, or a sidecar that
  cannot be read.

Find each where it was recorded; the top-level list holds only the staging
directory's own defects. Malformed names are reported as found; the report
never invents a typed id for them.

Expired leases, stale open markers, unreachable incarnations, collectable
slots and temporaries, and retired staging entries are not problems. They
are the states the lifecycle is meant to pass through. A count of zero
therefore does not mean all staged writes were published or recovered.

A selector that matches nothing is not a problem either. An unregistered
identity, a stateid that no open marker names, an incarnation that has no
directory or is not under `/.nfs/clients`, and an identity directory inode
given as `incarnation` each get an explanation under `notes`. The stateid
note means only what was read: without `staging=1`, retained staging that
names the stateid was not looked for. A filtered report counts problems
only for the selected clients and, with `staging=1`, the staging
directory.
