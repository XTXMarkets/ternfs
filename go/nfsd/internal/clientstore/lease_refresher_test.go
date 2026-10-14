// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func publishedExpiry(t *testing.T, cs *Store, clientID uint64) int64 {
	t.Helper()
	id, err := cs.fs.Lookup(vfs.InodeID(clientID), cs.LeaseName)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := cs.readLease(id)
	if err != nil {
		t.Fatal(err)
	}
	return lease.ExpiresUnixNano
}

// An unsettled publication RENAME blocks every later publication until it
// is fenced, and lease content only grows.
func TestRefresherUnsettledAttemptBlocksPublication(t *testing.T) {
	fs := &RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, clock := RefresherForTest(t, fs)
	r.SafePass(true)
	first := publishedExpiry(t, cs, clientID)

	*clock = clock.Add(LeaseWriteTerm)
	fs.Set(func() { fs.renameFail = 1 })
	r.SafePass(true)
	if attempt, _, _, _ := RecordImage(r); !attempt {
		t.Fatal("failed RENAME left no unsettled attempt")
	}
	// The fence fails: nothing new is constructed.
	fs.Set(func() { fs.fenceErr = ErrInjectedLease; fs.Constructs = 0 })
	for range 3 {
		r.SafePass(true)
	}
	if fs.Constructs != 0 {
		t.Fatalf("%d temps constructed behind an unsettled attempt", fs.Constructs)
	}
	if got := publishedExpiry(t, cs, clientID); got != first {
		t.Fatalf("lease changed behind an unsettled attempt: %d, was %d", got, first)
	}
	// Fenced, the next pass publishes again, with a larger expiry even on
	// an unchanged clock.
	fs.Set(func() { fs.fenceErr = nil })
	r.SafePass(true)
	if attempt, orphans, _, _ := RecordImage(r); attempt || orphans != 0 {
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
	fs := &RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, clock := RefresherForTest(t, fs)
	r.SafePass(true)
	first := publishedExpiry(t, cs, clientID)
	*clock = clock.Add(LeaseRenewAfter)
	fs.Set(func() { fs.renameFail = 1 })
	r.SafePass(true)
	r.Mu.Lock()
	expiry := r.Attempt.expiry
	r.Mu.Unlock()
	// The fence finds the edge gone; no new temp is made this pass.
	fs.Set(func() { fs.fenceErr = os.ErrNotExist; fs.constructFn = func() error { return ErrInjectedLease } })
	r.SafePass(true)
	attempt, _, proven, _ := RecordImage(r)
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
	fs := &RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir()), linkFail: true}
	cs, _, r, clock := RefresherForTest(t, fs)
	for range maxRecordOrphans + 3 {
		r.SafePass(true)
		*clock = clock.Add(10 * LeaseTime)
	}
	if _, orphans, _, _ := RecordImage(r); orphans != maxRecordOrphans {
		t.Fatalf("orphans %d, want %d", orphans, maxRecordOrphans)
	}
	if fs.Constructs != maxRecordOrphans {
		t.Fatalf("constructed %d temps, want %d (the fifth is refused)", fs.Constructs, maxRecordOrphans)
	}
	if fs.scraps != 0 {
		t.Fatal("an unresolved temp was scrapped")
	}
	cs.RefreshMu.Lock()
	charged := cs.orphans
	cs.RefreshMu.Unlock()
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
	fs.Set(func() { fs.linkFail = false })
	fs.applyLate(t)
	r.Mu.Lock()
	temps := []string{}
	for _, o := range r.orphans {
		temps = append(temps, o.Temp)
	}
	r.Mu.Unlock()
	r.settle()
	if _, orphans, _, _ := RecordImage(r); orphans != 0 {
		t.Fatalf("late LINK left %d obligations", orphans)
	}
	cs.RefreshMu.Lock()
	charged = cs.orphans
	cs.RefreshMu.Unlock()
	if charged != 0 {
		t.Fatalf("settled orphans kept %d credits", charged)
	}
	for _, temp := range temps {
		if _, err := fs.Lookup(r.Incarnation, temp); err == nil {
			t.Fatalf("settled temp %s still linked", temp)
		}
	}
	if _, err := fs.Lookup(r.Incarnation, cs.LeaseName); err == nil {
		t.Fatal("an orphan was published as the lease")
	}
}

// The process cap pauses temp creation for every record, and a
// ConstructFile error settles its obligation at once.
func TestRefresherGlobalCapAndConstructError(t *testing.T) {
	fs := &RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, _, r, _ := RefresherForTest(t, fs)
	cs.RefreshMu.Lock()
	cs.orphans = cs.orphanLimit
	cs.RefreshMu.Unlock()
	r.SafePass(true)
	if fs.Constructs != 0 {
		t.Fatal("temp constructed at the process cap")
	}
	cs.RefreshMu.Lock()
	cs.orphans = 0
	cs.RefreshMu.Unlock()

	fs.Set(func() { fs.constructFn = func() error { return ErrInjectedLease } })
	r.SafePass(true)
	if _, orphans, _, _ := RecordImage(r); orphans != 0 {
		t.Fatalf("ConstructFile error left %d obligations", orphans)
	}
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	if cs.orphans != 0 {
		t.Fatal("ConstructFile error kept its credit")
	}
}

