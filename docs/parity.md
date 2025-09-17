<!--
Copyright 2025 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

TernFS supports erasure coding in any configuration, as long as the parity and data blocks are between 1 and 16 and 0 and 16, respectively.

When working with spinning disks we want to not have seek time to dominate the time spent by the block services.

On those disks, seeking takes the time it takes to transfer 1.4MB at their cited "sustained transfer rate" (`291MB / 212IOPS = 1.4MB`). Let's be conservative and say 1.5MB.

If we rather arbitrarily aim to have seek time 50% of the the total "block read" operation, we should aim to have blocks which are on average 1.5MB. A conservative option is to pick spans that never go below that (apart from where we can't help it), by using span policies:

Max span size | Parity
--: | ---
3MB           | (1,4)
4.5MB         | (2,4)
...           | ...
15MB          | (10,4)

Note how the first max span size is 3MB to guarantee that no blocks in the next one go below 1.5MB.

Moreover, the first level is the level that should be stored in flash, to completely avoid any block < 1.5MB in HDDs. Once we're storing them in flash, we can have blocks as small as we like, i.e. we can have (10,4) even at the first level.

Moreover giving a bit of slack (say 2.5MB rather than 1.5MB) loses us very little -- 10% additional flash storage according to my current experiments.
