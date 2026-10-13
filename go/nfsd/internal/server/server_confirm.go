// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Confirmation of a pending incarnation N over its predecessor P, on the
// server side (canonical r3 §5.3, §5.4). The client store makes writes A, B
// and D and decides each result from evidence; the server acts on it once.
//
// While a confirmation exists, from just before its first write until its
// result has been applied, N and P are protected(): no path in this process
// marks, purges, expires, retires, reclaims or collects them. P is closing,
// so its SEQUENCE, BIND and CREATE_SESSION get DELAY and DESTROY_CLIENTID gets
// CLIENTID_BUSY. N's creation slot stays held for the sequence, so its retry
// gets DELAY or the settled outcome. A new EXCHANGE_ID for the identity gets
// DELAY. An unresolved result is retried by the same CREATE_SESSION sequence
// and by the lease sweeper; lease expiry never resolves it.

// creation holds one CREATE_SESSION's reservations: the incarnation's group
// hold, its session record and its creation slot for seq. They are released
// once, by finish, by the request or by the confirmation that took them over.
type creation struct {
	s          *Server
	c          *sessionClient
	seq        uint32
	id         SessionID
	fore, back channelAttrs
	endCreate  func()
	record     bool // the session-record reservation is held
	slot       bool // c.csBusy for seq belongs to this creation
	created    *session
	resume     bool // look for an unfinished replacement purge after commit
	// leaseRecord releases the refresher record reserved before the slot.
	leaseRecord func()
	// backGrant: CONN_BACK_CHAN was asked for on a transport that can carry
	// callbacks, so the result grants it.
	backGrant bool
	// The callback program and credential, for the session.
	cbProgram     uint32
	cbSec         callbackSec
	backRequested bool
}

// cacheLocked records the slot's outcome for seq and releases the slot.
func (cr *creation) cacheLocked(status uint32, reply createSessionResult) {
	c := cr.c
	c.csSeq, c.csCached, c.csStatus, c.csReply = cr.seq, true, status, reply
	c.csBusy, cr.slot = false, false
}

// commitLocked inserts the session and caches OK. It reports false, with
// nothing inserted, if the record was retired meanwhile.
func (cr *creation) commitLocked() (createSessionResult, bool) {
	ss, c := cr.s.sessions, cr.c
	// A retired record, or an incarnation this process has expired (its
	// wire record stays for cached replay), gets no new session.
	if g := ss.groups[c.incarnation]; c.retired || g != nil && g.lease == gateExpired {
		c.csBusy, cr.slot = false, false
		return createSessionResult{}, false
	}
	sess := &session{
		id:        cr.id,
		clientID:  c.incarnation,
		slots:     make([]sessionSlot, cr.fore.maxReqs),
		maxOps:    cr.fore.maxOps,
		maxReq:    cr.fore.maxReq,
		maxResp:   cr.fore.maxResp,
		maxCached: cr.fore.maxCached,

		cbProgram:     cr.cbProgram,
		cbSec:         cr.cbSec,
		backRequested: cr.backRequested,
		backMaxReq:    cr.back.maxReq,
		backMaxResp:   cr.back.maxResp,
		backMaxOps:    cr.back.maxOps,
		backMaxReqs:   cr.back.maxReqs,
	}
	sess.cb = newCBSender(cr.s, sess, cr.cbProgram, cr.cbSec)
	ss.insertSessionLocked(sess)
	cr.created = sess
	ss.confirmWireLocked(c)
	c.hasConfirm = false
	// Every successful CREATE_SESSION of a confirmed incarnation finishes a
	// replacement purge left by an earlier one, here or before a restart.
	cr.resume = true
	result := createSessionResult{id: cr.id, seq: cr.seq, foreAttrs: cr.fore, backAttr: cr.back, back: cr.backGrant}
	cr.cacheLocked(nfs4.NFS4_OK, result)
	return result, true
}

