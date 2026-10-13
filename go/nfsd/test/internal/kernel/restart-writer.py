#!/usr/bin/env python3
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Run inside the qemu guest by guest-init.sh for the kernel driver's
# -restart-mid-write mode. One descriptor writes and fsyncs a prefix, asks the
# host to restart nfsd, waits until it has, then writes and fsyncs the rest
# and closes. The host checks the published bytes through the backend; the
# lines printed here record what the guest's NFS client sent.

import os
import sys
import time

# Keep in step with restartWriterData in main.go.
NAME = "restart-mid-write.dat"
PREFIX = b"written before the nfsd restart\n" * 2048
SUFFIX = b"written after the nfsd restart\n" * 2048
OPS = ("EXCHANGE_ID", "CREATE_SESSION", "DESTROY_SESSION", "SEQUENCE", "OPEN",
       "OPEN_NOATTR", "CLOSE", "WRITE", "COMMIT", "RECLAIM_COMPLETE",
       "BIND_CONN_TO_SESSION", "TEST_STATEID", "FREE_STATEID")


def op_counts(mnt):
    # /proc/self/mountstats per-op lines: "OP: requests transmissions ..."
    counts, mine = {}, False
    with open("/proc/self/mountstats") as stats:
        for line in stats:
            if line.startswith("device "):
                mine = (" mounted on %s " % mnt) in line
                continue
            if not mine:
                continue
            name, sep, rest = line.strip().partition(":")
            if sep and name in OPS and rest.split():
                counts[name] = int(rest.split()[0])
    return counts


def report(label, counts):
    print("RESTART-OPS %s %s" % (label, " ".join(
        "%s=%d" % (op, counts.get(op, 0)) for op in OPS)), flush=True)


def wait_for(path, timeout):
    deadline = time.monotonic() + timeout
    while not os.path.exists(path):
        if time.monotonic() > deadline:
            return False
        time.sleep(0.2)
    return True


def main():
    mnt, control = sys.argv[1], sys.argv[2]
    path = os.path.join(mnt, NAME)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o644)
    os.write(fd, PREFIX)
    os.fsync(fd)
    before = op_counts(mnt)
    report("before", before)
    open(os.path.join(control, "ready"), "w").close()
    if not wait_for(os.path.join(control, "restarted"), 300):
        print("RESTART-WRITER host did not restart nfsd", flush=True)
        return 3
    started = time.monotonic()
    os.write(fd, SUFFIX)
    os.fsync(fd)
    os.close(fd)
    print("RESTART-WRITER recovered write and close in %.1fs" % (time.monotonic() - started), flush=True)
    after = op_counts(mnt)
    report("after", after)
    report("delta", {op: after.get(op, 0) - before.get(op, 0) for op in OPS})
    with open(path, "rb") as f:
        if f.read() != PREFIX + SUFFIX:
            print("RESTART-WRITER guest read back different bytes", flush=True)
            return 4
    return 0


if __name__ == "__main__":
    sys.exit(main())
