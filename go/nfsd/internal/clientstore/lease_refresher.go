// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The durable lease refresher (canonical r3 §9). This process's lease file,
// lease.<nfsdID>, is written only here: by one worker per incarnation that
// needs it, which holds leaseLocks for each cycle. Everything else reads the
// record from memory and asks the worker for work.
//
// A write goes through a uniquely named temp: ConstructFile, then LinkFile
// writes and links it, then RenameEdge moves that exact edge over the lease.
// Each temp is an orphan obligation from before its first call until its
// edge is consumed, by our RenameEdge, the fence of an unsettled attempt, or
// a RemoveEdge of the edge found pointing at it. Nothing settles on elapsed
// time or absence, nothing is scrapped, and a LinkFile is never retried.
// Settling an orphan never publishes it as the lease
// (P/C8/SETTLEMENT-CLARIFICATION.md).

type leaseRecordState uint8

const (
	LeaseRecordLive leaseRecordState = iota
	LeaseRecordStale
	LeaseRecordFleetExpired
)

type orphanPhase uint8

const (
	orphanConstructing orphanPhase = iota
	orphanLinking
	orphanLinked
	OrphanUnresolved
)

// maxRecordOrphans bounds one record's open temp obligations.
const maxRecordOrphans = 4

// LeaseWriteTerm is how far ahead a publication's expiry is written
// (canonical r3 §9.2 step 3): a refresh is due once the proven lease is
// less than LeaseTime + LeaseRenewAfter/2 ahead.
const LeaseWriteTerm = LeaseTime + LeaseRenewAfter

// leaseOrphan is one temp: from the credit taken before ConstructFile until
// its edge is consumed.
type leaseOrphan struct {
	Temp  string
	Inode vfs.InodeID
	Phase orphanPhase
}

// leaseAttempt is the one unsettled publication RENAME a record may have.
type leaseAttempt struct {
	orphan *leaseOrphan
	edge   vfs.Edge
	expiry int64
}

// leaseView is what SEQUENCE and MarkOpen read, from memory only.
type leaseView struct {
	State      leaseRecordState
	Proven     int64 // lower bound on the published expiry, unix nanoseconds
	VerifiedAt int64 // when the confirmed pointer was last read naming us
}

type LeaseRefresher struct {
	cs          *Store
	Incarnation vfs.InodeID

	// mu is never held across I/O. The worker's own fields (attempt,
	// orphans, identity, lastExpiry, backoff) change only in the worker,
	// which holds leaseLocks; mu orders them with readers.
	Mu         sync.Mutex
	identity   vfs.InodeID
	Proven     int64
	VerifiedAt int64
	State      leaseRecordState
	Attempt    *leaseAttempt
	orphans    []*leaseOrphan
	lastExpiry int64
	bootstrap  bool
	InCall     bool
	running    bool // a worker goroutine exists
	wake       chan struct{}
	Verify     bool // a verify pass is wanted
	lastPass   time.Time
	Backoff    time.Duration
	refs       int  // reservations that keep the record (CREATE_SESSION)
	Finished   bool // a terminal record's sessions have been retired
	pinned     bool // tests: a fixed live lease, no worker
	// raised is closed, and replaced, whenever proven rises or the record
	// turns terminal, for CREATE_SESSION's bounded bootstrap wait.
	raised chan struct{}
	// notBefore is a failed pass's backoff deadline: no settlement or write
	// runs before it, whatever wakes the worker. maintainAt is when the next
	// maintenance pass (settle, refresh, prune or the terminal steps) is
	// due, never before notBefore; a pass before it is a verify and only
	// reads. Verifies neither run nor move it. idleWaiting: the worker is
	// waiting with nothing scheduled, so a need that arrives starts a pass.
	NotBefore   time.Time
	MaintainAt  time.Time
	IdleWaiting bool
	// reclaim: the sweeper, or a reservation's release, found the running
	// record idle; its worker passes at once to free it (pass), and only
	// for that.
	reclaim bool
}

// raiseLocked wakes the waiters for proven. The caller holds r.mu.
func (r *LeaseRefresher) raiseLocked() {
	if r.raised != nil {
		close(r.raised)
		r.raised = nil
	}
}