// Only need or bootstrap authorizes a write. Bootstrap survives failed
// publication and clears on success; a verify pass only reads.
func TestRefresherBootstrapAndVerify(t *testing.T) {
	fs := &RefresherFaultVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r, _ := RefresherForTest(t, fs)
	need := false
	cs.LeaseNeed = func(uint64) bool { return need }
	defer func() { cs.LeaseNeed = nil }()

	r.SafePass(false)
	if fs.Constructs != 0 {
		t.Fatal("verify pass wrote a lease")
	}
	if _, _, _, bootstrap := RecordImage(r); bootstrap {
		t.Fatal("verify set bootstrap")
	}
	r.Mu.Lock()
	r.bootstrap = true
	r.maintainSoonLocked() // as the worker's wake does for bootstrap (wakeDue)
	r.Mu.Unlock()
	fs.Set(func() { fs.renameFail = 1 })
	r.SafePass(false)
	if _, _, _, bootstrap := RecordImage(r); !bootstrap {
		t.Fatal("failed publication cleared bootstrap")
	}
	// Before the failure's backoff deadline a pass only reads.
	fs.Set(func() { fs.Constructs = 0 })
	r.SafePass(false)
	if fs.Constructs != 0 {
		t.Fatal("a pass wrote before the failure deadline")
	}
	r.Mu.Lock()
	r.NotBefore = time.Now().Add(-time.Millisecond)
	r.MaintainAt = r.NotBefore
	r.Mu.Unlock()
	r.SafePass(false)
	if _, _, proven, bootstrap := RecordImage(r); bootstrap || proven <= cs.Now().UnixNano() {
		t.Fatalf("publication: bootstrap %t proven %d", bootstrap, proven)
	}
	if got := publishedExpiry(t, cs, clientID); got <= cs.Now().UnixNano() {
		t.Fatalf("published expiry %d", got)
	}

	// Cadence while needed, and backoff doubling to its cap.
	need = true
	wait, _ := r.schedule(false)
	if wait <= 0 || wait > leaseRefreshInterval {
		t.Fatalf("refresh cadence %s", wait)
	}
	r.Mu.Lock()
	r.Backoff = 0
	var backoffs []time.Duration
	for range 5 {
		backoffs = append(backoffs, r.nextBackoffLocked())
	}
	r.Mu.Unlock()
	want := []time.Duration{leaseBackoffMin, 2 * leaseBackoffMin, leaseBackoffMax, leaseBackoffMax, leaseBackoffMax}
	for i := range want {
		if backoffs[i] != want[i] {
			t.Fatalf("backoff %v, want %v", backoffs, want)
		}
	}
}

// Records are bounded; a record with a worker is not freed.
func TestRefresherRecordBudget(t *testing.T) {
	fs := vfs.NewLocalTernVFS(t.TempDir())
	cs, err := NewStore(fs)
	if err != nil {
		t.Fatal(err)
	}
	cs.RefresherLimit = 1
	release, ok := cs.ReserveLeaseRecord(1)
	if !ok {
		t.Fatal("first record refused")
	}
	if _, ok := cs.ReserveLeaseRecord(2); ok {
		t.Fatal("record past the budget")
	}
	if _, ok := cs.LeaseView(2); ok {
		t.Fatal("view created a record past the budget")
	}
	cs.RefreshMu.Lock()
	r := cs.Refreshers[1]
	cs.RefreshMu.Unlock()
	r.Mu.Lock()
	r.running = true
	r.Mu.Unlock()
	release()
	cs.SweepLeaseRecords()
	cs.RefreshMu.Lock()
	kept := cs.Refreshers[1] == r
	cs.RefreshMu.Unlock()
	if !kept {
		t.Fatal("record freed while its worker runs")
	}
	r.Mu.Lock()
	r.running = false
	r.Mu.Unlock()
	cs.SweepLeaseRecords()
	if _, ok := cs.ReserveLeaseRecord(2); !ok {
		t.Fatal("credit not returned after the worker exited")
	}
}

// A verify-only pass reads and does not write or remove anything: not
// even another process's long-dead lease slot, which only an authorized
// pass prunes.
func TestVerifyPassIsReadOnly(t *testing.T) {
	fs := &PassCountVFS{TernVFS: vfs.NewLocalTernVFS(t.TempDir())}
	cs, clientID, r := RefresherWorkerForTest(t, fs)
	r.SafePass(true) // publish a fresh lease
	dead, err := json.Marshal(DurableLease{ExpiresUnixNano: time.Now().Add(-10 * LeaseTime).UnixNano()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.TernVFS.CreateFile(vfs.InodeID(clientID), LeasePrefix+"otherprocess", bytes.NewReader(dead)); err != nil {
		t.Fatal(err)
	}
	cs.LeaseNeed = func(uint64) bool { return false }
	fs.Reset()
	for range 20 {
		cs.RequestLeaseVerify(clientID)
		time.Sleep(50 * time.Millisecond)
	}
	reads, fences, writes, removes := fs.Counts()
	if reads == 0 || reads > 3 {
		t.Fatalf("%d verify passes for 20 requests in 1s", reads)
	}
	if fences != 0 || writes != 0 || removes != 0 {
		t.Fatalf("verify passes wrote: fences %d writes %d removes %d", fences, writes, removes)
	}
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(clientID), LeasePrefix+"otherprocess"); err != nil {
		t.Fatal("a verify pass removed another process's slot")
	}
	// An authorized pass, with our lease live, prunes it.
	r.SafePass(true)
	if _, err := fs.TernVFS.Lookup(vfs.InodeID(clientID), LeasePrefix+"otherprocess"); err == nil {
		t.Fatal("an authorized pass kept another process's long-dead slot")
	}
}
