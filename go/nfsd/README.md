<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# nfsd

`nfsd` serves TernFS over NFSv4.1. It supports directory traversal, reads,
file creation, writes, removal and rename. It adapts mutable NFS files to
TernFS by staging writes on the nfsd host and publishing an immutable
replacement on the last write CLOSE or downgrade to read access.

**Successful fsync does not publish or replicate staged data.** Unpublished
data depends on the staging disk. Recovery with that disk is conditional,
and the implementation has known recovery defects. Several nfsd processes
can serve one TernFS namespace, but they do not form one NFS server with
shared sessions or transparent failover.

| Document | Contents |
| --- | --- |
| [Deployment](DEPLOYMENT.md) | Deployment modes, guarantees, failure behaviour and known issues. |
| [Recovery](RECOVERY.md) | Crash and lease recovery, staging inspection, quarantine and replacement procedures. |
| [Design](DESIGN.md) | Filehandles, staging, sessions, client identities, leases and namespace operations. |
| [Testing](test/README.md) | Test suites, prerequisites, commands and coverage limits. |

## Run

From `go/nfsd`, build and start a server using the TernFS registry's bincode
endpoint:

```sh
go build -o nfsd .
./nfsd -registry registry.example:55556 -addr :2049 -staging /var/lib/nfsd
```

Mount from Linux with NFSv4.1 explicitly:

```sh
sudo mount -t nfs -o vers=4.1,proto=tcp,hard nfs.example:/ /mnt/ternfs
```

The mountpoint must already exist. Keep the mount directed at one nfsd.
Read [Deployment](DEPLOYMENT.md) before choosing the staging disk or a
server replacement procedure.

Omitting `-staging` selects the server's
[read-only mode](DEPLOYMENT.md#read-only-servers). `-root DIR` selects a
local test backend instead of `-registry`; exactly one is required.
`-v` enables request logging. `-status-addr` serves a read-only JSON
[diagnostics API](DEPLOYMENT.md#diagnostics); `-store-only` serves only that
API, without the NFS server. `-influx-db-origin`, `-influx-db-org`,
`-influx-db-bucket` and `-filesystem` push [metrics](DEPLOYMENT.md#metrics).
`./nfsd -h` lists the other options.

## NFS profile

The server implements a restricted NFSv4.1 profile. NFSv4.0 requests receive
`NFS4ERR_MINOR_VERS_MISMATCH`. The compatibility flag
`-allow-minor-version-2` accepts NFSv4.2 compounds with 4.1 behaviour and
rejects 4.2-only operations with `NFS4ERR_NOTSUPP`. It exists for libnfs 8;
Linux mounts should still request `vers=4.1`.

Sessions and their reply caches can survive a restart of the daemon on the
same staging disk. Restoration is conditional and is not promised after a
host crash ([Design](DESIGN.md#requests-and-sessions)). Sessions do not move
between daemons. Byte-range locking,
share-deny enforcement, delegations, pNFS, hard links, named attributes and
special files are unsupported. There is no protocol reclaim grace period.
Ownership and permissions are synthetic. This is not a general-purpose
POSIX filesystem; the [deployment guarantees](DEPLOYMENT.md#data-guarantees)
describe the observable file semantics.