// Worker cadence. Variables so tests can shorten them.
var (
	leaseRefreshInterval = LeaseRenewAfter / 2
	leaseBackoffMin      = LeaseRenewAfter / 8
	leaseBackoffMax      = LeaseRenewAfter / 2
	LeaseVerifySpacing   = time.Second
)

// recordLocked returns the incarnation's record, creating it when create is
// set and a credit is free. The caller holds cs.refreshMu.
func (cs *Store) RecordLocked(id vfs.InodeID, create bool) (*LeaseRefresher, bool) {
	if r := cs.Refreshers[id]; r != nil {
		return r, true
	}
	if !create || len(cs.Refreshers) >= cs.RefresherLimit {
		return nil, false
	}
	r := &LeaseRefresher{cs: cs, Incarnation: id, wake: make(chan struct{}, 1)}
	cs.Refreshers[id] = r
	return r, true
}

// kickLocked starts or wakes the worker. The caller holds cs.refreshMu.
func (r *LeaseRefresher) kickLocked() {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if r.pinned || r.cs.LeaseNeed == nil || r.cs.leaseStopped {
		return
	}
	if !r.running {
		r.running = true
		r.cs.leaseWorkers.Add(1)
		go func() {
			defer r.cs.leaseWorkers.Done()
			r.run()
		}()
		return
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// leaseView reads the incarnation's record from memory, creating one (with
// a verify pass) if it has none. ok is false when no record credit is free.
func (cs *Store) LeaseView(clientID uint64) (leaseView, bool) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r, ok := cs.RecordLocked(vfs.InodeID(clientID), true)
	if !ok {
		return leaseView{}, false
	}
	r.Mu.Lock()
	view := leaseView{State: r.State, Proven: r.Proven, VerifiedAt: r.VerifiedAt}
	fresh := r.lastPass.IsZero() && !r.pinned
	r.Mu.Unlock()
	if fresh {
		cs.requestVerifyLocked(r)
	}
	return view, true
}

// requestLeaseVerify asks for a read-only pass: it never changes need or
// bootstrap. Requests coalesce.
func (cs *Store) RequestLeaseVerify(clientID uint64) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	if r, ok := cs.RecordLocked(vfs.InodeID(clientID), true); ok {
		cs.requestVerifyLocked(r)
	}
}

func (cs *Store) requestVerifyLocked(r *LeaseRefresher) {
	r.Mu.Lock()
	r.Verify = true
	r.Mu.Unlock()
	r.kickLocked()
}

// noteLeaseNeed tells the worker that need may have become true: a SEQUENCE
// committed. The condition itself is read from the server each pass.
func (cs *Store) NoteLeaseNeed(clientID uint64) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	if r, ok := cs.RecordLocked(vfs.InodeID(clientID), true); ok {
		r.kickLocked()
	}
}

// reserveLeaseRecord keeps a record for a CREATE_SESSION in progress; with
// no credit the caller gets DELAY before its slot.
func (cs *Store) ReserveLeaseRecord(clientID uint64) (func(), bool) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r, ok := cs.RecordLocked(vfs.InodeID(clientID), true)
	if !ok {
		return nil, false
	}
	r.Mu.Lock()
	r.refs++
	r.Mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			cs.RefreshMu.Lock()
			defer cs.RefreshMu.Unlock()
			r.Mu.Lock()
			r.refs--
			idle := r.idleLocked()
			r.Mu.Unlock()
			if idle {
				cs.reclaimLocked(r)
			}
		})
	}, true
}

// startLeaseBootstrap marks a committed CREATE_SESSION's incarnation for its
// first publication. It is cleared only by a publication or a terminal state.
func (cs *Store) StartLeaseBootstrap(clientID uint64) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r, ok := cs.RecordLocked(vfs.InodeID(clientID), true)
	if !ok {
		return
	}
	r.Mu.Lock()
	if r.State == LeaseRecordLive {
		r.bootstrap = true
	}
	r.Mu.Unlock()
	r.kickLocked()
}

// noteFleetExpired records a marker this process published or observed, so
// lease users see EXPIRED without waiting for the worker's evidence, and
// wakes the worker for the terminal steps.
func (cs *Store) NoteFleetExpired(clientID uint64) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r := cs.Refreshers[vfs.InodeID(clientID)]
	if r == nil {
		return
	}
	r.Mu.Lock()
	r.State = LeaseRecordFleetExpired
	r.bootstrap = false
	r.Mu.Unlock()
	r.kickLocked()
}

