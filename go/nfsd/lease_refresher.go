// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
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
	leaseRecordLive leaseRecordState = iota
	leaseRecordStale
	leaseRecordFleetExpired
)

type orphanPhase uint8

const (
	orphanConstructing orphanPhase = iota
	orphanLinking
	orphanLinked
	orphanUnresolved
)

// maxRecordOrphans bounds one record's open temp obligations.
const maxRecordOrphans = 4

// leaseWriteTerm is how far ahead a publication's expiry is written
// (canonical r3 §9.2 step 3): a refresh is due once the proven lease is
// less than nfsLeaseTime + nfsLeaseRenewAfter/2 ahead.
const leaseWriteTerm = nfsLeaseTime + nfsLeaseRenewAfter

// leaseOrphan is one temp: from the credit taken before ConstructFile until
// its edge is consumed.
type leaseOrphan struct {
	temp  string
	inode InodeID
	phase orphanPhase
}

// leaseAttempt is the one unsettled publication RENAME a record may have.
type leaseAttempt struct {
	orphan *leaseOrphan
	edge   Edge
	expiry int64
}

// leaseView is what SEQUENCE and MarkOpen read, from memory only.
type leaseView struct {
	state      leaseRecordState
	proven     int64 // lower bound on the published expiry, unix nanoseconds
	verifiedAt int64 // when the confirmed pointer was last read naming us
}

type leaseRefresher struct {
	cs          *ClientStore
	incarnation InodeID

	// mu is never held across I/O. The worker's own fields (attempt,
	// orphans, identity, lastExpiry, backoff) change only in the worker,
	// which holds leaseLocks; mu orders them with readers.
	mu         sync.Mutex
	identity   InodeID
	proven     int64
	verifiedAt int64
	state      leaseRecordState
	attempt    *leaseAttempt
	orphans    []*leaseOrphan
	lastExpiry int64
	bootstrap  bool
	inCall     bool
	running    bool // a worker goroutine exists
	wake       chan struct{}
	verify     bool // a verify pass is wanted
	lastPass   time.Time
	backoff    time.Duration
	refs       int  // reservations that keep the record (CREATE_SESSION)
	finished   bool // a terminal record's sessions have been retired
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
	notBefore   time.Time
	maintainAt  time.Time
	idleWaiting bool
	// reclaim: the sweeper, or a reservation's release, found the running
	// record idle; its worker passes at once to free it (pass), and only
	// for that.
	reclaim bool
}

// raiseLocked wakes the waiters for proven. The caller holds r.mu.
func (r *leaseRefresher) raiseLocked() {
	if r.raised != nil {
		close(r.raised)
		r.raised = nil
	}
}

// Worker cadence. Variables so tests can shorten them.
var (
	leaseRefreshInterval = nfsLeaseRenewAfter / 2
	leaseBackoffMin      = nfsLeaseRenewAfter / 8
	leaseBackoffMax      = nfsLeaseRenewAfter / 2
	leaseVerifySpacing   = time.Second
)

// recordLocked returns the incarnation's record, creating it when create is
// set and a credit is free. The caller holds cs.refreshMu.
func (cs *ClientStore) recordLocked(id InodeID, create bool) (*leaseRefresher, bool) {
	if r := cs.refreshers[id]; r != nil {
		return r, true
	}
	if !create || len(cs.refreshers) >= cs.refresherLimit {
		return nil, false
	}
	r := &leaseRefresher{cs: cs, incarnation: id, wake: make(chan struct{}, 1)}
	cs.refreshers[id] = r
	return r, true
}

// kickLocked starts or wakes the worker. The caller holds cs.refreshMu.
func (r *leaseRefresher) kickLocked() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pinned || r.cs.leaseNeed == nil || r.cs.leaseStopped {
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
func (cs *ClientStore) leaseView(clientID uint64) (leaseView, bool) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r, ok := cs.recordLocked(InodeID(clientID), true)
	if !ok {
		return leaseView{}, false
	}
	r.mu.Lock()
	view := leaseView{state: r.state, proven: r.proven, verifiedAt: r.verifiedAt}
	fresh := r.lastPass.IsZero() && !r.pinned
	r.mu.Unlock()
	if fresh {
		cs.requestVerifyLocked(r)
	}
	return view, true
}

