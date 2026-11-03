// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// Clients on the local client records (../clientrecords;
// README.md#clients-and-leases). A record holds a confirmed client's
// identity, principal, incarnation and boot verifier. Everything else about
// a client lives in this process: wire records, sessions, the logical lease
// and opens. A record update is a local write, fsync and rename with a
// known outcome, and the server acts on it only once it has returned.

const (
	// leaseTime is the NFSv4.1 lease this server grants.
	leaseTime = 90 * time.Second
	// leaseSweepInterval is how often the sweeper looks for clients whose
	// lease has run out.
	leaseSweepInterval = leaseTime / 2
	// maxSessionRecords bounds the session store's wire clientids and
	// sessions.
	maxSessionRecords = 4096
)

func (s *Server) now() time.Time { return (*s.clock.Load())() }

// SetClock replaces the server's clock. It is safe while connections and
// the sweeper read it.
func (s *Server) SetClock(now func() time.Time) { s.clock.Store(&now) }

// inStartupLease reports the first lease interval after start. Every
// confirmed record starts with a fresh logical lease, so no confirmed
// client expires in it; one that never comes back expires after it.
func (s *Server) inStartupLease() bool {
	return s.now().Before(s.startedAt.Add(leaseTime))
}

// clientStatus says whether an incarnation is a live client: its record
// confirms it (STALE_CLIENTID otherwise) and this process has not expired
// it (EXPIRED otherwise). An absent record counts as expired.
func (s *Server) clientStatus(clientID uint64) uint32 {
	if _, ok := s.records.Live(clientrecords.Incarnation(clientID)); !ok {
		return nfs4.NFS4ERR_STALE_CLIENTID
	}
	ss := s.sessions
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.expiredLocked(clientID) {
		return nfs4.NFS4ERR_EXPIRED
	}
	return nfs4.NFS4_OK
}

// clientLive reports a live client: what an open, or a recovered writer,
// needs to stay active.
func (s *Server) clientLive(clientID uint64) bool {
	return s.clientStatus(clientID) == nfs4.NFS4_OK
}

// recoveryKey is the recovery key of a confirmed incarnation's client boot,
// which its staged writers keep (clientrecords.Record.RecoveryKey).
func (s *Server) recoveryKey(clientID uint64) ([32]byte, uint32) {
	rec, ok := s.records.Live(clientrecords.Incarnation(clientID))
	if !ok {
		return [32]byte{}, nfs4.NFS4ERR_STALE_CLIENTID
	}
	return rec.RecoveryKey(), nfs4.NFS4_OK
}

// clientActive reports a live client whose lease has not run out. A
// lease that has run out is expired here, but not the client's opens: the
// caller may hold a pathname lock that expiring them needs, and the
// sweeper expires them instead.
func (s *Server) clientActive(clientID uint64) (bool, error) {
	if !s.clientLive(clientID) {
		return false, nil
	}
	expired, err := s.decideExpired(clientID, false)
	return !expired, err
}

// register decides an EXCHANGE_ID that no wire record of this process
// answers, from the identity's record (README.md#registration). A
// confirmed incarnation whose lease has run out is expired first. The same
// principal and boot verifier adopt a live confirmed incarnation, writing
// nothing; that is how a client whose session did not survive a restart
// keeps its incarnation and its writers. Another principal gets CLID_INUSE
// while the confirmed incarnation holds write state. Otherwise the client
// is new, or rebooted, or taking over the identity: incarnation 0 asks the
// caller for a new pending incarnation, which CREATE_SESSION confirms.
func (s *Server) register(owner []byte, verifier [8]byte, principal clientrecords.Principal) (incarnation uint64, adopted bool, status uint32) {
	rec, ok := s.records.Lookup(clientrecords.HashOwner(owner))
	if !ok || rec.Expired {
		return 0, false, nfs4.NFS4_OK
	}
	confirmed := uint64(rec.Incarnation)
	active, err := s.clientActive(confirmed)
	switch {
	case err != nil:
		return 0, false, statusOf(err)
	case !active:
	case rec.Principal != principal:
		if s.holdsWriteState(confirmed) {
			return 0, false, nfs4.NFS4ERR_CLID_INUSE
		}
	case rec.Verifier == verifier:
		return confirmed, true, nfs4.NFS4_OK
	}
	return 0, false, nfs4.NFS4_OK
}

// newIncarnationLocked chooses a pending incarnation for a new
// registration, one no group or record uses. The caller holds ss.mu.
func (s *Server) newIncarnationLocked() uint64 {
	for {
		id := clientrecords.NewIncarnation()
		if s.sessions.groups[uint64(id)] == nil && !s.records.InUse(id) {
			return uint64(id)
		}
	}
}

