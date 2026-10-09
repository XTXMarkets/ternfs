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
