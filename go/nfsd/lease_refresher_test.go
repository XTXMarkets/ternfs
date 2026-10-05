// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

var errInjectedLease = errors.New("injected lease fault")

// refresherFaultVFS controls the refresher's temp protocol. A failed LINK
// is held, unapplied, until applyLate links it, as a backend that applied
// it after the error would.
type refresherFaultVFS struct {
	TernVFS
	mu          sync.Mutex
	constructFn func() error
	linkFail    bool
	renameFail  int
	fenceErr    error
	constructs  int
	links       []string
	scraps      int
	late        []lateLink
}

type lateLink struct {
	id     InodeID
	cookie Cookie
	dir    InodeID
	name   string
	data   []byte
}

func (fs *refresherFaultVFS) ConstructFile(dir InodeID) (InodeID, Cookie, error) {
	fs.mu.Lock()
	fn := fs.constructFn
	fs.constructs++
	fs.mu.Unlock()
	if fn != nil {
		if err := fn(); err != nil {
			return 0, Cookie{}, err
		}
	}
	return fs.TernVFS.ConstructFile(dir)
}

func (fs *refresherFaultVFS) LinkFile(id InodeID, cookie Cookie, dir InodeID, name string, data io.Reader) error {
	fs.mu.Lock()
	fs.links = append(fs.links, name)
	fail := fs.linkFail && leaseTemp(name)
	fs.mu.Unlock()
	if fail {
		body, _ := io.ReadAll(data)
		fs.mu.Lock()
		fs.late = append(fs.late, lateLink{id, cookie, dir, name, body})
		fs.mu.Unlock()
		return errInjectedLease
	}
	return fs.TernVFS.LinkFile(id, cookie, dir, name, data)
}

func (fs *refresherFaultVFS) ScrapFile(id InodeID, cookie Cookie) error {
	fs.mu.Lock()
	fs.scraps++
	fs.mu.Unlock()
	return fs.TernVFS.ScrapFile(id, cookie)
}

func (fs *refresherFaultVFS) RenameEdge(src InodeID, old string, edge Edge, dst InodeID, name string) error {
	fs.mu.Lock()
	fail := fs.renameFail > 0 && leaseTemp(old)
	if fail {
		fs.renameFail--
	}
	fs.mu.Unlock()
	if fail {
		return errInjectedLease
	}
	return fs.TernVFS.RenameEdge(src, old, edge, dst, name)
}

func (fs *refresherFaultVFS) RemoveEdge(dir InodeID, name string, edge Edge) error {
	fs.mu.Lock()
	err := fs.fenceErr
	fs.mu.Unlock()
	if err != nil && leaseTemp(name) {
		return err
	}
	return fs.TernVFS.RemoveEdge(dir, name, edge)
}

func (fs *refresherFaultVFS) applyLate(t *testing.T) {
	t.Helper()
	fs.mu.Lock()
	late := fs.late
	fs.late = nil
	fs.mu.Unlock()
	for _, l := range late {
		if err := fs.TernVFS.LinkFile(l.id, l.cookie, l.dir, l.name, bytes.NewReader(l.data)); err != nil {
			t.Fatal(err)
		}
	}
}

func (fs *refresherFaultVFS) set(fn func()) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fn()
}

// refresherForTest returns a store-only client's record, kept by a
// reservation, and a clock the test moves.
func refresherForTest(t *testing.T, fs TernVFS) (*ClientStore, uint64, *leaseRefresher, *time.Time) {
	t.Helper()
	store, clientID := newConfirmedStoreClient(t, fs, []byte("client"), [8]byte{1}, clientOwner{})
	clock := time.Unix(1000, 0)
	store.now = func() time.Time { return clock }
	release, ok := store.reserveLeaseRecord(clientID)
	if !ok {
		t.Fatal("no record credit")
	}
	t.Cleanup(release)
	store.refreshMu.Lock()
	r := store.refreshers[InodeID(clientID)]
	store.refreshMu.Unlock()
	return store, clientID, r, &clock
}

func publishedExpiry(t *testing.T, cs *ClientStore, clientID uint64) int64 {
	t.Helper()
	id, err := cs.fs.Lookup(InodeID(clientID), cs.leaseName)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := cs.readLease(id)
	if err != nil {
		t.Fatal(err)
	}
	return lease.ExpiresUnixNano
}

