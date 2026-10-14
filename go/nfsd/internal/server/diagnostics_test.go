// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// diagnosticsFixture starts a server, registers a client with one session
// and opens a file, so every id kind the API resolves exists.
type diagnosticsFixture struct {
	srv      *Server
	addr     string
	wire     uint64
	session  SessionID
	stateid  nfs4.StateID
	fh       []byte
	incarnID uint64
}

func newDiagnosticsFixture(t *testing.T) *diagnosticsFixture {
	t.Helper()
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	t.Cleanup(cleanup)
	conn := dial(t, addr)
	t.Cleanup(func() { conn.Close() })
	xid := uint32(1)
	wire := setupClient(t, conn, &xid)
	sid, fh := openCreateFile(t, conn, &xid, wire, "f")
	ts := testSessionForClient(conn, wire)
	if ts == nil {
		t.Fatal("no test session for the client")
	}
	f := &diagnosticsFixture{srv: srv, addr: addr, wire: wire, session: ts.id, fh: fh}
	copy(f.stateid[:], sid[4:])
	f.incarnID = binary.BigEndian.Uint64(ts.id[4:12])
	return f
}

// /v1/incarnations joins the partial records of the session store, the
// client store and the open state into one record per incarnation.
func TestDiagnosticsIncarnationJoinsStores(t *testing.T) {
	f := newDiagnosticsFixture(t)
	incs, err := f.srv.Incarnations(inspect.IncarnationFilter{ID: f.incarnID})
	if err != nil {
		t.Fatal(err)
	}
	if len(incs.Records) != 1 {
		t.Fatalf("incarnations = %+v", incs)
	}
	inc := incs.Records[0]
	if inc.Group == nil || inc.WireClientID != wireText(f.wire) || len(inc.Sessions) != 1 || inc.Sessions[0] != sessionText(f.session) {
		t.Fatalf("session store part = %+v", inc)
	}
	if inc.Refresher == nil || inc.Refresher.State != inspect.LeaseLive || inc.Cached == nil {
		t.Fatalf("client store part = %+v", inc)
	}
	if inc.Opens != 1 || inc.Cached.Opens != 1 {
		t.Fatalf("opens = %d cached = %+v", inc.Opens, inc.Cached)
	}
}

// Owners follow the record filters: a wire, incarnation or owner filter
// lists only the owners of the records it matched.
func TestDiagnosticsClientOwnersFollowFilter(t *testing.T) {
	f := newDiagnosticsFixture(t)
	all, err := f.srv.Clients(inspect.ClientFilter{})
	if err != nil || len(all.Records) != 1 || len(all.Owners) != 1 {
		t.Fatalf("clients = %+v, %v", all, err)
	}
	c := all.Records[0]
	if got, _ := f.srv.Clients(inspect.ClientFilter{WireClientID: f.wire + 1}); len(got.Records) != 0 || len(got.Owners) != 0 {
		t.Fatalf("filter matched %d records, %d owners", len(got.Records), len(got.Owners))
	}
	for _, filter := range []inspect.ClientFilter{{WireClientID: f.wire}, {Incarnation: f.incarnID}, {OwnerHex: c.OwnerHex}} {
		if got, _ := f.srv.Clients(filter); len(got.Records) != 1 || len(got.Owners) != 1 || got.Owners[0].OwnerHex != c.OwnerHex {
			t.Fatalf("filter %+v = %+v", filter, got)
		}
	}
}

