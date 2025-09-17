#!/usr/bin/env bash

# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu -o pipefail

run=false
deploy=false
build_type='alpinedebug'

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        -run)
            run=true
            shift
            ;;
        -deploy)
            deploy=true
            shift
            ;;
        -build-type)
            shift
            build_type="$1"
            shift
            ;;
        *)
            echo "Bad usage"
            exit 2
            ;;
    esac
done

# Kills the current VM, starts it again, deploys to it and builds kmod, loads kmod, opens a tmux
# session with dmesg in one pane and a console in ~/tern in the other.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR

make KDIR=$SCRIPT_DIR/linux -j kmod

pkill -9 qemu || true
tmux kill-session -t uovo || true

while [ $(pgrep -c qemu) -gt 0 ]; do echo old qemu still running; sleep 1; done # sometimes qemu lingers?

# Windows:
#
# 0. dmesg
# 1. free shell
# 2. shell with ternfs running
# 3. trace_pipe
# 4. qemu running

tmux new-session -d -s uovo
tmux new-window -t uovo:4 './startvm.sh'

# Wait for VM to go up, and build
while ! scp -i image-key -P 2223 ternfs.ko fmazzol@localhost: ; do sleep 1; done

# Start dmesg as soon as it's booted (before we insert module)
tmux send-keys -t uovo:0 "ssh -i image-key -t fmazzol@localhost -p 2223 -i image-key dmesg -wHT | tee dmesg" Enter

# and trace_pipe
tmux new-window -t uovo:3 'ssh -i image-key -t fmazzol@localhost -p 2223 sudo cat /sys/kernel/debug/tracing/trace_pipe'

# Insert module
ssh -i image-key -p 2223 fmazzol@localhost 'sudo insmod ternfs.ko'

if [[ "$deploy" = true ]]; then
    # Deploy binaries
    ./vm_deploy.py --build-type "$build_type"
fi

# Create shells
tmux new-window -t uovo:1 'ssh -i image-key -t fmazzol@localhost -p 2223'
if [[ "$run" = true ]]; then
    tmux new-window -t uovo:2 'ssh -i image-key -t fmazzol@localhost -p 2223 "cd tern && ./ternrun -verbose -binaries-dir ~/tern -data-dir ~/tern-data/ -leader-only"'
fi

# Attach
tmux attach-session -t uovo:1

# ./tern/terntests -verbose -kmod -filter 'mounted|rsync|large' -short -binaries-dir $(pwd)/tern

# sudo sysctl fs.eggsfs.debug=1
# ./tern/terntests -kmod -filter 'mounted' -cfg fsTest.checkThreads=4 -cfg fsTest.numDirs=1 -cfg fsTest.numFiles=100 -short -binaries-dir $(pwd)/tern