func recordImage(r *leaseRefresher) (attempt bool, orphans int, proven int64, bootstrap bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attempt != nil, len(r.orphans), r.proven, r.bootstrap
}

// An unsettled publication RENAME blocks every later publication until it
// is fenced, and lease content only grows.
func TestRefresherUnsettledAttemptBlocksPublication(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, clock := refresherForTest(t, fs)
	r.safePass(true)
	first := publishedExpiry(t, cs, clientID)

	*clock = clock.Add(leaseWriteTerm)
	fs.set(func() { fs.renameFail = 1 })
	r.safePass(true)
	if attempt, _, _, _ := recordImage(r); !attempt {
		t.Fatal("failed RENAME left no unsettled attempt")
	}
	// The fence fails: nothing new is constructed.
	fs.set(func() { fs.fenceErr = errInjectedLease; fs.constructs = 0 })
	for range 3 {
		r.safePass(true)
	}
	if fs.constructs != 0 {
		t.Fatalf("%d temps constructed behind an unsettled attempt", fs.constructs)
	}
	if got := publishedExpiry(t, cs, clientID); got != first {
		t.Fatalf("lease changed behind an unsettled attempt: %d, was %d", got, first)
	}
	// Fenced, the next pass publishes again, with a larger expiry even on
	// an unchanged clock.
	fs.set(func() { fs.fenceErr = nil })
	r.safePass(true)
	if attempt, orphans, _, _ := recordImage(r); attempt || orphans != 0 {
		t.Fatalf("fence left attempt %t, orphans %d", attempt, orphans)
	}
	second := publishedExpiry(t, cs, clientID)
	if second <= first {
		t.Fatalf("lease did not grow: %d after %d", second, first)
	}
}

// An attempt settled because its edge is gone raises proven only from
// lease content read as evidence, never from the attempt's own expiry.
func TestRefresherEdgeGoneDoesNotRaiseProven(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, clock := refresherForTest(t, fs)
	r.safePass(true)
	first := publishedExpiry(t, cs, clientID)
	*clock = clock.Add(nfsLeaseRenewAfter)
	fs.set(func() { fs.renameFail = 1 })
	r.safePass(true)
	r.mu.Lock()
	expiry := r.attempt.expiry
	r.mu.Unlock()
	// The fence finds the edge gone; no new temp is made this pass.
	fs.set(func() { fs.fenceErr = os.ErrNotExist; fs.constructFn = func() error { return errInjectedLease } })
	r.safePass(true)
	attempt, _, proven, _ := recordImage(r)
	if attempt {
		t.Fatal("edge gone did not settle the attempt")
	}
	if proven >= expiry || proven != first {
		t.Fatalf("proven %d, want the published %d (attempt was %d)", proven, first, expiry)
	}
}

// A LINK that never applies keeps its obligation and credit; refresh pauses
// at four, nothing is scrapped or released on time or absence, and a late
// LINK then consumed settles every obligation.
func TestRefresherPersistentLinkFailure(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir()), linkFail: true}
	cs, _, r, clock := refresherForTest(t, fs)
	for range maxRecordOrphans + 3 {
		r.safePass(true)
		*clock = clock.Add(10 * nfsLeaseTime)
	}
	if _, orphans, _, _ := recordImage(r); orphans != maxRecordOrphans {
		t.Fatalf("orphans %d, want %d", orphans, maxRecordOrphans)
	}
	if fs.constructs != maxRecordOrphans {
		t.Fatalf("constructed %d temps, want %d (the fifth is refused)", fs.constructs, maxRecordOrphans)
	}
	if fs.scraps != 0 {
		t.Fatal("an unresolved temp was scrapped")
	}
	cs.refreshMu.Lock()
	charged := cs.orphans
	cs.refreshMu.Unlock()
	if charged != maxRecordOrphans {
		t.Fatalf("process orphan charge %d", charged)
	}
	// Each LinkFile used its own never-reused name.
	seen := map[string]bool{}
	for _, name := range fs.links {
		if seen[name] {
			t.Fatalf("LinkFile retried %s", name)
		}
		seen[name] = true
	}

	// The late LINKs apply; the next settle consumes them by their edges,
	// without publishing any of them.
	fs.set(func() { fs.linkFail = false })
	fs.applyLate(t)
	r.mu.Lock()
	temps := []string{}
	for _, o := range r.orphans {
		temps = append(temps, o.temp)
	}
	r.mu.Unlock()
	r.settle()
	if _, orphans, _, _ := recordImage(r); orphans != 0 {
		t.Fatalf("late LINK left %d obligations", orphans)
	}
	cs.refreshMu.Lock()
	charged = cs.orphans
	cs.refreshMu.Unlock()
	if charged != 0 {
		t.Fatalf("settled orphans kept %d credits", charged)
	}
	for _, temp := range temps {
		if _, err := fs.Lookup(r.incarnation, temp); err == nil {
			t.Fatalf("settled temp %s still linked", temp)
		}
	}
	if _, err := fs.Lookup(r.incarnation, cs.leaseName); err == nil {
		t.Fatal("an orphan was published as the lease")
	}
}

