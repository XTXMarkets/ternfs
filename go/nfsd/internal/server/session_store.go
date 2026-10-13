// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

type SessionID [nfs4.NFS4_SESSIONID_SIZE]byte

type sessionSlot struct {
	mu       sync.Mutex
	seq      uint32
	inUse    bool
	cached   bool
	reply    []byte // COMPOUND4res body of the last request
	occupied bool   // a request has completed on this slot
	digest   [32]byte
	user     effectiveUser
	reserved int
	buffer   *cachedReply
}

type session struct {
	id        SessionID
	clientID  uint64
	slots     []sessionSlot
	maxOps    uint32
	maxReq    uint32
	maxResp   uint32
	maxCached uint32
	// The callback program and credential CREATE_SESSION gave, kept for
	// the backchannel; backRequested is CONN_BACK_CHAN in its flags.
	cbProgram     uint32
	cbSec         callbackSec
	backRequested bool
	// cb is the backchannel sender; backMax* are the negotiated back
	// channel limits. recalled: the session was asked to use half its
	// slots while the reply cache is under pressure.
	cb                                               *cbSender
	backMaxReq, backMaxResp, backMaxOps, backMaxReqs uint32
	recalled                                         atomic.Bool

	destroyed atomic.Bool
	fatal     atomic.Bool

	// users counts store membership plus admitted slot requests, under
	// sessionStore.mu. The record stays counted until the last one ends.
	users int
}

// sessionClient is the wire record of one EXCHANGE_ID clientid. clientID is
// the volatile wire clientid; incarnation is the durable client it is bound
// to, which owns sessions and state. A process has at most one live wire
// record per incarnation.
type sessionClient struct {
	clientID    uint64
	incarnation uint64
	owner       string
	verifier    [8]byte
	principal   clientstore.RPCPrincipal
	createdAt   time.Time
	// confirmed is set by the first successful CREATE_SESSION. An adopted
	// record names an incarnation that was already durably confirmed.
	confirmed, adopted bool
	retired            bool

	// confirm is the client-store confirmation verifier still to be
	// applied by the first CREATE_SESSION.
	confirm    [8]byte
	hasConfirm bool

	// The creation slot (RFC 8881 section 18.36.4), under sessionStore.mu.
	// csSeq is the sequence of the cached outcome. It starts at 0, so the
	// first CREATE_SESSION is 1, and wraps. When csCached, the outcome is
	// csReply if csStatus is NFS4_OK, else the error csStatus. While csBusy,
	// the request for csBusySeq owns the slot; it runs without the mutex.
	csSeq     uint32
	csCached  bool
	csReply   createSessionResult
	csStatus  uint32
	csBusy    bool
	csBusySeq uint32

	// confirming is the confirmation holding the slot for csBusySeq.
	confirming *confirmation
}

type createSessionResult struct {
	id                  SessionID
	seq                 uint32
	foreAttrs, backAttr channelAttrs
	back                bool // CONN_BACK_CHAN granted
}

type channelAttrs struct {
	headerPad, maxReq, maxResp, maxCached, maxOps, maxReqs uint32
}

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
	records, pendingSessions          int
	recordLimit                       int
	wireIDs                           wireClientIDs
	now                               func() time.Time

	// ownerLocks orders each owner's registrations and destructions: the
	// durable registration and the wire publication happen together. Taken
	// before the client store's identity lock and before mu.
	ownerLocks keyedlock.Locker[string]
}

func newSessionStore(epoch uint32) *sessionStore {
	return &sessionStore{
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