// finish runs when the request returns, with no lock held: the committed
// session's bootstrap, the resumed replacement purge, then the release of
// every reservation still held. An uncached slot is rolled back. The purge
// contains its own panics, and the release is deferred, so it runs
// regardless.
func (cr *creation) finish() {
	s := cr.s
	defer cr.release()
	if cr.created != nil {
		created := cr.created
		cr.created = nil
		// The first durable publication: until it, SEQUENCE gets DELAY
		// (canonical r3 §9.1). The worker writes it; the reply waits a
		// bounded time for it, so a client's first SEQUENCE normally finds
		// it published. Clients such as libnfs treat a DELAY there as a
		// mount failure.
		s.clients.StartLeaseBootstrap(created.clientID)
		s.clients.AwaitLeasePublished(created.clientID, clientstore.BootstrapWait)
	}
	if cr.resume {
		cr.resume = false
		s.resumeReplacementPurge(cr.c.incarnation)
	}
}

// release ends the creation's reservations, once each.
func (cr *creation) release() {
	ss := cr.s.sessions
	if cr.slot {
		ss.mu.Lock()
		cr.c.csBusy = false
		ss.mu.Unlock()
		cr.slot = false
	}
	if cr.record {
		ss.releaseSessionRecord()
		cr.record = false
	}
	if cr.endCreate != nil {
		cr.endCreate()
		cr.endCreate = nil
	}
	if cr.leaseRecord != nil {
		cr.leaseRecord()
		cr.leaseRecord = nil
	}
}

// An unfinished replacement purge is owned by durable evidence: N confirmed,
// and N's reboot symlink naming the predecessor
// (ClientStore.ResumeReplacementPurge). That evidence outlives wire records,
// sessions and restarts, so the obligation cannot be lost.
//
// The server keeps only a hint: purgePending names the incarnations whose
// purge is known to have failed, so that a warm EXCHANGE_ID, which otherwise
// makes no store call, retries it. It is bounded by purgeLimit; at the limit
// a new hint is dropped, which only defers the retry to that incarnation's
// next successful CREATE_SESSION. Every successful CREATE_SESSION reads the
// evidence whether or not a hint exists, so restart, a dropped hint and
// cold adoption after the record retired are all covered. A cold
// EXCHANGE_ID adoption stays read-only. A purge whose n or selected target
// is protected by a confirmation is deferred, like a failure: the hint and
// the evidence stay.

// notePurgePending records that n's replacement purge is unfinished.
func (s *Server) notePurgePending(n uint64) {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if _, ok := s.purgePending[n]; ok {
		return
	}
	if len(s.purgePending) >= s.purgeLimit {
		s.log.Warn("replacement purge hints full; the next CREATE_SESSION retries", "clientid", n)
		return
	}
	s.purgePending[n] = struct{}{}
}

func (s *Server) hasPurgePending(n uint64) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	_, ok := s.purgePending[n]
	return ok
}

// resumeReplacementPurge finishes n's unfinished replacement purge, if its
// durable evidence shows one. A failure or a panic keeps the hint for the
// next retry; it never affects the caller's reply, which is an OK committed
// or replayed before. The evidence itself is never changed by a failure.
func (s *Server) resumeReplacementPurge(n uint64) {
	done := false
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic resuming a replacement purge; it is retried later",
				"clientid", n, "panic", recovered)
		}
		if !done {
			s.notePurgePending(n)
		}
	}()
	if _, err := s.clients.ResumeReplacementPurge(n); errors.Is(err, clientstore.ErrPurgeProtected) {
		s.log.Info("replacement purge deferred by a confirmation", "clientid", n)
		return
	} else if err != nil {
		s.log.Warn("replacement purge failed", "clientid", n, "err", err)
		return
	}
	done = true
	s.confirmMu.Lock()
	delete(s.purgePending, n)
	s.confirmMu.Unlock()
}

// confirmation is one identity's confirmation in this process.
type confirmation struct {
	identity vfs.InodeID
	owner    string
	n, p     uint64 // p is zero without a predecessor
	cr       *creation
	attempt  *clientstore.ConfirmAttempt // nil until CommitConfirmation returns
	applying bool                        // under confirmMu: one caller acts at a time
	done     bool                        // under confirmMu: the result has been applied

	// Local application stages, owned by the applying caller. Each is
	// recorded once its step has returned, so a panic resumes at the first
	// unfinished step and never repeats a finished one that is not
	// idempotent. See applyConfirmation.
	pClosedEnded bool // P marked replaced, or reopened
	pDropped     bool // P's sessions, record and local open state dropped
	published    bool // N's session inserted and its outcome cached
	purged       bool // FinishReplacement done, or its retry recorded
	status       uint32
	reply        createSessionResult
}

