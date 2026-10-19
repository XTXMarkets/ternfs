// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/server"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Every line carries the filesystem and instance tags, the measurements the
// dashboard queries are present, and histogram buckets end with +Inf.
func TestMetricsPayload(t *testing.T) {
	ss, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.NewServer(vfs.NewLocalTernVFS(t.TempDir()), ss, nil)
	if err != nil {
		t.Fatal(err)
	}
	// One served operation and one backend request, so the latency
	// histograms, with their +Inf buckets, are in the payload: the builder
	// validates every tag value and the first push on QA died on "+Inf".
	srv.ObserveOpForTest(nfs4.OP_WRITE, nfs4.NFS4_OK, 3*time.Millisecond)
	counters := client.NewClientCounters()
	counters.Shard[uint8(msgs.LOOKUP)][7].Timings.Add(200 * time.Microsecond)
	counters.Shard[uint8(msgs.LOOKUP)][7].Attempts++
	counters.CDC[uint8(msgs.MAKE_DIRECTORY)].Timings.Add(2 * time.Millisecond)
	counters.CDC[uint8(msgs.MAKE_DIRECTORY)].Attempts++
	p := &metricsPusher{srv: srv, counters: counters, filesystem: "qa_nfs", instance: "r73:2049", started: time.Now()}
	var b log.MetricsBuilder
	payload := buildPayload(t, p, &b)
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	if len(lines) < 6 {
		t.Fatalf("payload has %d lines:\n%s", len(lines), payload)
	}
	for _, line := range lines {
		if !strings.Contains(line, "filesystem=qa_nfs") || !strings.Contains(line, "instance=r73:2049") {
			t.Fatalf("line without the filesystem and instance tags: %s", line)
		}
		if !strings.HasPrefix(line, metricsPrefix) {
			t.Fatalf("line without the %s prefix: %s", metricsPrefix, line)
		}
	}
	for _, want := range []string{measConnections + ",", measAdmission + ",", measState + ",", measStaging + ",", measProcess + ",", measIO + ","} {
		if !strings.Contains(string(payload), want) {
			t.Errorf("payload lacks %s", want)
		}
	}
	if !strings.Contains(string(payload), "work_limit="+itoa(server.DefaultReplyWorkBudgetMiB<<20)+"i") {
		t.Errorf("payload lacks the reply-work limit:\n%s", payload)
	}
	backend := []string{
		measBackendLatency + ",filesystem=qa_nfs,instance=r73:2049,service=shard,kind=LOOKUP,le=+Inf count=1i",
		measBackend + ",filesystem=qa_nfs,instance=r73:2049,service=shard,kind=LOOKUP count=1i,attempts=1i",
		measBackendLatency + ",filesystem=qa_nfs,instance=r73:2049,service=cdc,kind=MAKE_DIRECTORY,le=+Inf count=1i",
		measBackend + ",filesystem=qa_nfs,instance=r73:2049,service=cdc,kind=MAKE_DIRECTORY count=1i,attempts=1i",
	}
	for _, want := range append(backend, measOpLatencyBkt+",filesystem=qa_nfs,instance=r73:2049,op=OP_WRITE,le=+Inf count=1i") {
		if !strings.Contains(string(payload), want) {
			t.Errorf("payload lacks %q:\n%s", want, payload)
		}
	}
	// Every push merges the shards again. The merge used to add them into
	// shard 0's live counters, so the second push reported 2 requests and
	// the third 3.
	for push := 2; push <= 3; push++ {
		payload := buildPayload(t, p, &b)
		for _, want := range backend {
			if !strings.Contains(string(payload), want) {
				t.Errorf("push %d lacks %q:\n%s", push, want, payload)
			}
		}
	}
	if n := counters.Shard[uint8(msgs.LOOKUP)][0].Timings.Count(); n != 0 {
		t.Errorf("building the payload put %d requests in shard 0", n)
	}
}

