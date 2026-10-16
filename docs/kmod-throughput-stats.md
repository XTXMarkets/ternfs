<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: GPL-2.0-or-later
-->

# Kernel client throughput counters

The kernel client exposes cumulative, host-wide byte counters in sysfs:

| File | Meaning |
| --- | --- |
| `/sys/fs/eggsfs/stats/block_read_bytes` | Block payload bytes received from block services |
| `/sys/fs/eggsfs/stats/block_write_bytes` | Block payload bytes accepted by TCP for sending to block services |

Both counters are unsigned 64-bit decimal numbers, aggregated across CPUs and
mounts. They start at zero when the module is loaded. Scraping does not reset
them. Use a counter-aware rate calculation to handle module reloads.

These measure block traffic, including erasure-coding overhead, read-ahead,
partial transfers, and repeated transfers on retries. They count bytes even
when the operation subsequently fails. Write bytes are counted when accepted
by the socket, without waiting for a server acknowledgement.

They exclude cached reads, inline file data carried by metadata RPCs, metadata
traffic, block protocol headers and CRCs, and TCP/IP overhead or TCP
retransmissions. They therefore measure block payload throughput, rather than
application read/write throughput or total network interface bandwidth.