// Destruction holds order destructive work against protection. Each
// destructive action on an incarnation (marking it expired, expiring or
// retiring its state or staging, or a resumed replacement purge, which holds
// one on n and one on its target) or on an identity (client GC) holds one for
// the whole action, from its first check through its last effect, backend
// waits included. A hold is refused once the incarnation or identity is
// protected. Registration installs protection first, which refuses every new
// hold, and then waits for the holds already admitted to end. So every
// destructive action either completes before protection is complete or does
// not start. Holds are counts under confirmMu, deleted at zero: as many as
// the destructive work in flight, and never held as a lock across backend
// calls. Each holder releases its hold when its action returns or unwinds,
// panics included. Ending the last hold on a key wakes a registration
// waiting for it (destroyEnded, on confirmMu).

// beginDestroy admits one destructive action on an incarnation. Zero, the
// unowned client, is never protected.
func (s *Server) beginDestroy(incarnation uint64) (func(), bool) {
	if incarnation == 0 {
		return func() {}, true
	}
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if _, ok := s.confirming[incarnation]; ok {
		return nil, false
	}
	s.destroying[incarnation]++
	return func() {
		s.confirmMu.Lock()
		defer s.confirmMu.Unlock()
		if s.destroying[incarnation]--; s.destroying[incarnation] == 0 {
			delete(s.destroying, incarnation)
			s.destroyEnded.Broadcast()
		}
	}, true
}

// beginDestroyIdentity admits client GC of an identity.
func (s *Server) beginDestroyIdentity(identity vfs.InodeID) (func(), bool) {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if _, ok := s.confirmingIdentity[identity]; ok {
		return nil, false
	}
	s.destroyingIdentity[identity]++
	return func() {
		s.confirmMu.Lock()
		defer s.confirmMu.Unlock()
		if s.destroyingIdentity[identity]--; s.destroyingIdentity[identity] == 0 {
			delete(s.destroyingIdentity, identity)
			s.destroyEnded.Broadcast()
		}
	}, true
}

// beginPurge admits a resumed replacement purge: it removes p's state and
// n's reboot edge, so it holds destruction holds on both. The client store
// calls it under the identity lock, after selecting p and before any
// removal; neither hold waits.
func (s *Server) beginPurge(n, p vfs.InodeID) (func(), bool) {
	endN, ok := s.beginDestroy(uint64(n))
	if !ok {
		return nil, false
	}
	endP, ok := s.beginDestroy(uint64(p))
	if !ok {
		endN()
		return nil, false
	}
	return func() {
		endP()
		endN()
	}, true
}

// protected reports whether incarnation is N or P of a confirmation.
func (s *Server) protected(incarnation uint64) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	_, ok := s.confirming[incarnation]
	return ok
}

func (s *Server) confirmingOwner(owner string) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	_, ok := s.confirmingOwners[owner]
	return ok
}

// registerConfirmation makes N and P protected before any write, atomically
// with N's record: a record retired meanwhile is refused, and once
// registered the record cannot be retired by case 4 or expiry. At most one
// confirmation exists per identity; the caller owns application.
//
// It is ordered with the identity's other work:
//   - the owner lock waits for an EXCHANGE_ID in flight to finish its durable
//     registration and wire publication; later ones see the owner
//     confirming once they hold the lock;
//   - the entry is installed first, so new destruction holds on N, P and the
//     identity are refused, and registration then waits for the holds
//     already admitted to end. No destructive action admitted earlier is
//     still running once protection is complete.
//
// The wait holds only the owner lock (and the caller's connection lock).
// No destruction-hold holder waits for either while it holds a hold: a warm
// EXCHANGE_ID's resumed purge takes its owner lock before its holds, and
// CREATE_SESSION's runs after its connection lock is released. So the wait
// is not part of a cycle through those locks.
// New holds are refused, so it cannot be starved. It ends when the admitted
// actions return or unwind and release their holds; how long that takes
// depends on their backend calls, lock waits and I/O, which have no time
// bound here, and there is no forced timeout or unprotection. Lock order:
// owner lock, sessions.mu, confirmMu.
func (s *Server) registerConfirmation(e *confirmation) bool {
	unlockOwner := s.sessions.ownerLocks.Lock(e.owner)
	defer unlockOwner()
	s.confirmMu.Lock()
	_, identityBusy := s.confirmingIdentity[e.identity]
	_, ownerBusy := s.confirmingOwners[e.owner]
	_, nBusy := s.confirming[e.n]
	_, pBusy := s.confirming[e.p]
	if identityBusy || ownerBusy || nBusy || e.p != 0 && pBusy {
		s.confirmMu.Unlock()
		return false
	}
	s.confirmingIdentity[e.identity] = e
	s.confirmingOwners[e.owner] = e
	s.confirming[e.n] = e
	if e.p != 0 {
		s.confirming[e.p] = e
	}
	e.applying = true
	for s.destroying[e.n] > 0 || e.p != 0 && s.destroying[e.p] > 0 ||
		s.destroyingIdentity[e.identity] > 0 {
		s.destroyEnded.Wait()
	}
	s.confirmMu.Unlock()

	s.sessions.mu.Lock()
	defer s.sessions.mu.Unlock()
	if e.cr.c.retired {
		s.confirmMu.Lock()
		s.removeConfirmationLocked(e)
		s.confirmMu.Unlock()
		return false
	}
	e.cr.c.confirming = e
	return true
}

