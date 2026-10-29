<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

The kernel module requires Linux 6.12 or newer. CI compiles against 6.12.44
and runs the QEMU tests on 6.14.9.

Prepare the compile-only 6.12.44 tree from the repository root:

```sh
(cd kmod && ./fetchlinux.sh 6.12.44)
```

This uses `config-kasan-6.12.49` as the seed configuration, then runs
`make olddefconfig` and `make modules_prepare`. It does not build a kernel
or change the `kmod/linux` symlink used by QEMU.

Prepare and build the 6.14.9 QEMU kernel as usual (skip this if it is already built):

```sh
(cd kmod && ./fetchlinux.sh)
(cd kmod/linux && make oldconfig && make prepare && make -j)
```

Run both compile checks with one command:

```sh
make -C kmod -j"$(nproc)" compile-check
```

To check just one version, add `COMPILE_KERNELS=6.12.44` or
`COMPILE_KERNELS=6.14.9`. Each check cleans the module objects and
builds `ternfs-client.o` with the kernel's warning flags and -Werror. The 6.14.9
check uses `kmod/linux`; the 6.12.44 check uses `kmod/linux-6.12.44`.
These checks stop before modpost because `modules_prepare` does not generate
`Module.symvers`. They validate compilation and linking of the module objects,
but do not produce a loadable `.ko` or validate exported kernel symbols.
The QEMU tests build and load the complete module against 6.14.9.

To build a loadable module against an already built kernel:

```sh
make -C kmod KDIR="$LINUX_DIR" -j ternfs-client-local
```

To get a `compile_commands.json` using Clang:

* Call the kmod source dir (where this file is located) `$KMOD_DIR`
* Download Linux 6.12 or newer, say `$LINUX_DIR`
* `cd $LINUX_DIR && make LLVM=1 defconfig && make LLVM=1 -j`
* `cd $KMOD_DIR && make KDIR=$LINUX_DIR LLVM=1 ternfs-client-local`
* `$LINUX_DIR/scripts/clang-tools/gen_compile_commands.py -d $KMOD_DIR` to generate `compile_commands.json`
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
