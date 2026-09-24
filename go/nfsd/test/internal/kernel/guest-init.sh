#!/usr/bin/env bash
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later
#
# PID 1 inside the qemu guest started by the kernel test driver (main.go). The
# root filesystem is
# the host's, shared over virtio-9p, so paths on the kernel command line are host
# paths. Mount nfsd through the guest kernel's NFSv4.0 client, run the
# selected functional cases against it, then power the guest off. Everything
# printed here also goes to the log file named on the command line.

export PATH=/usr/sbin:/usr/bin:/sbin:/bin HOME=/root

mount -t proc proc /proc
mount -t sysfs sys /sys
mount -t devtmpfs dev /dev 2>/dev/null || true
[[ -e /dev/fd ]] || ln -s /proc/self/fd /dev/fd
mkdir -p /dev/pts /dev/shm
mount -t devpts devpts /dev/pts
mount -t tmpfs shm /dev/shm
mount -t tmpfs run /run

nfsport=""
export_path=""
hostname_arg=guest
tests=all
log=/dev/null
mnt=""
functional=""
case_timeout=300
for arg in $(< /proc/cmdline); do
	case $arg in
	nfsport=*) nfsport=${arg#*=} ;;
	export=*) export_path=${arg#*=} ;;
	host=*) hostname_arg=${arg#*=} ;;
	tests=*) tests=${arg#*=} ;;
	log=*) log=${arg#*=} ;;
	mnt=*) mnt=${arg#*=} ;;
	functional=*) functional=${arg#*=} ;;
	case_timeout=*) case_timeout=${arg#*=} ;;
	esac
done

exec > >(tee -a "$log") 2>&1
finish() {
	echo "GUEST-DONE"
	sync
	sleep 1
	echo 1 > /proc/sys/kernel/sysrq
	echo o > /proc/sysrq-trigger
	sleep 30
	echo "power off did not complete" >&2
}
trap finish EXIT

# Each guest needs its own NFSv4 client identity; nfsd instances on one
# TernFS share the durable client store, and duplicate identities evict each
# other.
hostname "$hostname_arg"
echo "GUEST-INIT kernel=$(uname -r) host=$hostname_arg nfsport=$nfsport export=$export_path tests=$tests"
modprobe virtio_net
modprobe nfsv4
sleep 1
ip link set lo up
nic=$(ls /sys/class/net | grep -v '^lo$' | head -1)
ip link set "$nic" up
ip addr add 10.0.2.15/24 dev "$nic"
ip route add default via 10.0.2.2

diagnostics() {
	echo "--- blocked processes"
	for p in /proc/[0-9]*; do
		state=$(awk '/^State:/ { print $2 }' "$p/status" 2>/dev/null)
		[[ $state == D || $state == S ]] || continue
		cmd=$(tr '\0' ' ' < "$p/cmdline" 2>/dev/null)
		[[ -n $cmd ]] || continue
		case $cmd in *bash*|*tee*|*sleep*|*timeout*) continue ;; esac
		echo "pid=${p#/proc/} state=$state cmd=$cmd"
		head -12 "$p/stack" 2>/dev/null | sed 's/^/    /'
	done
	echo "--- dmesg"
	dmesg | tail -15
}

mkdir -p "$mnt"
# 10.0.2.2 is the host's loopback interface as seen through qemu user networking.
if ! mount -t nfs4 -o vers=4.0,proto=tcp,port="$nfsport",hard,timeo=600 "10.0.2.2:$export_path" "$mnt"; then
	echo "MOUNT-FAILED"
	dmesg | tail -20
	echo "suite exit=2"
	exit 2
fi
echo "MOUNT-OK $(grep " $mnt " /proc/mounts)"

if ! cd -- "$functional"; then
	echo "suite exit=2"
	exit 2
fi
shopt -s nullglob
scripts=([0-9][0-9][0-9][0-9]-*.sh)
if [[ $tests != all ]]; then
	if [[ ! $tests =~ ^[0-9]{4}(,[0-9]{4})*$ ]]; then
		echo "invalid functional case selection: $tests"
		echo "suite exit=2"
		exit 2
	fi
	IFS=, read -ra selectors <<< "$tests"
	for selector in "${selectors[@]}"; do
		matches=("$selector"-*.sh)
		if (( ${#matches[@]} == 0 )); then
			echo "no functional case matches: $selector"
			echo "suite exit=2"
			exit 2
		fi
	done
fi
status=0
ran=0
for script in "${scripts[@]}"; do
	number=${script%%-*}
	if [[ $tests != all ]]; then
		case ",$tests," in *",$number,"*) ;; *) continue ;; esac
	fi
	ran=$((ran + 1))
	printf '%s: ' "$script"
	# Detach stdin: tools such as vim -es wait on a terminal after errors.
	if timeout -s KILL "$case_timeout" "./$script" "$mnt" < /dev/null; then
		echo PASS
	else
		rc=$?
		echo "FAIL rc=$rc"
		status=1
		(( rc == 137 )) && diagnostics
	fi
done
if (( ran == 0 )); then
	echo "no functional cases ran for tests=$tests"
	status=2
fi
echo "suite exit=$status"
umount "$mnt" || umount -f "$mnt" || echo "umount failed"
exit "$status"
