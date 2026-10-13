// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"unsafe"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

const (
	maxReplyWorkBytes   = 512 << 20
	cachedReplyOverhead = 128
	maxSessionRecords   = 4096
	// A group is referenced by a wire record, a session, or a request or
	// CREATE_SESSION in progress. The first two are bounded by their own
	// caps; the credit bounds groups that only running work keeps.
	maxClientGroups = 2 * maxSessionRecords
)

// A cachedReply owns one allocation or a pre-effect capacity reservation.
// Slot/replay references, including retired generations, share its charge.
// All fields are protected by sessionStore.cacheMu; slot data also uses slot.mu.
type cachedReply struct {
	store  *sessionStore
	refs   int
	charge int
	data   []byte
}

func (b *cachedReply) retain() {
	b.store.cacheMu.Lock()
	defer b.store.cacheMu.Unlock()
	if b.refs == 0 {
		panic("retain released cached reply")
	}
	b.refs++
}
func (b *cachedReply) releaseLocked() {
	b.refs--
	if b.refs < 0 {
		panic("cached reply reference underflow")
	}
	if b.refs == 0 {
		b.data = nil
		b.store.cachedBytes -= b.charge
		b.store.cachedBuffers--
		b.charge = 0
	}
}
func (b *cachedReply) release() {
	if b != nil {
		b.store.cacheMu.Lock()
		b.releaseLocked()
		b.store.cacheMu.Unlock()
	}
}
func (b *cachedReply) storeBody(data []byte) {
	// The reservation precedes effects. Allocate exactly, with no append growth.
	if len(data)+cachedReplyOverhead > b.charge {
		panic("cached reply exceeds reserved capacity")
	}
	body := make([]byte, len(data))
	copy(body, data)
	b.store.cacheMu.Lock()
	defer b.store.cacheMu.Unlock()
	b.data = body
	actual := cap(body) + cachedReplyOverhead
	b.store.cachedBytes -= b.charge - actual
	b.charge = actual
}

// reserveSlotReply charges a new reply buffer for an admission without
// touching the slot: the old buffer stays attached, charged and replayable
// until installSlotReply at commit; undo releases only the new charge. The
// capacity check credits the old buffer only when nothing else holds it, so
// the total exceeds cacheLimit by at most one per-slot credit (canonical r3
// §7.1).
func (ss *sessionStore) reserveSlotReply(sess *session, slot *sessionSlot, bytes int) (*cachedReply, bool) {
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	if sess.destroyed.Load() || sess.fatal.Load() {
		return nil, false
	}
	old := slot.buffer
	credit := 0
	if old != nil && old.refs == 1 {
		credit = old.charge
	}
	charge := bytes + cachedReplyOverhead
	if charge < cachedReplyOverhead || ss.cachedBytes-credit+charge > ss.cacheLimit {
		return nil, false
	}
	ss.cachedBytes += charge
	ss.cachedBuffers++
	return &cachedReply{store: ss, refs: 1, charge: charge}, true
}

// installSlotReply swaps a committed admission's buffer into its slot and
// releases the old one. The caller holds slot.mu.
func (ss *sessionStore) installSlotReply(slot *sessionSlot, reply *cachedReply) {
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	old := slot.buffer
	slot.reply = nil
	slot.buffer = reply
	slot.reserved = reply.charge
	if old != nil {
		old.releaseLocked()
	}
}

// replyWork charges a conservative allocation envelope independently of cache.
// It covers the response writer, checkpoints/growth, variable-result scratch and
// full RPC output simultaneously. It lives until transport completion, not until
// slot finish. This ownership says nothing about the client's lease (N09).
type replyWork struct {
	store          *sessionStore
	capacity       int
	charge         int
	outputCapacity int
	view           *cachedReply
}

