# Block service space accounting

`ternblocks` supports XFS filesystems with metadata on a data device (for
example NVMe) and file contents on a realtime device (for example an HDD).
The `with_crc` directory must have XFS realtime inheritance enabled for its
blocks to use the realtime device.

For these services, `ternblocks` queries both devices through XFS ioctls.
It does not infer metadata space from `statfs`: XFS selects the data or
realtime device's counters according to the queried inode's flags.

Two flags control reserved headroom, in bytes per filesystem:

* `-reserved-storage` (default `0`): subtract from the block data device's
  total capacity and available space.
* `-reserved-xfs-metadata` (default `100000000`, 100 MB): subtract from the
  XFS metadata device's total capacity and available space. This only
  applies to services using XFS realtime storage.

Subtractions clamp at zero. Let `D` and `d` be the data capacity and free
space after its reserve, and `M` and `m` the metadata capacity and free
space after its reserve. The service advertises capacity `D` and available
space `min(d, floor(D * m / M))`. If `M` is zero, availability is zero.
For example, a 20 TB HDD with 40% free space and a metadata device with
10% free space after reserves advertises 2 TB available.

Other filesystems and XFS services without realtime inheritance retain
ordinary data-device accounting; the metadata reserve does not apply.
No registry or client protocol change is needed.

Capacity is sampled every ten seconds and before initial registration.
Writes are rejected when reported availability is zero; reads and erases
continue. An actual `ENOSPC` also clears advertised availability until the
next successful sample. Failed writes use the existing `INTERNAL_ERROR`
response and close the connection, so clients can retry; they do not count
as hardware I/O errors. Failed temporary writes are removed.
If the log destination returns `ENOSPC`, `ternblocks` falls back to stderr
instead of panicking. Logging resumes to the configured destination when
space becomes available there.

The reserve provides headroom for writes already in flight and for sampling
and registration delays; it is not a hard filesystem reservation. Size it
for the deployment's metadata consumption rate. A reserve at or above the
metadata device's capacity deliberately makes the service unwritable.
On a filesystem shared by multiple services, each observes the same
filesystem-wide free space and reserve.

The `eggsfs_blocks_storage` measurement includes:

* `capacity`, `available`: the effective space advertised to the registry.
* `data_capacity`, `data_available`: raw block-data device space, before
  the configured reserve.
* `xfs_metadata_capacity`, `xfs_metadata_available`,
  `xfs_metadata_reserved`: metadata space and configured reserve, emitted
  only for XFS realtime services. Capacity excludes an internal XFS log;
  available space already excludes XFS's own internal reservations.
* `no_space_errors`: requests rejected with `ENOSPC`, including rejections
  at zero reported availability.

The XFS counters are filesystem-wide; they do not account for per-user or
project quotas, inode limits, or allocation fragmentation. Those can still
cause writes to fail before the reported free blocks reach zero.
