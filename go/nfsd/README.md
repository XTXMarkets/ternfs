<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Testing nfsd

`nfsd` is an NFSv4.0 adapter for TernFS. TernFS is not a general-purpose
POSIX filesystem: files are immutable after creation, ownership and mode are
synthetic, and file locking is not supported. Tests must therefore distinguish
between:

- behavior which the server supports;
- operations which the server must reject with the correct NFS status; and
- NFS features which are outside the intended TernFS contract.

There are currently five test paths.

Test targets ending in `-cluster` build and start a temporary TernFS cluster.
Test targets without that suffix do not use a TernFS cluster. For example,
`test-libnfs` runs against the local filesystem backend. Pynfs only has a
cluster-backed mode, so its target is `test-pynfs-cluster`; there is no
`test-pynfs` target.

Cluster-backed targets use `ss` to check the fixed registry UDP port
`127.0.0.1:55556` before starting. They fail with an explicit error if another
test cluster is already using that port. This check does not require root.
Install `ss` (provided by the `iproute2` package on Debian and Ubuntu) before
running these targets.

## Protocol tests

Run the default Go tests from this directory:

```sh
make test
```

[`nfsd_test.go`](nfsd_test.go) starts the server on a local TCP socket with a
local filesystem backend. The tests construct NFS RPC messages directly. They
cover RPC framing, compound processing, filehandles, attributes, directory
traversal, reads, staged writes, client and open state, replay behavior, and
expected errors for unsupported operations.

**These tests run as part of the CI functional-test step through `go test ./...`.**

## TernFS-backed protocol tests

[`cluster_test.go`](cluster_test.go) contains tests behind the `ternnfs` build
tag. The intended command is:

```sh
make test-cluster
```

The test `TestMain` builds and starts a temporary TernFS cluster, including the
registry, block services, CDC and metadata shards. Each test starts an NFS
server backed by that cluster and sends NFS RPC messages directly.

This suite exercises the TernFS backend but still uses the test's own RPC
client.

**This test is currently not run in any CI workflow.**

## libnfs tests

[`libnfs_test.go`](libnfs_test.go) uses
[libnfs](https://github.com/sahlberg/libnfs) as an independent NFS client. The
tests are behind the `libnfs` build tag.

Fetch the pinned libnfs source:

```sh
make fetch-libnfs
```

The pinned release is libnfs 5.0.2.

Run the tests:

```sh
make test-libnfs
```

`test-libnfs` builds and installs libnfs under `.deps/libnfs-install` when
needed, then runs the 11 `TestLibnfs_*` cases verbosely against a server with
the local filesystem backend. It disables the Go test cache so that each
invocation exercises the client and server. The downloaded source and
installation are ignored by git. Use `make clean-libnfs` to remove them.
Fetching requires git; building requires CMake and a C compiler.

**This test is currently not run in any CI workflow.**

## pynfs protocol tests

[pynfs](https://github.com/linux-nfs/pynfs) is the Linux NFS project's
protocol test suite. The Makefile pins release `pynfs-0.5` and uses its
NFSv4.0 server tests.

Fetch and build pynfs:

```sh
make fetch-pynfs
make build-pynfs
```

The build requires Python 3, setuptools and PLY. On Debian and Ubuntu, install
the latter two with:

```sh
sudo apt-get install python3-setuptools python3-ply
```

Run the standard pynfs suite:

```sh
make test-pynfs-cluster
```

[`pynfs_test.go`](pynfs_test.go) reuses the `ternnfs` test harness. It starts a
temporary TernFS cluster and `nfsd`, runs pynfs with `--maketree --rundeps`,
and reads pynfs's JSON results so protocol failures fail the Go test. Pynfs
itself otherwise exits successfully when individual tests fail.

The Make target writes the complete console output to `pynfs.out` while also
displaying it. Set `PYNFS_OUTPUT` to use another path. The output file is not
ignored by git, so completed runs remain visible during review.

TernFS intentionally does not implement several POSIX and NFS features covered
by pynfs. The harness uses two mechanisms to exclude those tests:

* The default `PYNFS_TESTS` value uses pynfs flag selectors to exclude broad
  capability classes such as FIFO, socket, GSS and ACL tests.
* The [`pynfs_unsupported.txt`](pynfs_unsupported.txt) manifest lists
  individual locking and hard-link cases. Using the broad `nolock` and
  `nolink` selectors would also hide useful related tests.

Select flags or individual pynfs test codes with `PYNFS_TESTS`. The manifest
exclusions are appended after `PYNFS_TESTS`, so set `PYNFS_SKIP_FILE=` to
bypass this manifest.

Set additional runner options with `PYNFS_ARGS`. The default Go test timeout is
one hour because pynfs includes lease-expiry cases which deliberately sleep
for several minutes. Override it with `PYNFS_TIMEOUT`:

```sh
make test-pynfs-cluster PYNFS_TESTS='putrootfh getattr'
make test-pynfs-cluster PYNFS_TESTS='GETATTR1 GETATTR2' PYNFS_ARGS='--showtraffic'
make test-pynfs-cluster PYNFS_TESTS='all notimed noblock nochar nofifo nosocket nogss noacl nomode000'
make test-pynfs-cluster PYNFS_TESTS='all notimed' PYNFS_SKIP_FILE=
make test-pynfs-cluster PYNFS_TIMEOUT=2h
```

Pynfs tags eight standard cases as `timed`; these exercise lease expiry and
can each wait for 135 or 180 seconds with nfsd's 90-second lease. The harness
runs Python unbuffered and reports tests taking at least one second, sorted by
duration, after pynfs writes its results.

Use `make clean-pynfs` to remove the source and generated files. This suite is
not currently run in CI.

All Go test targets accept additional flags through `GO_TEST_FLAGS`. For
example:

```sh
make test-cluster GO_TEST_FLAGS='-v -run TestTernWriteAndRead'
```

## Linux kernel client integration test

CI runs a separate NFS test in a QEMU Ubuntu VM:

```sh
./ci.py --short --prepare-image <noble-cloud-image> --nfs --leader-only
```

[`ci_nfs.sh`](../../ci_nfs.sh) deploys a built TernFS cluster and `nfsd` into
the VM. [`terntests`](../terntests) then mounts the server using the Linux
NFSv4.0 client and runs `terntests -nfs -filter nfs`.

That filter selects exactly two tests:

- [`nfs mounted fs`](../terntests/terntests.go) runs the existing `fsTest`
  workload through its `posixHarness`, with the NFS mount as its root. In short
  mode this uses 10 directories and 500 files.
- [`nfs mutations`](../terntests/nfsmutate.go) is the NFS-specific mutation
  suite. It checks create/write/readback, out-of-order writes, rename, delete,
  timestamp updates, and rejection of in-place modification.

This is the only current test path using a kernel NFS client.
