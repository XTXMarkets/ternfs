// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// A served compound shows up in the counters: one count per operation and
// status, a latency observation per operation, and the bytes a READ returned.
func TestMetricsCountServedOperations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	srv, addr, cleanup := startTestServerWithServer(t, dir)
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	xid := uint32(1)
	clientID := setupClient(t, conn, &xid)
	status, reader, fh, _ := openFileForOwner(t, conn, &xid, clientID, "reader", "file.txt",
		nfs4.OPEN4_SHARE_ACCESS_READ, false, false)
	if status != nfs4.NFS4_OK {
		t.Fatal(nfs4.Nfsstat4Name(status))
	}
	readFileData(t, conn, &xid, fh, 0, 16)
	closeFile(t, conn, &xid, fh, reader)

	snap := srv.MetricsSnapshot()
	count := func(op, status string) uint64 {
		for _, c := range snap.Ops {
			if c.Op == op && c.Status == status {
				return c.Count
			}
		}
		return 0
	}
	for _, op := range []string{"OP_OPEN", "OP_READ", "OP_CLOSE", "OP_SEQUENCE"} {
		if count(op, "NFS4_OK") == 0 {
			t.Errorf("%s not counted: %+v", op, snap.Ops)
		}
	}
	if snap.Reads != 1 || snap.ReadBytes != 4 {
		t.Errorf("reads=%d bytes=%d, want 1 and 4", snap.Reads, snap.ReadBytes)
	}
	if snap.Compounds == 0 || snap.Connections != 1 || snap.ConnAccepted != 1 {
		t.Errorf("compounds=%d connections=%d accepted=%d", snap.Compounds, snap.Connections, snap.ConnAccepted)
	}
	if snap.Opens != 0 {
		t.Errorf("opens=%d after CLOSE", snap.Opens)
	}
	var read *OpLatencySnapshot
	for i := range snap.Latency {
		if snap.Latency[i].Op == "OP_READ" {
			read = &snap.Latency[i]
		}
	}
	if read == nil || read.Count != 1 || len(read.Cumulative) != len(OpLatencyBounds)+1 ||
		read.Cumulative[len(read.Cumulative)-1] != 1 {
		t.Fatalf("READ latency %+v", read)
	}
}

// Observations land in the bucket of the first bound at or above them, and
// the snapshot's cumulative counts are non-decreasing and end at the total.
func TestMetricsLatencyBuckets(t *testing.T) {
	m := newMetrics()
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 200*time.Microsecond) // <= 0.0005
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 3*time.Millisecond)   // <= 0.005
	m.observeOp(nfs4.OP_WRITE, nfs4.NFS4_OK, 2*time.Minute)        // overflow
	l := m.latency[nfs4.OP_WRITE]
	if l.buckets[0] != 1 || l.buckets[2] != 1 || l.buckets[len(OpLatencyBounds)] != 1 || l.count != 3 {
		t.Fatalf("buckets %v count %d", l.buckets, l.count)
	}
}