func (s *Server) removeConfirmationLocked(e *confirmation) {
	e.done, e.applying = true, false
	delete(s.confirmingIdentity, e.identity)
	delete(s.confirmingOwners, e.owner)
	delete(s.confirming, e.n)
	if e.p != 0 {
		delete(s.confirming, e.p)
	}
}

// acquire gives the caller exclusive application of an unresolved e.
func (s *Server) acquireConfirmation(e *confirmation) bool {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	if e.applying || e.done || e.attempt == nil {
		return false
	}
	e.applying = true
	return true
}

// release ends exclusive application of a still unresolved e.
func (s *Server) releaseConfirmation(e *confirmation) {
	s.confirmMu.Lock()
	e.applying = false
	s.confirmMu.Unlock()
}

// unregisterConfirmation removes the protection once e's result is applied.
func (s *Server) unregisterConfirmation(e *confirmation) {
	s.sessions.mu.Lock()
	if e.cr.c.confirming == e {
		e.cr.c.confirming = nil
	}
	s.sessions.mu.Unlock()
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	s.removeConfirmationLocked(e)
}

// applyConfirmation acts on a settled result, by the caller that holds e's
// application. It returns the CREATE_SESSION status and, for OK, its reply.
// complete reports that the result is fully applied and e unregistered;
// e.cr's remaining reservations are then the caller's to finish, after any
// connection lock is released. Otherwise e stays registered with its
// creation, and its caller releases application for a later resume.
//
// Application is a sequence of local stages, each recorded when it returns.
// A panic is contained: e keeps its protection and its retained creation,
// and the next application, by a retry or the sweeper, resumes at the first
// unfinished stage. The result itself never changes once settled.
//
//   - Committed: (1) mark P replaced; (2) drop P's sessions and record and
//     expire its local open state; (3) publish N's session and cache OK;
//     (4) FinishReplacement; (5) unregister. Step 1 is done at most once.
//     Step 2 may be repeated after a panic inside it: for a committed
//     replacement, detaching sessions already gone and removing open
//     markers already removed are no-ops. Step 3 is one locked section and
//     at most once. Step 4 contains its own panic. A failed or interrupted
//     purge stays owned by its durable evidence (N/reboot) and is resumed by
//     ResumeReplacementPurge, so a resumed application never repeats it.
//     Once published, the reply stays OK whatever happens later.
//   - Rolled back: reopen P at most once, then unregister.
//   - Conservative: mark P replaced at most once, drop P's sessions and
//     record (idempotent), retire N's record (idempotent), then unregister.
//     No durable state of N or P is removed.
//   - Unresolved: nothing changes.
func (s *Server) applyConfirmation(e *confirmation, result clientstore.ConfirmResult) (status uint32, reply createSessionResult, complete bool) {
	ss, cr := s.sessions, e.cr
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic applying a client confirmation; it stays protected and resumes",
				"clientid", e.n, "panic", recovered)
			status, reply, complete = nfs4.NFS4ERR_DELAY, createSessionResult{}, false
			if e.published {
				status, reply = e.status, e.reply
			}
		}
	}()
	locked := func(f func()) {
		ss.mu.Lock()
		defer ss.mu.Unlock()
		f()
	}
	switch result {
	case clientstore.ConfirmCommitted:
		// P is replaced, then N's session is inserted (§5.3 step 8).
		if e.p != 0 && !e.pClosedEnded {
			ss.replaced(e.p)
			e.pClosedEnded = true
		}
		if e.p != 0 && !e.pDropped {
			ss.dropClient(e.p)
			s.expireClientState(e.p)
			e.pDropped = true
		}
		if !e.published {
			locked(func() {
				reply, inserted := cr.commitLocked()
				// This creation's own purge is FinishReplacement's, on
				// B's evidence; a failure there is resumed later.
				cr.resume = false
				e.status, e.reply = nfs4.NFS4_OK, reply
				if !inserted {
					e.status = nfs4.NFS4ERR_STALE_CLIENTID
				}
				e.published = true
			})
		}
		if !e.purged {
			s.finishReplacement(e)
			e.purged = true
		}
		s.unregisterConfirmation(e)
		s.scheduleClientGC(e.n)
		return e.status, e.reply, true
	case clientstore.ConfirmRolledBack:
		// D never applied and confirmed is still P through Sp.
		if e.p != 0 && !e.pClosedEnded {
			ss.reopen(e.p)
			e.pClosedEnded = true
		}
		s.unregisterConfirmation(e)
		return nfs4.NFS4ERR_DELAY, createSessionResult{}, true
	case clientstore.ConfirmConservative:
		// Neither committed nor proved unchanged: P is replaced in this
		// process only, keeping its durable state, and N's record ends. A
		// client whose P is still valid adopts it again.
		if e.p != 0 && !e.pClosedEnded {
			ss.replaced(e.p)
			e.pClosedEnded = true
		}
		if e.p != 0 {
			ss.dropClient(e.p)
		}
		locked(func() {
			cr.c.csBusy, cr.slot = false, false
			ss.retireWireLocked(cr.c)
		})
		s.unregisterConfirmation(e)
		return nfs4.NFS4ERR_STALE_CLIENTID, createSessionResult{}, true
	}
	// Unresolved: protection, P's closing and N's slot all stay.
	return nfs4.NFS4ERR_DELAY, createSessionResult{}, false
}

