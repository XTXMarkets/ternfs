#!/usr/bin/env bash

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

# Runs the NFS integration tests inside the QEMU VM. This reuses the VM tooling
# that lives under kmod/; it mounts the userspace
# nfsd via the in-kernel NFSv4 client and runs the terntests suite over it.

set -eu -o pipefail

short=""
leader_only=""
preserve_ddir=""
filter="nfs"

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
        -filter)
            filter="$2"
            shift 2
            ;;
        *)
            echo "Bad usage -- only accepted flags are -short, -leader-only, -preserve-data-dir and -filter"
            exit 2
            ;;
    esac
done

echo "Running with short $short"
echo "Running with leader_only $leader_only"
echo "Running with preserve_ddir $preserve_ddir"
echo "Running with filter $filter"

REPO_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
# The QEMU tooling and SSH key live under kmod/; run from there so the relative
# paths in startvm.sh / vm_deploy.py resolve.
cd "$REPO_DIR/kmod"

# The VM listens on a fixed localhost:2223. A previous run (e.g. the kmod step)
# may have recorded a host key for it, and we boot a freshly-prepared image with
# new keys, so a stored key would mismatch and ssh would refuse. Don't use the
# known_hosts file at all for this throwaway VM.
SSH_OPTS=(-p 2223 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -i image-key)

# start VM in the background
function cleanup {
    echo 'Syncing logs'
    rsync -e "ssh ${SSH_OPTS[*]}" -avm --include='/tern-integrationtest.**/' --include=log --include=test-log --include=stderr --include=stdout --exclude='*' fmazzol@localhost:/tmp/ ./ || echo 'Could not sync logs'
    echo 'Terminating QEMU'
    pkill qemu
}
trap cleanup EXIT
./startvm.sh &>vm-out &

# Wait for VM to go up by trying to reach it over SSH
chmod 0600 image-key
ssh_attempts=0
while ! ssh "${SSH_OPTS[@]}" fmazzol@localhost true; do
    sleep 1
    ssh_attempts=$((ssh_attempts + 1))
    if [ $ssh_attempts -ge 60 ]; then
        echo "Couldn't reach qemu"
        exit 1
    fi
done

nfs_debs=(
    "http://archive.ubuntu.com/ubuntu/pool/main/k/keyutils/keyutils_1.6.3-3build1_amd64.deb"
    "http://archive.ubuntu.com/ubuntu/pool/main/n/nfs-utils/libnfsidmap1_2.6.4-3ubuntu5.1_amd64.deb"
    "http://archive.ubuntu.com/ubuntu/pool/main/r/rpcbind/rpcbind_1.2.6-7ubuntu2_amd64.deb"
    "http://archive.ubuntu.com/ubuntu/pool/main/n/nfs-utils/nfs-common_2.6.4-3ubuntu5.1_amd64.deb"
)
deb_dir=$(mktemp -d)
for url in "${nfs_debs[@]}"; do
    curl -fsSL -o "$deb_dir/$(basename "$url")" "$url"
done
scp -o Port=2223 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -i image-key "$deb_dir"/*.deb fmazzol@localhost:/tmp/
rm -rf "$deb_dir"
ssh "${SSH_OPTS[@]}" fmazzol@localhost "sudo dpkg -i /tmp/*.deb"


./vm_deploy.py

ssh "${SSH_OPTS[@]}" fmazzol@localhost "tern/terntests -verbose -nfs -filter '$filter' -cfg fsTest.dontMigrate -cfg fsTest.dontDefrag -cfg fsTest.corruptFileProb=0 $short $leader_only $preserve_ddir -binaries-dir tern 2>&1" | tee -a test-out

echo 'Unmounting NFS'
timeout -s KILL 300 ssh "${SSH_OPTS[@]}" fmazzol@localhost "grep nfs4 /proc/mounts | awk '{print \$2}' | xargs -r sudo umount" || true