// requestLeaseVerify asks for a read-only pass: it never changes need or
// bootstrap. Requests coalesce.
func (cs *ClientStore) requestLeaseVerify(clientID uint64) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	if r, ok := cs.recordLocked(InodeID(clientID), true); ok {
		cs.requestVerifyLocked(r)
	}
}

func (cs *ClientStore) requestVerifyLocked(r *leaseRefresher) {
	r.mu.Lock()
	r.verify = true
	r.mu.Unlock()
	r.kickLocked()
}

// noteLeaseNeed tells the worker that need may have become true: a SEQUENCE
// committed. The condition itself is read from the server each pass.
func (cs *ClientStore) noteLeaseNeed(clientID uint64) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	if r, ok := cs.recordLocked(InodeID(clientID), true); ok {
		r.kickLocked()
	}
}

// reserveLeaseRecord keeps a record for a CREATE_SESSION in progress; with
// no credit the caller gets DELAY before its slot.
func (cs *ClientStore) reserveLeaseRecord(clientID uint64) (func(), bool) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r, ok := cs.recordLocked(InodeID(clientID), true)
	if !ok {
		return nil, false
	}
	r.mu.Lock()
	r.refs++
	r.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			cs.refreshMu.Lock()
			defer cs.refreshMu.Unlock()
			r.mu.Lock()
			r.refs--
			idle := r.idleLocked()
			r.mu.Unlock()
			if idle {
				cs.reclaimLocked(r)
			}
		})
	}, true
}

// startLeaseBootstrap marks a committed CREATE_SESSION's incarnation for its
// first publication. It is cleared only by a publication or a terminal state.
func (cs *ClientStore) startLeaseBootstrap(clientID uint64) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r, ok := cs.recordLocked(InodeID(clientID), true)
	if !ok {
		return
	}
	r.mu.Lock()
	if r.state == leaseRecordLive {
		r.bootstrap = true
	}
	r.mu.Unlock()
	r.kickLocked()
}

// noteFleetExpired records a marker this process published or observed, so
// lease users see EXPIRED without waiting for the worker's evidence, and
// wakes the worker for the terminal steps.
func (cs *ClientStore) noteFleetExpired(clientID uint64) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r := cs.refreshers[InodeID(clientID)]
	if r == nil {
		return
	}
	r.mu.Lock()
	r.state = leaseRecordFleetExpired
	r.bootstrap = false
	r.mu.Unlock()
	r.kickLocked()
}

// bootstrapWait bounds how long CREATE_SESSION waits for the first
// publication before replying. A variable so tests can shorten it.
var bootstrapWait = 2 * time.Second

// awaitLeasePublished waits, up to wait, until the incarnation's record has
// a published lease in the future or turns terminal. CREATE_SESSION uses it
// so that a client's first SEQUENCE does not get DELAY while the bootstrap
// publication, which the worker writes, is still in flight. It reports
// whether the lease was published.
func (cs *ClientStore) awaitLeasePublished(clientID uint64, wait time.Duration) bool {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		cs.refreshMu.Lock()
		r := cs.refreshers[InodeID(clientID)]
		cs.refreshMu.Unlock()
		if r == nil {
			return false
		}
		r.mu.Lock()
		if r.state != leaseRecordLive {
			r.mu.Unlock()
			return false
		}
		if r.proven > cs.now().UnixNano() {
			r.mu.Unlock()
			return true
		}
		if r.raised == nil {
			r.raised = make(chan struct{})
		}
		raised := r.raised
		r.mu.Unlock()
		select {
		case <-raised:
		case <-deadline.C:
			return false
		}
	}
}

