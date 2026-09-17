#!/bin/sh

# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

set -eu

if [ "$#" -ne 3 ]; then
    echo "usage: $0 MODULE_PATH MODULE_NAME PASS_TEXT" >&2
    exit 2
fi
if [ "$(id -u)" -ne 0 ]; then
    echo "run this script as root" >&2
    exit 2
fi

module_path=$1
module_name=$2
pass_text=$3
line_before=$(dmesg | wc -l)
new_log=$(mktemp "${TMPDIR:-/tmp}/ternfs-module-test.XXXXXX")
loaded=0
command_status=0

cleanup() {
    if [ "$loaded" -eq 1 ]; then
        rmmod "$module_name" >/dev/null 2>&1 || true
    fi
    rm -f "$new_log"
}
trap cleanup EXIT HUP INT TERM

if insmod "$module_path"; then
    loaded=1
else
    command_status=$?
fi
if [ "$loaded" -eq 1 ]; then
    if rmmod "$module_name"; then
        loaded=0
    else
        command_status=$?
    fi
fi

dmesg | tail -n "+$((line_before + 1))" >"$new_log"
cat "$new_log"

if grep -E \
    'BUG:|KASAN:|WARNING:|Oops:|kernel panic|general protection fault|use-after-free|out-of-bounds' \
    "$new_log" >/dev/null; then
    echo "new kernel diagnostics contain a fault" >&2
    exit 1
fi
if [ "$command_status" -ne 0 ]; then
    echo "module load or unload failed with status $command_status" >&2
    exit "$command_status"
fi
if ! grep -F "$pass_text" "$new_log" >/dev/null; then
    echo "missing expected success text: $pass_text" >&2
    exit 1
fi

trap - EXIT HUP INT TERM
rm -f "$new_log"