// BootstrapWait bounds how long CREATE_SESSION waits for the first
// publication before replying. A variable so tests can shorten it.
var BootstrapWait = 2 * time.Second

// awaitLeasePublished waits, up to wait, until the incarnation's record has
// a published lease in the future or turns terminal. CREATE_SESSION uses it
// so that a client's first SEQUENCE does not get DELAY while the bootstrap
// publication, which the worker writes, is still in flight. It reports
// whether the lease was published.
func (cs *Store) AwaitLeasePublished(clientID uint64, wait time.Duration) bool {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		cs.RefreshMu.Lock()
		r := cs.Refreshers[vfs.InodeID(clientID)]
		cs.RefreshMu.Unlock()
		if r == nil {
			return false
		}
		r.Mu.Lock()
		if r.State != LeaseRecordLive {
			r.Mu.Unlock()
			return false
		}
		if r.Proven > cs.Now().UnixNano() {
			r.Mu.Unlock()
			return true
		}
		if r.raised == nil {
			r.raised = make(chan struct{})
		}
		raised := r.raised
		r.Mu.Unlock()
		select {
		case <-raised:
		case <-deadline.C:
			return false
		}
	}
}

// pinLeaseForTest gives an incarnation a fixed live lease with no worker,
// for tests whose sessions have no durable client.
func (cs *Store) PinLeaseForTest(clientID uint64) {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r, _ := cs.RecordLocked(vfs.InodeID(clientID), true)
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.pinned = true
	r.Proven = time.Now().Add(100 * 365 * 24 * time.Hour).UnixNano()
	r.VerifiedAt = r.Proven
	r.lastPass = time.Now()
}

// idleLocked reports that nothing references the record and nothing needs
// it. The caller holds cs.refreshMu and r.mu. A live record whose
// incarnation still has sessions here is referenced: SEQUENCE reads it. One
// whose lease is needed (an active request or a quiet period) is kept for
// its refresh. Both are read from the server, under sessionStore.mu, which
// is ordered after r.mu.
func (r *LeaseRefresher) idleLocked() bool {
	return !r.pinned && r.refs == 0 && !r.bootstrap && r.Attempt == nil &&
		len(r.orphans) == 0 && !r.Verify && !r.InCall &&
		(r.State == LeaseRecordLive && !r.sessionsLocked() || r.Finished) &&
		!r.need()
}

// reclaimLocked frees an idle record with no worker, or has its worker
// free it: the worker, which may be waiting with a deadline a day away,
// owns the record until it exits. The caller holds cs.refreshMu and has
// found the record idle.
func (cs *Store) reclaimLocked(r *LeaseRefresher) {
	r.Mu.Lock()
	running := r.running
	if running {
		r.reclaim = true
	}
	r.Mu.Unlock()
	if !running {
		cs.freeLocked(r)
		return
	}
	r.kickLocked()
}

// sessionsLocked reports the server's sessions for the incarnation. With no
// server, the store's cached client state stands in for them.
func (r *LeaseRefresher) sessionsLocked() bool {
	if r.cs.LeaseSessions == nil {
		_, cached := r.cs.LocalClients.Load(r.Incarnation)
		return cached
	}
	return r.cs.LeaseSessions(uint64(r.Incarnation))
}

// freeLocked forgets an idle record and returns its credit. The caller holds
// cs.refreshMu.
func (cs *Store) freeLocked(r *LeaseRefresher) {
	if cs.Refreshers[r.Incarnation] == r {
		delete(cs.Refreshers, r.Incarnation)
	}
}

func (r *LeaseRefresher) need() bool {
	if r.cs.LeaseNeed == nil {
		return false
	}
	return r.cs.LeaseNeed(uint64(r.Incarnation))
}

// run is the record's worker: passes until nothing references the record.
func (r *LeaseRefresher) run() {
	for {
		wait, exit := r.SafePass(false)
		if exit {
			return
		}
		if !r.await(wait) {
			return
		}
	}
}

