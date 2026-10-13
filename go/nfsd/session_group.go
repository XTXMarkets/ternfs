// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import "time"

// groupState is a client incarnation's admission state while a newly
// confirmed incarnation replaces it.
type groupState uint8

const (
	groupOpen     groupState = iota
	groupClosing             // replacement in progress: new requests wait
	groupReplaced            // replaced: its sessions are gone for good
)

// clientGroup is this process's membership of one durable client
// incarnation: its sessions and the requests running for it. A group lives, under sessionStore.mu, while a record, a session,
// a request or a CREATE_SESSION still refers to it, so a detached session
// that is still running keeps its incarnation's accounting.
//
// A group is fixed-size. Its members are the attached sessions in
// sessionStore.sessions whose clientID is the incarnation; the group only
// counts them. Each group holds one group credit from creation until it is
// forgotten, however long a detached request keeps it.
type clientGroup struct {
	incarnation uint64
	wire        *sessionClient
	members     int // attached sessions

	// Request holds. SEQUENCE takes a provisional hold before it can fail.
	// An admitted request, a replay included, holds an active one until its
	// reply is written or its connection fails. These are independent of
	// session.users, which also counts store membership and ends before the
	// reply is sent.
	provisional, active int
	// creating counts CREATE_SESSIONs in progress for this incarnation.
	creating int
	state    groupState

	// The logical lease gate (canonical r3 §8). quietUntil is extended when
	// an active hold ends; while it is in the future, or any hold exists,
	// this process never expires the client. lease is checking while
	// decideExpired observes, and expired once it has decided.
	quietUntil         time.Time
	lease              leaseGateState
	markerPhase        markerPhase
	renewedDuringCheck bool
	// dropped: the incarnation's sessions and wire record were dropped
	// (replaced, destroyed or terminal), so it has no logical lease left to
	// protect. A new session clears it.
	dropped bool

	reclaimComplete bool
}

type leaseGateState uint8

const (
	gateLive leaseGateState = iota
	gateChecking
	gateExpired
)

type markerPhase uint8

const (
	markerNone markerPhase = iota
	markerDeciding
	markerSubmitted
)

// busy reports work that replacement must not overtake.
func (g *clientGroup) busy() bool {
	return g.provisional+g.active+g.creating > 0
}

// expiredLocked reports that this process has expired the incarnation's
// logical lease. The caller holds ss.mu.
func (ss *sessionStore) expiredLocked(incarnation uint64) bool {
	g := ss.groups[incarnation]
	return g != nil && g.lease == gateExpired
}

// leaseHeldLocked reports the lease-hold conditions that forbid expiry:
// any hold, or a quiet period still running.
func (g *clientGroup) leaseHeldLocked(now time.Time) bool {
	return g.provisional > 0 || g.active > 0 || now.Before(g.quietUntil)
}

// groupLocked returns an incarnation's group, creating it with a group
// credit. It fails, before any effect, when every credit is held.
func (ss *sessionStore) groupLocked(incarnation uint64) (*clientGroup, bool) {
	if g := ss.groups[incarnation]; g != nil {
		return g, true
	}
	if ss.groupCredits >= ss.groupLimit {
		return nil, false
	}
	ss.groupCredits++
	g := &clientGroup{incarnation: incarnation}
	ss.groups[incarnation] = g
	return g, true
}

// groupAvailableLocked reports whether groupLocked would succeed.
func (ss *sessionStore) groupAvailableLocked(incarnation uint64) bool {
	return ss.groups[incarnation] != nil || ss.groupCredits < ss.groupLimit
}

// releaseGroupLocked forgets a group nothing refers to any more and returns
// its credit. A closing group is kept until its replacement ends, and a
// group whose quiet period is running, or whose expiry is being decided,
// keeps its gate; the sweeper forgets it afterwards (pruneGroupsLocked).
func (ss *sessionStore) releaseGroupLocked(g *clientGroup) {
	if g.wire == nil && g.members == 0 && !g.busy() &&
		g.state != groupClosing && g.lease != gateChecking &&
		(g.dropped || !ss.now().Before(g.quietUntil)) && ss.groups[g.incarnation] == g {
		delete(ss.groups, g.incarnation)
		ss.groupCredits--
	}
}

// pruneGroupsLocked forgets groups kept only by a quiet period that ended.
func (ss *sessionStore) pruneGroupsLocked() {
	for _, g := range ss.groups {
		ss.releaseGroupLocked(g)
	}
}

// hasSessions reports whether the incarnation has attached sessions here.
func (ss *sessionStore) hasSessions(incarnation uint64) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[incarnation]
	return g != nil && g.members > 0
}

// leaseNeed is the refresher's need condition for an incarnation: an active
// request, or a quiet period still running (canonical r3 §9.1).
func (ss *sessionStore) leaseNeed(incarnation uint64) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[incarnation]
	return g != nil && g.lease == gateLive && (g.active > 0 || ss.now().Before(g.quietUntil))
}