// pinLeaseForTest gives an incarnation a fixed live lease with no worker,
// for tests whose sessions have no durable client.
func (cs *ClientStore) pinLeaseForTest(clientID uint64) {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r, _ := cs.recordLocked(InodeID(clientID), true)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pinned = true
	r.proven = time.Now().Add(100 * 365 * 24 * time.Hour).UnixNano()
	r.verifiedAt = r.proven
	r.lastPass = time.Now()
}

// idleLocked reports that nothing references the record and nothing needs
// it. The caller holds cs.refreshMu and r.mu. A live record whose
// incarnation still has sessions here is referenced: SEQUENCE reads it. One
// whose lease is needed (an active request or a quiet period) is kept for
// its refresh. Both are read from the server, under sessionStore.mu, which
// is ordered after r.mu.
func (r *leaseRefresher) idleLocked() bool {
	return !r.pinned && r.refs == 0 && !r.bootstrap && r.attempt == nil &&
		len(r.orphans) == 0 && !r.verify && !r.inCall &&
		(r.state == leaseRecordLive && !r.sessionsLocked() || r.finished) &&
		!r.need()
}

// reclaimLocked frees an idle record with no worker, or has its worker
// free it: the worker, which may be waiting with a deadline a day away,
// owns the record until it exits. The caller holds cs.refreshMu and has
// found the record idle.
func (cs *ClientStore) reclaimLocked(r *leaseRefresher) {
	r.mu.Lock()
	running := r.running
	if running {
		r.reclaim = true
	}
	r.mu.Unlock()
	if !running {
		cs.freeLocked(r)
		return
	}
	r.kickLocked()
}

// sessionsLocked reports the server's sessions for the incarnation. With no
// server, the store's cached client state stands in for them.
func (r *leaseRefresher) sessionsLocked() bool {
	if r.cs.leaseSessions == nil {
		_, cached := r.cs.localClients.Load(r.incarnation)
		return cached
	}
	return r.cs.leaseSessions(uint64(r.incarnation))
}

// freeLocked forgets an idle record and returns its credit. The caller holds
// cs.refreshMu.
func (cs *ClientStore) freeLocked(r *leaseRefresher) {
	if cs.refreshers[r.incarnation] == r {
		delete(cs.refreshers, r.incarnation)
	}
}

func (r *leaseRefresher) need() bool {
	if r.cs.leaseNeed == nil {
		return false
	}
	return r.cs.leaseNeed(uint64(r.incarnation))
}