// await sleeps until the next pass is due, reporting false once the workers
// are stopped. A wake can only bring the pass earlier, and only for work
// allowed then (wakeDue); nothing postpones it. Requests therefore cannot
// drive passes at request rate, nor cut a failure backoff short.
func (r *LeaseRefresher) await(wait time.Duration) bool {
	due := time.Now().Add(wait)
	for {
		timer := time.NewTimer(time.Until(due))
		select {
		case <-r.wake:
		case <-timer.C:
		case <-r.cs.leaseStop:
		}
		timer.Stop()
		if r.stopped() {
			return false
		}
		if t := r.wakeDue(); t.Before(due) {
			due = t
		}
		if !time.Now().Before(due) {
			return true
		}
	}
}

// wakeDue is the earliest pass the worker's current requests allow: a
// verify no sooner than LeaseVerifySpacing after the last pass, read-only
// before the maintenance deadline. Bootstrap, a need arriving while the
// worker was idle, or a terminal state not yet acted on makes maintenance
// due, no sooner than the failure deadline.
func (r *LeaseRefresher) wakeDue() time.Time {
	need := r.need()
	now := time.Now()
	r.Mu.Lock()
	defer r.Mu.Unlock()
	due := now.Add(leaseIdleWait)
	if r.Verify {
		due = minTime(due, r.lastPass.Add(LeaseVerifySpacing))
	}
	if r.bootstrap || r.IdleWaiting && need || r.terminalLocked() {
		due = minTime(due, r.maintainSoonLocked())
	}
	if r.reclaim {
		// Only to free the record (pass); its deadlines stay.
		due = now
	}
	return due
}

// terminalLocked reports a stale or fleet-expired record whose terminal
// steps have not finished. The caller holds r.mu.
func (r *LeaseRefresher) terminalLocked() bool {
	return r.State != LeaseRecordLive && !r.Finished
}

// maintainSoonLocked makes maintenance due now, or at the failure deadline
// if one applies, and returns when. The caller holds r.mu.
func (r *LeaseRefresher) maintainSoonLocked() time.Time {
	start := time.Now()
	if r.NotBefore.After(start) {
		start = r.NotBefore
	}
	r.MaintainAt = minTime(r.MaintainAt, start)
	return r.MaintainAt
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// stopped reports stopLeaseWorkers; the worker then exits, leaving its
// record for the store's owner to discard with the store.
func (r *LeaseRefresher) stopped() bool {
	r.cs.RefreshMu.Lock()
	defer r.cs.RefreshMu.Unlock()
	if !r.cs.leaseStopped {
		return false
	}
	r.Mu.Lock()
	r.running = false
	r.Mu.Unlock()
	return true
}

// stopLeaseWorkers ends every worker after its current pass and waits for
// them. No worker starts after it.
func (cs *Store) StopLeaseWorkers() {
	cs.RefreshMu.Lock()
	if !cs.leaseStopped {
		cs.leaseStopped = true
		close(cs.leaseStop)
	}
	cs.RefreshMu.Unlock()
	cs.leaseWorkers.Wait()
}

func (r *LeaseRefresher) bootstrapSet() bool {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.bootstrap
}

// safePass runs a pass; a panic is treated as a failed pass and backs off.
func (r *LeaseRefresher) SafePass(authorize bool) (wait time.Duration, exit bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.Mu.Lock()
			r.InCall = false
			wait = r.nextBackoffLocked()
			r.NotBefore = time.Now().Add(wait)
			r.MaintainAt = r.NotBefore
			r.Mu.Unlock()
			exit = false
		}
	}()
	return r.pass(authorize)
}

// call runs one backend call with inCall set.
func (r *LeaseRefresher) call(fn func()) {
	r.Mu.Lock()
	r.InCall = true
	r.Mu.Unlock()
	defer func() {
		r.Mu.Lock()
		r.InCall = false
		r.Mu.Unlock()
	}()
	fn()
}

func (r *LeaseRefresher) nextBackoffLocked() time.Duration {
	if r.Backoff == 0 {
		r.Backoff = leaseBackoffMin
	} else if r.Backoff *= 2; r.Backoff > leaseBackoffMax {
		r.Backoff = leaseBackoffMax
	}
	return r.Backoff
}