// finishReplacement purges P on B's evidence. A failure or a panic leaves
// the reboot symlink as the durable obligation and notes the hint for
// resumeReplacementPurge; it never changes the committed outcome.
func (s *Server) finishReplacement(e *confirmation) {
	failed := true
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic purging a replaced client", "clientid", e.p, "panic", recovered)
		}
	}()
	defer func() {
		if failed {
			s.notePurgePending(e.n)
		}
	}()
	if _, err := s.clients.FinishReplacement(e.attempt); err != nil {
		// The reboot symlink stays as evidence for the next
		// CREATE_SESSION or case-2 EXCHANGE_ID of N.
		s.log.Warn("replacement purge failed", "clientid", e.n, "err", err)
		return
	}
	failed = false
}

// confirmPending confirms a pending record (§5.3) for a creation that holds
// N's slot. It returns the status and, for OK, the reply. handedOver reports
// that an unresolved confirmation now owns cr.
func (s *Server) confirmPending(cr *creation, confirm [8]byte, principal, caller clientstore.RPCPrincipal) (status uint32, reply createSessionResult, handedOver bool) {
	ss, c := s.sessions, cr.c
	retire := func() {
		ss.mu.Lock()
		ss.retireWireLocked(c)
		ss.mu.Unlock()
	}
	target, err := s.clients.PrepareConfirmation(c.incarnation, confirm)
	if err != nil {
		status := clientstore.ErrToNFS(err)
		if status == nfs4.NFS4ERR_STALE_CLIENTID {
			retire()
		}
		return status, reply, false
	}
	// Close P before the principal check and before any write. Commit
	// revalidates P and Sp after this, under the identity lock.
	var p uint64
	if target.P != 0 && target.P != target.N {
		p = uint64(target.P)
		if !ss.closeForReplacement(p) {
			return nfs4.NFS4ERR_DELAY, reply, false
		}
	}
	if principal != caller {
		if p != 0 {
			ss.reopen(p)
		}
		ss.mu.Lock()
		cr.cacheLocked(nfs4.NFS4ERR_CLID_INUSE, createSessionResult{})
		ss.mu.Unlock()
		return nfs4.NFS4ERR_CLID_INUSE, reply, false
	}
	e := &confirmation{identity: target.Identity, owner: c.owner, n: c.incarnation, p: p, cr: cr}
	if !s.registerConfirmation(e) {
		if p != 0 {
			ss.reopen(p)
		}
		return nfs4.NFS4ERR_DELAY, reply, false
	}
	// From registration, e owns cr unless its result is fully applied, and
	// application is released on every return. A panic here leaves e
	// registered and interrupted, for a retry or the sweeper to resume.
	complete := false
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic confirming a client; it stays protected and resumes", "clientid", e.n, "panic", recovered)
			status, reply, handedOver = nfs4.NFS4ERR_DELAY, createSessionResult{}, true
		}
		if !complete {
			s.releaseConfirmation(e)
		}
	}()
	// Every outcome, including a check that failed or could not be read
	// before any write, comes back as an attempt classified by its
	// read-back (see CommitConfirmation). Only a proved unchanged P is
	// reopened; a read error keeps P closed and N and P protected.
	a := s.clients.CommitConfirmation(target, confirm)
	if a.Panicked != nil {
		s.log.Error("panic confirming a client", "clientid", e.n, "panic", a.Panicked)
	}
	if a.PreWrite != nil {
		s.log.Info("client confirmation made no write", "clientid", e.n, "reason", a.PreWrite)
	}
	s.confirmMu.Lock()
	e.attempt = a
	s.confirmMu.Unlock()
	status, reply, complete = s.applyConfirmation(e, a.Result())
	// Unresolved, or interrupted: e keeps cr for a later resume.
	return status, reply, !complete
}

