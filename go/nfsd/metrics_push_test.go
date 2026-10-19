// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"io"
	"strconv"
	"strings"
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
	p := &metricsPusher{srv: srv, counters: counters, filesystem: "qa_nfs", instance: "r73:2049", started: time.Now()}
	var b log.MetricsBuilder
	p.build(&b, time.Now())
	payload, err := io.ReadAll(b.Payload())
	if err != nil {
		t.Fatal(err)
	}
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
	for _, want := range []string{
		measOpLatencyBkt + ",filesystem=qa_nfs,instance=r73:2049,op=OP_WRITE,le=+Inf count=1i",
		measBackendLatency + ",filesystem=qa_nfs,instance=r73:2049,service=shard,kind=LOOKUP,le=+Inf count=1i",
		measBackend + ",filesystem=qa_nfs,instance=r73:2049,service=shard,kind=LOOKUP count=1i,attempts=1i",
	} {
		if !strings.Contains(string(payload), want) {
			t.Errorf("payload lacks %q:\n%s", want, payload)
		}
	}
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
