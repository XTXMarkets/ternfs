# Block service disk I/O limits

`ternblocks` limits concurrent disk requests to prevent stalled filesystems
from exhausting Go's OS threads. Slow reads can exhaust threads just as
slow writes can; socket deadlines cannot interrupt regular-file syscalls.
These limits complement [space accounting](block-storage.md).

Requests have three independent admission budgets:

| Budget | Requests covered | Process limit | Per-service limit |
| --- | --- | --- | --- |
| Read | `FETCH_BLOCK`, `FETCH_BLOCK_WITH_CRC`, `CHECK_BLOCK` | `-max-reads=4096` | `-max-reads-per-block-service=128` |
| Write | `WRITE_BLOCK`, `TEST_WRITE` | `-max-writes=2048` | `-max-writes-per-block-service=64` |
| Erase | `ERASE_BLOCK` | `-max-erases=1024` | `-max-erases-per-block-service=1` |

With defaults, at most 7,168 client requests can perform filesystem work
simultaneously across the process, and at most 193 per block service.
Read and write requests cannot consume the erase budget, and stalled
writes cannot consume the read budget. Background capacity sampling and
block counting use their existing independent, sequential loops.

These defaults allow hosts with 30 NVMes to reach every device's
per-service limits simultaneously: 3,840 reads, 1,920 writes, and 30
erases. On hosts with 106 HDDs, the process limits allow averages of about
39 reads and 19 writes per drive when evenly distributed; erases remain
serialized at one per drive.
The combined process limit leaves 2,832 threads below Go's default
10,000-thread limit for runtime and background work.

Each operation type has a fixed pool of worker goroutines, sized by its
process limit. Requests reserve a per-service slot through a bounded
channel, then take an idle worker from that operation's pool. This happens
after parsing the request header, before opening files, querying metadata,
or allocating a write payload buffer. A worker never waits on a saturated
service's slot. The service map is fixed at startup; there is no shared
admission mutex or broadcast wakeup across services.

The worker performs the whole operation, including verification, `fsync`,
rename, cleanup, and the response. A write's verification reads stay within
its write assignment; they do not acquire another read worker.

Slots are released only when the worker's handler returns. Closing the client
connection or reaching its deadline cannot release a slot while a
filesystem syscall is still running. Once all slots for a stalled service
are occupied, further requests queue briefly or are rejected.

## Waiting and overload

* `-max-io-waiters=4096` bounds the total admission queue for the process.
* `-max-io-waiters-per-block-service=128` bounds the admission queue for one
  service.
* `-io-wait-timeout=1s` bounds time in that queue. The request's socket
  deadline, when enabled, can shorten this wait.

Queue limits apply across all three budgets. An immediately admissible
request does not need a queue position. Waiting requests hold no worker or
payload buffer. A request waiting for a worker reserves its service slot;
that reservation is returned if the wait expires. Admission is not strictly FIFO.
Setting either queue limit or the wait timeout to zero disables waiting.
Concurrency limits must be positive; queue limits and wait timeout must
not be negative. The existing `-max-erases-per-block-service` flag is
retained, but zero now produces a configuration error instead of being
rounded up to one.

When admission fails, the server sends the existing `TIMEOUT` error and
closes the connection. A writer still sending its body may observe a
connection reset instead. This preserves protocol compatibility and avoids
reusing a stream with an unread write body. Overload does not count as a
hardware I/O error or trigger block-service decommissioning.

Both ordinary writes and `TEST_WRITE` are capped at the existing maximum
block size of 100 MiB, so test writes cannot bypass the payload allocation
bound. Tests requiring larger transfers should send multiple requests.

## Monitoring and tuning

The `eggsfs_blocks_io` measurement has `failuredomain`, `pathprefix`,
`operation` (`read`, `write`, `erase`), and `scope` (`process`,
`blockservice`) tags. Per-service rows also carry `blockservice`. Fields:

* `active`: admitted requests still holding slots.
* `waiting`: requests currently queued for admission.
* `rejected`: cumulative admission rejections, including expired waits.
* `limit`: configured concurrent-request limit for that row.

Process and per-service rows are emitted from startup. Counters are sampled
independently with atomics, without pausing workers; they are monitoring
samples rather than a transactionally consistent snapshot.

Tune limits for the device count, latency, throughput, and write-buffer
memory budget. Keep the sum of process limits comfortably below Go's
thread limit, leaving room for startup, monitoring, logging, and other
runtime work. Raising per-service limits cannot exceed the corresponding
process limit.

Write limits count requests, not bytes. At the maximum block size, 2,048
active writes can buffer 200 GiB of payload before CRC and buffer-pool
overhead; choose a lower write limit where the host's memory budget
requires it.

The limits contain resource growth when I/O stalls; they cannot make an
unresponsive filesystem complete an operation. A filesystem shared by
several logical block services still shares underlying device resources,
so use the process limit to bound their aggregate load. The limits cover
requests with parsed headers, not the number of idle TCP connections.
