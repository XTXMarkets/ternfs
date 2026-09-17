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