// Payloads built while the client is counting requests stay consistent: for
// every request kind, count equals the +Inf bucket and the buckets do not
// decrease. Attempts is sampled separately and is not compared. Run under
// -race this also checks the exporter's reads are atomic.
func TestMetricsBackendDuringRequests(t *testing.T) {
	srv, err := server.NewServer(vfs.NewLocalTernVFS(t.TempDir()), staging.ReadOnlyStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	counters := client.NewClientCounters()
	p := &metricsPusher{srv: srv, counters: counters, filesystem: "qa_nfs", instance: "r73:2049", started: time.Now()}
	// The client counts until the exporter has built its payloads, so the
	// two always overlap.
	stop, done := make(chan struct{}), make(chan uint64, 1)
	go func() {
		var i uint64
		defer func() { done <- i }()
		for ; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			// As metadataRequest counts: the attempt, then the timing.
			shard := &counters.Shard[uint8(msgs.LOOKUP)][i%256]
			atomic.AddUint64(&shard.Attempts, 1)
			shard.Timings.Add(time.Duration(i%1000) * time.Microsecond)
			cdc := counters.CDC[uint8(msgs.MAKE_DIRECTORY)]
			atomic.AddUint64(&cdc.Attempts, 1)
			cdc.Timings.Add(time.Duration(i%1000) * time.Millisecond)
		}
	}()
	var b log.MetricsBuilder
	check := func(payload []byte) map[string]uint64 {
		t.Helper()
		counts := make(map[string]uint64)
		buckets := make(map[string][]uint64)
		for _, line := range strings.Split(strings.TrimSpace(string(payload)), "\n") {
			parts := strings.Fields(line) // series, fields, timestamp
			measurement, tags, _ := strings.Cut(parts[0], ",")
			fields := parts[1]
			if measurement != measBackend && measurement != measBackendLatency {
				continue
			}
			kind := tags[strings.Index(tags, "service="):]
			if measurement == measBackendLatency {
				kind = kind[:strings.Index(kind, ",le=")]
			}
			count, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(strings.Split(fields, ",")[0], "count="), "i"), 10, 64)
			if err != nil {
				t.Fatalf("line %q: %v", line, err)
			}
			if measurement == measBackend {
				counts[kind] = count
			} else {
				buckets[kind] = append(buckets[kind], count)
			}
		}
		for kind, n := range counts {
			bs := buckets[kind]
			if len(bs) != len(backendLatencyBounds)+1 || bs[len(bs)-1] != n {
				t.Fatalf("%s: count %d, buckets %v", kind, n, bs)
			}
			for i := 1; i < len(bs); i++ {
				if bs[i] < bs[i-1] {
					t.Fatalf("%s: buckets decrease: %v", kind, bs)
				}
			}
		}
		return counts
	}
	for i := 0; i < 20; i++ {
		check(buildPayload(t, p, &b))
	}
	close(stop)
	requests := <-done
	counts := check(buildPayload(t, p, &b))
	if len(counts) != 2 {
		t.Fatalf("backend kinds %v, want shard LOOKUP and cdc MAKE_DIRECTORY", counts)
	}
	for kind, n := range counts {
		if n != requests {
			t.Errorf("%s: count %d after %d requests", kind, n, requests)
		}
	}
}

// buildPayload builds one push into b, as push does.
func buildPayload(t *testing.T, p *metricsPusher, b *log.MetricsBuilder) []byte {
	t.Helper()
	b.Reset()
	p.build(b, time.Now())
	payload, err := io.ReadAll(b.Payload())
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

// A push that panics is reported, not propagated: the exporter runs in the
// NFS server's process and must not take it down.
func TestMetricsPushSurvivesPanic(t *testing.T) {
	p := &metricsPusher{filesystem: "qa_nfs", instance: "r73:2049"} // srv nil: build panics
	var b log.MetricsBuilder
	err := p.push(&b, time.Now())
	if err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Fatalf("push returned %v, want a panic reported as an error", err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestDefaultInstance(t *testing.T) {
	for _, tc := range []struct{ host, addr, want string }{
		{"ickpeggsfsr73.restech.cylon.local", "127.0.0.1:2049", "ickpeggsfsr73:2049"},
		{"fsgw1", ":12049", "fsgw1:12049"},
		{"fsgw1", "", "fsgw1:2049"},
	} {
		if got := defaultInstance(tc.host, tc.addr); got != tc.want {
			t.Errorf("defaultInstance(%q, %q) = %q, want %q", tc.host, tc.addr, got, tc.want)
		}
	}
}
