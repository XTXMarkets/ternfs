// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// diagnosticsFixture starts a server, registers a client with one session
// and opens a file, so every id kind the API resolves exists.
type diagnosticsFixture struct {
	srv      *Server
	addr     string
	wire     uint64
	session  SessionID
	stateid  [12]byte
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

func TestDiagnosticsEnvelopeAndStatus(t *testing.T) {
	f := newDiagnosticsFixture(t)
	env := f.srv.Envelope()
	if env.Mode != inspect.ModeServing || env.NfsdID == "" || env.WireEpoch == 0 {
		t.Fatalf("envelope = %+v", env)
	}
	if env.StateidEpoch != fmt.Sprintf("%08x", f.srv.opens.Epoch) {
		t.Fatalf("stateid epoch %q", env.StateidEpoch)
	}
	st, err := f.srv.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Accounting.Sessions != 1 || st.Accounting.Groups != 1 || st.Wire.LastCounter == 0 {
		t.Fatalf("status = %+v", st)
	}
	if st.Limits.LeaseTime != clientstore.LeaseTime.String() || st.Accounting.RecordLimit == 0 {
		t.Fatalf("limits = %+v accounting = %+v", st.Limits, st.Accounting)
	}
}

func TestDiagnosticsClientsIncarnationsSessions(t *testing.T) {
	f := newDiagnosticsFixture(t)
	clients, err := f.srv.Clients(inspect.ClientFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(clients.Records) != 1 || len(clients.Owners) != 1 {
		t.Fatalf("clients = %+v", clients)
	}
	c := clients.Records[0]
	if c.WireClientID != wireText(f.wire) || c.State != inspect.ClientConfirmed || c.Incarnation != incarnationText(f.incarnID) {
		t.Fatalf("client = %+v", c)
	}
	if c.CreationSlot.Seq == 0 || !c.CreationSlot.Cached || c.CreationSlot.Status != "NFS4_OK" {
		t.Fatalf("creation slot = %+v", c.CreationSlot)
	}
	if clients.Owners[0].Confirmed != c.WireClientID {
		t.Fatalf("owner = %+v", clients.Owners[0])
	}
	// Owners follow the record filters: a wire or incarnation filter lists
	// only the owners of the matched records.
	if got, _ := f.srv.Clients(inspect.ClientFilter{WireClientID: f.wire + 1}); len(got.Records) != 0 || len(got.Owners) != 0 {
		t.Fatalf("filter matched %d records, %d owners", len(got.Records), len(got.Owners))
	}
	for _, filter := range []inspect.ClientFilter{{WireClientID: f.wire}, {Incarnation: f.incarnID}, {OwnerHex: c.OwnerHex}} {
		if got, _ := f.srv.Clients(filter); len(got.Records) != 1 || len(got.Owners) != 1 || got.Owners[0].OwnerHex != c.OwnerHex {
			t.Fatalf("filter %+v = %+v", filter, got)
		}
	}

	incs, err := f.srv.Incarnations(inspect.IncarnationFilter{ID: f.incarnID})
	if err != nil {
		t.Fatal(err)
	}
	if len(incs.Records) != 1 {
		t.Fatalf("incarnations = %+v", incs)
	}
	inc := incs.Records[0]
	if inc.Group == nil || inc.Group.State != inspect.GroupOpen || inc.Group.LeaseGate != inspect.GateLive {
		t.Fatalf("group = %+v", inc.Group)
	}
	if inc.Refresher == nil || inc.Refresher.State != inspect.LeaseLive || inc.Refresher.Proven == nil {
		t.Fatalf("refresher = %+v", inc.Refresher)
	}
	if inc.WireClientID != wireText(f.wire) || len(inc.Sessions) != 1 || inc.Sessions[0] != sessionText(f.session) {
		t.Fatalf("incarnation = %+v", inc)
	}
	if inc.Opens != 1 || inc.Cached == nil || inc.Cached.Opens != 1 {
		t.Fatalf("opens = %d cached = %+v", inc.Opens, inc.Cached)
	}
	if incs.Limits.RefresherLimit == 0 || incs.Limits.GroupLimit == 0 {
		t.Fatalf("limits = %+v", incs.Limits)
	}

	sessions, err := f.srv.Sessions(inspect.SessionFilter{Slots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions.Records) != 1 {
		t.Fatalf("sessions = %+v", sessions)
	}
	s := sessions.Records[0]
	if s.ID != sessionText(f.session) || s.Incarnation != incarnationText(f.incarnID) || s.WireClientID != wireText(f.wire) {
		t.Fatalf("session = %+v", s)
	}
	if s.Fore.Slots == 0 || len(s.Slots) != int(s.Fore.Slots) || s.Users < 1 {
		t.Fatalf("session = %+v", s)
	}
	used := 0
	for _, slot := range s.Slots {
		if slot.Locked {
			t.Fatalf("slot %d locked with no request running", slot.Index)
		}
		if slot.Occupied {
			used++
			if slot.User == nil || slot.User.Flavor == "" {
				t.Fatalf("slot %d user = %+v", slot.Index, slot.User)
			}
		}
	}
	if used == 0 {
		t.Fatal("no slot has completed a request")
	}
	if len(s.StatusFlags) != 0 {
		t.Fatalf("status flags = %v for a session without a backchannel", s.StatusFlags)
	}
	brief, _ := f.srv.Sessions(inspect.SessionFilter{})
	if len(brief.Records[0].Slots) != 0 {
		t.Fatal("slots reported without Slots filter")
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
	if r := resolve(t, stateidText(f.stateid), inspect.KindStateid); !r.Known || r.State != "active" || r.Incarnation != inc {
		t.Fatalf("stateid = %+v", r)
	}
	if r := resolve(t, sessionText(f.session), inspect.KindSession); !r.Known || r.State != "live" || r.Incarnation != inc {
		t.Fatalf("session = %+v", r)
	}
	// An identity hash needs no server state; it points at the store report.
	if r := resolve(t, inspect.ID(inspect.KindIdentityHash, make([]byte, 32)), inspect.KindIdentityHash); r.Known || len(r.Links) != 1 {
		t.Fatalf("identity hash = %+v", r)
	}

	// A stateid is classified in one observation: active above, an expired
	// tombstone, and one that is neither.
	expired := diagnosticStateid(f.srv.opens.Epoch, 0xdead)
	f.srv.opens.AddTombstoneForTest(expired, f.incarnID)
	if r := resolve(t, stateidText(expired), inspect.KindStateid); !r.Known || r.State != "expired" || r.Incarnation != inc {
		t.Fatalf("expired stateid = %+v", r)
	}
	unknown := stateidText(diagnosticStateid(f.srv.opens.Epoch, 0xbeef))
	if r := resolve(t, unknown, inspect.KindStateid); r.Known || r.State != "" || r.Incomplete || !*r.IssuedByThisProcess {
		t.Fatalf("unknown stateid = %+v", r)
	}

	// Zero is rejected, not a wildcard; so are an untyped value, an unknown
	// kind and an equal-width value of another kind.
	for _, bad := range []string{inspect.ID(inspect.KindIncarnation, make([]byte, 8)), "4000000100000001", "0x4000000100000001", "filehandle:4000000100000001", "stateid:" + strings.TrimPrefix(wire, "wire-clientid:"), ""} {
		if _, err := inspect.Resolve(f.srv, bad); err == nil {
			t.Fatalf("Resolve(%q) accepted", bad)
		}
	}
}

func TestDiagnosticsConnections(t *testing.T) {
	f := newDiagnosticsFixture(t)
	conns, err := f.srv.Connections(inspect.ConnectionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(conns.Records) != 1 {
		t.Fatalf("connections = %+v", conns)
	}
	c := conns.Records[0]
	if c.InFlight != 0 || c.Requests < 3 || c.ConnectedAt.IsZero() || c.Remote == "" {
		t.Fatalf("connection = %+v", c)
	}
	if len(c.Sessions) != 1 || c.Sessions[0] != sessionText(f.session) || len(c.BackSessions) != 0 {
		t.Fatalf("associations = %+v", c)
	}
	sid := [16]byte(f.session)
	if got, _ := f.srv.Connections(inspect.ConnectionFilter{Session: &sid}); len(got.Records) != 1 {
		t.Fatalf("session filter matched %d", len(got.Records))
	}
	other := sid
	other[15]++
	if got, _ := f.srv.Connections(inspect.ConnectionFilter{Session: &other}); len(got.Records) != 0 {
		t.Fatalf("foreign session filter matched %d", len(got.Records))
	}
	st, _ := f.srv.Status()
	if st.Connections != 1 {
		t.Fatalf("status connections = %d", st.Connections)
	}

	// A second connection appears with no associations, and goes when closed.
	extra := dial(t, f.addr)
	deadline := time.Now().Add(2 * time.Second)
	for {
		conns, _ = f.srv.Connections(inspect.ConnectionFilter{})
		if len(conns.Records) == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(conns.Records) != 2 || len(conns.Records[1].Sessions) != 0 || conns.Records[1].Requests != 0 {
		t.Fatalf("after dial: %+v", conns.Records)
	}
	extra.Close()
	deadline = time.Now().Add(2 * time.Second)
	for {
		conns, _ = f.srv.Connections(inspect.ConnectionFilter{})
		if len(conns.Records) == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(conns.Records) != 1 {
		t.Fatalf("after close: %+v", conns.Records)
	}
}

func TestDiagnosticsOpensStagingConfirmations(t *testing.T) {
	f := newDiagnosticsFixture(t)
	opens, err := f.srv.Opens(inspect.OpenFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(opens.Records) != 1 || len(opens.Expired) != 0 {
		t.Fatalf("opens = %+v", opens)
	}
	o := opens.Records[0]
	if o.Stateid != stateidText(f.stateid) || o.FileID != inodeText(vfs.InodeID(binary.BigEndian.Uint64(f.fh))) || !o.Writer || o.Incarnation != incarnationText(f.incarnID) {
		t.Fatalf("open = %+v", o)
	}
	if o.DataID == o.FileID || len(o.Access) == 0 {
		t.Fatalf("writer should use its staging inode: %+v", o)
	}
	if opens.Views.WriterEntries == 0 || opens.Views.NameHints == 0 {
		t.Fatalf("views = %+v", opens.Views)
	}
	if got, _ := f.srv.Opens(inspect.OpenFilter{Stateid: &f.stateid}); len(got.Records) != 1 {
		t.Fatalf("stateid filter matched %d", len(got.Records))
	}
	if got, _ := f.srv.Opens(inspect.OpenFilter{File: f.incarnID}); len(got.Records) != 0 {
		t.Fatalf("file filter matched %d", len(got.Records))
	}

	stagingOut, err := f.srv.Staging(inspect.StagingFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if !stagingOut.Configured || len(stagingOut.Records) != 1 || len(stagingOut.Failed) != 0 {
		t.Fatalf("staging = %+v", stagingOut)
	}
	st := stagingOut.Records[0]
	if st.Locked || st.FileID != o.DataID || st.Stateid != o.Stateid || st.Incarnation != o.Incarnation || st.FileName != "f" {
		t.Fatalf("staging entry = %+v", st)
	}
	if st.Unlinked || st.Guarded || st.Retired || st.Removed || st.Hydrating {
		t.Fatalf("fresh writer flags = %+v", st)
	}
	raw, err := inspect.ParseID(inspect.KindInode, o.DataID)
	if err != nil {
		t.Fatal(err)
	}
	dataID := vfs.InodeID(binary.BigEndian.Uint64(raw))
	if got, _ := f.srv.Staging(inspect.StagingFilter{File: uint64(dataID)}); len(got.Records) != 1 {
		t.Fatalf("file filter matched %d", len(got.Records))
	}

	// A staging file held by a disk operation is reported locked.
	sf := f.srv.stagingStore.Get(dataID).(*staging.LocalFile)
	sf.LockForTest()
	done := make(chan inspect.Staging, 1)
	go func() {
		out, _ := f.srv.Staging(inspect.StagingFilter{})
		done <- out
	}()
	select {
	case out := <-done:
		if len(out.Records) != 1 || !out.Records[0].Locked || out.Records[0].Stateid != "" || out.Records[0].FileName != "f" {
			t.Fatalf("locked entry = %+v", out.Records)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Staging blocked on a held file")
	}
	sf.UnlockForTest()

	confs, err := f.srv.Confirmations()
	if err != nil {
		t.Fatal(err)
	}
	if len(confs.Records) != 0 || len(confs.DestroyHolds) != 0 || len(confs.PurgePending) != 0 || confs.PurgeLimit == 0 {
		t.Fatalf("confirmations = %+v", confs)
	}

	// A confirmation whose attempt a settle holds is reported locked.
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
		attempt.UnlockForTest()
	}()
	confs, _ = f.srv.Confirmations()
	if len(confs.Records) != 1 || confs.Records[0].Attempt == nil || !confs.Records[0].Attempt.Locked || confs.Records[0].Predecessor != incarnationText(5) {
		t.Fatalf("locked confirmation = %+v", confs.Records)
	}
	attempt.UnlockForTest()
	confs, _ = f.srv.Confirmations()
	if a := confs.Records[0].Attempt; a.Locked || a.Result != inspect.ConfirmUnresolved || len(a.Writes) != 0 {
		t.Fatalf("attempt = %+v", a)
	}
	attempt.LockForTest()
}