// run is the record's worker: passes until nothing references the record.
func (r *leaseRefresher) run() {
	for {
		wait, exit := r.safePass(false)
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
func (r *leaseRefresher) await(wait time.Duration) bool {
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
// verify no sooner than leaseVerifySpacing after the last pass, read-only
// before the maintenance deadline. Bootstrap, a need arriving while the
// worker was idle, or a terminal state not yet acted on makes maintenance
// due, no sooner than the failure deadline.
func (r *leaseRefresher) wakeDue() time.Time {
	need := r.need()
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	due := now.Add(leaseIdleWait)
	if r.verify {
		due = minTime(due, r.lastPass.Add(leaseVerifySpacing))
	}
	if r.bootstrap || r.idleWaiting && need || r.terminalLocked() {
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
func (r *leaseRefresher) terminalLocked() bool {
	return r.state != leaseRecordLive && !r.finished
}

// maintainSoonLocked makes maintenance due now, or at the failure deadline
// if one applies, and returns when. The caller holds r.mu.
func (r *leaseRefresher) maintainSoonLocked() time.Time {
	start := time.Now()
	if r.notBefore.After(start) {
		start = r.notBefore
	}
	r.maintainAt = minTime(r.maintainAt, start)
	return r.maintainAt
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// stopped reports stopLeaseWorkers; the worker then exits, leaving its
// record for the store's owner to discard with the store.
func (r *leaseRefresher) stopped() bool {
	r.cs.refreshMu.Lock()
	defer r.cs.refreshMu.Unlock()
	if !r.cs.leaseStopped {
		return false
	}
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
	return true
}

// stopLeaseWorkers ends every worker after its current pass and waits for
// them. No worker starts after it.
func (cs *ClientStore) stopLeaseWorkers() {
	cs.refreshMu.Lock()
	if !cs.leaseStopped {
		cs.leaseStopped = true
		close(cs.leaseStop)
	}
	cs.refreshMu.Unlock()
	cs.leaseWorkers.Wait()
}

func (r *leaseRefresher) bootstrapSet() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bootstrap
}

// safePass runs a pass; a panic is treated as a failed pass and backs off.
func (r *leaseRefresher) safePass(authorize bool) (wait time.Duration, exit bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.mu.Lock()
			r.inCall = false
			wait = r.nextBackoffLocked()
			r.notBefore = time.Now().Add(wait)
			r.maintainAt = r.notBefore
			r.mu.Unlock()
			exit = false
		}
	}()
	return r.pass(authorize)
}

// call runs one backend call with inCall set.
func (r *leaseRefresher) call(fn func()) {
	r.mu.Lock()
	r.inCall = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.inCall = false
		r.mu.Unlock()
	}()
	fn()
}

func (r *leaseRefresher) nextBackoffLocked() time.Duration {
	if r.backoff == 0 {
		r.backoff = leaseBackoffMin
	} else if r.backoff *= 2; r.backoff > leaseBackoffMax {
		r.backoff = leaseBackoffMax
	}
	return r.backoff
}

// pass is one worker cycle: settle, evidence, then refresh or the terminal
// steps (§9.2). authorize forces the refresh condition, for the store-only
// inline callers. It reports how long to wait, or exit when the record has
// been freed.
func (r *leaseRefresher) pass(authorize bool) (time.Duration, bool) {
	cs := r.cs
	unlock := cs.leaseLocks.lock(r.incarnation)
	defer unlock()
	if wait, exit, done := r.reclaimed(); done {
		return wait, exit
	}
	r.mu.Lock()
	now := time.Now()
	// Before the maintenance deadline a pass only reads the evidence. A
	// known terminal record's steps are due once any failure deadline has
	// passed.
	terminalDue := r.terminalLocked() && !now.Before(r.notBefore)
	readOnly := !authorize && now.Before(r.maintainAt) && !terminalDue
	r.verify = false
	r.lastPass = now
	r.mu.Unlock()
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
		var identity InodeID
		var valid bool
		var err error
		r.call(func() { identity, valid, err = cs.identityForIncarnation(r.incarnation) })
		switch {
		case err != nil:
			failed = true
		case !valid:
			r.mu.Lock()
			r.state = leaseRecordStale
			r.mu.Unlock()
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
	r.mu.Lock()
	state := r.state
	r.mu.Unlock()
	if state != leaseRecordLive {
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
func (r *leaseRefresher) settle() bool {
	cs, dir := r.cs, r.incarnation
	if a := r.attempt; a != nil {
		var err error
		r.call(func() { err = cs.fs.RemoveEdge(dir, a.orphan.temp, a.edge) })
		if err != nil && !isEdgeGone(err) {
			return false
		}
		r.consume(a.orphan)
		r.mu.Lock()
		r.attempt = nil
		r.mu.Unlock()
	}
	r.mu.Lock()
	orphans := append([]*leaseOrphan(nil), r.orphans...)
	r.mu.Unlock()
	for _, o := range orphans {
		if o.phase != orphanLinked && o.phase != orphanUnresolved || o.inode == 0 {
			continue
		}
		var edge Edge
		var err error
		r.call(func() { edge, err = cs.fs.LookupEdge(dir, o.temp) })
		if err != nil || edge.ID != o.inode {
			// Absent or unreadable: still an obligation.
			continue
		}
		r.call(func() { err = cs.fs.RemoveEdge(dir, o.temp, edge) })
		if err == nil || isEdgeGone(err) {
			r.consume(o)
		}
	}
	return true
}

// consume settles an orphan and returns its credits.
func (r *leaseRefresher) consume(o *leaseOrphan) {
	cs := r.cs
	r.mu.Lock()
	for i, have := range r.orphans {
		if have == o {
			r.orphans = append(r.orphans[:i], r.orphans[i+1:]...)
			r.mu.Unlock()
			cs.refreshMu.Lock()
			cs.orphans--
			cs.refreshMu.Unlock()
			return
		}
	}
	r.mu.Unlock()
}

// evidence reads, without writing: our lease's content, the expired marker
// and the confirmed pointer (§9.2 step 2). It also removes other processes'
// lease slots that expired at least a lease time ago.
func (r *leaseRefresher) evidence() {
	cs, dir := r.cs, r.incarnation
	start := cs.now().UnixNano()
	var lease durableLease
	var leaseErr error
	r.call(func() {
		var id InodeID
		if id, leaseErr = cs.fs.Lookup(dir, cs.leaseName); leaseErr == nil {
			lease, leaseErr = cs.readLease(id)
		}
	})
	var marked bool
	var markErr error
	r.call(func() { marked, markErr = cs.hasExpiredMarker(dir) })
	var target InodeID
	var found bool
	var ptrErr error
	r.call(func() { target, found, ptrErr = cs.readIdentitySymlink(r.identity, confirmedName) })
	r.mu.Lock()
	if leaseErr == nil && lease.ExpiresUnixNano > r.proven {
		r.proven = lease.ExpiresUnixNano
		r.raiseLocked()
	}
	if markErr == nil && marked {
		r.state = leaseRecordFleetExpired
	}
	if ptrErr == nil {
		if found && target == dir {
			r.verifiedAt = start
		} else if r.state == leaseRecordLive {
			r.state = leaseRecordStale
		}
	}
	r.mu.Unlock()
}

// pruneOthers removes other processes' lease slots that expired at least a
// lease time ago. It runs only in an authorized pass, while our own
// published lease is live, so the client is not expired and those slots are
// no longer evidence of anything; and only under a destruction hold, so
// never while a confirmation protects the incarnation. Ours is removed only
// by the terminal steps, after a marker. Verify passes do not call it.
func (r *leaseRefresher) pruneOthers() {
	cs, dir := r.cs, r.incarnation
	r.mu.Lock()
	live := r.state == leaseRecordLive && r.proven > cs.now().UnixNano()
	r.mu.Unlock()
	if !live {
		return
	}
	if cs.beginMarker != nil {
		end, ok := cs.beginMarker(dir)
		if !ok {
			return
		}
		defer end()
	}
	r.call(func() {
		if scan, err := cs.scanSlots(dir, leasePrefix); err == nil {
			var others []string
			for _, name := range scan.expiredNames {
				if name != cs.leaseName {
					others = append(others, name)
				}
			}
			_ = cs.removeExpiredSlots(dir, others)
		}
	})
}

// refresh publishes a new lease when ours is due (§9.2 step 3). It reports
// false on a failure that should back off.
func (r *leaseRefresher) refresh() bool {
	cs, dir := r.cs, r.incarnation
	now := cs.now()
	r.mu.Lock()
	due := r.proven < now.Add(nfsLeaseTime+nfsLeaseRenewAfter/2).UnixNano()
	r.mu.Unlock()
	if !due {
		return true
	}
	o, ok := r.takeOrphan()
	if !ok {
		// At a cap: refresh pauses until an obligation settles.
		return false
	}
	expiry := now.Add(leaseWriteTerm).UnixNano()
	if expiry <= r.lastExpiry {
		expiry = r.lastExpiry + 1
	}
	data, err := json.Marshal(durableLease{ExpiresUnixNano: expiry})
	if err != nil {
		r.consume(o)
		return false
	}
	var id InodeID
	var cookie Cookie
	r.call(func() { id, cookie, err = cs.fs.ConstructFile(dir) })
	if err != nil {
		// No LINK was sent for this never-reused name.
		r.consume(o)
		return false
	}
	r.mu.Lock()
	o.inode, o.phase = id, orphanLinking
	r.mu.Unlock()
	r.call(func() { err = cs.fs.LinkFile(id, cookie, dir, o.temp, bytes.NewReader(data)) })
	r.mu.Lock()
	if err != nil {
		// Its outcome is unknown and it is never retried.
		o.phase = orphanUnresolved
		r.mu.Unlock()
		return false
	}
	o.phase = orphanLinked
	r.mu.Unlock()
	var edge Edge
	r.call(func() { edge, err = cs.fs.LookupEdge(dir, o.temp) })
	if err != nil || edge.ID != id {
		return false
	}
	r.lastExpiry = expiry
	r.mu.Lock()
	r.attempt = &leaseAttempt{orphan: o, edge: edge, expiry: expiry}
	r.mu.Unlock()
	r.call(func() { err = cs.fs.RenameEdge(dir, o.temp, edge, dir, cs.leaseName) })
	if err != nil {
		// Unsettled: no later publication starts until it is fenced.
		return false
	}
	r.consume(o)
	r.mu.Lock()
	r.attempt = nil
	if expiry > r.proven {
		r.proven = expiry
	}
	r.bootstrap = false
	r.raiseLocked()
	r.mu.Unlock()
	return true
}

// takeOrphan takes an orphan credit and records a new temp, below the
// per-record and process caps.
func (r *leaseRefresher) takeOrphan() (*leaseOrphan, bool) {
	cs := r.cs
	suffix, err := randomHex(8)
	if err != nil {
		return nil, false
	}
	r.mu.Lock()
	full := len(r.orphans) >= maxRecordOrphans
	r.mu.Unlock()
	if full {
		return nil, false
	}
	cs.refreshMu.Lock()
	if cs.orphans >= cs.orphanLimit {
		cs.refreshMu.Unlock()
		return nil, false
	}
	cs.orphans++
	cs.refreshMu.Unlock()
	o := &leaseOrphan{temp: tempPrefix + suffix, phase: orphanConstructing}
	r.mu.Lock()
	r.orphans = append(r.orphans, o)
	r.mu.Unlock()
	return o, true
}

// terminate handles a stale or fleet-expired record once its attempt is
// settled: remove our lease through its edge, best effort, then retire the
// incarnation's sessions. It writes nothing more. It reports false when the
// retirement must be retried.
func (r *leaseRefresher) terminate() bool {
	cs, dir := r.cs, r.incarnation
	r.mu.Lock()
	r.bootstrap = false
	finished := r.finished
	r.raiseLocked()
	r.mu.Unlock()
	if finished {
		return true
	}
	// Our lease goes through its edge, best effort, under the same
	// destruction hold as the retirement: never while a confirmation
	// protects the incarnation.
	cleanup := func() {
		r.call(func() {
			if edge, err := cs.fs.LookupEdge(dir, cs.leaseName); err == nil {
				_ = cs.fs.RemoveEdge(dir, cs.leaseName, edge)
			}
		})
	}
	if cs.leaseTerminal == nil {
		cleanup()
	} else if !cs.leaseTerminal(uint64(dir), cleanup) {
		return false
	}
	cs.localClients.Delete(dir)
	r.mu.Lock()
	r.finished = true
	r.mu.Unlock()
	return true
}

// reclaimed handles a reclamation wake before anything else in a pass: the
// record is freed if it is still idle, and otherwise the worker goes back
// to its deadlines. Either way no store call is made, and no deadline
// moves. done is false when no reclamation was asked for.
func (r *leaseRefresher) reclaimed() (wait time.Duration, exit, done bool) {
	cs := r.cs
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.reclaim {
		return 0, false, false
	}
	r.reclaim = false
	if r.idleLocked() {
		r.running = false
		cs.freeLocked(r)
		return 0, true, true
	}
	return r.nextPassLocked(time.Until(r.maintainAt)), false, true
}

// verified ends a read-only pass, leaving the deadlines as they were, with
// one exception: a terminal state the evidence has just found makes the
// terminal steps due, once any failure deadline has passed; then done is
// false and the pass goes on to them. A record left idle is freed, which
// needs no store call.
func (r *leaseRefresher) verified() (wait time.Duration, exit, done bool) {
	cs := r.cs
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
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
	return r.nextPassLocked(time.Until(r.maintainAt)), false, true
}

// nextPassLocked is the wait before the next pass: the maintenance wait, or
// sooner for a pending verify. The caller holds r.mu.
func (r *leaseRefresher) nextPassLocked(maintain time.Duration) time.Duration {
	if r.verify {
		maintain = min(maintain, time.Until(r.lastPass.Add(leaseVerifySpacing)))
	}
	return max(maintain, 0)
}

// schedule decides the next maintenance pass, and the next pass, or frees
// the record.
func (r *leaseRefresher) schedule(failed bool) (time.Duration, bool) {
	cs := r.cs
	need := r.need()
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.idleWaiting = false
	if failed {
		wait := r.nextBackoffLocked()
		r.notBefore = time.Now().Add(wait)
		r.maintainAt = r.notBefore
		return wait, false
	}
	r.backoff, r.notBefore = 0, time.Time{}
	wait, exit := r.maintenanceLocked(need)
	if exit {
		return 0, true
	}
	r.maintainAt = time.Now().Add(wait)
	return r.nextPassLocked(wait), false
}

// maintenanceLocked is the wait before the next maintenance pass, or exit
// once the record is freed. The caller holds cs.refreshMu and r.mu.
func (r *leaseRefresher) maintenanceLocked(need bool) (time.Duration, bool) {
	cs := r.cs
	switch {
	case r.attempt != nil || len(r.orphans) > 0:
		// Obligations are checked again on every settle cycle.
		return leaseBackoffMax, false
	case r.state == leaseRecordLive && (need || r.bootstrap):
		wait := leaseRefreshInterval
		due := time.Duration(r.proven-cs.now().Add(nfsLeaseTime+nfsLeaseRenewAfter/2).UnixNano()) + time.Millisecond
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
	r.idleWaiting = true
	return leaseIdleWait, false
}

// leaseIdleWait bounds an idle worker's sleep; it is woken for any work.
const leaseIdleWait = 24 * time.Hour

// sweepLeaseRecords reclaims idle records, so a record nothing references
// or needs any more is freed on the sweeper's tick, by its worker if it has
// one. Other workers are only woken, which brings nothing forward that
// wakeDue does not allow.
func (cs *ClientStore) sweepLeaseRecords() {
	cs.refreshMu.Lock()
	defer cs.refreshMu.Unlock()
	for _, r := range cs.refreshers {
		r.mu.Lock()
		idle, running := r.idleLocked(), r.running
		r.mu.Unlock()
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
func (cs *ClientStore) refreshInline(clientID uint64) (leaseView, error) {
	cs.refreshMu.Lock()
	r, ok := cs.recordLocked(InodeID(clientID), true)
	cs.refreshMu.Unlock()
	if !ok {
		return leaseView{}, nfs4.Error(nfs4.NFS4ERR_DELAY)
	}
	if cs.serverDecides() {
		view, _ := cs.leaseView(clientID)
		if view.proven <= cs.now().UnixNano() {
			cs.requestLeaseVerify(clientID)
		}
		return view, nil
	}
	r.safePass(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	return leaseView{state: r.state, proven: r.proven, verifiedAt: r.verifiedAt}, nil
}

// leaseStatusError maps a record's view to the status a lease user sees.
func (cs *ClientStore) leaseStatusError(view leaseView) error {
	switch {
	case view.state == leaseRecordStale:
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	case view.state == leaseRecordFleetExpired:
		return nfs4.Error(nfs4.NFS4ERR_EXPIRED)
	case view.proven <= cs.now().UnixNano():
		return nfs4.Error(nfs4.NFS4ERR_DELAY)
	}
	return nil
}