// The process cap pauses temp creation for every record, and a
// ConstructFile error settles its obligation at once.
func TestRefresherGlobalCapAndConstructError(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, _, r, _ := refresherForTest(t, fs)
	cs.refreshMu.Lock()
	cs.orphans = cs.orphanLimit
	cs.refreshMu.Unlock()
	r.safePass(true)
	if fs.constructs != 0 {
		t.Fatal("temp constructed at the process cap")
	}
	cs.refreshMu.Lock()
	cs.orphans = 0
	cs.refreshMu.Unlock()

	fs.set(func() { fs.constructFn = func() error { return errInjectedLease } })
	r.safePass(true)
	if _, orphans, _, _ := recordImage(r); orphans != 0 {
		t.Fatalf("ConstructFile error left %d obligations", orphans)
	}
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	if cs.orphans != 0 {
		t.Fatal("ConstructFile error kept its credit")
	}
}

// Only need or bootstrap authorizes a write. Bootstrap survives failed
// publication and clears on success; a verify pass only reads.
func TestRefresherBootstrapAndVerify(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, _ := refresherForTest(t, fs)
	need := false
	cs.leaseNeed = func(uint64) bool { return need }
	defer func() { cs.leaseNeed = nil }()

	r.safePass(false)
	if fs.constructs != 0 {
		t.Fatal("verify pass wrote a lease")
	}
	if _, _, _, bootstrap := recordImage(r); bootstrap {
		t.Fatal("verify set bootstrap")
	}
	r.mu.Lock()
	r.bootstrap = true
	r.maintainSoonLocked() // as the worker's wake does for bootstrap (wakeDue)
	r.mu.Unlock()
	fs.set(func() { fs.renameFail = 1 })
	r.safePass(false)
	if _, _, _, bootstrap := recordImage(r); !bootstrap {
		t.Fatal("failed publication cleared bootstrap")
	}
	// Before the failure's backoff deadline a pass only reads.
	fs.set(func() { fs.constructs = 0 })
	r.safePass(false)
	if fs.constructs != 0 {
		t.Fatal("a pass wrote before the failure deadline")
	}
	r.mu.Lock()
	r.notBefore = time.Now().Add(-time.Millisecond)
	r.maintainAt = r.notBefore
	r.mu.Unlock()
	r.safePass(false)
	if _, _, proven, bootstrap := recordImage(r); bootstrap || proven <= cs.now().UnixNano() {
		t.Fatalf("publication: bootstrap %t proven %d", bootstrap, proven)
	}
	if got := publishedExpiry(t, cs, clientID); got <= cs.now().UnixNano() {
		t.Fatalf("published expiry %d", got)
	}

	// Cadence while needed, and backoff doubling to its cap.
	need = true
	wait, _ := r.schedule(false)
	if wait <= 0 || wait > leaseRefreshInterval {
		t.Fatalf("refresh cadence %s", wait)
	}
	r.mu.Lock()
	r.backoff = 0
	var backoffs []time.Duration
	for range 5 {
		backoffs = append(backoffs, r.nextBackoffLocked())
	}
	r.mu.Unlock()
	want := []time.Duration{leaseBackoffMin, 2 * leaseBackoffMin, leaseBackoffMax, leaseBackoffMax, leaseBackoffMax}
	for i := range want {
		if backoffs[i] != want[i] {
			t.Fatalf("backoff %v, want %v", backoffs, want)
		}
	}
}

