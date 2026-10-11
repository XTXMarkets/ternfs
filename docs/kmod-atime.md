# Kernel client access times

By default, TernFS updates regular-file atime when the Linux VFS requests an
update on data access. Read-only mounts suppress those requests. This includes
read-only bind mounts into containers, even when the underlying mount is
writable.

For workloads where reads through those mounts must keep files alive under
atime-based retention, enable the optional open-based update on the client host:

```sh
sudo sysctl -w fs.eggsfs.atime_on_open=1
```

`fs.eggsfs.atime_on_open` is a host-wide boolean, defaults to `0`, and can be
changed at runtime. With it enabled, opening a published regular file directly
updates its atime in TernFS, subject to `fs.eggsfs.atime_update_interval_sec`.
The client refreshes metadata before writing so an update from another client
can satisfy the interval. Throttled opens do not advance the cached timestamp
and postpone the next update.

This path intentionally bypasses read-only, `noatime`, and `relatime` mount
policy. Explicit `O_NOATIME` opens still opt out. `O_PATH` handles do not invoke
the file-open callback; directories and unpublished files are unaffected.
An open counts as access even if no data is read. An error refreshing or
updating atime is logged but does not fail the open. File contents and mtime
are unchanged.

The normal VFS-driven update path remains active. Setting the sysctl back to
`0` disables only the additional open-based updates.

Set this after the module has loaded (for example in the service that mounts
TernFS, alongside `fs.eggsfs.atime_update_interval_sec`). This setting does not
retroactively repair atimes for past reads.
