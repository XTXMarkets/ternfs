#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

short=""
leader_only=""
preserve_ddir=""

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -short)
            short="-short"
            shift
            ;;
        -leader-only)
            leader_only="-leader-only"
            shift
            ;;
        -preserve-data-dir)
            preserve_ddir="-preserve-data-dir"
            shift
	    ;;
        *)
            echo "Bad usage -- only accepted flags are -short, -leader-only and -preserve-data-dir"
            exit 2
            ;;
    esac
done

echo "Running with short $short"
echo "Running with leader_only $leader_only"
echo "Running with preserve_ddir $preserve_ddir"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

# build kernel module
make "KDIR=${SCRIPT_DIR}/linux" -j ternfs-client

# start VM in the background
function cleanup {
    echo 'Syncing logs'
    rsync -e "ssh -p 2223 -i image-key" -avm --include='/tern-integrationtest.**/' --include=log --include=test-log --include=stderr --include=stdout --exclude='*' fmazzol@localhost:/tmp/ ./ || echo 'Could not sync logs'
    echo 'Terminating QEMU'
    pkill qemu
}
trap cleanup EXIT
./startvm.sh &>vm-out &

# Wait for VM to go up by trying to copy the kernel module to it
chmod 0600 image-key
scp_attempts=0
while ! scp -v -P 2223 -o StrictHostKeyChecking=no -i image-key ternfs-client.ko fmazzol@localhost: ; do
    sleep 1
    scp_attempts=$((scp_attempts + 1))
    if [ $scp_attempts -ge 20 ]; then
        echo "Couldn't reach qemu"
        exit 1
    fi
done

# Deploy ternfs
./vm_deploy.py

# Insert module
ssh -p 2223 -i image-key fmazzol@localhost "sudo insmod ternfs-client.ko"

# Set up permissions to read kmsg
ssh -p 2223 -i image-key fmazzol@localhost "sudo chmod 666 /dev/kmsg"

# Log dmesg
ssh -p 2223 -i image-key fmazzol@localhost "stdbuf -oL sudo dmesg -wT" > dmesg &
dmesg_pid=$!

# Trace metadata requests

ssh -p 2223 -i image-key fmazzol@localhost "sudo sh -c 'echo 1 > /sys/kernel/tracing/events/eggsfs/eggsfs_metadata_request/enable'"
ssh -p 2223 -i image-key fmazzol@localhost "sudo cat /sys/kernel/tracing/trace_pipe" > trace &
trace_pid=$!

# Do not test migrations/scrubbing since we test this outside qemu anyway
# (it's completely independent from the kmod code)
ssh -p 2223 -i image-key fmazzol@localhost "tern/terntests -verbose -kmsg -kmod -filter 'large file|cp|utime|dir seek|ftruncate' -block-service-killer -cfg fsTests.dontMigrate -cfg fsTests.dontDefrag -cfg fsTest.corruptFileProb=0 -outgoing-packet-drop 0.02 $short $leader_only $preserve_ddir -binaries-dir tern" | tee -a test-out
ssh -p 2223 -i image-key fmazzol@localhost "tern/terntests -verbose -kmsg -kmod -filter 'mounted' -block-service-killer -cfg fsTests.dontMigrate -cfg fsTests.dontDefrag -cfg fsTest.corruptFileProb=0 -outgoing-packet-drop 0.02 $short $leader_only $preserve_ddir -binaries-dir tern" | tee -a test-out
ssh -p 2223 -i image-key fmazzol@localhost "tern/terntests -verbose -kmsg -kmod -filter 'mounted' -block-service-killer -cfg fsTest.readWithMmap -cfg fsTests.dontMigrate -cfg fsTests.dontDefrag -cfg fsTest.corruptFileProb=0 -outgoing-packet-drop 0.02 $short $leader_only $preserve_ddir -binaries-dir tern" | tee -a test-out
ssh -p 2223 -i image-key fmazzol@localhost "tern/terntests -verbose -kmsg -kmod -filter 'rsync' -block-service-killer -cfg fsTests.dontMigrate -cfg fsTests.dontDefrag -cfg fsTest.corruptFileProb=0 -outgoing-packet-drop 0.02 $short $leader_only $preserve_ddir -binaries-dir tern" | tee -a test-out

echo 'Unmounting'
timeout -s KILL 300 ssh -p 2223 -i image-key fmazzol@localhost "grep eggsfs /proc/mounts | awk '{print \$2}' | xargs -r sudo umount"

echo 'Removing module'
timeout -s KILL 300 ssh -p 2223 -i image-key fmazzol@localhost "sudo rmmod ternfs-client"

kill $trace_pid
kill $dmesg_pid

echo 'Checking for BUG/WARNING in dmesg'
set +e
grep -e BUG -e WARNING dmesg >/dev/null
grep_failed="$?"
set -e
if [[ "$grep_failed" -eq "0" ]]; then
    echo "BUG|WARNING found in dmesg:"
    grep -A 2 -e BUG -e WARNING dmesg | tail -10
    exit 1
fi