// A slot held by a running request must not stall the dump: the snapshot
// tries the lock and reports the slot locked instead of waiting.
func TestDiagnosticsSkipsLockedSlot(t *testing.T) {
	f := newDiagnosticsFixture(t)
	sess := f.srv.sessions.lookup(f.session)
	if sess == nil || len(sess.slots) < 2 {
		t.Fatalf("session %v has %d slots", f.session, len(sess.slots))
	}
	sess.slots[0].mu.Lock()
	defer sess.slots[0].mu.Unlock()
	done := make(chan inspect.Sessions, 1)
	go func() {
		out, _ := f.srv.Sessions(inspect.SessionFilter{Slots: true})
		done <- out
	}()
	select {
	case out := <-done:
		slots := out.Records[0].Slots
		if !slots[0].Locked || slots[0].Seq != 0 {
			t.Fatalf("slot 0 = %+v, want locked with no detail", slots[0])
		}
		if slots[1].Locked {
			t.Fatalf("slot 1 = %+v, want unlocked", slots[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Sessions blocked on a held slot")
	}
}

// Lookup resolves each id kind against this process's state. A stateid is
// classified in one observation: a live open, an expired tombstone, or one
// this process issued and no longer knows.
func TestDiagnosticsLookup(t *testing.T) {
	f := newDiagnosticsFixture(t)
	resolve := func(t *testing.T, id, kind string) inspect.Resolution {
		t.Helper()
		r, err := inspect.Resolve(f.srv, id)
		if err != nil || r.Kind != kind || r.ID != id {
			t.Fatalf("Resolve(%s) = %+v, %v", id, r, err)
		}
		return r
	}
	wire, inc := wireText(f.wire), incarnationText(f.incarnID)
	if r := resolve(t, wire, inspect.KindWireClientID); !r.Known || r.Incarnation != inc || r.IssuedByThisProcess == nil || !*r.IssuedByThisProcess {
		t.Fatalf("wire clientid = %+v", r)
	}
	if r := resolve(t, inc, inspect.KindIncarnation); !r.Known || r.WireClientID != wire {
		t.Fatalf("incarnation = %+v", r)
	}
	if r := resolve(t, inodeText(vfs.InodeID(binary.BigEndian.Uint64(f.fh))), inspect.KindInode); !r.Known || r.InodeType != "file" {
		t.Fatalf("inode = %+v", r)
	}
	if r := resolve(t, sessionText(f.session), inspect.KindSession); !r.Known || r.State != "live" || r.Incarnation != inc {
		t.Fatalf("session = %+v", r)
	}
	// An identity hash needs no server state; it points at the store report.
	if r := resolve(t, inspect.ID(inspect.KindIdentityHash, make([]byte, 32)), inspect.KindIdentityHash); r.Known || len(r.Links) != 1 {
		t.Fatalf("identity hash = %+v", r)
	}

	if r := resolve(t, stateidText(f.stateid), inspect.KindStateid); !r.Known || r.State != "active" || r.Incarnation != inc {
		t.Fatalf("live stateid = %+v", r)
	}
	expired := diagnosticStateid(f.srv.opens.Epoch, 0xdead)
	f.srv.opens.AddTombstoneForTest(expired, f.incarnID)
	if r := resolve(t, stateidText(expired), inspect.KindStateid); !r.Known || r.State != "expired" || r.Incarnation != inc {
		t.Fatalf("expired stateid = %+v", r)
	}
	unknown := stateidText(diagnosticStateid(f.srv.opens.Epoch, 0xbeef))
	if r := resolve(t, unknown, inspect.KindStateid); r.Known || r.State != "" || r.Incomplete || !*r.IssuedByThisProcess {
		t.Fatalf("unknown stateid = %+v", r)
	}
}

// A confirmation whose attempt a settle holds is reported locked rather
// than waited for, and in full once the settle ends.
func TestDiagnosticsSkipsLockedConfirmation(t *testing.T) {
	f := newDiagnosticsFixture(t)
	attempt := &clientstore.ConfirmAttempt{}
	attempt.LockForTest()
	c := &confirmation{identity: 7, owner: "o", n: 11, p: 5, attempt: attempt, applying: true}
	f.srv.confirmMu.Lock()
	f.srv.confirmingIdentity[7] = c
	f.srv.confirmMu.Unlock()
	defer func() {
		f.srv.confirmMu.Lock()
		delete(f.srv.confirmingIdentity, 7)
		f.srv.confirmMu.Unlock()
	}()
	confs, _ := f.srv.Confirmations()
	if len(confs.Records) != 1 || confs.Records[0].Attempt == nil || !confs.Records[0].Attempt.Locked || confs.Records[0].Predecessor != incarnationText(5) {
		attempt.UnlockForTest()
		t.Fatalf("locked confirmation = %+v", confs.Records)
	}
	attempt.UnlockForTest()
	confs, _ = f.srv.Confirmations()
	if a := confs.Records[0].Attempt; a.Locked || a.Result != inspect.ConfirmUnresolved || len(a.Writes) != 0 {
		t.Fatalf("attempt = %+v", a)
	}
}
