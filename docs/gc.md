<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Garbage Collection

## Snapshots

When you delete a file with `rm` on TernFS, the file contents do not get deleted -- a "snapshot" will still be present. You can witness these snapshot in the web UI by enabling "Show snapshot edges" in the "Browse" view.

Snapshotted files survive until they expire. The expiry time is determined by the `SNAPSHOT` policy of the directory they live in. Again, the snapshot policy is viewable when browsing a directory on TernWeb. The root directory in Iceland has:

```
{DeleteAfterTime:ActiveDeleteAfterTime(720h0m0s) DeleteAfterVersions:InactiveDeleteAfterVersions()}
```

Which basically means "permanently delete snapshots after a month". `DeleteAfterVersions` is used to bound the number of versions of the same filename, and it is currently not used.

Directories inherit the policy from the parent directory if they don't have a policy set explicitly.

When we talk of "garbage collection" in TernFS we're talking of the processes that scans the filesystem for expired snapshots, and actually permanently delete them, including their file contents. This process is split in two independent workflows, "directory collection", and "file destruction".

## Directory collection

Directory collection is the process of listing contents of all directories and removing the snapshot _metadata_. In pseudocode:

```
while True:
    for shard in shards (in parallel):
       for directory in shard (possibly split between workers):
           for snapshot in directory:
               if snapshot has expired:
                   if snapshot is a file, make the file transient and remove the directory entry
                   if snapshot is a directory, check if the directory is empty, and if it is, remove it
           if the directory has no directory entries after collection (snapshot or otherwise), remove it
```

The above papers over non-owning snapshots, which are basically directory entries which are not responsible for the content they contain -- in that case the snapshot can just removed. The actual code is in [`collectdirectories.go`](https://github.com/XTXMarkets/ternfs/blob/main/go/cleanup/collectdirectories.go).

The important bit above is the "make the file transient", which feeds into the next section.

## File destruction

File destruction is simpler: it goes thorugh the transient files, and permanently removes the expired ones. All transient files have a _deadline_, which is bumped by one hour every time the transient file is modified. This is to ensure that files that are currently being worked on are not deleted. In pseudocode:

```
while True:
    for shard in shards (in parallel):
        for transient file in shard (possibly split between workers):
            if the deadline has expired, remove all blocks by talking to the block services, then delete the transient file metadata
```

The actual code is in [`destructfiles.go`](https://github.com/XTXMarkets/ternfs/blob/main/go/lib/destructfiles.go).