// admissionStatusLocked is the status for new work on an incarnation's
// existing sessions while it is being replaced.
func admissionStatusLocked(g *clientGroup) uint32 {
	switch g.state {
	case groupClosing:
		return NFS4ERR_DELAY
	case groupReplaced:
		return NFS4ERR_BADSESSION
	}
	return NFS4_OK
}

func (ss *sessionStore) admissionStatus(incarnation uint64) uint32 {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if g := ss.groups[incarnation]; g != nil {
		return admissionStatusLocked(g)
	}
	return NFS4_OK
}

// requestHold is one request's hold on its incarnation group.
type requestHold struct {
	store  *sessionStore
	group  *clientGroup
	active bool
}

// holdRequest takes a provisional hold for a SEQUENCE on sess. The closing
// and replaced checks, and the lease gate, share the lock with the hold, so
// replacement and expiry either see the hold or the request sees the new
// state: DELAY while expiry is being decided, BADSESSION once it was.
func (ss *sessionStore) holdRequest(sess *session) (*requestHold, uint32) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[sess.clientID]
	if g == nil || ss.sessions[sess.id] != sess {
		return nil, NFS4ERR_BADSESSION
	}
	if status := admissionStatusLocked(g); status != NFS4_OK {
		return nil, status
	}
	switch g.lease {
	case gateChecking:
		return nil, NFS4ERR_DELAY
	case gateExpired:
		return nil, NFS4ERR_BADSESSION
	}
	g.provisional++
	return &requestHold{store: ss, group: g}, NFS4_OK
}

// gateStatus is the gate's status for an admission holding a provisional
// hold, read again at the renewal check.
func (h *requestHold) gateStatus() uint32 {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	switch h.group.lease {
	case gateChecking:
		return NFS4ERR_DELAY
	case gateExpired:
		return NFS4ERR_BADSESSION
	}
	return NFS4_OK
}

// activate turns an admitted request's provisional hold into an active one.
func (h *requestHold) activate() {
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	h.group.provisional--
	h.group.active++
	h.active = true
}

// release ends the hold, once. A nil hold is a no-op. Ending an active
// hold, after the reply is written or the connection failed, renews the
// logical lease whatever the durable refresh did; a failed admission's
// provisional hold renews nothing.
func (h *requestHold) release() {
	if h == nil || h.group == nil {
		return
	}
	ss := h.store
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if h.active {
		h.group.active--
		if quiet := ss.now().Add(nfsLeaseTime); !h.group.dropped && quiet.After(h.group.quietUntil) {
			h.group.quietUntil = quiet
		}
	} else {
		h.group.provisional--
	}
	if h.group.active < 0 || h.group.provisional < 0 {
		panic("request hold underflow")
	}
	ss.releaseGroupLocked(h.group)
	h.group = nil
}

// beginCreate counts a CREATE_SESSION in progress for an incarnation, so
// replacement cannot invalidate the incarnation while it may still insert a
// session. The caller calls the returned function when it is done.
func (ss *sessionStore) beginCreate(incarnation uint64) (func(), uint32) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g, ok := ss.groupLocked(incarnation)
	if !ok {
		return nil, NFS4ERR_DELAY
	}
	switch g.state {
	case groupClosing:
		ss.releaseGroupLocked(g)
		return nil, NFS4ERR_DELAY
	case groupReplaced:
		ss.releaseGroupLocked(g)
		return nil, NFS4ERR_STALE_CLIENTID
	}
	g.creating++
	return func() {
		ss.mu.Lock()
		defer ss.mu.Unlock()
		g.creating--
		ss.releaseGroupLocked(g)
	}, NFS4_OK
}

// closeForReplacement starts replacing an incarnation. It fails while any
// request or CREATE_SESSION is in progress for it, or when no group credit
// is left; otherwise new requests wait until reopen or replaced.
func (ss *sessionStore) closeForReplacement(incarnation uint64) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g, ok := ss.groupLocked(incarnation)
	if !ok {
		return false
	}
	if g.busy() || g.state != groupOpen {
		ss.releaseGroupLocked(g)
		return false
	}
	g.state = groupClosing
	return true
}

// reopen ends a replacement that did not happen.
func (ss *sessionStore) reopen(incarnation uint64) {
	ss.endReplacement(incarnation, groupOpen)
}

// replaced records that an incarnation was replaced. Its sessions are
// dropped separately.
func (ss *sessionStore) replaced(incarnation uint64) {
	ss.endReplacement(incarnation, groupReplaced)
}

func (ss *sessionStore) endReplacement(incarnation uint64, state groupState) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[incarnation]
	if g == nil || g.state != groupClosing {
		panic("replacement ended without closing")
	}
	g.state = state
	ss.releaseGroupLocked(g)
}
