// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
)

// OpLatencyBounds are the upper bounds, in seconds, of the NFS operation
// latency histogram; a final bucket counts everything above the last one.
// They are chosen so that the interesting edges of an editing workload
// fall on bucket boundaries: a shard round trip, a shard mutation, a save,
// a large publish, and the Linux client's 15 s session backoff.
var OpLatencyBounds = []float64{0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 15, 60}

// Metrics are the server's cumulative counters. Gauges are read from the
// server's own state when a snapshot is taken (MetricsSnapshot). The main
// package exports snapshots over the TernFS metrics path; this package
// only counts.
type Metrics struct {
	mu      sync.Mutex
	ops     map[opStatusKey]uint64
	latency map[uint32]*opLatency

	compounds                atomic.Uint64
	reads, writes            atomic.Uint64
	readBytes, writeBytes    atomic.Uint64
	connAccepted             atomic.Uint64
	connClosedByClient       atomic.Uint64
	connClosedByServer       atomic.Uint64
	connIdleTimeouts         atomic.Uint64
	connReadErrors           atomic.Uint64
	parseErrors              atomic.Uint64
	workWaits, workWaitNanos atomic.Uint64
	workWaiters              atomic.Int64
	publishes                atomic.Uint64
}

type opStatusKey struct{ op, status uint32 }

type opLatency struct {
	buckets  []uint64 // one per bound, plus the overflow bucket
	sumNanos uint64
	count    uint64
}

func newMetrics() *Metrics {
	return &Metrics{ops: make(map[opStatusKey]uint64), latency: make(map[uint32]*opLatency)}
}

// observeOp records one operation's status and duration, under the opcode
// its result carries on the wire, so a client's arbitrary opcodes cannot
// add series or names that are not valid tags.
func (m *Metrics) observeOp(op, status uint32, d time.Duration) {
	op = wireOperation(op)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ops[opStatusKey{op, status}]++
	l := m.latency[op]
	if l == nil {
		l = &opLatency{buckets: make([]uint64, len(OpLatencyBounds)+1)}
		m.latency[op] = l
	}
	secs := d.Seconds()
	i := sort.SearchFloat64s(OpLatencyBounds, secs) // first bound >= secs
	l.buckets[i]++
	l.sumNanos += uint64(max(d, 0))
	l.count++
}

// opResult is an operation result not yet counted, because a panic before
// the compound returns can still replace it with SERVERFAULT.
type opResult struct {
	op, status uint32
	elapsed    time.Duration
	set        bool
}

// observeResult counts a pending result once, and clears it.
func (m *Metrics) observeResult(r *opResult) {
	if r.set {
		m.observeOp(r.op, r.status, r.elapsed)
		*r = opResult{}
	}
}

// connEnd is why a connection ended. A handler keeps the first cause it
// observes and counts it once, when it returns.
type connEnd uint32

const (
	connEndUnset      connEnd = iota // every exit records a cause; counted as a server close
	connEndServer                    // a failed reply write, a panic, a request refused or dropped, a conflicting XID
	connEndClient                    // EOF or reset
	connEndIdle                      // no request within the idle timeout
	connEndReadError                 // any other read failure
	connEndParseError                // an RPC that would not parse
)

func (m *Metrics) connectionEnded(why connEnd) {
	switch why {
	case connEndClient:
		m.connClosedByClient.Add(1)
	case connEndIdle:
		m.connIdleTimeouts.Add(1)
	case connEndReadError:
		m.connReadErrors.Add(1)
	case connEndParseError:
		m.parseErrors.Add(1)
	default:
		m.connClosedByServer.Add(1)
	}
}

func (m *Metrics) addIO(read bool, n int) {
	if read {
		m.reads.Add(1)
		m.readBytes.Add(uint64(n))
	} else {
		m.writes.Add(1)
		m.writeBytes.Add(uint64(n))
	}
}

// OpCount is one (operation, status) counter.
type OpCount struct {
	Op, Status string
	Count      uint64
}

// OpLatencySnapshot is one operation's cumulative histogram: Cumulative[i]
// counts observations at or below OpLatencyBounds[i], and the final entry
// counts all of them.
type OpLatencySnapshot struct {
	Op         string
	Cumulative []uint64
	Count      uint64
	SumNanos   uint64
}