// READDIR streams into the response writer (covered by capacity) and
// additionally holds at most one backend page and two prepared candidates.
//
// A RemoteTernVFS page is one shard ReadDirResp, bounded by the UDP MTU. An
// edge encodes at least TargetId(8) + NameHash(8) + name length(1) + one name
// byte + CreationTime(8). Each edge is held as a msgs.CurrentEdge and a
// DirEntry, with its name allocated twice by decoding (bytes, then string);
// each name allocation rounds up by less than 16 bytes. Array size classes
// round up by at most a quarter, and 4 KiB covers request/decoder temporaries.
// A candidate holds an exact-capacity attribute buffer, one more is measured
// for the mask, and the writer reuses one name buffer; the candidates' names
// can come from an earlier, otherwise dropped, page.
//
// LocalTernVFS (local development root) reads the whole directory to simulate
// hash order, so its temporaries are directory-sized and not covered.
const (
	readdirPageEdges    = msgs.MAX_UDP_MTU / (8 + 8 + 1 + 1 + 8)
	readdirPageBytes    = readdirPageEdges*int(unsafe.Sizeof(msgs.CurrentEdge{})+unsafe.Sizeof(DirEntry{}))*5/4 + 2*(msgs.MAX_UDP_MTU+16*readdirPageEdges) + 4<<10
	readdirAttrCapacity = 512 // A full supported-attribute encoding is 312 bytes.
	readdirPrepareBytes = readdirPageBytes + 3*readdirAttrCapacity + 3*(maxTernNameLength+16)
)

func (req *rpcRequest) acquireReplyWork(s *Server) bool {
	if req.replyWork != nil {
		return true
	}
	capacity := maxRPCMessage
	readdir := false
	if req.proc == procCompound {
		if args, ok := ReadCOMPOUND4args(req.body); ok {
			it := args.Argarray()
			if it.Next() && it.Argarray().Disc() == OP_SEQUENCE {
				a := it.Argarray().Value().AsSEQUENCE4args()
				if sess := s.sessions.lookup(readSessionID(a.SaSessionid())); sess != nil {
					capacity = int(sess.maxResp)
					if a.SaCachethis() != 0 {
						capacity = min(capacity, int(sess.maxCached))
					}
				}
			}
			// Operations past the limit are never executed.
			for n := 1; !readdir && n <= maxCompoundOperations && it.Next(); n++ {
				readdir = it.Argarray().Disc() == OP_READDIR
			}
			// A failed SEQUENCE still echoes its tag without consuming a slot.
			capacity = max(capacity, rpcAcceptedHeaderSize+12+((len(args.Tag().Data())+3)&^3)+8)
		}
	}
	// Go append growth and temporary variable results are covered conservatively.
	// READ is admitted before its buffer; READLINK is bounded separately below.
	// One READDIR runs at a time, so one preparation allowance covers them all.
	charge := 8 * (capacity + (64 << 10) + 1024)
	if readdir {
		charge += readdirPrepareBytes
	}
	ss := s.sessions
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	if ss.workBytes+charge > ss.workLimit {
		return false
	}
	ss.workBytes += charge
	ss.workReplies++
	req.replyWork = &replyWork{store: ss, capacity: capacity, charge: charge}
	return true
}

// releaseReply runs once the reply is written or will never be: it ends the
// request's hold on its incarnation and its reply work.
func (req *rpcRequest) releaseReply() {
	after := req.afterReply
	req.afterReply = nil
	for _, f := range after {
		f()
	}
	req.hold.release()
	req.hold = nil
	work := req.replyWork
	if work == nil {
		return
	}
	req.replyWork = nil
	if work.view != nil {
		work.view.release()
		work.view = nil
	}
	work.store.cacheMu.Lock()
	work.store.workBytes -= work.charge
	work.store.workReplies--
	work.store.cacheMu.Unlock()
}
func (req *rpcRequest) joinRPCBody(body []byte) []byte {
	header := buildRPCReply(req.xid, acceptSuccess)
	reply := make([]byte, len(header)+len(body))
	copy(reply, header)
	copy(reply[len(header):], body)
	if req.replyWork != nil {
		if cap(reply) > req.replyWork.capacity {
			panic("RPC result exceeds allocation envelope")
		}
		req.replyWork.outputCapacity = cap(reply)
	}
	return reply
}

