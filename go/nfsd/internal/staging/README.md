<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# staging

Package staging keeps the local copy of every writable open's file on the
nfsd host until CLOSE publishes it. Each staged writer has a sparse data
file and a sidecar of metadata, both named by the writer's transient TernFS
inode. The server decides when to create, publish, retire, detach and
discard a writer; this package stores its bytes and checkpoints and
recovers them after a restart.

[store.go](store.go) defines the `Store` and `File` interfaces.
[local_store.go](local_store.go) is the directory-backed store and its
startup recovery, [local_file.go](local_file.go) one staged file, and
[sidecar.go](sidecar.go) the sidecar encoding. The writer lifecycle the
server drives is in [Design](../../DESIGN.md#writes-and-publication); what
an operator can recover is in [Recovery](../../RECOVERY.md#startup-and-quarantine).

## Directory layout

```text
<staging>/
    <inode>.staging                 data, sparse
    <inode>.meta                    sidecar
    .<inode>.meta.tmp-<random>      sidecar being written
    quarantine/<inode>-<random>/    a pair moved aside
    aliases/                        the server's publication alias log
    sessions/                       the server's session files
```

`<inode>` is the transient inode as 16 lowercase hex digits. The directory
is created mode 0700 and sidecars are written mode 0600. This package owns
the pairs, their temporaries and `quarantine/`. It skips `aliases/` and
`sessions/`, which belong to the server; see
[Filehandles and client views](../../DESIGN.md#filehandles-and-client-views)
and [Requests and sessions](../../DESIGN.md#requests-and-sessions).

## Overlay files

A staged file has a base when it replaces an immutable inode. A create's
base is the empty placeholder published at its name. With a base, the data
file is an overlay:

- `dirty` ranges hold the writer's bytes and are authoritative.
- `hydrated` ranges hold clean base bytes already copied locally.
- Anything else is read from the base when needed.

A read fills its missing ranges from the base, then copies them in where
they are still clean. A write marks its range dirty. A truncation marks the
cut range dirty, so regrowing the file reads zeros, not base bytes. Copying
in never overwrites dirty bytes. Without a base the data file is plain
local data.

Hydration copies the clean ranges in. `StartHydration` runs one background
worker for the file, copying clean gaps in 1 MiB chunks until none are
left, a read fails or it is cancelled. `FinishHydration` cancels and waits
for that worker, then copies the remaining gaps with four parallel workers
until the file is complete. Removal cancels hydration. When hydration
starts, how many run at once and how foreground reads take priority is the
server's policy; see [Design](../../DESIGN.md#writes-and-publication).

## Sidecars

A sidecar records what a later CLOSE or recovery needs:

| Field | Meaning |
| --- | --- |
| Directory, name | Where CLOSE publishes. |
| Cookie | The TernFS construction cookie for linking the transient inode. |
| Stateid, incarnation | The owning open and client incarnation. |
| Open owner, owner known | The open-owner, to separate writers of one client; an empty owner is valid. |
| Recovery key | A hash of the client's stable identity, boot verifier and principal, for recovery after its incarnation expires. |
| Base inode, base size | The immutable version being replaced, or a create's placeholder. |
| Size, dirty ranges | The checkpointed logical size and dirty ranges. |
| Attributes | Change, mtime, atime and ctime, and whether metadata changed. |
| Exclusive, verifier | An EXCLUSIVE4 create and its verifier. |
| Read-only | A create with read-only share access whose initial size or times are staged. |
| Unlinked | Detached from its publication target. |
| Guarded | A namespace operation was in flight. |
| Retired | Its lease expired; the data is kept for recovery. |

The format is NFS8 and is the only one read or written. Decoding requires
the exact layout: known flag bits only, normalized dirty ranges, sizes that
fit a signed 64-bit offset and no trailing bytes. The byte layout is
documented beside the encoder in [sidecar.go](sidecar.go).

## Checkpoints

`Sync`, used by stable WRITE, COMMIT and size changes, syncs the data file
and then saves the sidecar if the checkpoint has changed: live size, dirty
ranges, attributes and the metadata flag. Every sidecar save writes a
temporary in the same directory, syncs it, renames it over the sidecar and
syncs the directory. A failed save marks the checkpoint dirty, so the next
`Sync` tries again.

A checkpoint is metadata: the size and dirty ranges that make the data
file's bytes authoritative. It does not copy the bytes. A later write
inside a checkpointed dirty range changes the same data file, so it can
survive a restart even though no newer checkpoint records it.

Namespace updates (`SetGuard`, `Detach`, `Retarget`) save only the
namespace fields on top of the last data checkpoint. They never checkpoint
unstable data. A failed namespace save keeps the new state in memory and
marks the checkpoint dirty for `Sync` to repair.

`Retire` cancels hydration, syncs the data, saves the full checkpoint with
`Retired` set and closes the data file. It does nothing if the file has
been rebound to another owner since the caller checked. `Rebind` gives a
writer a new incarnation and stateid, on recovery or when an earlier CLOSE
hands shared staging to another open. It clears `Retired`, reopening a
retired writer's data file. If its save fails it keeps the previous
owner in memory and tries to write the previous sidecar back. That can fail
too; the checkpoint is then marked dirty, and the sidecar on disk may be
either version. A failed rebind of a retired writer also closes the data
file it reopened.

## Startup recovery and quarantine

`NewLocalStore` scans the directory before the server starts:

1. It removes sidecar temporaries.
2. For each `<inode>.staging` it opens the data file and loads
   `<inode>.meta`.
3. A sidecar with another recognized `NFSn` format stops startup with an
   error. Drain or recover that staging with a compatible binary first; see
   [Upgrade the server](../../RECOVERY.md#upgrade-the-server).
4. A missing or undecodable sidecar, a `Guarded` one, or one both
   `Unlinked` and `Retired` is quarantined without asking the backend what
   happened. Startup continues.
5. A file with a base is truncated to its checkpointed size, and its dirty
   ranges come from the sidecar. Inside those ranges the data file's
   current bytes are used, including any written after the checkpoint.
   Outside them the bytes are read from the base again, so a write after
   the checkpoint outside its ranges is not recovered. A file without a
   base keeps its data file's size.
6. The entry is registered. A retired entry has no open data file. An
   unlinked entry is not indexed by target.

A data file that cannot be opened, examined or truncated is logged and
skipped where it is. A sidecar without a data file is not loaded. Which
recovered writers a client may use is the server's decision; see
[Writer recovery](../../RECOVERY.md#writer-recovery).

Quarantine moves a pair into a new directory under `quarantine/`. It syncs
the new directory, `quarantine/` and the staging directory before the
moves, renames the data file and the sidecar, and syncs all three again.
Moving the pair is not atomic. A failed move leaves each half where it
reached, and nothing is deleted to finish it.

At run time `Quarantine` also leaves a tombstone for the life of the
process: `Get`, `GetMeta` and `Entries` no longer return the entry,
`Failed` reports it, `Create` of the same inode gets `ErrRemoved` and
`Remove` does nothing. The server quarantines a writer whose namespace
operation has an unknown outcome, and a staging whose hydration or LinkFile
failed during CLOSE while other opens read it. A tombstone is not persisted; a restart changes
the write verifier.

## Read-only store

`ReadOnlyStore` is used when `-staging` is omitted. It holds nothing.
`Create` and every namespace or ownership update get `os.ErrPermission`,
lookups find nothing, and `Quarantine` and `Remove` do nothing. See
[read-only servers](../../DEPLOYMENT.md#read-only-servers).

## Locking

- The store mutex guards the entry and target maps. It is never held across
  file I/O.
- A per-inode lock serializes `Create`, `Quarantine` and `Remove` of one
  inode, so cleanup cannot delete a creation in progress while unrelated
  creations overlap.
- `LocalFile.mu` covers one file's I/O, including fsync and sidecar saves.
  Namespace updates take it, then the store mutex briefly to reindex.
- A separate mutex orders the file's hydration worker.

The server's pathname lock serializes namespace updates of a target.
Diagnostics and metrics only try `LocalFile.mu`, so a file busy with disk
I/O is reported as busy; see
[snapshot consistency](../../inspect/README.md#snapshot-consistency).