// pass is one worker cycle: settle, evidence, then refresh or the terminal
// steps (§9.2). authorize forces the refresh condition, for the store-only
// inline callers. It reports how long to wait, or exit when the record has
// been freed.
func (r *LeaseRefresher) pass(authorize bool) (time.Duration, bool) {
	cs := r.cs
	unlock := cs.LeaseLocks.Lock(r.Incarnation)
	defer unlock()
	if wait, exit, done := r.reclaimed(); done {
		return wait, exit
	}
	r.Mu.Lock()
	now := time.Now()
	// Before the maintenance deadline a pass only reads the evidence. A
	// known terminal record's steps are due once any failure deadline has
	// passed.
	terminalDue := r.terminalLocked() && !now.Before(r.NotBefore)
	readOnly := !authorize && now.Before(r.MaintainAt) && !terminalDue
	r.Verify = false
	r.lastPass = now
	r.Mu.Unlock()
	if readOnly {
		if r.identity != 0 {
			r.evidence()
		}
		if wait, exit, done := r.verified(); done {
			return wait, exit
		}
		// The evidence found the record terminal, and nothing defers its
		// steps: this pass goes on to them, settling first.
	}

	failed := false
	if r.identity == 0 {
		var identity vfs.InodeID
		var valid bool
		var err error
		r.call(func() { identity, valid, err = cs.identityForIncarnation(r.Incarnation) })
		switch {
		case err != nil:
			failed = true
		case !valid:
			r.Mu.Lock()
			r.State = LeaseRecordStale
			r.Mu.Unlock()
		default:
			r.identity = identity
		}
	}
	settled := r.settle()
	if !settled {
		failed = true
	}
	if r.identity != 0 {
		r.evidence()
	}
	r.Mu.Lock()
	state := r.State
	r.Mu.Unlock()
	if state != LeaseRecordLive {
		if settled && !r.terminate() {
			failed = true
		}
	} else if r.identity != 0 && settled && (authorize || r.need() || r.bootstrapSet()) {
		if !r.refresh() {
			failed = true
		} else {
			r.pruneOthers()
		}
	}
	return r.schedule(failed)
}

// settle fences the unsettled attempt and consumes orphans whose edges are
// found (§9.2 step 1, §9.3). It reports false while an attempt is unsettled.
func (r *LeaseRefresher) settle() bool {
	cs, dir := r.cs, r.Incarnation
	if a := r.Attempt; a != nil {
		var err error
		r.call(func() { err = cs.fs.RemoveEdge(dir, a.orphan.Temp, a.edge) })
		if err != nil && !isEdgeGone(err) {
			return false
		}
		r.consume(a.orphan)
		r.Mu.Lock()
		r.Attempt = nil
		r.Mu.Unlock()
	}
	r.Mu.Lock()
	orphans := append([]*leaseOrphan(nil), r.orphans...)
	r.Mu.Unlock()
	for _, o := range orphans {
		if o.Phase != orphanLinked && o.Phase != OrphanUnresolved || o.Inode == 0 {
			continue
		}
		var edge vfs.Edge
		var err error
		r.call(func() { edge, err = cs.fs.LookupEdge(dir, o.Temp) })
		if err != nil || edge.ID != o.Inode {
			// Absent or unreadable: still an obligation.
			continue
		}
		r.call(func() { err = cs.fs.RemoveEdge(dir, o.Temp, edge) })
		if err == nil || isEdgeGone(err) {
			r.consume(o)
		}
	}
	return true
}

// consume settles an orphan and returns its credits.
func (r *LeaseRefresher) consume(o *leaseOrphan) {
	cs := r.cs
	r.Mu.Lock()
	for i, have := range r.orphans {
		if have == o {
			r.orphans = append(r.orphans[:i], r.orphans[i+1:]...)
			r.Mu.Unlock()
			cs.RefreshMu.Lock()
			cs.orphans--
			cs.RefreshMu.Unlock()
			return
		}
	}
	r.Mu.Unlock()
}

