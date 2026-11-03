// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
)

// The logical lease and its expiry (README.md#lease-gate). The lease is
// kept per incarnation in the group's gate, in memory only. A request's
// hold, or a quiet period still running, keeps it; a restart gives every
// confirmed record a fresh one.

// decideExpired reports whether the client is expired, deciding it when its
// lease has run out. A live record is first written expired, while the gate
// is checking, so SEQUENCE gets DELAY until the decision is durable; then
// the client's state in this process is retired and its record removed.
// An incarnation no record confirms is expired with no write. With
// expireState its opens are expired too; OPEN's recovery checks, which hold
// a pathname lock that expiry may need, leave them to the sweeper. A failed
// record write returns the gate to live; the sweeper tries again.
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
	case g.lease == gateChecking || g.leaseHeldLocked(ss.now()) || g.creating > 0 || s.inStartupLease():
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
		err := s.records.Expire(clientrecords.Incarnation(clientID))
		ss.mu.Lock()
		if err != nil && !errors.Is(err, clientrecords.ErrNotCurrent) {
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
	if err := s.removeRecord(clientID); err != nil {
		// Expired on disk: the next start, or a registration of the
		// identity, removes it.
		s.log.Warn("expired client record not removed", "incarnation", incarnationText(clientID), "err", err)
	}
	return true, nil
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