// retryHeldConfirmation settles a held confirmation for a CREATE_SESSION
// retry of its sequence, through the creation the confirmation owns: the
// retry reserves no group hold or session record of its own. A committed
// result is published on this connection, so the association it needs is
// reserved first, as a token counted against the quota: with the connection
// full the retry gets RESOURCE and nothing is settled or applied. The token
// is committed only for an OK reply whose session exists, and aborted
// otherwise. An association the connection already has costs nothing.
func (s *Server) retryHeldConfirmation(e *confirmation, st *compoundState) (status uint32, reply createSessionResult, done func()) {
	token, ok := st.connection.reserve(s, e.cr.id)
	if !ok {
		return nfs4.NFS4ERR_RESOURCE, reply, nil
	}
	defer token.abort()
	// A granted backchannel goes to this connection too, reserved alike.
	var bt *backToken
	if e.cr.backGrant {
		if bt, ok = st.connection.reserveBack(s, e.cr.id); !ok {
			return nfs4.NFS4ERR_RESOURCE, reply, nil
		}
		defer bt.abort()
	}
	status, reply, done = s.retryConfirmation(e)
	// A committed creation whose record was retired meanwhile publishes no
	// session; it keeps no association either.
	if sess := s.sessions.lookup(reply.id); status == nfs4.NFS4_OK && sess != nil {
		token.commit()
		if reply.back {
			s.commitBackChannel(st, bt, sess)
		}
	}
	return status, reply, done
}

// retryConfirmation settles and applies a confirmation that is unresolved or
// whose application was interrupted, for a retry of its sequence or the
// sweeper. done, when not nil, finishes the original creation's
// reservations; the caller runs it after releasing any connection lock.
// Application is released on every return, panics included.
func (s *Server) retryConfirmation(e *confirmation) (status uint32, reply createSessionResult, done func()) {
	if !s.acquireConfirmation(e) {
		return nfs4.NFS4ERR_DELAY, reply, nil
	}
	complete := false
	defer func() {
		if !complete {
			s.releaseConfirmation(e)
		}
	}()
	result := s.clients.SettleConfirmation(e.attempt)
	status, reply, complete = s.applyConfirmation(e, result)
	if !complete {
		return status, reply, nil
	}
	return status, reply, e.cr.finish
}

// settleConfirmations retries every unresolved confirmation, from the lease
// sweeper. Lease time alone never resolves one.
func (s *Server) settleConfirmations() {
	s.confirmMu.Lock()
	pending := make([]*confirmation, 0, len(s.confirmingIdentity))
	for _, e := range s.confirmingIdentity {
		pending = append(pending, e)
	}
	s.confirmMu.Unlock()
	for _, e := range pending {
		if _, _, done := s.retryConfirmation(e); done != nil {
			done()
		}
	}
}
