// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"time"
	"unsafe"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// DefaultReplyWorkBudgetMiB is the default -reply-work-budget-mib. A READ
// compound reserves about 8.5 MiB, so this admits roughly a thousand READs
// at once; WRITE compounds reserve a few hundred KiB.
const DefaultReplyWorkBudgetMiB = 8192

const (
	maxReplyWorkBytes   = DefaultReplyWorkBudgetMiB << 20
	cachedReplyOverhead = 128

	// Reply envelope accounting, shared by admission (acquireReplyWork) and
	// execution (compoundBody), which must agree on every term.
	compoundHeaderBytes = 12 // status, tag length, result count
	sequenceResultBytes = 44 // SEQUENCE resok: session id, sequence and slot ids, flags
	failedResultBytes   = 8  // one operation's opnum and status
	// The envelope is replyWorkMultiplier times the bytes the compound can
	// return, plus replyWorkFixedBytes, covering the response writer, its
	// checkpoints and growth, variable-result scratch and the full encoded
	// output existing at once.
	replyWorkMultiplier = 8
	replyWorkFixedBytes = 64<<10 + 1024
	// replyBoundSlack covers the fixed-size results execution adds around
	// the per-operation bounds: a trailing error result and the handle
	// obligation of an OPEN or CREATE.
	replyBoundSlack = 256
	// A group is referenced by a wire record, a session, or a request or
	// CREATE_SESSION in progress. The first two are bounded by their own
	// caps; the credit bounds groups that only running work keeps.
	maxClientGroups = 2 * clientstore.MaxSessionRecords
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

// variableReplyOperation reports an operation whose result is sized from
// the remaining reply budget at execution rather than from a fixed bound.
func variableReplyOperation(op uint32) bool {
	return op == nfs4.OP_READDIR || op == nfs4.OP_READLINK
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
	readdirPageBytes    = readdirPageEdges*int(unsafe.Sizeof(msgs.CurrentEdge{})+unsafe.Sizeof(vfs.DirEntry{}))*5/4 + 2*(msgs.MAX_UDP_MTU+16*readdirPageEdges) + 4<<10
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
		if args, ok := nfs4.ReadCOMPOUND4args(req.body); ok {
			it := args.Argarray()
			tagBytes := (len(args.Tag().Data()) + 3) &^ 3
			// The reply the compound can produce: the RPC and COMPOUND
			// headers, the SEQUENCE result, and each operation's bound, as
			// execution enforces them. READDIR and READLINK size their
			// results from whatever budget remains, so a compound with one
			// keeps the session's full response size.
			bounded := rpcAcceptedHeaderSize + compoundHeaderBytes + tagBytes
			variable := false
			if it.Next() {
				first := it.Argarray().Value()
				if first.Disc() == nfs4.OP_SEQUENCE {
					bounded += sequenceResultBytes
					a := first.AsSEQUENCE4args()
					if sess := s.sessions.lookup(readSessionID(a.SaSessionid())); sess != nil {
						capacity = int(sess.maxResp)
						if a.SaCachethis() != 0 {
							capacity = min(capacity, int(sess.maxCached))
						}
					}
				} else {
					bounded += int(s.operationReplyBound(first))
					variable = variable || variableReplyOperation(first.Disc())
				}
			}
			// Operations past the limit are never executed.
			for n := 1; n <= maxCompoundOperations && it.Next(); n++ {
				op := it.Argarray().Value()
				readdir = readdir || op.Disc() == nfs4.OP_READDIR
				variable = variable || variableReplyOperation(op.Disc())
				bounded += int(s.operationReplyBound(op))
			}
			if !variable {
				capacity = min(capacity, bounded+replyBoundSlack)
			}
			// A failed SEQUENCE still echoes its tag without consuming a slot.
			capacity = max(capacity, rpcAcceptedHeaderSize+compoundHeaderBytes+tagBytes+failedResultBytes)
		}
	}
	// Go append growth and temporary variable results are covered conservatively.
	// READ is admitted before its buffer; READLINK is bounded separately below.
	// One READDIR runs at a time, so one preparation allowance covers them all.
	charge := replyWorkMultiplier * (capacity + replyWorkFixedBytes)
	if readdir {
		charge += readdirPrepareBytes
	}
	ss := s.sessions
	ss.cacheMu.Lock()
	defer ss.cacheMu.Unlock()
	if charge > ss.workLimit {
		return false // Can never be admitted.
	}
	// The limit is a bound on memory, not a reason to drop the connection:
	// a client with more requests in flight than the budget admits waits
	// here, holding its connection admission slot, until replies release
	// work or its connection closes. Dropping the connection instead made
	// the Linux client replay the burst and back off for 15 s on the
	// retried slots.
	//
	// Admission is decided here, under cacheMu, as work is charged: a
	// request whose connection has closed is refused even when capacity is
	// free, including capacity released while it waited, so it neither
	// executes nor holds work for a reply nobody will read.
	admitted, waited := false, false
	var waitStart time.Time
	for {
		if req.connection != nil && req.connection.isClosed() {
			break
		}
		if ss.workBytes+charge <= ss.workLimit {
			admitted = true
			break
		}
		if !waited {
			waited, waitStart = true, time.Now()
			s.metrics.workWaits.Add(1)
			s.metrics.workWaiters.Add(1)
		}
		ss.workFreed.Wait()
	}
	if waited {
		s.metrics.workWaiters.Add(-1)
		s.metrics.workWaitNanos.Add(uint64(time.Since(waitStart)))
	}
	if !admitted {
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
	work.store.workFreed.Broadcast()
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

// replyAccounting is the store's reply-cache and request figures with their
// limits, for tests and diagnostics.
type replyAccounting struct {
	CacheBytes, CacheBuffers, WorkBytes, WorkReplies int
	CacheLimit, WorkLimit                            int
	Sessions, Records, PendingSessions               int
	RecordLimit, WireLimit                           int
	Groups, GroupCredits, GroupLimit                 int
	ActiveRequests, ProvisionalRequests              int
}

func (ss *sessionStore) accounting() replyAccounting {
	ss.cacheMu.Lock()
	r := replyAccounting{
		CacheBytes: ss.cachedBytes, CacheBuffers: ss.cachedBuffers, CacheLimit: ss.cacheLimit,
		WorkBytes: ss.workBytes, WorkReplies: ss.workReplies, WorkLimit: ss.workLimit,
	}
	ss.cacheMu.Unlock()
	ss.mu.Lock()
	r.Sessions = len(ss.sessions)
	r.Records = ss.records
	r.PendingSessions = ss.pendingSessions
	r.RecordLimit, r.WireLimit = ss.recordLimit, ss.wireLimit
	r.Groups = len(ss.groups)
	r.GroupCredits, r.GroupLimit = ss.groupCredits, ss.groupLimit
	for _, g := range ss.groups {
		r.ActiveRequests += g.active
		r.ProvisionalRequests += g.provisional
	}
	ss.mu.Unlock()
	return r
}