// confirmPending confirms a pending registration for a creation that holds
// its slot, and inserts the session (README.md#client-confirmation). The
// record goes first: it is durable before the reply, and before the
// incarnation it replaces is purged. A crash after it leaves the record
// purging, which the next start finishes. The owner lock orders the
// confirmation with the owner's registrations and destruction.
func (s *Server) confirmPending(cr *creation, caller clientrecords.Principal) (uint32, createSessionResult) {
	ss, c := s.sessions, cr.c
	if c.principal != caller {
		// A registration belongs to the principal that made it.
		ss.mu.Lock()
		cr.cacheLocked(nfs4.NFS4ERR_CLID_INUSE, createSessionResult{})
		ss.mu.Unlock()
		return nfs4.NFS4ERR_CLID_INUSE, createSessionResult{}
	}
	unlockOwner := ss.ownerLocks.Lock(c.owner)
	defer unlockOwner()
	ss.mu.Lock()
	retired := c.retired
	ss.mu.Unlock()
	if retired {
		// A later EXCHANGE_ID superseded the registration.
		return nfs4.NFS4ERR_STALE_CLIENTID, createSessionResult{}
	}
	owner := []byte(c.owner)
	id := clientrecords.Incarnation(c.incarnation)
	h := clientrecords.HashOwner(owner)
	old, exists := s.records.Lookup(h)
	if exists && old.Incarnation != id {
		// An earlier reboot's purge or an expiry still to finish, or a
		// lease that has run out.
		if status := s.settleRecord(old); status != nfs4.NFS4_OK {
			return status, createSessionResult{}
		}
		if _, err := s.decideExpired(uint64(old.Incarnation), true); err != nil {
			return statusOf(err), createSessionResult{}
		}
		old, exists = s.records.Lookup(h)
	}
	var p uint64
	if exists && old.Incarnation != id {
		// A reboot or a takeover: the confirmed incarnation is replaced.
		// New requests on its sessions wait from here.
		p = uint64(old.Incarnation)
		if !ss.closeForReplacement(p) {
			return nfs4.NFS4ERR_DELAY, createSessionResult{}
		}
		if old.Principal != c.principal && s.holdsWriteState(p) {
			// The principal check again: the incarnation may have
			// opened for writing since the registration. It comes after
			// admission closed, so no OPEN can succeed behind it.
			ss.reopen(p)
			ss.mu.Lock()
			cr.cacheLocked(nfs4.NFS4ERR_CLID_INUSE, createSessionResult{})
			ss.mu.Unlock()
			return nfs4.NFS4ERR_CLID_INUSE, createSessionResult{}
		}
	}
	if !exists || old.Incarnation != id {
		if _, err := s.records.Confirm(owner, c.verifier, c.principal, id); err != nil {
			if p != 0 {
				ss.reopen(p)
			}
			if errors.Is(err, clientrecords.ErrUnfinished) {
				// The sweeper expired P meanwhile; the retry finishes it.
				return nfs4.NFS4ERR_DELAY, createSessionResult{}
			}
			s.log.Error("client record not written; CREATE_SESSION fails",
				"incarnation", incarnationText(c.incarnation), "err", err)
			return nfs4.NFS4ERR_SERVERFAULT, createSessionResult{}
		}
	}
	if p != 0 {
		ss.replaced(p)
		s.purgeReplaced(p)
		if err := s.records.FinishPurge(id); err != nil {
			// The record stays purging. The next start, or the next
			// registration that replaces this incarnation, finishes it.
			s.log.Warn("client record still purging", "incarnation", incarnationText(c.incarnation), "err", err)
		}
	}
	ss.mu.Lock()
	reply, inserted := cr.commitLocked()
	ss.mu.Unlock()
	if !inserted {
		return nfs4.NFS4ERR_STALE_CLIENTID, createSessionResult{}
	}
	return nfs4.NFS4_OK, reply
}

// purgeReplaced ends, in this process, an incarnation a reboot or a
// takeover replaced: its sessions, its wire record and its opens, whose
// staged writers are retired. Staged writers recovered from an earlier
// process are retired by the sweeper. None of them resumes: no record
// confirms the incarnation any more.
func (s *Server) purgeReplaced(p uint64) {
	s.sessions.dropClient(p)
	s.expireClientState(p)
}

// retireClient ends an expired or destroyed incarnation's state in this
// process: its wire record, its sessions (cancelling their callbacks,
// returning their credits and removing their files) and, with withState,
// its opens. OPEN's recovery checks hold a pathname lock that retiring the
// opens' staging may need, and leave the opens to the sweeper.
func (s *Server) retireClient(clientID uint64, withState bool) {
	s.sessions.retireWireOf(clientID)
	s.retireIncarnationSessions(clientID)
	if withState {
		s.expireClientState(clientID)
	}
}

// removeRecord removes the record confirming clientID, once its sessions
// are retired. A record already gone, or confirming another incarnation,
// is not an error.
func (s *Server) removeRecord(clientID uint64) error {
	err := s.records.Remove(clientrecords.Incarnation(clientID))
	if errors.Is(err, clientrecords.ErrNotCurrent) {
		return nil
	}
	return err
}

// settleRecord finishes a transition a record shows unfinished: a reboot's
// purge, or an expiry whose record was not removed.
func (s *Server) settleRecord(rec clientrecords.Record) uint32 {
	if rec.Purging != 0 {
		s.purgeReplaced(uint64(rec.Purging))
		if err := s.records.FinishPurge(rec.Incarnation); err != nil {
			s.log.Warn("client record still purging", "incarnation", incarnationText(uint64(rec.Incarnation)), "err", err)
			return nfs4.NFS4ERR_DELAY
		}
	}
	if rec.Expired {
		s.retireClient(uint64(rec.Incarnation), true)
		if err := s.removeRecord(uint64(rec.Incarnation)); err != nil {
			s.log.Warn("expired client record not removed", "incarnation", incarnationText(uint64(rec.Incarnation)), "err", err)
			return nfs4.NFS4ERR_DELAY
		}
	}
	return nfs4.NFS4_OK
}

// settlePurge and settleExpiry are clientrecords.Finish's callbacks at
// startup, before anything is served: the incarnation's sessions were not
// restored, and the sweeper retires its staged writers.
func (s *Server) settlePurge(rec clientrecords.Record) error {
	s.purgeReplaced(uint64(rec.Purging))
	return nil
}

func (s *Server) settleExpiry(rec clientrecords.Record) error {
	s.retireClient(uint64(rec.Incarnation), true)
	return nil
}

// statusOf is the status of an error that carries one, and DELAY for any
// other: a backend failure the client retries.
func statusOf(err error) uint32 {
	var status nfs4.Error
	if errors.As(err, &status) {
		return uint32(status)
	}
	return nfs4.NFS4ERR_DELAY
}
