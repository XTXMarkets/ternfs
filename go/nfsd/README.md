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

There are currently four test paths.

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