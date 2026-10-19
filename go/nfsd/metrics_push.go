// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/server"
)

// Metrics are pushed as InfluxDB line protocol, as the other TernFS Go
// services do, so VictoriaMetrics stores each field as
// <measurement>_<field> with the tags as labels. Every line carries the
// filesystem and instance tags, so one dashboard separates filesystems and
// several nfsd processes on one host.
const (
	metricsPrefix          = "eggsfs_nfsd_"
	defaultMetricsInterval = 10 * time.Second

	// Measurements. Counters are cumulative since process start; rate()
	// applies. Gauges are the value at the push.
	measOps            = metricsPrefix + "ops"                    // tags op, status; count
	measOpLatencyBkt   = metricsPrefix + "op_latency_bucket"      // tags op, le; count
	measOpLatency      = metricsPrefix + "op_latency"             // tag op; count, sum_nanos
	measIO             = metricsPrefix + "io"                     // tag direction; ops, bytes
	measCompounds      = metricsPrefix + "compounds"              // count
	measConnections    = metricsPrefix + "connections"            // open (gauge) and close counters
	measAdmission      = metricsPrefix + "admission"              // in-flight, reply work, cache, sessions
	measState          = metricsPrefix + "state"                  // opens, aliases, publishes
	measStaging        = metricsPrefix + "staging"                // files, bytes, retired, unlinked, busy, hydrations
	measBackend        = metricsPrefix + "backend"                // tags service, kind; count, attempts
	measBackendLatency = metricsPrefix + "backend_latency_bucket" // tags service, kind, le; count
	measProcess        = metricsPrefix + "process"                // Go runtime figures
)

// backendLatencyBounds are the upper bounds, in seconds, of the exported
// backend request latency histogram. The client's own histogram is finer;
// its bins are folded into these.
var backendLatencyBounds = []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 10}

type metricsPusher struct {
	srv        *server.Server
	counters   *client.ClientCounters // nil without a TernFS backend
	db         *log.InfluxDB
	filesystem string
	instance   string
	started    time.Time
	log        *slog.Logger
}

// run pushes a snapshot every interval until stop is closed. A failed or
// panicking push is logged and the next one tried; metrics must never take
// the NFS server down.
func (p *metricsPusher) run(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var b log.MetricsBuilder
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			if err := p.push(&b, now); err != nil {
				p.log.Warn("metrics push failed", "err", err)
			}
		}
	}
}

// push builds and sends one snapshot, turning a panic in either into an
// error.
func (p *metricsPusher) push(b *log.MetricsBuilder, now time.Time) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("metrics push panicked: %v", r)
		}
	}()
	b.Reset()
	p.build(b, now)
	return p.db.SendMetrics(b.Payload())
}

func (p *metricsPusher) tags(b *log.MetricsBuilder) {
	b.Tag("filesystem", p.filesystem)
	b.Tag("instance", p.instance)
}

func leText(bound float64) string { return strconv.FormatFloat(bound, 'g', -1, 64) }

