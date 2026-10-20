<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# server

Package server is the NFSv4.1 server: connections and RPC framing,
compounds and operations, sessions and their slots, the logical lease,
client confirmation, the staging lifecycle, the publication alias log and
the backchannel. It builds on [clientstore](../clientstore/README.md),
[openstate](../openstate/README.md), [staging](../staging/README.md) and
[sessionstore](../sessionstore/store.go). How these fit together is in
[Design](../../DESIGN.md); what operators can rely on is in
[Deployment](../../DEPLOYMENT.md) and [Recovery](../../RECOVERY.md).

The main files are [server.go](server.go) (connections, compounds, the
sweeper), [ops_session.go](ops_session.go) (session operations and
SEQUENCE), [session_persist.go](session_persist.go),
[replay_intent.go](replay_intent.go) and [replay_decide.go](replay_decide.go)
(persistent sessions), [lease_gate.go](lease_gate.go) and
[client_group.go](client_group.go) (the logical lease),
[confirmation.go](confirmation.go), [ops_open.go](ops_open.go),
[staging_lifecycle.go](staging_lifecycle.go), [alias_log.go](alias_log.go),
[backchannel.go](backchannel.go) and
[backchannel_sender.go](backchannel_sender.go).

## Request handling

### Connections and admission

Each TCP connection has one reader. Before reading a frame, it takes one of
the connection's admission slots, 64 by default
(`-max-in-flight-per-connection`). Excess requests wait in socket buffers.
The read deadline is five minutes from the start of each frame read. An RPC
message is at most 2 MiB.

Each COMPOUND runs in its own goroutine; operations within it run in order.
Other calls, such as NULL, run on the reader. A callback REPLY frame is read
under an admission slot like any frame, but the slot is released at once:
the frame goes to the callback waiting for it, or is dropped and counted,
without running a request or touching session accounting. Replies share one
writer lock, each with its own five-minute write deadline, and may go out in
any order; the XID identifies each.

An identical request reusing an in-flight XID on the same connection is
dropped, and the original sends the one reply. Reusing that XID with
different bytes closes the connection. This guard lasts through the reply
write. It does not cover reconnects; the session slot does.

How a connection ends:

- A clean EOF may be a TCP half-close. Accepted requests run on and send
  their replies before the socket closes.
- A read timeout, reset, other read failure or a frame that does not parse
  closes the socket first and wakes requests waiting for reply work.
- A reply write failure, a panic in a request, a request refused or dropped
  without a reply, or a conflicting XID closes the connection.

Closing the socket does not stop requests already executing. Their backend
calls run to completion, and their replies are not sent. The handler
returns, and the connection is counted as closed by its first cause, only
when every worker has finished. A request releases its lease hold and its
reply work when its worker finishes: after its reply has been written, or
skipped because the connection had closed.

### Reply work

Before executing, an admitted request reserves reply work from a
process-wide budget, `-reply-work-budget-mib` (8 GiB by default). The
charge is eight times the sum of its reply capacity and a 65 KiB
allowance, plus a READDIR preparation allowance when the compound has one.

The reply capacity is the sum of the per-operation bounds execution
enforces, so a WRITE compound reserves little and a 1 MiB READ about
8.5 MiB. A compound with an operation whose result is sized at execution,
such as READDIR or READLINK, reserves the session's full response size, or
its cache size when it asks for caching.

A request that does not fit waits, keeping its admission slot, until work
is released. Admission is decided as work is charged: a request whose
connection has closed is refused even when capacity is free, so it neither
runs nor holds work for a reply nobody will read. A request larger than the
whole budget is refused, and its connection is closed.

## Sessions and slots

### Negotiated limits

Every compound starts with SEQUENCE, except a lone EXCHANGE_ID,
CREATE_SESSION, DESTROY_SESSION, DESTROY_CLIENTID or BIND_CONN_TO_SESSION,
which must then be the only operation. DESTROY_SESSION of the compound's
own session must be its last operation.

CREATE_SESSION limits the fore channel to 2 MiB requests and responses,
64 KiB cached replies, 128 operations and between 1 and 64 slots. Channel
attributes too small to carry a SEQUENCE get `NFS4ERR_TOOSMALL`. `PERSIST`
is never granted: the Linux client behaves the same either way, except that
it would switch exclusive creates to GUARDED4 and lose their verifier.

