// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

type SessionID [nfs4.NFS4_SESSIONID_SIZE]byte

type sessionStore struct {
	mu                                sync.Mutex
	epoch                             uint32
	next                              uint64
	sessions                          map[SessionID]*session
	clients                           map[uint64]*sessionClient // by wire clientid
	owners                            map[string]*wireOwner
	wireLimit                         int
	groups                            map[uint64]*clientGroup // by incarnation
	groupCredits, groupLimit          int
	cacheMu                           sync.Mutex
	cachedBytes                       int
	cachedBuffers                     int
	cacheLimit                        int
	workBytes, workReplies, workLimit int
	// workFreed is signalled under cacheMu when reply work is released or a
	// connection closes, for requests waiting in acquireReplyWork.
	workFreed                *sync.Cond
	records, pendingSessions int
	recordLimit              int
	wireIDs                  wireClientIDs
	now                      func() time.Time

	// ownerLocks orders each owner's registrations and destructions: the
	// durable registration and the wire publication happen together. Taken
	// before the client store's identity lock and before mu.
	ownerLocks keyedlock.Locker[string]
}

func newSessionStore(epoch uint32) *sessionStore {
	ss := &sessionStore{
		epoch:       epoch,
		sessions:    make(map[SessionID]*session),
		clients:     make(map[uint64]*sessionClient),
		owners:      make(map[string]*wireOwner),
		wireLimit:   clientstore.MaxSessionRecords,
		groups:      make(map[uint64]*clientGroup),
		groupLimit:  maxClientGroups,
		cacheLimit:  maxSessionCacheBytes,
		workLimit:   maxReplyWorkBytes,
		recordLimit: clientstore.MaxSessionRecords,
		now:         time.Now,
	}
	ss.workFreed = sync.NewCond(&ss.cacheMu)
	return ss
}

// wakeWorkWaiters lets requests waiting for reply work re-check their
// connection, which has closed.
func (ss *sessionStore) wakeWorkWaiters() {
	ss.cacheMu.Lock()
	ss.workFreed.Broadcast()
	ss.cacheMu.Unlock()
}

// A session ID is the issuing process's epoch, the incarnation and a
// counter. A session never outlives its process, so an ID from another
// epoch is unknown here.
func (ss *sessionStore) newSessionIDLocked(clientID uint64) SessionID {
	ss.next++
	var id SessionID
	binary.BigEndian.PutUint32(id[0:4], ss.epoch)
	binary.BigEndian.PutUint64(id[4:12], clientID)
	binary.BigEndian.PutUint32(id[12:16], uint32(ss.next))
	return id
}

func (ss *sessionStore) lookup(id SessionID) *session {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.sessions[id]
}

// dropClient forgets an incarnation's EXCHANGE_ID record and its sessions.
// Every detached session keeps its record counted until its own slots are
// drained, and its running requests keep the group.
func (ss *sessionStore) dropClient(clientID uint64) {
	ss.mu.Lock()
	retired := ss.dropClientLocked(clientID)
	ss.mu.Unlock()
	ss.drainRetired(retired)
}

// destroyClientRecord is DESTROY_CLIENTID's final step. Under the same lock
// as the retirement it checks that the record is still its incarnation's
// live record (else STALE_CLIENTID), and that no CREATE_SESSION owns its
// creation slot and no session belongs to the incarnation (else
// CLIENTID_BUSY). A creator that starts or inserts a session after the
// caller's earlier checks is therefore never overtaken; one that has not yet
// taken the slot later finds the record retired.
func (ss *sessionStore) destroyClientRecord(c *sessionClient) uint32 {
	ss.mu.Lock()
	if c.retired {
		ss.mu.Unlock()
		return nfs4.NFS4ERR_STALE_CLIENTID
	}
	// A closing incarnation is the predecessor of a confirmation in progress.
	if g := ss.groups[c.incarnation]; c.csBusy || g != nil && (g.members > 0 || g.state == groupClosing) {
		ss.mu.Unlock()
		return nfs4.NFS4ERR_CLIENTID_BUSY
	}
	retired := ss.dropClientLocked(c.incarnation)
	ss.mu.Unlock()
	ss.drainRetired(retired)
	return nfs4.NFS4_OK
}

// dropClientLocked detaches an incarnation's sessions and retires its wire
// record. The caller drains the returned sessions without ss.mu.
func (ss *sessionStore) dropClientLocked(clientID uint64) []*session {
	g := ss.groups[clientID]
	if g == nil {
		return nil
	}
	g.dropped, g.quietUntil = true, time.Time{}
	var retired []*session
	for _, sess := range ss.sessions {
		if g.members == 0 {
			break
		}
		if sess.clientID != clientID {
			continue
		}
		sess.destroyed.Store(true)
		if ss.detachSessionLocked(sess) {
			retired = append(retired, sess)
		}
	}
	if g.wire != nil {
		ss.retireWireLocked(g.wire)
	}
	ss.releaseGroupLocked(g)
	return retired
}

func (ss *sessionStore) drainRetired(retired []*session) {
	for i, sess := range retired {
		retired[i] = nil // The batch no longer references a released record.
		ss.drainSession(sess)
		ss.releaseSessionUse(sess)
	}
}

func (ss *sessionStore) destroySession(id SessionID) bool {
	ss.mu.Lock()
	sess := ss.sessions[id]
	ss.mu.Unlock()
	if sess == nil {
		return false
	}
	ss.retireSession(sess, false)
	return true
}

func verifierBytes(verifier nfs4.Verifier4) [8]byte {
	var data [8]byte
	for i := range data {
		data[i] = verifier.Data(i)
	}
	return data
}

func readSessionID(m nfs4.Sessionid4) SessionID {
	var id SessionID
	for i := range id {
		id[i] = m.Data(i)
	}
	return id
}

func writeSessionID(m nfs4.Sessionid4, id SessionID) {
	for i, b := range id {
		m.SetData(i, b)
	}
}
