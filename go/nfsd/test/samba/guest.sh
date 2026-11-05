#!/usr/bin/env bash
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Run by guest-init.sh in the qemu guest once nfsd is mounted. Starts smbd with
# a share on the mount and a tmpfs control share with the same options, runs
# each smbtorture suite against both, and writes the raw output to the results
# directory on the host. The driver compares the two; this script fails only
# when it cannot run the suites.
#
# usage: guest.sh MOUNT RESULTS_DIR SUITES SUITE_TIMEOUT_SECONDS

set -u
mnt=$1
results=$2
suites=$3
suite_timeout=$4
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
run=/run/samba
control=/run/samba-control
conf=$run/smb.conf
password=torture
user=nfsdsmb
uid=61001

for tool in smbd smbtorture smbpasswd smbcontrol; do
	if ! command -v "$tool" > /dev/null; then
		echo "SAMBA $tool is not installed (Debian and Ubuntu: samba samba-testsuite)"
		exit 2
	fi
done

# The guest's root is the host's, writable only where the user running qemu
# may write. Everything Samba and smbtorture write goes under $run.
mkdir -p "$run"/{private,lock,state,cache,pid,ncalrpc,binddns,log,torture} "$control" "$mnt/samba"
cd "$run"
sed -e "s|@RUN@|$run|g" -e "s|@NFS@|$mnt/samba|g" -e "s|@CONTROL@|$control|g" \
	-e "s|@DFREE@|$here/dfree.sh|g" -e "s|@USER@|$user|g" "$here/smb.conf.in" > "$conf"
# The guest's /etc is the host's. Resolve its own name without touching it.
printf '127.0.0.1 localhost %s\n' "$(hostname)" > "$run/hosts"
mount --bind "$run/hosts" /etc/hosts 2> /dev/null || true

# The SMB user is an ordinary user, as Windows users are through winbind,
# added to a copy of the host's passwd and group files.
cp /etc/passwd "$run/passwd"
cp /etc/group "$run/group"
echo "$user:x:$uid:$uid:smbtorture:/nonexistent:/bin/false" >> "$run/passwd"
echo "$user:x:$uid:" >> "$run/group"
if ! mount --bind "$run/passwd" /etc/passwd || ! mount --bind "$run/group" /etc/group; then
	echo "SAMBA cannot add the user $user"
	exit 2
fi
if ! printf '%s\n%s\n' "$password" "$password" | smbpasswd -c "$conf" -s -a "$user" > /dev/null; then
	echo "SAMBA smbpasswd failed for $user"
	exit 2
fi
if ! smbd -s "$conf" -l "$run/log" -D; then
	echo "SAMBA smbd did not start"
	exit 2
fi
# Port 445 (01BD) listening on loopback.
for _ in $(seq 1 100); do
	grep -q ':01BD 00000000:0000 0A' /proc/net/tcp && break
	sleep 0.2
done
if ! grep -q ':01BD 00000000:0000 0A' /proc/net/tcp; then
	echo "SAMBA smbd is not listening"
	tail -20 "$run"/log/log.smbd 2>/dev/null
	exit 2
fi
echo "SAMBA-OK $(smbd --version) user=$user"

mkdir -p "$results/nfs" "$results/control"
for suite in ${suites//,/ }; do
	for share in nfs control; do
		start=$(date +%s)
		timeout -s KILL "$suite_timeout" smbtorture -s "$conf" "//127.0.0.1/$share" \
			-U "$user%$password" --basedir="$run/torture" --option=torture:timelimit=15 "$suite" \
			> "$results/$share/$suite.txt" 2>&1 < /dev/null
		rc=$?
		# The driver rejects a suite that timed out, crashed or never ran.
		echo "$rc" > "$results/$share/$suite.rc"
		echo "SAMBA share=$share suite=$suite rc=$rc took=$(( $(date +%s) - start ))s"
	done
done

smbcontrol -s "$conf" smbd shutdown 2> /dev/null
sleep 1
cp -r "$run/log" "$results/smbd-logs"
umount /etc/hosts 2> /dev/null || true
umount /etc/passwd /etc/group
exit 0