SEQUENCE checks the compound against its session: `NFS4ERR_BADSLOT`,
`NFS4ERR_BAD_HIGH_SLOT`, `NFS4ERR_TOO_MANY_OPS` and `NFS4ERR_REQ_TOO_BIG`,
and `NFS4ERR_REP_TOO_BIG` or `NFS4ERR_REP_TOO_BIG_TO_CACHE` when even a
minimal reply cannot fit. With `sa_cachethis` set, the whole reply must fit
the cache limit; it is not silently served uncached.

Each operation's reply bound is then checked before it is dispatched. An
operation that would exceed the remaining response or cache budget gets
`NFS4ERR_REP_TOO_BIG` or `NFS4ERR_REP_TOO_BIG_TO_CACHE` without executing;
earlier operations may already have taken effect. When a GETFH follows an
OPEN or CREATE before any other handle change, the OPEN or CREATE must also
have room for the results through that GETFH, plus the next operation's
error result, so that the new handle is never lost after its effect. A
READDIR or READLINK in that span gets the OPEN or CREATE
`NFS4ERR_UNSAFE_COMPOUND` before anything is created. A read-only operation
with a variable result can be replaced by a size error after its result is
built. A mutation that exceeds its fixed bound after executing cannot be
recast as a pre-effect failure: the session is retired and the connection
closed.

### Slot replay

A slot records the sequence number, a digest of the request and the
effective user of its last request. The digest leaves out the SEQUENCE
arguments, because the Linux client re-encodes a retransmission with its
current highest slot. A request on a slot is:

| Sequence number | Result |
| --- | --- |
| One more than the slot's, or 1 on an unused slot, while the slot is not in use | A new request. |
| Equal, while the original is still running | `NFS4ERR_DELAY`. |
| Equal, with a different digest or user | `NFS4ERR_SEQ_FALSE_RETRY`. |
| Equal, after the original completed | The recorded reply. |
| Anything else | `NFS4ERR_SEQ_MISORDERED`. |

A slot whose lock is held at that instant, for example by a diagnostics
snapshot, gives `NFS4ERR_DELAY`.

A completed request's recorded reply is the whole reply when the client set
`sa_cachethis`. Otherwise it is the SEQUENCE result alone, followed, when
the compound had more operations, by `NFS4ERR_RETRY_UNCACHED_REP` for the
next one. `NFS4ERR_OP_ILLEGAL` and the errors for operations NFSv4.1
removed keep their original status, as RFC 8881 section 2.10.6.1.3
requires. A session that is destroyed or fatal drops its slots' replies.

An operation whose effect cannot be established retires its session as
fatal right after it returns, so that no recorded reply can assert an
outcome the server does not know. An operation rejected before dispatch,
by the compound checks above, runs no effect of its own and does not by
itself retire the session; effects of earlier operations in the compound
remain. On the native backend every mutation error has an
unknown outcome, timeouts included, except a REMOVE that finds its edge
already gone. The local test backend treats its recognized syscall errors
as definite rejections; any other error from it, such as an injected
transport or decoding error, is unknown. Staging I/O errors in WRITE,
COMMIT, SETATTR and a sized OPEN count too.

