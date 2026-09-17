<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

These kernel test modules exercise production helpers without a running
TernFS cluster. Build against the kernel used by a disposable test VM:

```sh
make -C /path/to/kernel/build M="$PWD/kmod/tests" modules
```

Run a test module as root with the reusable log-checking wrapper:

```sh
./run_module_test.sh ./ternfs-policy-test.ko \
    ternfs_policy_test 'ternfs-policy-test: PASS'
```

The wrapper records the current kernel-log length, loads and unloads the
module, prints only new messages, requires the supplied success text, and
fails if those messages contain a KASAN report or other kernel fault.

Inside that VM, run the policy-cache test:

```sh
sudo insmod ternfs-policy-test.ko
sudo dmesg | grep ternfs-policy-test
sudo rmmod ternfs_policy_test
```

The test checks that 257 distinct inode keys retain their own values, updates
do not affect other keys, and policy tags remain distinct. It prints `PASS`
on success; a failed check logs the reason and returns an error from module
initialization. Use a KASAN-enabled kernel to also check memory accesses.

The write-page test runs the production copy helper with real kernel iterators
and pages, including empty and short iterators and failures of the first or
later page allocation. It checks returned progress, retained contents, page
ownership, and RSS accounting:

```sh
sudo insmod ternfs-write-test.ko
sudo dmesg | grep ternfs-write-test
sudo rmmod ternfs_write_test
```

The inline-read test poisons a real page before filling it through the
production helper. It checks content preservation and zeroed tails for empty,
one-byte, 255-byte and full-page inline data, then reuses the same page to
ensure shorter contents cannot expose bytes from an earlier fill:

```sh
sudo insmod ternfs-inline-read-test.ko
sudo dmesg | grep ternfs-inline-read-test
sudo rmmod ternfs_inline_read_test
```

The symlink test includes the production read-link implementation while
substituting metadata and span I/O at their external boundaries. It checks
metadata, span lookup, buffer allocation, page allocation and page-fetch
failures, including exact buffer, span and page cleanup. Empty, inline and
block-backed successes must return terminated contents and release everything
through the production destructor:

```sh
./run_module_test.sh ./ternfs-symlink-test.ko \
    ternfs_symlink_test 'ternfs-symlink-test: PASS'
```

The file-open test uses a real initialized inode mutex and snapshots all
transient file fields around the production open callback. It covers NONE,
READING and WRITING states with read-only, write-only and read/write modes,
including repeated rejected read-only reopens followed by a writable reopen.
Every return must preserve the expected state and leave the inode unlocked:

```sh
./run_module_test.sh ./ternfs-open-test.ko \
    ternfs_open_test 'ternfs-open-test: PASS'
```

The transient-lifetime test includes the production span, cleanup and write
page-accounting helpers. It uses real slab objects, pages, inode locking,
semaphores and an `mm` reference to check span refcounts, current and
per-block page cleanup, RSS balance, idempotent file cleanup, preserved
errors, non-owner eviction, and waiting for an outstanding flush before
dropping the writer's `mm`:

```sh
./run_module_test.sh ./ternfs-transient-test.ko \
    ternfs_transient_test 'ternfs-transient-test: PASS'
```

The symlink-create test includes the production post-create helper with
write, flush, trace and dentry-publication boundaries substituted. It uses a
real initialized inode and `iput`, retaining one observer reference so error
paths can verify exactly one reference drop without invoking fabricated VFS
teardown. It checks original errors, ordering, path contents, link count,
publication ownership and inode-lock release:

```sh
./run_module_test.sh ./ternfs-symlink-create-test.ko \
    ternfs_symlink_create_test 'ternfs-symlink-create-test: PASS'
```
