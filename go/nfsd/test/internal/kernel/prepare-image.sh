#!/usr/bin/env bash
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Build the guest kernel image used by run-qemu.sh: a copy of a locally
# installed Linux kernel and a busybox initramfs which loads the virtio and
# 9p modules, mounts the host root filesystem over virtio-9p and switches to
# guest-init.sh. The guest reuses the host's /lib/modules, so the kernel must
# be installed on this machine (Debian: linux-image-amd64).

set -euo pipefail

usage() {
	echo "usage: $0 -o OUTPUT_DIR [-k KERNEL_VERSION]" >&2
	exit 2
}

output=""
version="${KERNEL_VERSION:-}"
while (( $# > 0 )); do
	case $1 in
	-o) output=$2; shift 2 ;;
	-k) version=$2; shift 2 ;;
	*) usage ;;
	esac
done
[[ -n $output ]] || usage

missing=()
for tool in qemu-system-x86_64 busybox cpio modprobe; do
	command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done
if (( ${#missing[@]} > 0 )); then
	echo "missing tools: ${missing[*]}" >&2
	echo "Debian: apt-get install qemu-system-x86 busybox-static cpio kmod linux-image-amd64 nfs-common" >&2
	exit 1
fi
if ! busybox --list 2>/dev/null | grep -qx switch_root; then
	echo "busybox lacks switch_root; install busybox-static" >&2
	exit 1
fi

if [[ -z $version ]]; then
	# Newest installed kernel which has both an image and a module tree.
	for candidate in $(ls /lib/modules 2>/dev/null | sort -V -r); do
		if [[ -f /boot/vmlinuz-$candidate ]]; then
			version=$candidate
			break
		fi
	done
fi
if [[ -z $version || ! -f /boot/vmlinuz-$version ]]; then
	echo "no kernel found under /boot with modules under /lib/modules; install linux-image-amd64" >&2
	exit 1
fi
modules=/lib/modules/$version
[[ -f $modules/modules.dep ]] || depmod -a "$version"
for required in nfsv4 virtio_net; do
	if ! modprobe -S "$version" -n -q "$required"; then
		echo "kernel $version lacks the $required module" >&2
		exit 1
	fi
done

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/bin" "$work/proc" "$work/sys" "$work/dev" "$work/newroot" "$work/modules"
cp "$(command -v busybox)" "$work/bin/busybox"

# Modules the initramfs must load before it can mount the 9p root, in
# dependency order. Builtin modules are reported by modprobe and skipped.
: > "$work/modules/order"
for target in virtio_pci 9pnet_virtio 9p; do
	modprobe -S "$version" --show-depends "$target"
done | awk '$1 == "insmod" && !seen[$2]++ { print $2 }' | while read -r path; do
	name=$(basename "$path")
	name=${name%%.ko*}
	case $path in
	*.ko) cp "$path" "$work/modules/$name.ko" ;;
	*.ko.xz) xz -dc "$path" > "$work/modules/$name.ko" ;;
	*.ko.gz) gzip -dc "$path" > "$work/modules/$name.ko" ;;
	*.ko.zst) zstd -dc "$path" > "$work/modules/$name.ko" ;;
	*) echo "unsupported module format: $path" >&2; exit 1 ;;
	esac
	echo "$name" >> "$work/modules/order"
done

cat > "$work/init" <<'INIT'
#!/bin/busybox sh
/bin/busybox --install -s /bin
export PATH=/bin
mount -t proc proc /proc
mount -t sysfs sys /sys
mount -t devtmpfs dev /dev
for m in $(cat /modules/order); do
	insmod "/modules/$m.ko" || echo "insmod $m failed"
done
if mount -t 9p -o trans=virtio,version=9p2000.L,msize=1048576,cache=loose hostroot /newroot; then
	init=/newroot/$(sed -n 's/.*guest_init=\([^ ]*\).*/\1/p' /proc/cmdline)
	exec switch_root /newroot "${init#/newroot}"
fi
echo "GUEST-ROOT-FAILED"
dmesg | tail -20
poweroff -f
INIT
chmod +x "$work/init"

mkdir -p "$output"
(cd "$work" && find . | cpio -o -H newc 2>/dev/null | gzip -1) > "$output/initrd.img"
cp "/boot/vmlinuz-$version" "$output/vmlinuz"
echo "$version" > "$output/version"
echo "kernel $version: $output/vmlinuz $output/initrd.img (modules: $(tr '\n' ' ' < "$work/modules/order"))"