A slot restored after a restart has two more cases, described under
[Restart replay decision](#restart-replay-decision): it accepts, once, any
sequence number later than the one it recorded, and a retransmission whose
record is an intent runs again.

### Reply cache accounting

Cached replies share a process limit of 64 MiB. A new request reserves its
buffer before it commits: the full reply budget if it asks for caching,
otherwise the size of its SEQUENCE result plus, when another operation
follows, that operation's error result. The slot's previous reply stays
attached and replayable until the new request commits, so the total can
exceed the limit by at most one buffer per slot. A reservation that does not
fit gets `NFS4ERR_RESOURCE`. A buffer is trimmed to what was recorded once
the reply is complete.

Under pressure SEQUENCE asks sessions to use fewer slots. Below half the
limit, the target highest slot is the session's highest. Once the cache is
three quarters full, a session charged more than its share is given half
its slots as the target, and is sent CB_RECALL_SLOT when it has a usable
backchannel. It keeps the reduced target until the cache is below half full
again; see [Callback sender](#callback-sender).

### SEQUENCE admission

SEQUENCE does no backend I/O. Holding the slot's lock, it takes in order:

1. a provisional hold on the client's logical lease, which gets
   `NFS4ERR_DELAY` while expiry is being decided or the incarnation is being
   replaced, and `NFS4ERR_BADSESSION` once it has expired or been replaced;
2. a use of the session, so its record is not released;
3. the connection's association with the session, against its quota
   (`NFS4ERR_RESOURCE`);
4. a reply buffer (`NFS4ERR_RESOURCE`);
5. a renewal check from memory: `NFS4ERR_DELAY` with no lease record
   credit; `NFS4ERR_BADSESSION` for a stale or fleet-expired record; and
   `NFS4ERR_DELAY`, with a verify requested, when this process's published
   lease is not in the future or the confirmed pointer has not been read in
   the last 45 seconds.

Any failure releases what was taken, in reverse, and changes nothing.
Otherwise the request commits, which cannot fail: the association is
published, the hold becomes active and the refresher is told the lease is
needed. CREATE_SESSION waits up to two seconds for its client's first lease
publication, so a client's first SEQUENCE normally does not get DELAY; the
first SEQUENCE after an idle period still can.

### Connection associations

A connection is associated with a session by CREATE_SESSION,
BIND_CONN_TO_SESSION or its first SEQUENCE for it. A connection carries
requests for at most 64 sessions, and callbacks for at most 64; destroyed
sessions are pruned from both. A request that would need a new association
on a full connection gets `NFS4ERR_RESOURCE` before it changes anything.
DESTROY_SESSION needs the connection to be associated with the session
(`NFS4ERR_CONN_NOT_BOUND_TO_SESSION`).

## Persistent sessions

With a staging directory, sessions are kept on disk so that a
retransmission after a restart gets its original reply (RFC 8881
section 2.10.6.5). Without one, sessions live in memory only.

### Session files on disk

Each session has one file under `sessions/` in the staging directory,
written by [sessionstore](../sessionstore/store.go). The file's layout is
fixed for the session's life: a header, then two record areas per slot.
The header holds the session record: the incarnation, the wire clientid's
value, both channels' attributes, the callback program, whether a
backchannel was requested and the callback credential. These values let a
restart rebuild the session; the wire clientid's registration record itself
is not stored. Nothing grows and nothing is compacted.

A slot keeps only its last request's record. Each write goes to the area
that does not hold the slot's newest record, and every record carries a
CRC32C. On load, the valid record with the later sequence number, in serial
order across wraparound, wins, and at equal sequence numbers a completion
beats its intent. A torn write therefore damages only a record the slot no
longer needs: a torn new request leaves the previous request's record, and a
torn completion leaves its intent. Nothing is fsynced. A `pwrite` survives
the death of the process, but not necessarily a host crash. A file of
another format version is not used. The byte layout and the version history
are documented in [store.go](../sessionstore/store.go).

The server writes:

- the file, when CREATE_SESSION has its slot and before the session is
  committed;
- a completed request's slot record, before its reply goes to the socket:
  the whole reply when cached, the SEQUENCE result otherwise;
- an intent, before a protected effect; see
  [Intents and the mutation gate](#intents-and-the-mutation-gate).

If CREATE_SESSION cannot make the file, it removes what the attempt left
and the session lives in memory only. If even that removal fails,
CREATE_SESSION gets `NFS4ERR_SERVERFAULT`, since a restart could find the
file.

If a record write fails, the file no longer holds the slot's newest
record. It is unlinked and the session goes on in memory only. If the
unlink fails too, the file is *poisoned*: no further protected effect is
permitted (`NFS4ERR_SERVERFAULT`), and the session is retired once its slot
is released. An effect permitted earlier, whose intent the file holds, may
still finish. A poisoned file lacks only replies, as a file does after a
kill before a reply write, so a restart from it is as safe as one after a
kill.

### Session restoration

At startup, before listening, the server reads every session file. A file
is rebuilt into a session only when:

- it is readable and of the current format;
- it recorded at least one request, since otherwise there is nothing to
  answer and the client may never have learnt of the session;
- its session record decodes;
- its incarnation is still the confirmed one;
- the session gets a lease record, reply-cache space for every recorded
  reply, a session record and a client group.

Any other file is removed, and its session is unknown: a client using it
gets `NFS4ERR_BADSESSION`. Every session ID found on disk, restored or not,
is never given to a new session. Restored slots are marked as restored.
The server then starts each rebuilt incarnation's lease
[bootstrap](../clientstore/README.md#bootstrap) and waits up to two seconds
for each; one not published by then is logged, and startup continues.

A restored session is retired later like any other, for example when its
incarnation expires. A client using it then gets `NFS4ERR_BADSESSION`.

### Intents and the mutation gate

RENAME, REMOVE and CREATE of a directory or symlink write an intent to the
compound's slot before their effect. The intent names the operation's index
in the compound and the identity of what was there before: the source or
removed edge, by inode and creation time, and the names and directories.
TernFS never reuses an inode ID, so a later process can decide from TernFS
whether the effect happened. If the intent cannot be written, the file is
unlinked and the session goes on in memory, so the effect runs without
protection and the session is not restored. Only if the file stays on disk
without the intent, because the unlink failed, does the operation get
`NFS4ERR_SERVERFAULT` before its effect.

A file can protect only one effect per request, and a retransmission after
a restart runs the whole compound again. So while a session's file is on
disk, each compound may run at most one mutating operation. Before a second
one of any kind runs, OPEN, WRITE, CLOSE, SETATTR and COMMIT included, the
file is removed and the session goes on in memory only. A first mutating
operation that changed nothing still counts. If the file cannot be removed,
the second operation gets `NFS4ERR_SERVERFAULT` before its effect, and the
session is retired.

### When a session file is removed

| Event | Effect |
| --- | --- |
| The session ends: destroyed, expired, replaced or retired | Removed with its last use. If the unlink fails, a restart may restore it. |
| A record write fails | Removed; the session goes on in memory. |
| A compound's second mutating operation | Removed before it runs. |
| The session's first CB_COMPOUND | Removed before it is sent, because the file does not record the callback sequence. CB_NULL probes leave the file alone. |
| An accepted BACKCHANNEL_CTL | Removed before the callback program and credential change. |
| A CREATE_SESSION that inserts no session | Removed. |
| Unusable or not restored at startup | Removed. |

A session whose file has gone continues in memory and is not restored by a
later restart. When a removal that must come first fails, the action is
refused: the operation gets `NFS4ERR_SERVERFAULT`, or the callback is not
sent, and the session is retired.

### Restart replay decision

A retransmission to a restored slot is answered in one of three ways:

- Its record is a completed request's: the recorded reply, as for any slot.
- Its record is an intent with no reply: the earlier process was running the
  request when it died, between the intent and the reply record. The
  compound runs again, and the operation the intent names is decided against
  TernFS first.
- Its sequence number is later than recorded, in serial order, by any
  amount: the records of later requests were never written. It runs as a
  new request, once; ordinary rules apply from then on. An unused restored
  slot likewise takes any first sequence number. The slot must not be in
  use. While the file was on disk, the request could have
  had at most one mutating operation. RENAME, REMOVE and CREATE of a
  directory or symlink would have written an intent. The others, alone,
  repeat safely or fail: an exclusive OPEN matches its verifier, a GUARDED
  one gets `NFS4ERR_EXIST`, and a CLOSE finds its publication.

When a compound runs again with an intent, each operation is decided:

| Operation | Decision | Result |
| --- | --- | --- |
| RENAME at the intent's index, same names and directories | Source edge still in place | Not applied: runs now. |
| | Source edge gone, destination holds the source inode | Applied: success, without running again. |
| | Anything else | `NFS4ERR_DEADSESSION`. |
| REMOVE at the intent's index, same name and directory | Edge still in place | Not applied: runs now. |
| | Edge gone or replaced | Applied: success. |
| CREATE at the intent's index | Always | `NFS4ERR_DEADSESSION`: nothing shows whether it made what is, or is not, at the name. |
| Another operation, or other names or directories, at the intent's index | | `NFS4ERR_DEADSESSION`. |
| A mutating operation before the intent's index | | `NFS4ERR_DEADSESSION`. |
| Any mutating operation, when the intent cannot be decoded | | `NFS4ERR_DEADSESSION`. |
| A non-mutating operation | | Runs. |

A backend lookup that fails while deciding returns its error, mapped to
NFS. An operation decided as not applied runs on its recorded intent only
if the intent it would write now is the same; if its target has changed
since the decision, it gets `NFS4ERR_DEADSESSION`. After
`NFS4ERR_DEADSESSION` the client re-establishes its session. The client's
view of this is in [Client recovery](../../RECOVERY.md#client-recovery).

## Lease gate

Each client incarnation has a group here: its sessions, its wire record,
the requests running for it and its logical lease gate. The model and its
timings are in [Two leases](../../DESIGN.md#two-leases).

A SEQUENCE takes a provisional hold, which becomes active when it commits.
An active hold ends when the request's worker finishes, and then extends
the quiet period to one lease from that moment. A validated CB_SEQUENCE
reply also extends it. While any hold exists or the quiet period runs, the
lease is held and this process does not expire the client.

### Expiry decision

`decideExpired` is the only path that writes the `expired` marker. Its
caller holds a destruction hold on the client, so it never runs while a
confirmation protects it. If the lease is held, or a decision is already
in progress, it changes nothing. Otherwise the gate turns to checking,
which makes new SEQUENCEs get `NFS4ERR_DELAY`, and it reads the client
store:

| Observation | Action |
| --- | --- |
| A read error | The gate returns to live. Nothing is published or expired; a later sweep tries again. |
| The incarnation directory is gone | Expired here. |
| The confirmed pointer names another incarnation | Its open markers and lease slots are purged, then expired here. |
| An `expired` marker exists | Another process decided; expired here. |
| Some slot is live, or no slot exists yet | The gate returns to live. A first OPEN may still be registering its lease. |
| Slots exist and none is live | A renewal or hold that arrived during the check wins. Otherwise the marker is written, and the client is expired here even if that write fails. |

Expiring a client here retires its wire record, its sessions, cancelling
their callbacks, and its local open state. Once expired, its SEQUENCEs get
`NFS4ERR_BADSESSION` and a CREATE_SESSION for it gets
`NFS4ERR_STALE_CLIENTID`.

The refresher's terminal steps, for a stale or fleet-expired record, take
another path: they close the gate and retire the incarnation's sessions
under a destruction hold, but keep its wire record. A committed
CREATE_SESSION result still replays, and a new CREATE_SESSION for the
incarnation inserts no session and gets `NFS4ERR_STALE_CLIENTID`.

### Sweeper

The sweeper runs every 45 seconds. Each tick it:

1. settles confirmations left unresolved;
2. schedules client-store collection for identities that asked for a
   recheck;
3. retires the staging of expired clients; see
   [Sweep and startup grace](#sweep-and-startup-grace);
4. runs the expiry decision for every client with opens, cached store
   state or a group here;
5. sweeps client views and compacts the
   [alias log](#publication-alias-log);
6. forgets groups kept only by a quiet period that has ended;
7. has idle lease records freed.

A panic in a tick is logged, and the next tick runs.

## Client confirmation

The client store makes the confirmation writes and decides each outcome;
see [Confirmation](../clientstore/README.md#confirmation). The server
decides when to confirm and acts on each result once.

### Protection and destruction holds

Before the first write, the server closes admission for the predecessor P
in this process, so its new requests wait, and registers protection for the
pending incarnation N and P. While protected, nothing in this process
marks, purges, expires, retires, reclaims or collects them:

- P's SEQUENCE, BIND_CONN_TO_SESSION and CREATE_SESSION get `NFS4ERR_DELAY`,
  and DESTROY_CLIENTID gets `NFS4ERR_CLIENTID_BUSY`.
- N's CREATE_SESSION slot stays held for its sequence, so its retry waits
  or gets the settled outcome.
- A new EXCHANGE_ID for the identity gets `NFS4ERR_DELAY`.

Every destructive action takes a destruction hold on the incarnation, or
on the identity for collection, from its first check to its last effect,
backend waits included. A new hold is refused once protection is
installed. Registration installs protection first and then waits for the
holds already admitted to end. Every destructive action therefore either
finishes before protection is complete or does not start.

### Applying a result

| Result | Action | CREATE_SESSION gets |
| --- | --- | --- |
| Committed | Mark P replaced; drop P's sessions and wire record and expire its local open state; insert N's session; purge P on the attempt's evidence; unregister. | `NFS4_OK` |
| Rolled back | Reopen P; unregister. | `NFS4ERR_DELAY` |
| Conservative | Mark P replaced and drop its sessions here, keeping its durable state; retire N's wire record; unregister. | `NFS4ERR_STALE_CLIENTID` |
| Unresolved | Nothing changes. N and P stay protected. | `NFS4ERR_DELAY` |

An unresolved result is retried by the same CREATE_SESSION sequence and by
the sweeper. Lease expiry and elapsed time never resolve it. Application
runs in stages, each recorded when it returns, so a panic resumes at the
first unfinished stage and never repeats one that is not idempotent. Once
published, an OK reply stays OK.

### Resumed purge

A purge that fails, or is interrupted, stays owned by its durable evidence:
N confirmed and N's `reboot` symlink naming P. Every successful
CREATE_SESSION reads that evidence and resumes the purge. A purge known to
have failed is also noted in a bounded set of hints, so that an EXCHANGE_ID
that otherwise makes no store call retries it. A purge that a confirmation
protects is deferred, keeping the hint and the evidence. A failed purge
never changes a committed reply.

## Opens

Stateids, owners, downgrades and what readers see are in
[openstate](../openstate/README.md); the client-store side of opens is in
[Open markers](../clientstore/README.md#open-markers).

### Stateid checks

READ, WRITE, SETATTR and OPEN_DOWNGRADE resolve a stateid that is not a
special stateid through one lookup, in this order:

1. The local open. Another client's stateid gets `NFS4ERR_BAD_STATEID`.
2. If it is not a local open, recovered staging: the stateid of an earlier
   process's writer, presented on the staging inode or its base, and owned
   by this client.
3. A failed writer's staging gets `NFS4ERR_IO`, after ordinary stateid
   errors.
4. For recovered staging, the sidecar must still name the stateid and the
   client. If the client store definitely shows the open inactive, because
   its marker is gone, its incarnation is no longer confirmed or its lease
   is dead, the staging is retired and the operation gets
   `NFS4ERR_EXPIRED`. Otherwise the server adopts the writer as a live open.
   When adoption is refused, because the staging is read-only or the owner
   already has a live open of the file, READ, WRITE and SETATTR run on the
   sidecar state.
5. For a local open, its durable state must be active: see
   [Stateid scope](../openstate/README.md#stateid-scope). An open whose own
   marker is definitely gone loses its entry.

The decisive check before a retirement or a local expiry runs under a
destruction hold, so it gives `NFS4ERR_DELAY` while a confirmation protects
the client. A store read error gives the store's NFS status, or
`NFS4ERR_DELAY`.

Other operations differ:

- CLOSE looks up the local open, and otherwise the recovered staging found
  the same way, and closes that writer through its sidecar. Under the
  pathname and file locks it checks again whether another request has
  adopted it meanwhile; if so the live open answers.
- OPEN_DOWNGRADE of a recovered writer has no sidecar fallback. If the
  owner already has a live open of the file, it gets `NFS4ERR_DELAY` before
  anything is published, and both opens are kept; see
  [Downgrades](../openstate/README.md#downgrades).
- TEST_STATEID and FREE_STATEID use the open state's own rules, plus this
  client's recovered staging, and read no durable state; see
  [Stateid scope](../openstate/README.md#stateid-scope).
- A SETATTR with a special stateid skips the lookup and uses the client's
  writer of the handle, or the handle, directly. A SETATTR with another
  stateid that changes the size needs the lookup to pass. A timestamp-only
  SETATTR whose stateid fails the lookup gets the lookup's error before any
  effect if the stateid is known to be another client's: live, expired or
  naming its recovered staging. Otherwise it goes on with the client's
  writer of the handle, or the handle, and the remaining checks; a failed
  writer there still gets `NFS4ERR_IO`.

### Writer recovery mechanics

The recovery contract is in
[Writer recovery](../../RECOVERY.md#writer-recovery). The server has three
paths:

- **Old stateid.** As above. READ, WRITE, SETATTR and OPEN_DOWNGRADE adopt
  the writer. CLOSE closes it through its sidecar unless another request
  adopted it meanwhile. COMMIT has no stateid: it syncs the client's
  recovered writers of the handle without adopting them.
- **Reopen by pathname.** A writable or creating OPEN at a name selects a
  unique staging entry for that target whose read-only flag matches the
  requested access. Within the same incarnation,
  any open-owner may take shared staging whose stateid an earlier process
  issued. From another incarnation, the sidecar must carry this client's
  recovery key and the same open-owner, and that incarnation must be
  expired. The check holds a destruction hold on it and runs the expiry
  decision. More than one candidate gets `NFS4ERR_EXPIRED`.
- **Cold writable CLAIM_FH.** When this process has no current name for
  the handle, typically after a restart, it preselects, without locks, a
  unique healthy, linked writer of this client whose recorded name holds the
  handle and whose known owner matches. None gives `NFS4ERR_STALE`, and more
  than one `NFS4ERR_EXPIRED`. Under the pathname lock, the pathname
  selection must pick the same entry, or the OPEN gets `NFS4ERR_STALE`.

OPEN supports `CLAIM_NULL` and `CLAIM_FH`. A read `CLAIM_FH` needs no name
and uses the client's view. `CLAIM_PREVIOUS` gets `NFS4ERR_NO_GRACE`. Every
other claim type gets `NFS4ERR_NOTSUPP`.

### Exclusive creates

An EXCLUSIVE4 or EXCLUSIVE4_1 OPEN of a missing name creates it, recording
the verifier in staging. EXCLUSIVE4_1 sets no attributes; its attribute set
is empty. A retry at a name that exists reuses the staging only if its base
is the inode now at the name, it is exclusive with the same verifier, and
it belongs to the same client and open-owner, with the same read-only
access. It must also be this process's live open with the same stateid, or
a writer an earlier process left. Otherwise the retry gets `NFS4ERR_EXIST`.
A publication at the name since then invalidates the verifier. The verifier
is checked only within one process; it is not a reservation across hosts.
A retry by the same client boot after its lease expired gets
`NFS4ERR_EXIST`; see
[Lease expiry leaves retired staging](../../DEPLOYMENT.md#lease-expiry-leaves-retired-staging).

## Publication alias log

Aliases in memory, and how long they live, are in
[openstate](../openstate/README.md#publication-aliases). The server also
records them in `aliases/published.jsonl` under the staging directory, one
JSON line per alias with its client, handle, data inode and expiry.

- Each publication appends its alias, with an expiry one lease ahead.
  Appends are not synced; losing them leaves old handles showing old
  content, as before the log existed.
- Each sweep rewrites the log with the aliases still in memory, through a
  temporary file and a rename. An alias whose handle the client still has
  open is written with an expiry one lease from then. A publication waits
  for a rewrite in progress, since its append to the file being replaced
  would be lost.
- At startup the log is read and every alias whose recorded expiry is still
  in the future is installed. Reading stops at the first line that does not
  parse, keeping the aliases before it.

An alias of a handle that stayed open through a long outage can therefore
have expired by the time the process starts again.

## Staging lifecycle

### Sweep and startup grace

Each sweep groups the staging entries by client and, for each client,
takes a destruction hold and runs the expiry decision. If the client is
expired, its staging is retired: a linked writer is retired in place, an
unlinked one is discarded, and one already retired and now unlinked is
quarantined. A writer rebound since the entry was read is left alone.

For the first lease after startup, 90 seconds, this staging sweep does not
run. Nothing else is held back: the sweeper's ordinary client expiry still
runs, and retires the staging of a client's local opens when it expires
the client, and the check before a namespace change can still retire an
inactive recovered writer. This is local cleanup only, not an NFS grace
period.

Before a namespace change, every non-retired writer at the name is checked:
it counts as active while its durable open marker is active or a
confirmation protects its client, and is otherwise retired; see
[Writer lifecycle](../../DESIGN.md#writer-lifecycle).

### Failed-writer tombstones

A writer whose namespace operation has an unknown outcome, or whose
publication failed in hydration or LinkFile while other opens read it, is
quarantined. Its staging entry stays as a tombstone for the life of the
process; see
[staging](../staging/README.md#startup-recovery-and-quarantine). READ and
WRITE with its stateid get `NFS4ERR_IO`. The writer index in
[client views](../openstate/README.md#client-views) remembers it too, so
that COMMIT and SETATTR through the handle still get `NFS4ERR_IO` after its
CLOSE, rather than COMMIT acknowledging the failed writes with the current
verifier. Other writers of the client remain usable. Tombstones are not
persisted; a restart changes the write verifier.

### Hydration

The first write or size change to a staging with a base starts its
background hydration. Across the process, at most two background base reads
run at once. Each waits up to 10 ms for foreground reads in progress to go
first. The last write CLOSE, or a downgrade to READ, cancels the background
worker and finishes the remaining ranges with foreground reads before
uploading the file. The mechanics are in
[Overlay files](../staging/README.md#overlay-files).

## Backchannel

### Grants and binding

CREATE_SESSION with `CREATE_SESSION4_FLAG_CONN_BACK_CHAN`, on a connection
that can carry callbacks, is granted a backchannel there. The back
association is reserved before the creation slot, and also on a connection
that replays a cached CREATE_SESSION or retries one held for confirmation.
The callback credential is the first `AUTH_NONE` or `AUTH_SYS` entry
offered, sent verbatim. A list offering only `RPCSEC_GSS` gets
`NFS4ERR_NOENT` and one with no usable entry `NFS4ERR_INVAL`, both before
the slot and only when a backchannel is asked for. Only `SP4_NONE` state
protection is offered.

BIND_CONN_TO_SESSION (RFC 8881 section 18.34.3) accepts FORE, BACK,
FORE_OR_BOTH and BACK_OR_BOTH; anything else gets `NFS4ERR_INVAL`. An
existing association that satisfies the request is returned unchanged; one
that does not gets `NFS4ERR_INVAL`, since a BIND never changes an existing
association. With none, FORE establishes the fore channel only and BACK the
back channel only. FORE_OR_BOTH establishes the fore channel, and the back
channel too when the session asked for a backchannel and the connection can
carry one; BACK_OR_BOTH establishes the back channel, and the fore channel
under the same condition. A back channel the session or connection cannot
have gets `NFS4ERR_INVAL`. The decision and both quotas are taken in one
step, so concurrent BINDs are ordered and `NFS4ERR_RESOURCE` changes
nothing.

BACKCHANNEL_CTL replaces the callback program and credential, after
removing the session file; see
[When a session file is removed](#when-a-session-file-is-removed).

### Callback sender

Callbacks use ONC RPC version 2, the session's callback program, and
program version 1, the `NFS_CB` version in the NFSv4.1 XDR, which Linux
sends and accepts; RFC 8881 section 18.36.3's prose says 4. Only two
callbacks are sent:

- CB_NULL probes each new back connection, after the reply that bound it.
  Until it is answered, for at most ten seconds, the path counts as up for
  the status flags only. After that it counts as down.
- CB_COMPOUND carrying CB_SEQUENCE on slot 0 and CB_RECALL_SLOT, under
  reply-cache pressure; see [Reply cache accounting](#reply-cache-accounting).

SEQUENCE's status flags come from the session's sender and the client's
other sessions, and each is decided on its own.
`SEQ4_STATUS_CB_PATH_DOWN_SESSION` is raised when the session asked for a
backchannel, or has a recall pending, and its path is not up.
`SEQ4_STATUS_CB_PATH_DOWN` is raised when the session's path is not up,
some session of the same client asked for a backchannel, and none of the
client's sessions has a path up. A session that asked for no backchannel
can therefore get `SEQ4_STATUS_CB_PATH_DOWN` alone.
`SEQ4_STATUS_BACKCHANNEL_FAULT` is raised after a fault; see
[Reply validation](#reply-validation).

Each session has one sender, with at most one callback outstanding. It goes
only to a connection whose path is up, with a ten-second timeout and
server-wide XIDs. An unanswered callback is never resent while its
connection is up; a timeout only marks the path unresponsive. After a
disconnect the same slot and sequence go out on another back connection
that is up, or wait for a BIND. A late, validated reply marks a timed-out
path up again. The frame is checked against the negotiated back channel
limits each time it is sent; if the credential has grown past them, the
recall is dropped unsent.

### Reply validation

A reply is validated whole before anything in it is used. A valid
CB_SEQUENCE OK consumes the sequence and renews the client's logical lease;
a CB_RECALL_SLOT error after it is the client declining the recall, which
is recorded and not retried. `NFS4ERR_DELAY` from CB_SEQUENCE is retried
with the same sequence up to three times, with a growing backoff, and then
given up. Any other error, or a malformed, truncated or inconsistent reply,
sets `SEQ4_STATUS_BACKCHANNEL_FAULT` and stops recalls for that session,
without renewing. Replies with unknown XIDs are dropped and counted. A
client that never asks for a backchannel sees no backchannel status flags.
There are no delegation callbacks.