// Records are bounded; a record with a worker is not freed.
func TestRefresherRecordBudget(t *testing.T) {
	fs := NewLocalTernVFS(t.TempDir())
	cs, err := NewClientStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	cs.refresherLimit = 1
	release, ok := cs.reserveLeaseRecord(1)
	if !ok {
		t.Fatal("first record refused")
	}
	if _, ok := cs.reserveLeaseRecord(2); ok {
		t.Fatal("record past the budget")
	}
	if _, ok := cs.leaseView(2); ok {
		t.Fatal("view created a record past the budget")
	}
	cs.refreshMu.Lock()
	r := cs.refreshers[1]
	cs.refreshMu.Unlock()
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()
	release()
	cs.sweepLeaseRecords()
	cs.refreshMu.Lock()
	kept := cs.refreshers[1] == r
	cs.refreshMu.Unlock()
	if !kept {
		t.Fatal("record freed while its worker runs")
	}
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
	cs.sweepLeaseRecords()
	if _, ok := cs.reserveLeaseRecord(2); !ok {
		t.Fatal("credit not returned after the worker exited")
	}
}

// The lease gate: a read error returns it to live with nothing published;
// an active or quiet group is never expired; expiry refuses admission.
func TestLeaseGateDecisions(t *testing.T) {
	fs := &admissionLeaseVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.clients.stopLeaseWorkers)
	clientID, confirm, err := srv.clients.SetClientID([8]byte{1}, []byte("gate"), clientOwner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := srv.clients.ConfirmClientID(clientID, confirm, rpcPrincipal{}); err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	moveSessionForTest(srv.sessions, sess, clientID, sess.id)
	srv.clients.pinLeaseForTest(clientID)
	srv.clients.setClock(func() time.Time { return time.Now().Add(10 * nfsLeaseTime) })
	gate := func() leaseGateState {
		srv.sessions.mu.Lock()
		defer srv.sessions.mu.Unlock()
		return srv.sessions.groups[clientID].lease
	}

	// Active: no decision, no read.
	hold, status := srv.sessions.holdRequest(sess)
	if status != NFS4_OK {
		t.Fatal(Nfsstat4Name(status))
	}
	hold.activate()
	fs.fail = true
	if expired, err := srv.decideExpired(clientID, true); expired || err != nil {
		t.Fatalf("active client decided %t %v", expired, err)
	}
	// Released: quiet until nfsLeaseTime after the reply.
	hold.release()
	srv.sessions.mu.Lock()
	quiet := srv.sessions.groups[clientID].quietUntil
	srv.sessions.mu.Unlock()
	if quiet.Before(srv.sessions.now().Add(nfsLeaseTime - time.Second)) {
		t.Fatalf("quiet until %s", quiet)
	}
	if expired, _ := srv.decideExpired(clientID, true); expired {
		t.Fatal("quiet client expired")
	}
	endQuietForTest(srv)
	// A read error: live again, no marker, state kept.
	if expired, err := srv.decideExpired(clientID, true); expired || err == nil {
		t.Fatalf("read error decided %t %v", expired, err)
	}
	if gate() != gateLive {
		t.Fatal("read error left the gate deciding")
	}
	fs.fail = false
	if _, err := fs.Lookup(InodeID(clientID), expiredName); err == nil {
		t.Fatal("marker published on a read error")
	}
	if srv.sessions.lookup(ts.id) == nil {
		t.Fatal("read error retired the session")
	}

	// Expired: admission is refused.
	srv.sessions.mu.Lock()
	srv.sessions.groups[clientID].lease = gateExpired
	srv.sessions.mu.Unlock()
	if _, status := srv.sessions.holdRequest(sess); status != NFS4ERR_BADSESSION {
		t.Fatalf("admission after expiry %s", Nfsstat4Name(status))
	}
	srv.sessions.mu.Lock()
	srv.sessions.groups[clientID].lease = gateChecking
	srv.sessions.mu.Unlock()
	if _, status := srv.sessions.holdRequest(sess); status != NFS4ERR_DELAY {
		t.Fatalf("admission while deciding %s", Nfsstat4Name(status))
	}
}

// trapVFS blocks every backend call while armed.
type trapVFS struct {
	TernVFS
	armed   bool
	mu      sync.Mutex
	calls   []string
	release chan struct{}
}