// evidence reads, without writing: our lease's content, the expired marker
// and the confirmed pointer (§9.2 step 2). It also removes other processes'
// lease slots that expired at least a lease time ago.
func (r *LeaseRefresher) evidence() {
	cs, dir := r.cs, r.Incarnation
	start := cs.Now().UnixNano()
	var lease DurableLease
	var leaseErr error
	r.call(func() {
		var id vfs.InodeID
		if id, leaseErr = cs.fs.Lookup(dir, cs.LeaseName); leaseErr == nil {
			lease, leaseErr = cs.readLease(id)
		}
	})
	var marked bool
	var markErr error
	r.call(func() { marked, markErr = cs.hasExpiredMarker(dir) })
	var target vfs.InodeID
	var found bool
	var ptrErr error
	r.call(func() { target, found, ptrErr = cs.ReadIdentitySymlink(r.identity, ConfirmedName) })
	r.Mu.Lock()
	if leaseErr == nil && lease.ExpiresUnixNano > r.Proven {
		r.Proven = lease.ExpiresUnixNano
		r.raiseLocked()
	}
	if markErr == nil && marked {
		r.State = LeaseRecordFleetExpired
	}
	if ptrErr == nil {
		if found && target == dir {
			r.VerifiedAt = start
		} else if r.State == LeaseRecordLive {
			r.State = LeaseRecordStale
		}
	}
	r.Mu.Unlock()
}

// pruneOthers removes other processes' lease slots that expired at least a
// lease time ago. It runs only in an authorized pass, while our own
// published lease is live, so the client is not expired and those slots are
// no longer evidence of anything; and only under a destruction hold, so
// never while a confirmation protects the incarnation. Ours is removed only
// by the terminal steps, after a marker. Verify passes do not call it.
func (r *LeaseRefresher) pruneOthers() {
	cs, dir := r.cs, r.Incarnation
	r.Mu.Lock()
	live := r.State == LeaseRecordLive && r.Proven > cs.Now().UnixNano()
	r.Mu.Unlock()
	if !live {
		return
	}
	if cs.BeginMarker != nil {
		end, ok := cs.BeginMarker(dir)
		if !ok {
			return
		}
		defer end()
	}
	r.call(func() {
		if scan, err := cs.scanSlots(dir, LeasePrefix); err == nil {
			var others []string
			for _, name := range scan.expiredNames {
				if name != cs.LeaseName {
					others = append(others, name)
				}
			}
			_ = cs.removeExpiredSlots(dir, others)
		}
	})
}

// refresh publishes a new lease when ours is due (§9.2 step 3). It reports
// false on a failure that should back off.
func (r *LeaseRefresher) refresh() bool {
	cs, dir := r.cs, r.Incarnation
	now := cs.Now()
	r.Mu.Lock()
	due := r.Proven < now.Add(LeaseTime+LeaseRenewAfter/2).UnixNano()
	r.Mu.Unlock()
	if !due {
		return true
	}
	o, ok := r.TakeOrphan()
	if !ok {
		// At a cap: refresh pauses until an obligation settles.
		return false
	}
	expiry := now.Add(LeaseWriteTerm).UnixNano()
	if expiry <= r.lastExpiry {
		expiry = r.lastExpiry + 1
	}
	data, err := json.Marshal(DurableLease{ExpiresUnixNano: expiry})
	if err != nil {
		r.consume(o)
		return false
	}
	var id vfs.InodeID
	var cookie vfs.Cookie
	r.call(func() { id, cookie, err = cs.fs.ConstructFile(dir) })
	if err != nil {
		// No LINK was sent for this never-reused name.
		r.consume(o)
		return false
	}
	r.Mu.Lock()
	o.Inode, o.Phase = id, orphanLinking
	r.Mu.Unlock()
	r.call(func() { err = cs.fs.LinkFile(id, cookie, dir, o.Temp, bytes.NewReader(data)) })
	r.Mu.Lock()
	if err != nil {
		// Its outcome is unknown and it is never retried.
		o.Phase = OrphanUnresolved
		r.Mu.Unlock()
		return false
	}
	o.Phase = orphanLinked
	r.Mu.Unlock()
	var edge vfs.Edge
	r.call(func() { edge, err = cs.fs.LookupEdge(dir, o.Temp) })
	if err != nil || edge.ID != id {
		return false
	}
	r.lastExpiry = expiry
	r.Mu.Lock()
	r.Attempt = &leaseAttempt{orphan: o, edge: edge, expiry: expiry}
	r.Mu.Unlock()
	r.call(func() { err = cs.fs.RenameEdge(dir, o.Temp, edge, dir, cs.LeaseName) })
	if err != nil {
		// Unsettled: no later publication starts until it is fenced.
		return false
	}
	r.consume(o)
	r.Mu.Lock()
	r.Attempt = nil
	if expiry > r.Proven {
		r.Proven = expiry
	}
	r.bootstrap = false
	r.raiseLocked()
	r.Mu.Unlock()
	return true
}

