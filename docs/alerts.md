<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Common TernFS alerts and what to do with them

* **`ternregistry`** `some decommissioned block services have to be replaced:`. This non-clearable alert is warning you that some drive which has been marked as DECOMMISSIONED has not been replaced. It's not urgent and therefore it's usually silenced, but it does require action eventually.

* **`ternblocks`**, **`terngcscrub`**: Alerts warning you about `BLOCK_IO_ERROR_FILE` or `BLOCK_IO_ERROR_DEVICE`. Mostly due to two reasons:

    1. One disk sector has gone bad. `BLOCK_IO_ERROR_FILE` is meant to detect this specific case, but it's not perfect.
    2. One drive is dead. As you probably guessed, this should manifest as `BLOCK_IO_ERROR_DEVICE`.

    The second option is usually easy to recognize since it causes a flurry of alerts rather than just a couple.
    
    Note that `ternscrub` will only alert if it caused the read that resulted in an error -- in which case you'll get alerts both in `terngcscrub` and `ternblocks`. If the error is caused by a bona-fide read from a node, you'll only get alerts in `ternblocks`.
    
    The action in this case is mostly checking if the drive needs to be migrated (if the drive is dead), following [the guide](https://github.com/XTXMarkets/ternfs/blob/main/docs/disk-failure.md). Afterwards the alerts can be cleared.

* **`ternblocks`**: `could not update block service info for block service ...`

    This alert is telling you that `ternblocks` is encountering IO errors when counting the number of blocks in a given block service. Like `BLOCK_IO_ERROR_DEVICE`, it almost certainly means that the disk needs to be decommisioned, see [guide](https://github.com/XTXMarkets/ternfs/blob/main/docs/disk-failure.md).

* **`terngcdirectories`**, **`terngcfiles`**, **`terngcscrub`**: Various timeout errors in the GC processes, such as:

    ```
    metadatareq.go:66 timed out when receiving resp 7317692486976124071 &{DirId:0x20000000678b0044 TargetId:0x40000005becf8144 Name:.20210619_20210716_1141_ezEcF4VT_LrFgx5M4_0_.mat.ldoNeo CreationTime:31d 08h 33m ago} of typ *msgs.RemoveNonOwnedEdgeReq (started 20.2519345s ago) to shard 68, might retry
    ```
    
    These timeout alerts are non clearable and will clear themselves. They tend to popup when the system is busy.
    
    See [the docs for garbage collection](https://github.com/XTXMarkets/ternfs/blob/main/docs/gc.md) for more info about what the GC processes do.

* **ternshard**: `blocks not found when swapping blocks, are you running two migrations at once`

    When a hard drive fails (both entirely, but also a single sector), we need to remove the blocks that can't be read from anymore from the files which have them.
    
    Two processes are mostly dedicated to this: `ternmigrate` and `ternscrub`. The first one looks at block services which have been manually set as decommissioned (see
[the disk migration guide](https://github.com/XTXMarkets/ternfs/blob/main/docs/disk-failure.md)) and moves all the blocks that were in the decommissioned drives out of the files which have them.

    `terngcscrub`, instead, just blindly reads all files, and when it encounters a block which can't be read or is just not found, it repairs the file in question.
    
    To do this repairing, these processes create a temporary transient file, reconstruct the block into the transient file, and then swap blocks between the transient file and the file we're repairing, so that the bad block end up in the transient file, from where it'll be garbage collected in due time.
    
    This is done with the `SwapBlocks` shard request. The alert above happens when something goes to swap a block but it does not find the expected block to swap in the file. This can happen when scrubbing and migration are trying to repair the same block, although this should be uncommon and ideally investigated.