func (fs *trapVFS) trap(name string) {
	fs.mu.Lock()
	armed := fs.armed
	if armed {
		fs.calls = append(fs.calls, name)
	}
	fs.mu.Unlock()
	if armed {
		<-fs.release
	}
}
func (fs *trapVFS) Stat(id InodeID) (NodeInfo, error) { fs.trap("Stat"); return fs.TernVFS.Stat(id) }
func (fs *trapVFS) Lookup(d InodeID, n string) (InodeID, error) {
	fs.trap("Lookup")
	return fs.TernVFS.Lookup(d, n)
}
func (fs *trapVFS) LookupParent(id InodeID) (InodeID, error) {
	fs.trap("LookupParent")
	return fs.TernVFS.LookupParent(id)
}
func (fs *trapVFS) Readdir(d InodeID, h uint64) ([]DirEntry, uint64, error) {
	fs.trap("Readdir")
	return fs.TernVFS.Readdir(d, h)
}
func (fs *trapVFS) ReadAll(id InodeID) ([]byte, error) {
	fs.trap("ReadAll")
	return fs.TernVFS.ReadAll(id)
}
func (fs *trapVFS) Readlink(id InodeID) (string, error) {
	fs.trap("Readlink")
	return fs.TernVFS.Readlink(id)
}
func (fs *trapVFS) LookupEdge(d InodeID, n string) (Edge, error) {
	fs.trap("LookupEdge")
	return fs.TernVFS.LookupEdge(d, n)
}

// SEQUENCE never calls the store: admission and its DELAY both answer from
// memory while every backend call blocks.
func TestSequenceAdmissionDoesNoStoreIO(t *testing.T) {
	fs := &trapVFS{TernVFS: NewLocalTernVFS(t.TempDir()), release: make(chan struct{})}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	fs.mu.Lock()
	fs.armed = true
	fs.mu.Unlock()
	defer close(fs.release)
	done := make(chan uint32, 2)
	go func() {
		_, res := admissionRequest(t, srv, admissionSequence(body, ts, 0, 1, true), 1, nil)
		done <- res.Status()
		setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) { r.proven = 0 })
		_, res = admissionRequest(t, srv, admissionSequence(body, ts, 0, 2, true), 2, nil)
		done <- res.Status()
	}()
	for _, want := range []uint32{NFS4_OK, NFS4ERR_DELAY} {
		if got := awaitValue(t, done, "SEQUENCE with a blocked store"); got != want {
			t.Fatalf("SEQUENCE %s, want %s", Nfsstat4Name(got), Nfsstat4Name(want))
		}
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.calls) != 0 {
		t.Fatalf("SEQUENCE called the store: %v", fs.calls)
	}
}

// Association tokens: a commit by BIND survives another request's abort,
// a pending-only failure leaves nothing, and pending tokens count toward
// the quota so SEQUENCE gets RESOURCE before any renewal work.
func TestAssociationTokens(t *testing.T) {
	srv, err := NewServer(NewLocalTernVFS(t.TempDir()), readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	conn := &rpcConnection{}
	pending, ok := conn.reserve(srv, ts.id)
	if !ok || !pending.new {
		t.Fatal("reserve")
	}
	if !conn.bind(srv, ts.id) {
		t.Fatal("bind")
	}
	pending.abort()
	if !conn.associated(ts.id) {
		t.Fatal("abort removed BIND's association")
	}
	other := connectionSession(t, srv)
	failed, _ := conn.reserve(srv, other.id)
	failed.abort()
	failed.abort()
	conn.mu.Lock()
	if _, ok := conn.sessions[other.id]; ok || len(conn.pending) != 0 {
		conn.mu.Unlock()
		t.Fatal("pending-only failure left an association")
	}
	conn.mu.Unlock()

	// Fill the quota with pending tokens.
	var tokens []*associationToken
	for len(tokens) < maxConnectionSessions-1 {
		s := connectionSession(t, srv)
		tok, ok := conn.reserve(srv, s.id)
		if !ok {
			t.Fatalf("reserve %d refused", len(tokens))
		}
		tokens = append(tokens, tok)
	}
	if _, ok := conn.reserve(srv, other.id); ok {
		t.Fatal("pending tokens not counted against the quota")
	}
	// SEQUENCE on the full connection: RESOURCE, with no verify requested.
	sess := srv.sessions.lookup(other.id)
	setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) { r.proven = 0 })
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	_, res := admissionRequest(t, srv, admissionSequence(body, other, 0, 1, true), 1, conn)
	admissionStatus(t, res, NFS4ERR_RESOURCE, OP_SEQUENCE)
	setLeaseForTest(srv, sess.clientID, func(r *leaseRefresher) {
		if r.verify {
			t.Error("RESOURCE admission requested a verify")
		}
	})
	for _, tok := range tokens {
		tok.abort()
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.pending) != 0 || len(conn.sessions) != 1 {
		t.Fatalf("pending %d sessions %d", len(conn.pending), len(conn.sessions))
	}
}

