<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

These kernel test modules exercise production helpers without a running
TernFS cluster. Build against the kernel used by a disposable test VM:

```sh
make -C /path/to/kernel/build M="$PWD/kmod/tests" modules
```

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