// takeOrphan takes an orphan credit and records a new temp, below the
// per-record and process caps.
func (r *LeaseRefresher) TakeOrphan() (*leaseOrphan, bool) {
	cs := r.cs
	suffix, err := randomHex(8)
	if err != nil {
		return nil, false
	}
	r.Mu.Lock()
	full := len(r.orphans) >= maxRecordOrphans
	r.Mu.Unlock()
	if full {
		return nil, false
	}
	cs.RefreshMu.Lock()
	if cs.orphans >= cs.orphanLimit {
		cs.RefreshMu.Unlock()
		return nil, false
	}
	cs.orphans++
	cs.RefreshMu.Unlock()
	o := &leaseOrphan{Temp: TempPrefix + suffix, Phase: orphanConstructing}
	r.Mu.Lock()
	r.orphans = append(r.orphans, o)
	r.Mu.Unlock()
	return o, true
}

// terminate handles a stale or fleet-expired record once its attempt is
// settled: remove our lease through its edge, best effort, then retire the
// incarnation's sessions. It writes nothing more. It reports false when the
// retirement must be retried.
func (r *LeaseRefresher) terminate() bool {
	cs, dir := r.cs, r.Incarnation
	r.Mu.Lock()
	r.bootstrap = false
	finished := r.Finished
	r.raiseLocked()
	r.Mu.Unlock()
	if finished {
		return true
	}
	// Our lease goes through its edge, best effort, under the same
	// destruction hold as the retirement: never while a confirmation
	// protects the incarnation.
	cleanup := func() {
		r.call(func() {
			if edge, err := cs.fs.LookupEdge(dir, cs.LeaseName); err == nil {
				_ = cs.fs.RemoveEdge(dir, cs.LeaseName, edge)
			}
		})
	}
	if cs.LeaseTerminal == nil {
		cleanup()
	} else if !cs.LeaseTerminal(uint64(dir), cleanup) {
		return false
	}
	cs.LocalClients.Delete(dir)
	r.Mu.Lock()
	r.Finished = true
	r.Mu.Unlock()
	return true
}

// reclaimed handles a reclamation wake before anything else in a pass: the
// record is freed if it is still idle, and otherwise the worker goes back
// to its deadlines. Either way no store call is made, and no deadline
// moves. done is false when no reclamation was asked for.
func (r *LeaseRefresher) reclaimed() (wait time.Duration, exit, done bool) {
	cs := r.cs
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if !r.reclaim {
		return 0, false, false
	}
	r.reclaim = false
	if r.idleLocked() {
		r.running = false
		cs.freeLocked(r)
		return 0, true, true
	}
	return r.nextPassLocked(time.Until(r.MaintainAt)), false, true
}

// verified ends a read-only pass, leaving the deadlines as they were, with
// one exception: a terminal state the evidence has just found makes the
// terminal steps due, once any failure deadline has passed; then done is
// false and the pass goes on to them. A record left idle is freed, which
// needs no store call.
func (r *LeaseRefresher) verified() (wait time.Duration, exit, done bool) {
	cs := r.cs
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if r.terminalLocked() {
		if due := r.maintainSoonLocked(); !time.Now().Before(due) {
			return 0, false, false
		}
	}
	if r.idleLocked() {
		r.running = false
		cs.freeLocked(r)
		return 0, true, true
	}
	return r.nextPassLocked(time.Until(r.MaintainAt)), false, true
}

// nextPassLocked is the wait before the next pass: the maintenance wait, or
// sooner for a pending verify. The caller holds r.mu.
func (r *LeaseRefresher) nextPassLocked(maintain time.Duration) time.Duration {
	if r.Verify {
		maintain = min(maintain, time.Until(r.lastPass.Add(LeaseVerifySpacing)))
	}
	return max(maintain, 0)
}

