<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd diagnostics API

nfsd serves a read-only JSON API about its own state. The `/v1`
resources are snapshots of the running process's memory: client records,
live sessions and slots, wire client records and the logical lease gate,
opens, staged writers and backchannel paths. Some of that state also has a
copy on disk. The client records are the files under `clients/` in the
staging directory, and `/v1/clients` shows the copy the process has made
durable. Session files under `sessions/` record each session's slots and
its wire clientid value, and `aliases/` records publication aliases; the
registration record a wire clientid indexes is process-local. The API
reads no file itself.

This package defines the response types and the HTTP handlers. The server
implements its `Source` interface; this package does not reach into server
internals. The server's snapshots are in
[diagnostics.go](../internal/server/diagnostics.go) and
[diagnostics_snapshots.go](../internal/server/diagnostics_snapshots.go).

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
| 400 | A filter or id that does not parse. |
| 404 | No resource at that path. |
| 405 | A method other than GET or HEAD. |
| 500 | Any other failure. |

## Resources

`GET /` is the index. It lists every resource and its filters, the meaning
of every state word the API uses, and a few general notes. Read it first.

| Path | Content | Filters |
| --- | --- | --- |
| `/v1/status` | Process identifiers, limits, reply-cache and request accounting, wire clientid issuer and hydration activity. | |
| `/v1/lookup` | Resolves one typed id; see [Lookup](#lookup). | `id` (required) |
| `/v1/clients` | The client records, one per client identity, and the wire clientids this process issued, each bound to an incarnation, with the CREATE_SESSION replay slot. See [Client records](#client-records). | `wire`, `incarnation`, `owner` (hex) |
| `/v1/incarnations` | Per incarnation: session group and lease gate, sessions and opens. | `id` |
| `/v1/sessions` | Channel limits, SEQUENCE status flags, backchannel sender and connections, whether the session has a file and was restored, and each slot. | `id`, `incarnation`, `slots=0` omits slots |
| `/v1/connections` | Live TCP connections: requests in flight, the sessions each carries in either direction, and callback path state. | `session` |
| `/v1/opens` | Open stateids with file, data inode, owner and access; expired stateid tombstones; the client views behind each handle. | `stateid`, `incarnation`, `file` |
| `/v1/staging` | The staging store in memory: each writer's target, owner, size, dirty and hydrated ranges and flags, and the failed tombstones. | `file`, `incarnation` |
| `/debug/pprof/` | Go profiling. Not in the index, and not JSON. `goroutine?debug=2` is the first thing to take during a stall. | |

On `/v1/staging`, a `file` filter applies to the failed tombstones too. An
`incarnation` filter does not, since a tombstone records no owner.

## Client records

`client_records` in `/v1/clients` lists the client records: one per client
identity, from its first confirmation to its expiry or destruction. A
reboot or takeover keeps the record and replaces its incarnation. Each shows the `identity_hash` that names its file under
`clients/`, the EXCHANGE_ID owner as hex, the principal, the `confirmed`
incarnation with its boot verifier and `confirmed_at`, and `expired`
between the expiry decision and the record's removal. A record whose update
failed to sync is shown as it was before the update.

A `wire` or `incarnation` filter keeps the record that confirms the
matched incarnation, and an `owner` filter the record of that
owner. An identity that registered but never confirmed has no record; its
pending incarnation appears only as a wire record.

## Process identities

Every JSON response, the index and errors included, carries `now`,
`server_id`, `stateid_epoch` and `wire_epoch`. The profiling pages
are not JSON and carry none of them.

- `server_id` is the staging directory's `server-id`
  ([server identity](../internal/staging/README.md#server-identity)). The
  server owner Linux sees at EXCHANGE_ID is `ternfs-nfsd/<server_id>`. It
  survives restarts and moves of the staging directory.
- `stateid_epoch` is the first four bytes of every stateid and session id
  the process issued.
- `wire_epoch` is the high 32 bits of every wire clientid it issued. It is
  zero until the NFS endpoint is bound.

Startup logs contain `server_id` and `stateid_epoch`, and the wire clientid
epoch once the endpoint is bound. The `session created` log line records a
wire clientid, its incarnation and its session id together, in the spellings
the API uses. It is the one place all three appear. Grep a fleet's logs for
these values to find the process which issued a stateid, session or
clientid.

## Typed ids

Ids the API routes on are typed `kind:hex` strings, lowercase and of fixed
width:

| Kind | Hex digits | What it is |
| --- | --- | --- |
| `wire-clientid` | 16 | The clientid EXCHANGE_ID returned: epoch in the high 32 bits, counter in the low. The record it indexes is process-local. |
| `incarnation` | 16 | A client incarnation, chosen at random when it registers and kept in its client record once confirmed. |
| `inode` | 16 | Any other inode: a filehandle, a staging data inode, a directory or a base. |
| `stateid` | 24 | The stateid `other` field. The first four bytes are the stateid epoch. |
| `session` | 32 | A session id: stateid epoch, incarnation, counter. |
| `identity-hash` | 64 | SHA-256 of the EXCHANGE_ID owner, naming its client record file. |

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

The clientid an NFSv4.1 client sees is not the incarnation its client
record holds. It is a wire clientid. No client record holds it, and the
registration record it indexes lives only in the issuing process's memory.
A session file records its value. Only the issuing process can resolve a
wire clientid.

```sh
curl -s 'http://127.0.0.1:9049/v1/lookup?id=wire-clientid:4000000100000001' | jq .
```

`/v1/lookup` takes one typed id. The answer gives `epoch` and
`issued_by_this_process` for kinds that carry an epoch, `known` for whether
this process has state for the id, a `note` explaining the result, and
`links` to the resources that describe it. An `identity-hash` is not looked
up; the answer links to `/v1/clients`, which lists each record's hash.

A stateid lookup also checks recovered staging from earlier daemon epochs,
so `issued_by_this_process=false` does not mean the stateid is unknown
here. If busy staging files prevent a complete check, the answer has
`incomplete=true`, and `known=false` then does not establish that no
staging names the stateid. Retry when the files are no longer busy.

## Snapshot consistency

Each store copies its own state under its own locks, one store at a time,
so counts across resources may disagree slightly. Registry and record locks
are taken normally; they are held only for map and field access.

Two locks are only tried, because a request path tries them or holds them
across I/O: a session slot and a staging file. An entry whose lock is held
at that instant is reported with `locked: true`:

- A locked slot shows only its index.
- A locked staging entry keeps its file, target and hydration activity. Its
  other fields are unavailable, not zero-valued observations.

Trying those locks avoids waiting for their holders. It does not avoid all
interference: a SEQUENCE that arrives while the API holds a slot for its
brief copy sees a busy slot and retries with `NFS4ERR_DELAY`, as it would
for any other holder.

`/v1/staging` reports `incomplete: true` when it contains locked entries.
With an `incarnation` filter, a locked entry whose owner is unknown is kept
as a possible match rather than hiding a client's busy writer. Sessions
have no response-level `incomplete` flag; their locked slots are reported
in place. `hydrating` is true until the hydration
worker completes, including after cancellation.
