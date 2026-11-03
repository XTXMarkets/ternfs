// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
)

// Retiring a client's sessions keeps their records counted until each
// session's last use ends, so no new record is admitted against them.
func TestClientRetirementHoldsRecords(t *testing.T) {
	srv, _ := newConnectionTestServer(t, 2)
	ss := srv.sessions
	const client, count = 77, 64
	ss.recordLimit = count
	var batch []*session
	ss.mu.Lock()
	for i := 0; i < count; i++ {
		sess := &session{clientID: client, slots: make([]sessionSlot, maxSessionSlots),
			maxOps: maxCompoundOperations, maxReq: maxRPCMessage, maxResp: maxRPCMessage,
			maxCached: maxSessionReplyCached}
		sess.id, _ = ss.newSessionIDLocked(client)
		ss.insertSessionLocked(sess)
		batch = append(batch, sess)
	}
	ss.mu.Unlock()
	// One continuing user: an admitted request still running on the last session.
	user := batch[count-1]
	ts := newTestSession(user.id, client, maxSessionSlots)
	seq := srv.startSequence(sequenceArgsForTest(t, admissionSequence(buildCompoundBody(nil, nil), ts, 1, 1, true)))
	if seq.status != nfs4.NFS4_OK || seq.slot == nil {
		t.Fatalf("admission %s", nfs4.Nfsstat4Name(seq.status))
	}
	// Pause every session's slot so the bulk retirement stalls wherever it starts.
	for _, sess := range batch {
		sess.slots[0].mu.Lock()
	}
	done := make(chan struct{})
	go func() {
		ss.dropClient(client)
		close(done)
	}()
	waitFor := func(what string, cond func(replyAccounting) bool) replyAccounting {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); ; {
			a := ss.accounting()
			if cond(a) {
				return a
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: %+v", what, a)
			}
			time.Sleep(time.Millisecond)
		}
	}
	// The batch is detached from lookup yet its records stay counted.
	waitFor("batch detached", func(a replyAccounting) bool { return a.Sessions == 0 })
	testutil.AssertBlocked(t, done, "client-wide retirement")
	if a := ss.accounting(); a.Records != count {
		t.Fatalf("stalled batch returned record credit %+v", a)
	}
	if _, ok := ss.reserveSessionRecord(client + 1); ok {
		t.Fatal("new record admitted against a stalled retiring batch")
	}
	if seq := srv.startSequence(sequenceArgsForTest(t, admissionSequence(buildCompoundBody(nil, nil),
		newTestSession(SessionID{0xff}, client, maxSessionSlots), 0, 1, true))); seq.status != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("unknown session %s", nfs4.Nfsstat4Name(seq.status))
	}
	// Retirement drains in map order. Each released slot can return at most its
	// own session's record, and never the continuing user's.
	for i, sess := range batch {
		time.Sleep(time.Millisecond)
		if a := ss.accounting(); a.Records < count-i || a.Records > count {
			t.Fatalf("%d of %d drained, records %+v", i, count, a)
		}
		sess.slots[0].mu.Unlock()
	}
	<-done
	// The admitted request still owns its drained session's record.
	if a := ss.accounting(); a.Records != 1 || a.Sessions != 0 || a.Groups != 1 || a.ActiveRequests != 1 {
		t.Fatalf("continuing user lost its record or group %+v", a)
	}
	ss.recordLimit = 1
	if _, ok := ss.reserveSessionRecord(client + 1); ok {
		t.Fatal("record admitted while a request still uses its retired session")
	}
	seq.finish(nil)
	if a := ss.accounting(); a.Records != 0 || a.CacheBytes != 0 || a.CacheBuffers != 0 || a.Groups != 1 || a.GroupCredits != 1 {
		t.Fatalf("retirement did not release %+v", a)
	}
	// The transport's hold outlives the slot's use, and keeps its group
	// credit, until the reply is done.
	seq.hold.release()
	if a := ss.accounting(); a.Groups != 0 || a.GroupCredits != 0 || a.ActiveRequests != 0 {
		t.Fatalf("group outlived its last request %+v", a)
	}
	if _, ok := ss.reserveSessionRecord(client + 1); !ok {
		t.Fatal("released record capacity not reusable")
	}
	ss.releaseSessionRecord()
	// A retired session cannot be re-admitted after its last use ended.
	if seq := srv.startSequence(sequenceArgsForTest(t, admissionSequence(buildCompoundBody(nil, nil), ts, 2, 1, true))); seq.status != nfs4.NFS4ERR_BADSESSION {
		t.Fatalf("retired session admitted %s", nfs4.Nfsstat4Name(seq.status))
	}
}