// MetricsSnapshot is everything the exporter needs, taken at one instant.
type MetricsSnapshot struct {
	Ops     []OpCount
	Latency []OpLatencySnapshot

	Compounds, Reads, Writes, ReadBytes, WriteBytes uint64

	Connections, InFlight, MaxInFlight            int
	ConnAccepted, ConnClosedByClient              uint64
	ConnClosedByServer, ConnIdleTimeouts          uint64
	ConnReadErrors, ParseErrors                   uint64
	WorkBytes, WorkLimit, WorkReplies             int
	WorkWaiters                                   int64
	WorkWaits, WorkWaitNanos                      uint64
	CacheBytes, CacheLimit                        int
	Sessions, Records, Groups, ActiveRequests     int
	Opens, Aliases                                int
	Publishes                                     uint64
	StagingFiles, StagingRetired, StagingUnlinked int
	StagingBusy                                   int
	StagingBytes                                  uint64
	HydrationsInFlight                            int
}

// MetricsSnapshot reads the counters and the gauges the server can report
// about itself. It takes each lock briefly and never blocks on the backend.
func (s *Server) MetricsSnapshot() MetricsSnapshot {
	m := s.metrics
	snap := MetricsSnapshot{
		Compounds: m.compounds.Load(), Reads: m.reads.Load(), Writes: m.writes.Load(),
		ReadBytes: m.readBytes.Load(), WriteBytes: m.writeBytes.Load(),
		ConnAccepted: m.connAccepted.Load(), ConnClosedByClient: m.connClosedByClient.Load(),
		ConnClosedByServer: m.connClosedByServer.Load(), ConnIdleTimeouts: m.connIdleTimeouts.Load(),
		ConnReadErrors: m.connReadErrors.Load(), ParseErrors: m.parseErrors.Load(),
		WorkWaiters: m.workWaiters.Load(), WorkWaits: m.workWaits.Load(), WorkWaitNanos: m.workWaitNanos.Load(),
		Publishes: m.publishes.Load(),
	}
	m.mu.Lock()
	for k, n := range m.ops {
		snap.Ops = append(snap.Ops, OpCount{Op: nfs4.NfsOpnum4Name(k.op), Status: nfs4.Nfsstat4Name(k.status), Count: n})
	}
	for op, l := range m.latency {
		c := make([]uint64, len(l.buckets))
		var running uint64
		for i, n := range l.buckets {
			running += n
			c[i] = running
		}
		snap.Latency = append(snap.Latency, OpLatencySnapshot{Op: nfs4.NfsOpnum4Name(op), Cumulative: c, Count: l.count, SumNanos: l.sumNanos})
	}
	m.mu.Unlock()
	sort.Slice(snap.Ops, func(i, j int) bool {
		return snap.Ops[i].Op < snap.Ops[j].Op || snap.Ops[i].Op == snap.Ops[j].Op && snap.Ops[i].Status < snap.Ops[j].Status
	})
	sort.Slice(snap.Latency, func(i, j int) bool { return snap.Latency[i].Op < snap.Latency[j].Op })

	s.connsMu.Lock()
	snap.Connections = len(s.conns)
	for c := range s.conns {
		snap.InFlight += int(c.inFlight.Load())
	}
	s.connsMu.Unlock()
	snap.MaxInFlight = snap.Connections * s.MaxInFlightPerConn

	acc := s.sessions.accounting()
	snap.WorkBytes, snap.WorkLimit, snap.WorkReplies = acc.WorkBytes, acc.WorkLimit, acc.WorkReplies
	snap.CacheBytes, snap.CacheLimit = acc.CacheBytes, acc.CacheLimit
	snap.Sessions, snap.Records, snap.Groups, snap.ActiveRequests = acc.Sessions, acc.Records, acc.Groups, acc.ActiveRequests

	snap.Opens = s.opens.StateCount()
	snap.Aliases = len(s.views.Snapshot().Aliases)
	snap.HydrationsInFlight = len(s.hydrationSlots)
	// A staging file's lock is held across writes, truncates and fsyncs, so
	// the files are inspected as the diagnostics do, without waiting for it.
	// A busy file is counted, but its size and flags are not known. Sizes
	// are live, including UNSTABLE writes the checkpoint does not have yet.
	if local, ok := s.stagingStore.(*staging.LocalStore); ok {
		records, _ := local.Snapshot()
		for _, r := range records {
			snap.StagingFiles++
			if r.Locked {
				snap.StagingBusy++
				continue
			}
			snap.StagingBytes += r.Size
			if r.Retired {
				snap.StagingRetired++
			}
			if r.Unlinked {
				snap.StagingUnlinked++
			}
		}
	}
	return snap
}
