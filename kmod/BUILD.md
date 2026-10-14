<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

To get a `compile_commands.json`:

* Call the kmod source dir (where this file is located) `$KMOD_DIR`
* Download some version of the linux source (I used `linux-5.4.237` to be in line with Ubuntu 20.20), say `$LINUX_DIR`
* `cd $LINUX_DIR && make LLVM=1 defconfig && make LLVM=1 -j`
* `cd $KMOD_DIR && make KDIR=$LINUX_DIR LLVM=1 kmod`
* `$LINUX_DIR/scripts/gen_compile_commands.py -d $KMOD_DIR` to generate `compile_commands.json`
* Replace all occurrences of `"directory": $KMOD_DIR` with `"directory": $LINUX_DIR` in the generated `$KMOD_DIR/compile_commands.json`

## Registry hostname resolution

Mounting with a registry hostname requires the kernel DNS resolver userspace
upcall from `keyutils`. Configure `request-key` to handle `dns_resolver` keys
with `key.dns_resolver`, for example:

```
create dns_resolver * * /sbin/key.dns_resolver %k
```

Multi-address failover requires a resolver helper that supports the
`ipv4 list` option. Helpers that return only one address, such as `cifs.upcall`,
allow hostname mounts but do not provide multi-address failover.
