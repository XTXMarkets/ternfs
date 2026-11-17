<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Testing nfsd

The suites exercise different layers. A passing raw-protocol test does not
prove that a Linux mount recovers, and a passing in-place restart test does
not prove host or staging-disk failover.

Tests should distinguish supported behaviour, correctly rejected operations
and unsupported features. The current contract and defects are described in
[Deployment](../DEPLOYMENT.md).

## Suites

| Suite | Client and backend | What it checks |
| --- | --- | --- |
| Go unit tests | Raw RPC and direct calls; local filesystem backend. | Protocol handling, state, staging and injected failures. |
| `ternnfs` tests | Raw RPC; temporary TernFS cluster. | Translation to native TernFS operations. |
| libnfs | Separate client and nfsd processes; TernFS backend. | Behaviour through the libnfs client library. |
| pynfs | NFSv4.1 protocol client; TernFS backend. | Selected protocol conformance cases. |
| Functional scripts | Applications on an existing mount, or a QEMU kernel client. | File workflows and application-visible results. |
| Samba | smbtorture through smbd on the QEMU kernel client; TernFS backend. | SMB2 behaviour over nfsd, compared with a tmpfs share. |
| CI NFS integration | Linux client in the CI VM; TernFS backend. | Mounted filesystem, mutation and outage workloads, with the limitations below. |

## Unit and native-backend tests

Run from `go/nfsd`:

```sh
make test
make test GO_TEST_FLAGS='-race -count=1'
make test-cluster
make test-cluster GO_TEST_FLAGS='-v -run TestTernLargeFile'
```

`make test` runs `go test . ./internal/...`. The untagged tests include
[nfsd_test.go](../internal/server/nfsd_test.go), the
[client record tests](../internal/clientrecords/records_test.go), session tests
and staging tests. `make test` does not include the diagnostics API package;
run its typed-ID tests from `go/nfsd` with:

```sh
go test ./inspect
```

[cluster_test.go](../internal/server/cluster_test.go) uses the `ternnfs`
build tag and starts a registry, block services, CDC and metadata shards.
`make test-cluster` runs it in `./internal/server` and requires `ss` from
iproute2 to check the fixed registry UDP port 55556.
Do not run two copies against that port. The test harness builds backend
binaries unless given its `-binaries-dir` argument. These tagged tests are
not selected by the current CI workflow.

Local-backend tests do not reproduce native UDP retransmission, snapshot
collection or partially completed native file uploads. Tests named
"restart" may rebuild objects in one test process; inspect the fixture
before treating one as a process-crash test.

## Process harness

Run the remaining Make commands from `go/nfsd/test`.
[Makefile](Makefile) provides targets for libnfs, pynfs and functional tests.
Targets ending in `-cluster` start a temporary `ternrun` backend. Other
libnfs and pynfs targets require an existing registry's bincode endpoint:

```sh
make test-libnfs-cluster
make test-pynfs-cluster
make test-libnfs TEST_ARGS='-registry HOST:PORT'
make test-pynfs TEST_ARGS='-registry HOST:PORT'
```

Both modes start their own nfsd processes. They create and mutate test data
in the selected backend. Use an isolated test filesystem, and run one suite
at a time against it: each nfsd the harness starts serves the whole
filesystem, and two daemons on one tree are not supported.

Shared options in `TEST_ARGS`:

| Option | Effect |
| --- | --- |
| `-binaries-dir DIR` | Use prebuilt backend binaries with a cluster target. |
| `-nfsd PATH` | Test this nfsd binary instead of building the source. |
| `-artifacts-dir DIR` | Choose the local parent directory for run artifacts. |

Relative paths are resolved from `go/nfsd/test`. Failed runs retain logs and
test data; successful runs clean up. The harness enables
`-allow-minor-version-2` for libnfs. Linux and pynfs still request 4.1.

## libnfs

[libnfs](libnfs) tests run client and server in separate processes. They
need Linux, Go, CMake and a C/C++ toolchain, but no kernel mount or root.
Make builds pinned libnfs 8.0.0 under `.deps`. It unpacks
`.deps/libnfs-libnfs-8.0.0.tar.gz` if present; otherwise it clones the tag.
libnfs 8 offers sessions as NFSv4.2, hence the server compatibility flag.

