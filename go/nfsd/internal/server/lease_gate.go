// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The logical lease and its expiry (README.md#lease-gate). Each process is its
// own server owner, so the RFC lease is this process's logical lease, kept
// per incarnation in the group's gate. Expiry needs both the lease-hold
// conditions (no hold, quiet period over) and the absence of
// unresolved-confirmation protection; the caller holds a destruction hold
// for the second. Neither stands in for the other.

// decideExpired is the only path that publishes an expired marker. It
// reports whether the client is expired. With expireState, the client's
// local state is then expired too; OPEN's recovery checks, which hold a
// pathname lock that expiry may need, leave that to the sweeper. The caller
// holds a destruction hold on clientID. A read error returns the gate to
// live with nothing published or expired; the sweeper tries again on its
// next tick.
func (s *Server) decideExpired(clientID uint64, expireState bool) (bool, error) {
	ss := s.sessions
	ss.mu.Lock()
	// A client with no group here gets a temporary entry while it runs.
	g, ok := ss.groupLocked(clientID)
	if !ok {
		ss.mu.Unlock()
		return false, nil
	}
	switch {
	case g.lease == gateExpired:
		ss.mu.Unlock()
		if expireState {
			s.expireLocally(clientID)
		}
		ss.mu.Lock()
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
		return true, nil
	case g.lease == gateChecking || g.leaseHeldLocked(ss.now()):
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
		return false, nil
	}
	g.lease, g.markerPhase, g.renewedDuringCheck = gateChecking, markerDeciding, false
	ss.mu.Unlock()
	live := func() {
		ss.mu.Lock()
		g.lease, g.markerPhase = gateLive, markerNone
		ss.releaseGroupLocked(g)
		ss.mu.Unlock()
	}

	obs, err := s.clients.ObserveLease(clientID)
	switch {
	case err != nil:
		live()
		return false, err
	case obs.Collected:
		s.clients.LocalClients.Delete(vfs.InodeID(clientID))
	case obs.Unconfirmed:
		// Replaced: nothing to mark; its durable state is purged.
		if err := s.clients.ForgetUnconfirmed(clientID); err != nil {
			live()
			return false, err
		}
	case obs.Marked:
		// Another process decided.
		s.clients.NoteFleetExpired(clientID)
	case obs.Live || !obs.Found:
		// No slot yet means a first OPEN may be registering its lease.
		live()
		return false, nil
	default:
		// Arbitration: a renewal during the observation, or a hold taken
		// since, wins (C13 adds callback renewal here).
		ss.mu.Lock()
		if g.renewedDuringCheck || g.leaseHeldLocked(ss.now()) {
			g.lease, g.markerPhase = gateLive, markerNone
			ss.releaseGroupLocked(g)
			ss.mu.Unlock()
			return false, nil
		}
		g.markerPhase = markerSubmitted
		ss.mu.Unlock()
		if err := s.clients.PublishExpiredMarker(clientID); err != nil {
			// The marker is uncertain; the refresher reports fleetExpired
			// if it exists. Expiry proceeds either way.
			s.log.Warn("publish expired marker", "clientid", clientID, "err", err)
		} else {
			s.clients.NoteFleetExpired(clientID)
		}
	}
	ss.mu.Lock()
	g.lease, g.markerPhase = gateExpired, markerNone
	ss.mu.Unlock()
	s.clients.LocalClients.Delete(vfs.InodeID(clientID))
	if expireState {
		s.expireLocally(clientID)
	}
	ss.mu.Lock()
	ss.releaseGroupLocked(g)
	ss.mu.Unlock()
	return true, nil
}

// expireLocally ends an expired client's local state: its wire record, its
// sessions (cancelling their callbacks and returning their credits) and its
// opens. It does not depend on the refresher or on the expired marker
// having been written, and claims nothing about fleet expiry. The caller
// holds a destruction hold.
func (s *Server) expireLocally(clientID uint64) {
	s.sessions.retireWireOf(clientID)
	s.retireIncarnationSessions(clientID)
	s.expireClientState(clientID)
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

// leaseTerminal retires an incarnation whose refresher found it stale (the
// identity confirmed another incarnation) or fleet-expired (an expired
// marker exists). It reports false while a confirmation protects it, to be
// retried. Committed CREATE_SESSION results are not rewritten.
func (s *Server) leaseTerminal(clientID uint64, cleanup func()) bool {
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return false
	}
	defer end()
	// The gate closes before the cleanup's store I/O, so no CREATE_SESSION
	// commits a new session while it runs (commitLocked).
	ss := s.sessions
	ss.mu.Lock()
	if g := ss.groups[clientID]; g != nil {
		g.lease = gateExpired
	}
	ss.mu.Unlock()
	cleanup()
	// Only the sessions: the wire record stays, so its committed
	// CREATE_SESSION results still replay (README.md#expiry-decision).
	s.retireIncarnationSessions(clientID)
	s.expireClientState(clientID)
	return true
}