// build appends one push's lines to b.
func (p *metricsPusher) build(b *log.MetricsBuilder, now time.Time) {
	snap := p.srv.MetricsSnapshot()

	for _, c := range snap.Ops {
		b.Measurement(measOps)
		p.tags(b)
		b.Tag("op", c.Op)
		b.Tag("status", c.Status)
		b.FieldU64("count", c.Count)
		b.Timestamp(now)
	}
	for _, l := range snap.Latency {
		for i, bound := range server.OpLatencyBounds {
			b.Measurement(measOpLatencyBkt)
			p.tags(b)
			b.Tag("op", l.Op)
			b.Tag("le", leText(bound))
			b.FieldU64("count", l.Cumulative[i])
			b.Timestamp(now)
		}
		b.Measurement(measOpLatencyBkt)
		p.tags(b)
		b.Tag("op", l.Op)
		b.Tag("le", "+Inf")
		b.FieldU64("count", l.Count)
		b.Timestamp(now)
		b.Measurement(measOpLatency)
		p.tags(b)
		b.Tag("op", l.Op)
		b.FieldU64("count", l.Count)
		b.FieldU64("sum_nanos", l.SumNanos)
		b.Timestamp(now)
	}
	for _, d := range []struct {
		direction string
		ops, n    uint64
	}{{"read", snap.Reads, snap.ReadBytes}, {"write", snap.Writes, snap.WriteBytes}} {
		b.Measurement(measIO)
		p.tags(b)
		b.Tag("direction", d.direction)
		b.FieldU64("ops", d.ops)
		b.FieldU64("bytes", d.n)
		b.Timestamp(now)
	}
	b.Measurement(measCompounds)
	p.tags(b)
	b.FieldU64("count", snap.Compounds)
	b.Timestamp(now)

	b.Measurement(measConnections)
	p.tags(b)
	b.FieldU64("open", uint64(snap.Connections))
	b.FieldU64("accepted", snap.ConnAccepted)
	b.FieldU64("closed_by_client", snap.ConnClosedByClient)
	b.FieldU64("closed_by_server", snap.ConnClosedByServer)
	b.FieldU64("idle_timeouts", snap.ConnIdleTimeouts)
	b.FieldU64("read_errors", snap.ConnReadErrors)
	b.FieldU64("parse_errors", snap.ParseErrors)
	b.Timestamp(now)

	b.Measurement(measAdmission)
	p.tags(b)
	b.FieldU64("in_flight", uint64(snap.InFlight))
	b.FieldU64("max_in_flight", uint64(snap.MaxInFlight))
	b.FieldU64("work_bytes", uint64(snap.WorkBytes))
	b.FieldU64("work_limit", uint64(snap.WorkLimit))
	b.FieldU64("work_replies", uint64(snap.WorkReplies))
	b.FieldU64("work_waiters", uint64(max(snap.WorkWaiters, 0)))
	b.FieldU64("work_waits", snap.WorkWaits)
	b.FieldU64("work_wait_nanos", snap.WorkWaitNanos)
	b.FieldU64("cache_bytes", uint64(snap.CacheBytes))
	b.FieldU64("cache_limit", uint64(snap.CacheLimit))
	b.FieldU64("sessions", uint64(snap.Sessions))
	b.FieldU64("records", uint64(snap.Records))
	b.FieldU64("groups", uint64(snap.Groups))
	b.FieldU64("active_requests", uint64(snap.ActiveRequests))
	b.Timestamp(now)

	b.Measurement(measState)
	p.tags(b)
	b.FieldU64("opens", uint64(snap.Opens))
	b.FieldU64("aliases", uint64(snap.Aliases))
	b.FieldU64("publishes", snap.Publishes)
	b.Timestamp(now)

	b.Measurement(measStaging)
	p.tags(b)
	b.FieldU64("files", uint64(snap.StagingFiles))
	b.FieldU64("bytes", snap.StagingBytes)
	b.FieldU64("retired", uint64(snap.StagingRetired))
	b.FieldU64("unlinked", uint64(snap.StagingUnlinked))
	b.FieldU64("busy", uint64(snap.StagingBusy))
	b.FieldU64("hydrations_in_flight", uint64(snap.HydrationsInFlight))
	b.Timestamp(now)

	if p.counters != nil {
		p.buildBackend(b, now)
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	b.Measurement(measProcess)
	p.tags(b)
	b.FieldU64("heap_alloc_bytes", ms.HeapAlloc)
	b.FieldU64("sys_bytes", ms.Sys)
	b.FieldU64("goroutines", uint64(runtime.NumGoroutine()))
	b.FieldU64("gc_pause_nanos", ms.PauseTotalNs)
	b.FieldU64("gc_count", uint64(ms.NumGC))
	b.FieldU64("uptime_seconds", uint64(max(now.Sub(p.started), 0).Seconds()))
	b.Timestamp(now)
}

// buildBackend folds the TernFS client's per-shard, per-kind request
// counters into one series per request kind. The client keeps updating
// them, so each kind is read once into a detached snapshot, and its count
// and buckets all come from that one histogram. Attempts is sampled
// separately and may be a request ahead of or behind the count.
func (p *metricsPusher) buildBackend(b *log.MetricsBuilder, now time.Time) {
	type kindCounters struct {
		service, kind string
		attempts      uint64
		hist          []timing.HistogramBin
		total         uint64
	}
	var kinds []kindCounters
	add := func(service, kind string, c *client.ReqCounters) {
		hist := c.Timings.Histogram()
		var total uint64
		for _, bin := range hist {
			total += bin.Count
		}
		if total == 0 && c.Attempts == 0 {
			return
		}
		kinds = append(kinds, kindCounters{service, kind, c.Attempts, hist, total})
	}
	for k, perShard := range p.counters.Shard {
		add("shard", msgs.ShardMessageKind(k).String(), client.MergeReqCounters(perShard[:]))
	}
	for k, c := range p.counters.CDC {
		snap := c.Snapshot()
		add("cdc", msgs.CDCMessageKind(k).String(), &snap)
	}
	sort.Slice(kinds, func(i, j int) bool {
		return kinds[i].service < kinds[j].service || kinds[i].service == kinds[j].service && kinds[i].kind < kinds[j].kind
	})
	for _, kc := range kinds {
		hist, total := kc.hist, kc.total
		b.Measurement(measBackend)
		p.tags(b)
		b.Tag("service", kc.service)
		b.Tag("kind", kc.kind)
		b.FieldU64("count", total)
		b.FieldU64("attempts", kc.attempts)
		b.Timestamp(now)
		for _, bound := range backendLatencyBounds {
			var n uint64
			for _, bin := range hist {
				if bin.UpperBound.Seconds() <= bound {
					n += bin.Count
				}
			}
			b.Measurement(measBackendLatency)
			p.tags(b)
			b.Tag("service", kc.service)
			b.Tag("kind", kc.kind)
			b.Tag("le", leText(bound))
			b.FieldU64("count", n)
			b.Timestamp(now)
		}
		b.Measurement(measBackendLatency)
		p.tags(b)
		b.Tag("service", kc.service)
		b.Tag("kind", kc.kind)
		b.Tag("le", "+Inf")
		b.FieldU64("count", total)
		b.Timestamp(now)
	}
}

// defaultInstance names this nfsd process for the instance tag:
// the host's short name and the NFS port, so several nfsd processes on
// one host stay apart.
func defaultInstance(hostname, addr string) string {
	port := "2049"
	if i := len(addr) - 1; i >= 0 {
		for j := i; j >= 0; j-- {
			if addr[j] == ':' {
				if j < i {
					port = addr[j+1:]
				}
				break
			}
		}
	}
	for i, c := range hostname {
		if c == '.' {
			hostname = hostname[:i]
			break
		}
	}
	return fmt.Sprintf("%s:%s", hostname, port)
}
