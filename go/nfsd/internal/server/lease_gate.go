// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import "github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"

// The logical lease and its expiry (README.md#lease-gate). The lease is
// kept per incarnation in the group's gate, in memory only. A request's
// hold, or a quiet period still running, keeps it; a restart gives every
// confirmed record a fresh one.

// decideExpired reports whether the client is expired, deciding it when its
// lease has run out. A live record is first removed, while the gate is
// checking, so SEQUENCE and CREATE_SESSION get DELAY until the decision is
// durable; then the client's state in this process is retired. An
// incarnation no record confirms is expired with no write. With
// expireState its opens are expired too; OPEN's recovery checks, which hold
// a pathname lock that expiry may need, leave them to the sweeper. A failed
// removal returns the gate to live; the sweeper tries again.
func (s *Server) decideExpired(clientID uint64, expireState bool) (bool, error) {
	ss := s.sessions
	ss.mu.Lock()
	// A client with no group here gets a temporary entry while it runs.
	g, ok := ss.groupLocked(clientID)
	if !ok {
		ss.mu.Unlock()
		return false, nil
	}
	release := func() {
		ss.mu.Lock()
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
	}
	switch {
	case g.lease == gateExpired:
	case g.lease == gateChecking || g.leaseHeldLocked(ss.now()) || g.creating > 0 || s.startupLeaseHolds(clientID):
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
		return false, nil
	case g.wire != nil && g.wire.pending():
		// A pending registration: unused ones are reclaimed after a lease
		// (reclaimUnconfirmedLocked).
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
		return false, nil
	default:
		g.lease = gateChecking
		ss.mu.Unlock()
		err := s.removeRecord(clientID)
		ss.mu.Lock()
		if err != nil {
			g.lease = gateLive
			ss.releaseGroupLocked(g)
			ss.mu.Unlock()
			return false, err
		}
		g.lease = gateExpired
	}
	ss.mu.Unlock()
	defer release()
	s.retireClient(clientID, expireState)
	return true, nil
}

// startupLeaseHolds reports the fresh lease a restart gives a confirmed
// record. It protects only an incarnation a record confirms: one with no
// record is expired at once, so the same client boot can recover its
// retained writers by recovery key straight after a restart.
func (s *Server) startupLeaseHolds(clientID uint64) bool {
	if !s.inStartupLease() {
		return false
	}
	_, live := s.records.Live(clientrecords.Incarnation(clientID))
	return live
}

// retireIncarnationSessions detaches the incarnation's sessions under the
// store lock and drains them outside it.
func (s *Server) retireIncarnationSessions(clientID uint64) {
	ss := s.sessions
	ss.mu.Lock()
	var retired []*session
	for _, sess := range ss.sessions {
		if sess.clientID == clientID {
			sess.destroyed.Store(true)
			if ss.detachSessionLocked(sess) {
				retired = append(retired, sess)
			}
		}
	}
	ss.mu.Unlock()
	ss.drainRetired(retired)
}