// schedule decides the next maintenance pass, and the next pass, or frees
// the record.
func (r *LeaseRefresher) schedule(failed bool) (time.Duration, bool) {
	cs := r.cs
	need := r.need()
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.IdleWaiting = false
	if failed {
		wait := r.nextBackoffLocked()
		r.NotBefore = time.Now().Add(wait)
		r.MaintainAt = r.NotBefore
		return wait, false
	}
	r.Backoff, r.NotBefore = 0, time.Time{}
	wait, exit := r.maintenanceLocked(need)
	if exit {
		return 0, true
	}
	r.MaintainAt = time.Now().Add(wait)
	return r.nextPassLocked(wait), false
}

// maintenanceLocked is the wait before the next maintenance pass, or exit
// once the record is freed. The caller holds cs.refreshMu and r.mu.
func (r *LeaseRefresher) maintenanceLocked(need bool) (time.Duration, bool) {
	cs := r.cs
	switch {
	case r.Attempt != nil || len(r.orphans) > 0:
		// Obligations are checked again on every settle cycle.
		return leaseBackoffMax, false
	case r.State == LeaseRecordLive && (need || r.bootstrap):
		wait := leaseRefreshInterval
		due := time.Duration(r.Proven-cs.Now().Add(LeaseTime+LeaseRenewAfter/2).UnixNano()) + time.Millisecond
		if due < wait {
			wait = max(due, 0)
		}
		return wait, false
	}
	if r.idleLocked() {
		r.running = false
		cs.freeLocked(r)
		return 0, true
	}
	if r.terminalLocked() {
		return leaseBackoffMax, false
	}
	// Referenced (sessions or a reservation) but not needed: no maintenance
	// until a SEQUENCE or bootstrap makes it due. A verify only reads.
	r.IdleWaiting = true
	return leaseIdleWait, false
}

// leaseIdleWait bounds an idle worker's sleep; it is woken for any work.
const leaseIdleWait = 24 * time.Hour

// sweepLeaseRecords reclaims idle records, so a record nothing references
// or needs any more is freed on the sweeper's tick, by its worker if it has
// one. Other workers are only woken, which brings nothing forward that
// wakeDue does not allow.
func (cs *Store) SweepLeaseRecords() {
	cs.RefreshMu.Lock()
	defer cs.RefreshMu.Unlock()
	for _, r := range cs.Refreshers {
		r.Mu.Lock()
		idle, running := r.idleLocked(), r.running
		r.Mu.Unlock()
		if idle {
			cs.reclaimLocked(r)
		} else if running {
			r.kickLocked()
		}
	}
}

// refreshInline runs one pass in the caller, for store-only use (no server
// worker): Renew and MarkOpen publish through the same cycle. The caller
// may hold the identity lock. With a server the worker owns every pass, and
// a server's terminal steps take identity ownership under leaseLocks, so
// running a pass here under the identity lock would invert that order: it
// only reads the record instead, and asks for a verify.
func (cs *Store) refreshInline(clientID uint64) (leaseView, error) {
	cs.RefreshMu.Lock()
	r, ok := cs.RecordLocked(vfs.InodeID(clientID), true)
	cs.RefreshMu.Unlock()
	if !ok {
		return leaseView{}, nfs4.Error(nfs4.NFS4ERR_DELAY)
	}
	if cs.serverDecides() {
		view, _ := cs.LeaseView(clientID)
		if view.Proven <= cs.Now().UnixNano() {
			cs.RequestLeaseVerify(clientID)
		}
		return view, nil
	}
	r.SafePass(true)
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return leaseView{State: r.State, Proven: r.Proven, VerifiedAt: r.VerifiedAt}, nil
}

// leaseStatusError maps a record's view to the status a lease user sees.
func (cs *Store) leaseStatusError(view leaseView) error {
	switch {
	case view.State == LeaseRecordStale:
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	case view.State == LeaseRecordFleetExpired:
		return nfs4.Error(nfs4.NFS4ERR_EXPIRED)
	case view.Proven <= cs.Now().UnixNano():
		return nfs4.Error(nfs4.NFS4ERR_DELAY)
	}
	return nil
}