// A replay renews: its commit activates a hold like a new request.
func TestSequenceReplayRenews(t *testing.T) {
	srv, err := NewServer(NewLocalTernVFS(t.TempDir()), readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := connectionSession(t, srv)
	sess := srv.sessions.lookup(ts.id)
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	wire := admissionSequence(body, ts, 0, 1, true)
	admissionRequest(t, srv, wire, 1, nil)
	endQuietForTest(srv)
	admissionRequest(t, srv, wire, 2, nil)
	srv.sessions.mu.Lock()
	defer srv.sessions.mu.Unlock()
	g := srv.sessions.groups[sess.clientID]
	if g == nil || !g.quietUntil.After(time.Now()) {
		t.Fatal("replay did not renew")
	}
}

// An idle client's first SEQUENCE gets DELAY while a verify reads its
// lease; the worker's read-only pass then admits the retry.
func TestIdleClientResumesAfterVerify(t *testing.T) {
	fs := &refresherFaultVFS{TernVFS: NewLocalTernVFS(t.TempDir())}
	srv, err := NewServer(fs, readOnlyStagingStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	addr, stop := serveTestServer(t, srv)
	defer stop()
	conn := dial(t, addr)
	defer conn.Close()
	ts, err := createTestSession(conn, 1, "idle-client", [8]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	clientID := incarnationForTest(t, srv, ts.clientID)
	setLeaseForTest(srv, clientID, func(r *leaseRefresher) { r.verifiedAt = 0 })
	fs.set(func() { fs.constructs = 0 })
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	reply, err := sendRawRPC(conn, 2, procCompound, withSequence(body, ts, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != NFS4ERR_DELAY {
		t.Fatalf("idle SEQUENCE %s, want DELAY", Nfsstat4Name(got))
	}
	for deadline := time.Now().Add(testChannelTimeout); ; time.Sleep(time.Millisecond) {
		reply, err = sendRawRPC(conn, 3, procCompound, withSequence(body, ts, 0, 1))
		if err != nil {
			t.Fatal(err)
		}
		if sequenceStatus(reply) == NFS4_OK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("retry %s", Nfsstat4Name(sequenceStatus(reply)))
		}
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.constructs != 0 {
		t.Fatal("the verify wrote a lease")
	}
}

// CREATE_SESSION replies once the worker has published the client's first
// lease, so a client that treats DELAY as failure (libnfs at mount) gets OK
// from its first SEQUENCE.
func TestCreateSessionWaitsForFirstPublication(t *testing.T) {
	srv, addr, cleanup := startTestServerWithServer(t, t.TempDir())
	defer cleanup()
	conn := dial(t, addr)
	defer conn.Close()
	clientID, seq, status, err := exchangeID(conn, 1, "first-sequence", [8]byte{6})
	if err != nil || status != NFS4_OK {
		t.Fatalf("EXCHANGE_ID %s %v", Nfsstat4Name(status), err)
	}
	reply, err := sendRawRPC(conn, 2, procCompound, backCreateSessionBody(clientID, seq, 0))
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil || res.Status() != NFS4_OK {
		t.Fatalf("CREATE_SESSION %v %v", res.Status(), err)
	}
	it := res.Resarray()
	it.Next()
	ok := it.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	if !srv.clients.leaseProven(InodeID(incarnationForTest(t, srv, clientID))) {
		t.Fatal("CREATE_SESSION replied before the first publication")
	}
	ts := newTestSession(readSessionID(ok.CsrSessionid()), clientID, ok.CsrForeChanAttrs().CaMaxrequests())
	body := buildCompoundBody(nil, func(*COMPOUND4argsWriter) {})
	reply, err = sendRawRPC(conn, 3, procCompound, withSequence(body, ts, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceStatus(reply); got != NFS4_OK {
		t.Fatalf("first SEQUENCE %s", Nfsstat4Name(got))
	}
}
