<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Disk Failures

## How TernFS stores files

* Each file is made out of a bunch of spans.
* Each span is made out of a bunch of blocks. Currently we organize the spans such that any 4 blocks inside a span can fail for whatever reason and the span is still readable.
* Each block has a unique block id, and is stored in a "block service".
* A block service is really an individual disk, and also has a unique "block service id".
* Many block services (i.e. disks) are grouped in a single server. In this context we call such a server a "failure domain", since the block services within it can fail all at once.
* All block services in a single server are serviced by a single process (`eggsblocks`).
* You can witness this hierarchy in the front page of the web UI.

## What to do when a disk fails

Note that this section is out of date. It was written when there was no automatic marking of failed disks. Moreover now it is not necessary to restart block services after decommissioning. Leaving it here since many concepts still apply, but should be updated.

From time to time disks will fail. As they do, things will just keep working as normal (both the read and write end), as long as we don't have more than 4 disks failed at once before we migrated away from them.

We might notice that disks fail in a variety of ways (alerts in the block service, hostmon, etc.). The best telltale sign of a failed disk is a flurry of `BLOCK_IO_ERROR_DEVICE` clearable alerts in the block service, and a non-clearable alert saying "could not update block service info for block service ...". See also  the [alerts doc](https://github.com/XTXMarkets/ternfs/blob/main/docs/alerts.md). But in any case, once we know a disk has failed, these are the steps to restore sanity.

1. Establish the block service id. Maybe the alerts that caused you to notice that the disk has failed already contain this, but if they don't, the easiest thing is to check which mount corresponds to which block service id by searching through block services in the front page of the web ui. We'll refer to this fictional failing block service with the id `0xbadc0ffee0ddf00d`.

1. If you're still investigating (i.e. you're not sure that the disk is actually dead), you can set the block service as read-only in the meantime to stop clients from attempting to write to it:

       % terncli -registry <registry-addr> -prod blockservice-flags -id 0xbadc0ffee0ddf00d -set NO_WRITE

    If you realize that the block service is actually fine, you can remove the `NO_WRITE` flag with

       % terncli -registry <registry-addr> -prod blockservice-flags -id 0xbadc0ffee0ddf00d -unset NO_WRITE

    The flags are visible in the web ui.

    Note that flags will take a bit to propagate (shouldn't be more than 2 minutes). However clients are smart, and they will automatically recover from failed reads/writes, so things will keep working throughout save from the occasional alert in `dmesg`.

1. Once you're sure that the disk is gone, you can mark it as such with

       % terncli -registry <registry-addr> -prod blockservice-flags -id 0xbadc0ffee0ddf00d -set DECOMMISSIONED

    Clients will stop reading/writing to the block service (again, with some delay). Note that setting a block service as decommissioned is a permanent action: from this point on, the block service is gone forever, and can only be migrated out of. Or in other words, the command above is not something to be put in automated scripts -- a human must vet it, at least for now.

1. Restart the block services in the machine where `0xbadc0ffee0ddf00d` used to live in.

    This will forget about the old block service id -- from now on requests asking for blocks in that block service will return `BLOCK_NOT_FOUND`. This means that there's going to be a short interval of time (around two minutes, see comment about flag propagation in point 1) whereby you might get `BLOCK_NOT_FOUND` alerts as shards realize that the block service is now decommissioned. These alerts can be binned.

1. `ternmigrate` instances will now start migrating the block service, you'll get a non-alerting alert such as

       % migrate.go:238 0xbadc0ffee0ddf00d: migrated 306565.72MB in 414548 blocks in 394912 files, at 1129.97MB/s (recent), 1021.56MB/s (overall)

    informing you of the progress.

1. Once the migration finishes (i.e. once the `ternmigrate` alert disappears) we're "safe", in the sense that we won't lose data because of the failed disk. Afterwards (or while the migration runs, but there's no urgency), do whatever you need to do to replace the disk.

    Once the disk is replaced (you can check the serial to make sure the right disk is replaced), you will have to umount the old mount, format the new disk, mount the new disk and chown the mount.

1. After you've replaced the disk and the partition is mounted, restart `ternblocks` again so that a new block service will be spun up.

    Again, clients will automatically work around the temporary failures due to the restart.