```sh
make test-libnfs-cluster GO_TEST_FLAGS=-short
make test-libnfs-cluster GO_TEST_FLAGS='-run TestLibnfs/recovery'
make test-libnfs TEST_ARGS='-registry HOST:PORT' \
    GO_TEST_FLAGS='-run TestLibnfs/ReadLargeFile'
```

`-short` skips timed cases. `LIBNFS_TIMEOUT` overrides the default
15-minute test timeout. This suite is not currently selected by CI.

## pynfs

The harness pins [pynfs](https://github.com/linux-nfs/pynfs) release
`pynfs-0.5` and runs the `nfs4.1` server tests with `--minorversion=1`.
It requires Python 3, setuptools and PLY. Build the generated modules with:

```sh
make fetch-pynfs
make build-pynfs
make test-pynfs-cluster
```

The [runner](pynfs/main.go) uses `--maketree --rundeps` and reads JSON results
so individual protocol failures fail the command. Pynfs's process exit
status alone does not establish success. Console output is also written to
`pynfs.out`; `PYNFS_OUTPUT` selects another path. That output is not ignored
by git.

The default selector is `all noblock nochar nofifo nosocket nodeleg`.
[pynfs_unsupported.txt](pynfs/pynfs_unsupported.txt) excludes individual cases
requiring unsupported semantics such as locks, share reservations, hard
links or grace-period reclaim. Exclusions are appended to the selector;
set `PYNFS_SKIP_FILE=` to bypass the manifest.

```sh
make test-pynfs-cluster PYNFS_TESTS='sequence create_session'
make test-pynfs-cluster PYNFS_TESTS='SEQ1 OPEN2' PYNFS_ARGS='--showtraffic'
make test-pynfs-cluster TEST_ARGS=-short
make test-pynfs-cluster PYNFS_TESTS='all nodeleg' PYNFS_SKIP_FILE=
make test-pynfs-cluster PYNFS_TIMEOUT=2h
```

`PYNFS_TIMEOUT` defaults to one hour. `-short` excludes lease-waiting cases
by test code because this suite has no broad `timed` flag. The runner lists
tests taking at least a second by duration. `make clean-pynfs` removes the
downloaded and generated files. This suite is not currently selected by CI.

## Application workflows

[functional](functional) contains numbered scripts for an existing mount.
Each creates its own directory, removes it on success, and preserves it on
failure. The [catalog](functional/README.md) describes each case.

```sh
make test-functional FUNCTIONAL_TEST_ROOT=/mnt/qa-nfs/test
./functional/0007-dd-direct.sh /mnt/qa-nfs/test
strace -ff -o /tmp/0005.trace \
    ./functional/0005-vim-edit.sh /mnt/qa-nfs/test
```

Use a dedicated test directory. The runner discovers numbered cases
automatically; manual probes should not be added under numbered names.

## Linux kernel client in QEMU

The [kernel driver](internal/kernel/main.go) boots a guest which mounts the
test nfsd with `vers=4.1` and runs the functional scripts. It shares the host
filesystem over virtio-9p and reaches nfsd through QEMU user networking.
It needs no host NFS mount, root, tap or KVM access; without KVM it uses TCG.

On Debian the prerequisites are `qemu-system-x86`, `busybox-static`, `cpio`,
`kmod`, `linux-image-amd64` and `nfs-common`, plus the application tools used
by the selected scripts. [prepare-image.sh](internal/kernel/prepare-image.sh)
builds an initramfs from an installed kernel; `KERNEL_VERSION` selects it.

```sh
make test-functional-cluster TEST_ARGS='-binaries-dir /path/to/binaries'
make test-functional-cluster FUNCTIONAL_TESTS=0005,0009
```

The target rebuilds the image before running. `FUNCTIONAL_TIMEOUT` defaults
to 30 minutes. Each guest uses a distinct hostname for its client identity.

For the explicit crash test, build the image, then run:

```sh
make build-kernel-image
go run ./internal/kernel -cluster -image-dir .deps/kernel -restart-mid-write
```

[restart-writer.py](internal/kernel/restart-writer.py) holds one descriptor,
fsyncs a prefix, waits while the driver kills nfsd with SIGKILL and starts
it on the same address and staging disk, then writes the suffix and closes.
The driver checks published contents through the backend. It does not test
DNS migration, disk loss, old read-handle aliases, namespace changes during
recovery or a crash partway through native CLOSE publication.

The driver also accepts `-registry`, `-binaries-dir`, `-nfsd` and
`-artifacts-dir` directly.

## Samba over the kernel client

The fsgw hosts serve the tree to Windows through Samba on a kernel NFS mount
of nfsd. The driver's `-samba` mode runs that arrangement in the same guest:
it mounts nfsd with `lookupcache=none` added, as
[Client mounts](../DEPLOYMENT.md#one-writable-server) requires, starts smbd
with [smb.conf.in](samba/smb.conf.in), and runs smbtorture's SMB2 suites
against a share on the mount and against a tmpfs share with the same
options. The share carries the settings [Samba](../DEPLOYMENT.md#samba)
lists for a share over nfsd.

Besides the kernel image prerequisites above, this needs the `samba` and
`samba-testsuite` packages on the host, on Debian and Ubuntu alike. The
guest runs the host's smbd and smbtorture from the 9p root, and the driver
checks for them before it starts anything; nothing is started on the host.
The `samba` package enables and starts its own smbd on the host when
installed, which this does not use; stop it with `systemctl disable --now
smbd nmbd` if it is not wanted. In a minimal container the package's
post-install step can fail on `root:adm`; the binaries are installed
regardless.

```sh
make test-samba-cluster TEST_ARGS='-binaries-dir /path/to/binaries'
make test-samba TEST_ARGS='-registry HOST:PORT'
make test-samba-cluster SAMBA_SUITES=smb2.sharemode,smb2.delete-on-close-perms
```

A case that fails on both shares is Samba or configuration and is reported
as `both-failed`. A case that fails only on nfsd, or does not run there,
fails the run unless [known_differences.txt](samba/known_differences.txt)
lists it with its reason. A listed case that no longer differs is reported as
`stale-manifest-entry`. smbtorture's raw output and the smbd logs are kept
under the run's `samba` directory when the run fails.

The default suites leave out `smb2.lease`, `smb2.oplock` and
`smb2.durable-open`, which fail on both shares because the fsgw shares set
`oplocks = no`. `SAMBA_TIMEOUT` defaults to 45 minutes, and each suite may
take up to 20 (`-case-timeout`). The default suites take about ten minutes
under TCG when backend writes take under a millisecond. A file created
through Samba costs about three backend writes, so with a cluster on a disk
whose fsync takes 10 ms, such as a large encrypted volume chosen with
`TMPDIR`, `smb2.dir` alone takes about seven minutes. Results depend on the
host's Samba version; the control share absorbs most of that, but a new
version can change case names in the manifest. This suite is not currently
selected by CI.

The SMB user is an ordinary user, as Windows users are through winbind, and
the share lists it in `admin users`. nfsd's ownership and modes are
synthetic: every file is owned by `nobody` with mode 0644, and every
directory 0755. Samba checks a user's access against those, so without
`admin users` an ordinary user can read but cannot create, overwrite or
make directories. The share's `valid users` is therefore the only access
control, and the suite says nothing about permissions.

## Separate CI NFS integration

From the repository root, with the Ubuntu Noble cloud image the
[CI workflow](../../../.github/workflows/main.yml) downloads, the CI VM path
is:

```sh
curl -fsSL -O https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
./ci.py --short --prepare-image "$PWD/noble-server-cloudimg-amd64.img" --nfs --leader-only
```

[ci_nfs.sh](../../../ci_nfs.sh) deploys the cluster and nfsd into the VM.
[terntests](../../terntests/terntests.go) mounts it and selects three NFS
workloads: the mounted filesystem test, [mutations](../../terntests/nfsmutate.go),
and [lease recovery](../../terntests/nfsoutage.go). The latter disconnects
a TCP proxy for 100 seconds and checks new-file and replacement writes
across the outage; it does not crash nfsd.

This path currently mounts with `vers=4.0`, which this server rejects.
Its private-writers mutation step also assumes independent staging for
two opens through one mount, while writers on one client now share staging.
Those tests need updating before this path establishes NFSv4.1 coverage.

Each workload unmounts and stops its daemon before namespace cleanup,
which deletes the whole filesystem. Run this only against
the temporary test cluster. The
[CI workflow](../../../.github/workflows/main.yml) selects this kernel-client
path; it does not select the separate libnfs, pynfs or functional QEMU suites.