func (ss *sessionStore) reserveSessionRecord(clientID uint64) (SessionID, bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.records+ss.pendingSessions >= ss.recordLimit || ss.next >= uint64(^uint32(0)) {
		return SessionID{}, false
	}
	ss.pendingSessions++
	return ss.newSessionIDLocked(clientID), true
}
func (ss *sessionStore) releaseSessionRecord() {
	ss.mu.Lock()
	ss.pendingSessions--
	ss.mu.Unlock()
}

// A session record is counted from insertion until its last use ends: store
// membership, then draining after detachment, and every admitted request.
// Detachment alone never returns record credit, so a stalled retirement
// (including a client-wide batch) still holds its records.
func (ss *sessionStore) insertSessionLocked(sess *session) {
	// The caller obtained the group first: CREATE_SESSION holds it from
	// beginCreate.
	g, ok := ss.groupLocked(sess.clientID)
	if !ok {
		panic("session inserted without a group credit")
	}
	ss.sessions[sess.id] = sess
	sess.users = 1
	ss.records++
	g.members++
	g.dropped = false
}
func (ss *sessionStore) useSession(sess *session) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if sess.users == 0 {
		return false
	}
	sess.users++
	return true
}
func (ss *sessionStore) releaseSessionUse(sess *session) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	sess.users--
	switch {
	case sess.users < 0:
		panic("session use underflow")
	case sess.users == 0:
		ss.records--
	}
}

// detachSessionLocked removes a session from lookup. It reports whether the
// caller now owns the membership use, to release after drainSession.
func (ss *sessionStore) detachSessionLocked(sess *session) bool {
	if ss.sessions[sess.id] != sess {
		return false
	}
	delete(ss.sessions, sess.id)
	if sess.cb != nil {
		sess.cb.cancelLocked()
	}
	if g := ss.groups[sess.clientID]; g != nil {
		g.members--
		ss.releaseGroupLocked(g)
	}
	return true
}

func (ss *sessionStore) retireSession(sess *session, fatal bool) {
	if fatal {
		sess.fatal.Store(true)
	} else {
		sess.destroyed.Store(true)
	}
	ss.mu.Lock()
	detached := ss.detachSessionLocked(sess)
	ss.mu.Unlock()
	ss.drainSession(sess)
	if detached {
		ss.releaseSessionUse(sess)
	}
}

func (ss *sessionStore) drainSession(sess *session) {
	// Never hold store.mu while waiting on a slot: admission holds slot.mu and
	// may look up a connection association under store.mu.
	for i := range sess.slots {
		slot := &sess.slots[i]
		slot.mu.Lock()
		buffer := slot.buffer
		slot.reply = nil
		slot.buffer = nil
		slot.reserved = 0
		if buffer != nil {
			buffer.release()
		}
		slot.mu.Unlock()
	}
}

type replyAccounting struct {
	CacheBytes, CacheBuffers, WorkBytes, WorkReplies int
	Sessions, Records, PendingSessions               int
	Groups, GroupCredits                             int
	ActiveRequests, ProvisionalRequests              int
}

func (ss *sessionStore) accounting() replyAccounting {
	ss.cacheMu.Lock()
	r := replyAccounting{CacheBytes: ss.cachedBytes, CacheBuffers: ss.cachedBuffers, WorkBytes: ss.workBytes, WorkReplies: ss.workReplies}
	ss.cacheMu.Unlock()
	ss.mu.Lock()
	r.Sessions = len(ss.sessions)
	r.Records = ss.records
	r.PendingSessions = ss.pendingSessions
	r.Groups = len(ss.groups)
	r.GroupCredits = ss.groupCredits
	for _, g := range ss.groups {
		r.ActiveRequests += g.active
		r.ProvisionalRequests += g.provisional
	}
	ss.mu.Unlock()
	return r
}
