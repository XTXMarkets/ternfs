// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// NFSv4.1 sessions (RFC 8881 section 2.10).
//
// Sessions are held in memory and end with the nfsd process: after a
// restart, every earlier session ID gets BADSESSION, and the client
// registers again with EXCHANGE_ID and CREATE_SESSION (canonical r3 §6).
// The slot table and its reply cache replace NFSv4.0 open-owner seqids:
// a retransmitted request on a slot returns the cached reply instead of
// running again, and requests on different slots run concurrently.

const (
	// maxSessionReplyCached bounds one cached reply. Clients only ask for
	// caching of non-idempotent compounds, whose replies are small.
	maxSessionReplyCached = 64 << 10

	// maxSessionSlots bounds one session's concurrent requests. A session
	// may span connections, each with its own admission limit.
	maxSessionSlots = 64

	// A channel must carry at least a SEQUENCE request and its reply.
	minSessionRequest  = 128
	minSessionResponse = 128
)

type SessionID [NFS4_SESSIONID_SIZE]byte

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
	principal   rpcPrincipal
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
	ownerLocks keyedLocker[string]
}

const maxSessionCacheBytes = 64 << 20

func newSessionStore(epoch uint32) *sessionStore {
	return &sessionStore{
		epoch:       epoch,
		sessions:    make(map[SessionID]*session),
		clients:     make(map[uint64]*sessionClient),
		owners:      make(map[string]*wireOwner),
		wireLimit:   maxSessionRecords,
		groups:      make(map[uint64]*clientGroup),
		groupLimit:  maxClientGroups,
		cacheLimit:  maxSessionCacheBytes,
		workLimit:   maxReplyWorkBytes,
		recordLimit: maxSessionRecords,
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
		return NFS4ERR_STALE_CLIENTID
	}
	// A closing incarnation is the predecessor of a confirmation in progress.
	if g := ss.groups[c.incarnation]; c.csBusy || g != nil && (g.members > 0 || g.state == groupClosing) {
		ss.mu.Unlock()
		return NFS4ERR_CLIENTID_BUSY
	}
	retired := ss.dropClientLocked(c.incarnation)
	ss.mu.Unlock()
	ss.drainRetired(retired)
	return NFS4_OK
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

func verifierBytes(verifier Verifier4) [8]byte {
	var data [8]byte
	for i := range data {
		data[i] = verifier.Data(i)
	}
	return data
}

func readSessionID(m Sessionid4) SessionID {
	var id SessionID
	for i := range id {
		id[i] = m.Data(i)
	}
	return id
}

func writeSessionID(m Sessionid4, id SessionID) {
	for i, b := range id {
		m.SetData(i, b)
	}
}

// sessionlessOp reports operations which may start a compound without
// SEQUENCE. Each must then be the only operation (RFC 8881 section 2.10.6.2).
func sessionlessOp(opnum uint32) bool {
	switch opnum {
	case OP_EXCHANGE_ID, OP_CREATE_SESSION, OP_DESTROY_SESSION,
		OP_DESTROY_CLIENTID, OP_BIND_CONN_TO_SESSION:
		return true
	}
	return false
}

// sequenceResult is the outcome of the SEQUENCE that starts a compound.
type sequenceResult struct {
	status uint32
	sess   *session
	slot   *sessionSlot
	slotID uint32
	seq    uint32
	// replay holds a cached reply body for a retransmission.
	replay []byte
	cache  bool
	budget uint32 // full RPC result ceiling for this call
	view   *cachedReply
	store  *sessionStore // set while the request holds a session use
	// hold is the admitted request's active hold on its incarnation. The
	// caller passes it to the transport (rpcRequest.hold).
	hold *requestHold
}

// releaseUse ends the admitted request's use of its session, once.
func (r *sequenceResult) releaseUse() {
	if r.store != nil {
		r.store.releaseSessionUse(r.sess)
		r.store = nil
	}
}

type sequenceAdmission struct {
	digest             [32]byte
	user               effectiveUser
	requestBytes       uint32
	operations         uint32
	sequenceReplyBytes uint32
	connection         *rpcConnection
	uncachedBytes      uint32
}

// startSequence validates SEQUENCE and claims its slot. The caller must
// call finishSequence once the compound reply is complete.
func (s *Server) startSequence(args SEQUENCE4args, admission ...sequenceAdmission) sequenceResult {
	a := sequenceAdmission{digest: sha256.Sum256(args.m[:]), operations: 1,
		sequenceReplyBytes: rpcAcceptedHeaderSize + 12 + 44}
	if len(admission) != 0 {
		a = admission[0]
	}
	id := readSessionID(args.SaSessionid())
	sess := s.sessions.lookup(id)
	// An unknown session, including one an earlier process issued, is never
	// reconstructed (RFC 8881 §2.10.13.1.4).
	if sess == nil || sess.destroyed.Load() || sess.fatal.Load() {
		return sequenceResult{status: NFS4ERR_BADSESSION}
	}
	slotID := args.SaSlotid()
	if slotID >= uint32(len(sess.slots)) {
		return sequenceResult{status: NFS4ERR_BADSLOT}
	}
	if args.SaHighestSlotid() < slotID || args.SaHighestSlotid() >= uint32(len(sess.slots)) {
		return sequenceResult{status: NFS4ERR_BAD_HIGH_SLOT}
	}
	switch {
	case a.operations > min(sess.maxOps, uint32(maxCompoundOperations)):
		return sequenceResult{status: NFS4ERR_TOO_MANY_OPS}
	case a.requestBytes > sess.maxReq:
		return sequenceResult{status: NFS4ERR_REQ_TOO_BIG}
	case max(a.sequenceReplyBytes, a.uncachedBytes) > sess.maxResp:
		return sequenceResult{status: NFS4ERR_REP_TOO_BIG}
	case args.SaCachethis() != 0 && max(a.sequenceReplyBytes, a.uncachedBytes) > sess.maxCached:
		return sequenceResult{status: NFS4ERR_REP_TOO_BIG_TO_CACHE}
	}
	slot := &sess.slots[slotID]
	seq := args.SaSequenceid()

	// slot.seq is the sequence id of the request running or last completed
	// on the slot.
	if !slot.mu.TryLock() {
		return sequenceResult{status: NFS4ERR_DELAY}
	}
	defer slot.mu.Unlock()
	replay := slot.occupied && seq == slot.seq
	switch {
	case replay:
		// Retransmission of the last request on this slot.
		if slot.inUse {
			return sequenceResult{status: NFS4ERR_DELAY}
		}
		if slot.digest != a.digest || slot.user != a.user {
			return sequenceResult{status: NFS4ERR_SEQ_FALSE_RETRY}
		}
		if slot.buffer == nil {
			return sequenceResult{status: NFS4ERR_BADSESSION}
		}
	case !slot.inUse && (slot.occupied && seq == slot.seq+1 ||
		!slot.occupied && seq == 1):
	default:
		return sequenceResult{status: NFS4ERR_SEQ_MISORDERED}
	}

	// Admission (canonical r3 §7.1) holds slot.mu throughout and does no
	// backend I/O. Each step takes a resource; until the commit nothing in
	// the slot, the cache, the association or the lease changes, and undo
	// releases the resources taken, exactly once, in reverse.
	var undo []func()
	committed := false
	defer func() {
		if !committed {
			for i := len(undo) - 1; i >= 0; i-- {
				undo[i]()
			}
		}
	}()

	// Lease gate: a provisional hold. DELAY while expiry is decided,
	// BADSESSION once it was. Releasing it renews nothing.
	hold, status := s.sessions.holdRequest(sess)
	if status != NFS4_OK {
		return sequenceResult{status: status}
	}
	undo = append(undo, hold.release)
	// Pin: while held, the session record cannot be released.
	if !s.sessions.useSession(sess) {
		return sequenceResult{status: NFS4ERR_BADSESSION}
	}
	undo = append(undo, func() { s.sessions.releaseSessionUse(sess) })
	// Association, counted against the connection's quota while pending.
	token, ok := a.connection.reserve(s, id)
	if !ok {
		return sequenceResult{status: NFS4ERR_RESOURCE}
	}
	undo = append(undo, token.abort)
	// Reply reservation: a new request charges a new buffer while the old
	// one stays attached and replayable.
	budget := sess.maxResp
	if args.SaCachethis() != 0 {
		budget = min(budget, sess.maxCached)
	}
	var reply *cachedReply
	if !replay {
		reservation := int(max(a.sequenceReplyBytes, a.uncachedBytes))
		if args.SaCachethis() != 0 {
			reservation = int(budget)
		}
		if reply, ok = s.sessions.reserveSlotReply(sess, slot, reservation); !ok {
			return sequenceResult{status: NFS4ERR_RESOURCE}
		}
		undo = append(undo, reply.release)
	}
	// Renewal check, from memory only.
	if status := s.renewalCheck(hold, sess); status != NFS4_OK {
		return sequenceResult{status: status}
	}
	if sess.destroyed.Load() || sess.fatal.Load() {
		return sequenceResult{status: NFS4ERR_BADSESSION}
	}

	// Commit, which cannot fail.
	committed = true
	token.commit()
	hold.activate()
	s.clients.noteLeaseNeed(sess.clientID)
	if replay {
		slot.buffer.retain()
		s.sessions.releaseSessionUse(sess)
		return sequenceResult{status: NFS4_OK, sess: sess, replay: slot.reply, view: slot.buffer, hold: hold}
	}
	s.sessions.installSlotReply(slot, reply)
	slot.seq, slot.occupied, slot.inUse = seq, true, true
	slot.cached = false
	slot.digest, slot.user = a.digest, a.user
	return sequenceResult{
		hold:   hold,
		status: NFS4_OK,
		sess:   sess,
		slot:   slot,
		slotID: slotID,
		seq:    seq,
		cache:  args.SaCachethis() != 0,
		budget: budget,
		store:  s.sessions,
	}
}

// finishSequence records the reply for a request which claimed a slot.
func (r *sequenceResult) finish(reply []byte) {
	if r.slot == nil {
		if r.view != nil {
			r.view.release()
			r.view = nil
		}
		return
	}
	slot := r.slot
	defer r.releaseUse() // After the slot is unlocked.
	slot.mu.Lock()
	defer slot.mu.Unlock()
	slot.inUse = false
	slot.cached = r.cache
	if r.sess.destroyed.Load() || r.sess.fatal.Load() {
		slot.reply = nil
		slot.cached = false
		slot.reserved = 0
		if slot.buffer != nil {
			slot.buffer.release()
			slot.buffer = nil
		}
		return
	}
	var saved []byte
	if r.cache {
		saved = reply
	} else {
		// Save exactly SEQUENCE plus its original tag. If a following result
		// exists, replay replaces it with RETRY_UNCACHED_REP, never inventing one.
		tagLen := int(binary.BigEndian.Uint32(reply[4:]))
		countOff := 8 + (tagLen+3)&^3
		seqEnd := countOff + 4 + 44
		// One exact-capacity buffer, including the next operation's error arm.
		count := binary.BigEndian.Uint32(reply[countOff:])
		size := seqEnd
		if count > 1 {
			size += int(resultErrorSize(binary.BigEndian.Uint32(reply[seqEnd:])))
		}
		saved = make([]byte, seqEnd, size)
		copy(saved, reply[:seqEnd])
		binary.BigEndian.PutUint32(saved[countOff:], 1)
		if count > 1 {
			op := binary.BigEndian.Uint32(reply[seqEnd:])
			status := uint32(NFS4ERR_RETRY_UNCACHED_REP)
			if op == OP_ILLEGAL || mandatoryUnsupported(op) {
				// RFC 8881 §2.10.6.1.3 forbids replacing these required errors.
				status = binary.BigEndian.Uint32(reply[seqEnd+4:])
			}
			binary.BigEndian.PutUint32(saved[:4], status)
			binary.BigEndian.PutUint32(saved[countOff:], 2)
			saved = appendResultError(saved, op, status)
		}
	}
	slot.buffer.storeBody(saved)
	slot.reply = slot.buffer.data
	slot.reserved = slot.buffer.charge
}

// renewalCheck is admission's renewal check (canonical r3 §7.1 step 6). It
// reads the gate and the refresher's record from memory, never the store.
// A lease this process cannot prove, or a confirmed pointer not read
// recently, gives DELAY and asks the refresher to verify.
func (s *Server) renewalCheck(hold *requestHold, sess *session) uint32 {
	if status := hold.gateStatus(); status != NFS4_OK {
		return status
	}
	view, ok := s.clients.leaseView(sess.clientID)
	if !ok {
		// No refresher record credit.
		return NFS4ERR_DELAY
	}
	if view.state != leaseRecordLive {
		return NFS4ERR_BADSESSION
	}
	now := s.clients.now().UnixNano()
	if view.proven <= now || view.verifiedAt < now-int64(nfsLeaseRenewAfter) {
		s.clients.requestLeaseVerify(sess.clientID)
		return NFS4ERR_DELAY
	}
	return NFS4_OK
}

func (s *Server) writeSequenceOK(w *COMPOUND4resWriter, seq *sequenceResult) {
	ew := w.AppendResarray_Sequence()
	ok := ew.SetValue_Nfs4Ok()
	writeSessionID(ok.SrSessionid(), seq.sess.id)
	ok.SetSrSequenceid(seq.seq)
	ok.SetSrSlotid(seq.slotID)
	highest := uint32(len(seq.sess.slots) - 1)
	ok.SetSrHighestSlotid(highest)
	ok.SetSrTargetHighestSlotid(s.slotTarget(seq.sess))
	ok.SetSrStatusFlags(s.sequenceFlags(seq.sess))
	w.Resume(ew.Finish())
}

func writeSequenceError(w *COMPOUND4resWriter, status uint32) {
	ew := w.AppendResarray_Sequence()
	ew.SetValue_Default(status)
	w.Resume(ew.Finish())
}

// serverOwnerMajor identifies this export for session trunking checks.
// Every nfsd serving one TernFS shares durable client state, but sessions
// and stateids are process-local, so each process is its own server owner.
func (s *Server) serverOwnerMajor() []byte {
	return []byte("ternfs-nfsd/" + s.clients.nfsdID)
}

var serverScope = []byte("ternfs")

func (s *Server) opExchangeID(
	args EXCHANGEID4args,
	st *compoundState,
	w *COMPOUND4resWriter,
) uint32 {
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_ExchangeId()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	flags := args.EiaFlags()
	if flags&^(EXCHGID4_FLAG_SUPP_MOVED_REFER|EXCHGID4_FLAG_SUPP_MOVED_MIGR|
		EXCHGID4_FLAG_BIND_PRINC_STATEID|EXCHGID4_FLAG_MASK_PNFS|
		EXCHGID4_FLAG_UPD_CONFIRMED_REC_A) != 0 {
		return fail(NFS4ERR_INVAL)
	}
	if args.EiaClientImplIdCount() > 1 {
		return fail(NFS4ERR_BADXDR)
	}
	if args.EiaStateProtectType() != SP4_NONE {
		return fail(NFS4ERR_ENCR_ALG_UNSUPP)
	}
	owner := args.EiaClientowner()
	ownerKey := string(owner.CoOwnerid())
	verifier := verifierBytes(owner.CoVerifier())
	ss := s.sessions
	if flags&EXCHGID4_FLAG_UPD_CONFIRMED_REC_A != 0 {
		// An update of a confirmed record (RFC 8881 section 18.35.5,
		// cases 6-9). nfsd has no per-client properties for an update to
		// change, so a valid update returns the record unchanged. It only
		// reads, so it needs no ordering with a confirmation beyond waiting
		// while one is in progress (§5.4).
		if s.confirmingOwner(ownerKey) {
			return fail(NFS4ERR_DELAY)
		}
		ss.mu.Lock()
		var c *sessionClient
		if o := ss.owners[ownerKey]; o != nil {
			c = o.confirmed
		}
		ss.mu.Unlock()
		if c == nil {
			return fail(NFS4ERR_NOENT)
		}
		incarnation, status := s.clients.confirmedRecord(owner.CoOwnerid(), verifier, st.principal)
		if status != NFS4_OK {
			return fail(status)
		}
		if incarnation != c.incarnation {
			// This process's confirmed record is no longer the durable one.
			return fail(NFS4ERR_NOENT)
		}
		return s.writeExchangeIDResult(w, c, true)
	}

	// One registration at a time per owner, so the wire record this request
	// publishes names the durable registration it made. Otherwise a slower
	// request could publish a pending incarnation that a faster one has
	// already superseded in the store.
	unlockOwner := ss.ownerLocks.lock(ownerKey)
	defer unlockOwner()
	// Checked under the owner lock, which a confirmation's registration
	// also takes: either this request's durable registration and wire
	// publication finish before protection is installed, or it sees the
	// owner confirming and waits (§5.4).
	if s.confirmingOwner(ownerKey) {
		return fail(NFS4ERR_DELAY)
	}

	// Case 2: a retransmission or a client that already holds the record.
	// The record and its creation sequence are unchanged; nothing is written.
	ss.mu.Lock()
	if o := ss.owners[ownerKey]; o != nil && o.confirmed != nil &&
		o.confirmed.verifier == verifier && o.confirmed.principal == st.principal &&
		!ss.expiredLocked(o.confirmed.incarnation) {
		c := o.confirmed
		ss.mu.Unlock()
		if s.hasPurgePending(c.incarnation) {
			// The only store call case 2 makes: a failed purge's retry.
			s.resumeReplacementPurge(c.incarnation)
		}
		return s.writeExchangeIDResult(w, c, true)
	}
	if o := ss.owners[ownerKey]; o != nil && o.confirmed != nil && ss.expiredLocked(o.confirmed.incarnation) {
		// This process expired the record's incarnation: it is not returned
		// as live. The record is retired and registration starts again from
		// the store; while a confirmation protects it, the client waits.
		if o.confirmed.confirming != nil {
			ss.mu.Unlock()
			return fail(NFS4ERR_DELAY)
		}
		ss.retireWireLocked(o.confirmed)
	}
	if status := s.wireCapacityLocked(0); status != NFS4_OK {
		ss.mu.Unlock()
		return fail(status)
	}
	ss.mu.Unlock()

	incarnation, confirm, adopted, err := s.clients.ExchangeID(
		verifier, owner.CoOwnerid(), clientOwner{principal: st.principal})
	if err != nil {
		var inUse clientInUseError
		if errors.As(err, &inUse) {
			return fail(NFS4ERR_CLID_INUSE)
		}
		return fail(clientStoreErrToNFS(err))
	}

	ss.mu.Lock()
	o := ss.owners[ownerKey]
	if o != nil && o.confirmed != nil && o.confirmed.incarnation == incarnation {
		// A concurrent CREATE_SESSION confirmed the record this request would
		// have adopted; answer as case 2.
		c := o.confirmed
		ss.mu.Unlock()
		return s.writeExchangeIDResult(w, c, true)
	}
	if g := ss.groups[incarnation]; g != nil && g.state == groupClosing {
		// Another client is replacing this incarnation.
		ss.mu.Unlock()
		return fail(NFS4ERR_DELAY)
	}
	if o != nil && o.unconfirmed != nil && o.unconfirmed.confirming != nil {
		// The record is being confirmed: it is not superseded (§5.4).
		ss.mu.Unlock()
		return fail(NFS4ERR_DELAY)
	}
	// Case 4: a new EXCHANGE_ID supersedes the owner's unconfirmed record,
	// and an incarnation has one live record.
	if o != nil && o.unconfirmed != nil {
		ss.retireWireLocked(o.unconfirmed)
	}
	if g := ss.groups[incarnation]; g != nil && g.wire != nil {
		ss.retireWireLocked(g.wire)
	}
	if status := s.wireCapacityLocked(incarnation); status != NFS4_OK {
		ss.mu.Unlock()
		return fail(status)
	}
	clientID, err := ss.wireIDs.next()
	if err != nil {
		ss.mu.Unlock()
		s.log.Error("cannot issue an EXCHANGE_ID clientid", "err", err)
		return fail(NFS4ERR_SERVERFAULT)
	}
	// wireCapacityLocked checked the group credit under this lock.
	g, _ := ss.groupLocked(incarnation)
	// A cold adoption is how a client regains an incarnation that this
	// process treated as replaced without proof.
	g.state = groupOpen
	c := &sessionClient{
		clientID:    clientID,
		incarnation: incarnation,
		owner:       ownerKey,
		verifier:    verifier,
		principal:   st.principal,
		createdAt:   s.clients.now(),
		adopted:     adopted,
		confirm:     confirm,
		hasConfirm:  !adopted,
	}
	ss.addWireLocked(c)
	ss.mu.Unlock()

	s.scheduleClientGC(incarnation)
	return s.writeExchangeIDResult(w, c, false)
}

// wireCapacityLocked admits a new wire record and the group credit its
// incarnation needs; before the incarnation is known (0), any free credit.
// At either cap, records that never reached CREATE_SESSION within a lease
// interval are discarded first. An allocator that cannot issue another
// clientid is a server fault; existing records keep working.
func (s *Server) wireCapacityLocked(incarnation uint64) uint32 {
	ss := s.sessions
	if err := ss.wireIDs.available(); err != nil {
		s.log.Error("cannot issue an EXCHANGE_ID clientid", "err", err)
		return NFS4ERR_SERVERFAULT
	}
	admits := func() bool {
		return len(ss.clients) < ss.wireLimit && ss.groupAvailableLocked(incarnation)
	}
	if !admits() {
		ss.reclaimUnconfirmedLocked(s.clients.now())
	}
	if !admits() {
		return NFS4ERR_DELAY
	}
	return NFS4_OK
}

func (s *Server) writeExchangeIDResult(w *COMPOUND4resWriter, c *sessionClient, confirmed bool) uint32 {
	s.sessions.mu.Lock()
	nextSeq := c.csSeq + 1
	s.sessions.mu.Unlock()
	ew := w.AppendResarray_ExchangeId()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetEirClientid(c.clientID)
	ok.SetEirSequenceid(nextSeq)
	resFlags := uint32(EXCHGID4_FLAG_USE_NON_PNFS)
	if confirmed {
		resFlags |= EXCHGID4_FLAG_CONFIRMED_R
	}
	ok.SetEirFlags(resFlags)
	ok.SetEirStateProtectType(SP4_NONE)
	ok.SetEirStateProtect_None()
	sow := ok.StartEirServerOwner()
	sow.SetSoMinorId(0)
	ok.Resume(sow.SetSoMajorId(s.serverOwnerMajor()).Finish())
	ok.SetEirServerScope(serverScope)
	impl := ok.StartEirServerImplId()
	ok.Resume(impl.Finish())
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return NFS4_OK
}

func readChannelAttrs(m ChannelAttrs4) channelAttrs {
	return channelAttrs{
		headerPad: m.CaHeaderpadsize(),
		maxReq:    m.CaMaxrequestsize(),
		maxResp:   m.CaMaxresponsesize(),
		maxCached: m.CaMaxresponsesizeCached(),
		maxOps:    m.CaMaxoperations(),
		maxReqs:   m.CaMaxrequests(),
	}
}

func writeChannelAttrs(w ChannelAttrs4Writer, a channelAttrs) []byte {
	w.SetCaHeaderpadsize(a.headerPad)
	w.SetCaMaxrequestsize(a.maxReq)
	w.SetCaMaxresponsesize(a.maxResp)
	w.SetCaMaxresponsesizeCached(a.maxCached)
	w.SetCaMaxoperations(a.maxOps)
	w.SetCaMaxrequests(a.maxReqs)
	return w.Finish()
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func (s *Server) negotiateForeChannel(req channelAttrs) channelAttrs {
	return channelAttrs{
		maxReq:    minU32(req.maxReq, maxRPCMessage),
		maxResp:   minU32(req.maxResp, maxRPCMessage),
		maxCached: minU32(req.maxCached, maxSessionReplyCached),
		maxOps:    minU32(req.maxOps, maxCompoundOperations),
		maxReqs:   max(1, minU32(req.maxReqs, maxSessionSlots)),
	}
}

func (s *Server) opCreateSession(
	args CREATESESSION4args,
	st *compoundState,
	w *COMPOUND4resWriter,
) uint32 {
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_CreateSession()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	clientID := args.CsaClientid()
	seq := args.CsaSequence()
	if args.CsaFlags()&^(CREATE_SESSION4_FLAG_PERSIST|CREATE_SESSION4_FLAG_CONN_BACK_CHAN|
		CREATE_SESSION4_FLAG_CONN_RDMA) != 0 {
		return fail(NFS4ERR_INVAL)
	}
	fore := readChannelAttrs(args.CsaForeChanAttrs())
	back := readChannelAttrs(args.CsaBackChanAttrs())
	if args.CsaForeChanAttrs().CaRdmaIrdCount() > 1 ||
		args.CsaBackChanAttrs().CaRdmaIrdCount() > 1 {
		return fail(NFS4ERR_BADXDR)
	}
	if fore.maxReq < minSessionRequest || fore.maxResp < minSessionResponse ||
		back.maxReq < minSessionRequest || back.maxResp < minSessionResponse ||
		fore.maxOps < 2 {
		return fail(NFS4ERR_TOOSMALL)
	}
	// The callback credential is checked before the slot, and only when a
	// backchannel is asked for: clients without one may offer none (libnfs
	// sends an empty list).
	backRequested := args.CsaFlags()&CREATE_SESSION4_FLAG_CONN_BACK_CHAN != 0
	var cbSec callbackSec
	if backRequested {
		var status uint32
		if cbSec, status = parseCallbackSec(args.CsaSecParms()); status != NFS4_OK {
			return fail(status)
		}
	}

	// This request's reservations are released when it returns. An
	// unresolved confirmation takes them over instead. No connection lock is
	// held across store calls: the association is a token, counted against
	// the connection's quota from before the slot (canonical r3 §7.2).
	cr := &creation{s: s, seq: seq, back: back, cbProgram: args.CsaCbProgram(), cbSec: cbSec, backRequested: backRequested}
	handedOver := false
	var settled func() // finishes an earlier confirmation this retry applied
	var token *associationToken
	var backTok *backToken
	defer func() {
		token.abort() // a no-op once committed
		backTok.abort()
		if settled != nil {
			settled()
		}
		if !handedOver {
			cr.finish()
		}
	}()
	ss := s.sessions
	// associate publishes a newly created session's association, reserved
	// before the slot. A record retired meanwhile publishes no session and
	// keeps no association.
	associate := func(reply createSessionResult) uint32 {
		if sess := ss.lookup(reply.id); sess != nil {
			token.commit()
			if reply.back {
				s.commitBackChannel(st, backTok, sess)
			}
		}
		return writeCreateSessionResult(w, reply)
	}
	// replayAssociation is a cached OK's association: an associated session
	// costs nothing; a new association needs quota.
	replayAssociation := func(reply createSessionResult) uint32 {
		if ss.lookup(reply.id) == nil {
			return NFS4_OK
		}
		t, ok := st.connection.reserve(s, reply.id)
		if !ok {
			return NFS4ERR_RESOURCE
		}
		var bt *backToken
		if reply.back {
			if bt, ok = st.connection.reserveBack(s, reply.id); !ok {
				t.abort()
				return NFS4ERR_RESOURCE
			}
		}
		t.commit()
		if sess := ss.lookup(reply.id); sess != nil && reply.back {
			s.commitBackChannel(st, bt, sess)
		} else {
			bt.abort()
		}
		return NFS4_OK
	}
	// The clientid is looked up only among this process's wire records:
	// never issued, from another epoch or retired is STALE_CLIENTID, before
	// any slot or store effect.
	ss.mu.Lock()
	c := ss.wireRecordLocked(clientID)
	if c == nil {
		ss.mu.Unlock()
		return fail(NFS4ERR_STALE_CLIENTID)
	}
	// Outcomes that create nothing are decided from the slot before any
	// reservation, so they need no group credit, session record or serial,
	// and work at those limits: a held confirmation's retry, which uses its
	// own creation's reservations; a replay of the cached outcome; and the
	// waiting and misordered sequences. A closing or replaced incarnation
	// gives the statuses beginCreate would. Only a new creation reserves,
	// and then checks the slot again below.
	var held *confirmation
	var early, cached bool
	var earlyStatus uint32
	var reply createSessionResult
	switch g := ss.groups[c.incarnation]; {
	case c.csBusy && seq == c.csBusySeq && c.confirming != nil:
		held, early = c.confirming, true
	case g != nil && g.state == groupClosing:
		early, earlyStatus = true, NFS4ERR_DELAY
	case g != nil && g.state == groupReplaced:
		early, earlyStatus = true, NFS4ERR_STALE_CLIENTID
	case c.csBusy && seq == c.csBusySeq:
		early, earlyStatus = true, NFS4ERR_DELAY
	case c.csCached && seq == c.csSeq:
		early, earlyStatus = true, c.csStatus
		cached, reply = c.csStatus == NFS4_OK, c.csReply
	case c.csBusy || seq != c.csSeq+1:
		// Misordering never overwrites the cached outcome.
		early, earlyStatus = true, NFS4ERR_SEQ_MISORDERED
	}
	ss.mu.Unlock()
	switch {
	case held != nil:
		// A retry of a held confirmation uses the reservations its creation
		// already owns: no new group hold or session record, so it settles
		// at the record cap.
		status, reply, done := s.retryHeldConfirmation(held, st)
		settled = done
		if status != NFS4_OK {
			return fail(status)
		}
		return writeCreateSessionResult(w, reply)
	case cached:
		// A cached result never rebuilds its session: once destroyed, the
		// session stays BADSESSION and the connection gains nothing. A
		// session already associated with the connection replays at the
		// quota; only a new association is refused there.
		if status := replayAssociation(reply); status != NFS4_OK {
			return fail(status)
		}
		return writeCreateSessionResult(w, reply)
	case early:
		return fail(earlyStatus)
	}
	cr.c = c
	incarnation := c.incarnation

	// Every resource is reserved before the slot is looked at. A failed
	// reservation leaves the slot as it was. The connection's association
	// quota is checked below, only where a new association is needed: a
	// replay of an associated session needs none.
	//
	// Replacement of this incarnation waits until the session is inserted
	// or this request fails.
	end, status := ss.beginCreate(incarnation)
	if status != NFS4_OK {
		return fail(status)
	}
	cr.endCreate = end
	// The bounded volatile session record and a non-reused local serial.
	id, admitted := ss.reserveSessionRecord(incarnation)
	if !admitted {
		return fail(NFS4ERR_DELAY)
	}
	cr.id, cr.record = id, true
	// The refresher record that bootstrap will use (canonical r3 §9.4).
	if cr.leaseRecord, admitted = s.clients.reserveLeaseRecord(incarnation); !admitted {
		return fail(NFS4ERR_DELAY)
	}
	// The new session's association on this connection, and its back
	// association when a backchannel is asked for and this transport can
	// carry one: both before the slot, so a full connection changes nothing.
	if token, admitted = st.connection.reserve(s, id); !admitted {
		return fail(NFS4ERR_RESOURCE)
	}
	cr.backGrant = backRequested && st.connection.carriesCallbacks()
	if cr.backGrant {
		if backTok, admitted = st.connection.reserveBack(s, id); !admitted {
			return fail(NFS4ERR_RESOURCE)
		}
	}

	// The creation slot, checked again now that the reservations are held:
	// it may have moved while unlocked. While a request owns it, the same
	// sequence waits, the cached sequence replays and any other is
	// misordered.
	ss.mu.Lock()
	if c.retired {
		ss.mu.Unlock()
		return fail(NFS4ERR_STALE_CLIENTID)
	}
	switch {
	case c.csBusy && seq == c.csBusySeq:
		// The request owning the slot is running, or a confirmation was
		// registered since the check above; its retry settles it.
		ss.mu.Unlock()
		return fail(NFS4ERR_DELAY)
	case c.csCached && seq == c.csSeq:
		reply, cachedStatus := c.csReply, c.csStatus
		ss.mu.Unlock()
		if cachedStatus != NFS4_OK {
			return fail(cachedStatus)
		}
		// This request creates nothing: its own tokens go, and the
		// cached result's association follows the replay rule.
		token.abort()
		backTok.abort()
		if status := replayAssociation(reply); status != NFS4_OK {
			return fail(status)
		}
		return writeCreateSessionResult(w, reply)
	case c.csBusy || seq != c.csSeq+1:
		// Misordering never overwrites the cached outcome.
		ss.mu.Unlock()
		return fail(NFS4ERR_SEQ_MISORDERED)
	}
	c.csBusy, c.csBusySeq = true, seq
	cr.slot = true // Any exit that caches no outcome rolls the slot back.
	confirmed, confirm, hasConfirm, principal := c.confirmed, c.confirm, c.hasConfirm, c.principal
	ss.mu.Unlock()
	cr.fore = s.negotiateForeChannel(fore)

	if hasConfirm {
		// A pending incarnation: closing its predecessor, the principal
		// check, writes A, B and D, and acting on their result (§5.3).
		status, reply, owned := s.confirmPending(cr, confirm, principal, st.principal)
		handedOver = owned
		if status != NFS4_OK {
			return fail(status)
		}
		return associate(reply)
	}
	if !confirmed && principal != st.principal {
		// An unconfirmed (adopted) record belongs to the principal that
		// registered it.
		ss.mu.Lock()
		cr.cacheLocked(NFS4ERR_CLID_INUSE, createSessionResult{})
		ss.mu.Unlock()
		return fail(NFS4ERR_CLID_INUSE)
	}
	// An adopted or already confirmed record: check, without writing, that
	// the incarnation is still the confirmed one (§5.2). If not,
	// STALE_CLIENTID ends the record and leaves its slot untouched; a store
	// that proves the incarnation stale must not leave a record a later
	// EXCHANGE_ID would return as confirmed.
	confirmedNow, err := s.clients.IsConfirmed(incarnation)
	if err != nil {
		return fail(clientStoreErrToNFS(err))
	}
	if !confirmedNow {
		ss.mu.Lock()
		ss.retireWireLocked(c)
		ss.mu.Unlock()
		return fail(NFS4ERR_STALE_CLIENTID)
	}
	ss.mu.Lock()
	// A later EXCHANGE_ID may have retired the record while this request
	// ran (case 4); its delayed CREATE_SESSION joins no record.
	reply, inserted := cr.commitLocked()
	ss.mu.Unlock()
	if !inserted {
		return fail(NFS4ERR_STALE_CLIENTID)
	}
	return associate(reply)
}

func writeCreateSessionResult(w *COMPOUND4resWriter, r createSessionResult) uint32 {
	ew := w.AppendResarray_CreateSession()
	ok := ew.SetValue_Nfs4Ok()
	writeSessionID(ok.CsrSessionid(), r.id)
	ok.SetCsrSequence(r.seq)
	var flags uint32
	if r.back {
		flags = CREATE_SESSION4_FLAG_CONN_BACK_CHAN
	}
	ok.SetCsrFlags(flags)
	ok.Resume(writeChannelAttrs(ok.StartCsrForeChanAttrs(), r.foreAttrs))
	ok.Resume(writeChannelAttrs(ok.StartCsrBackChanAttrs(), r.backAttr))
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opDestroySession(args DESTROYSESSION4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_DestroySession()
	id := readSessionID(args.DsaSessionid())
	if s.sessions.lookup(id) == nil {
		r.SetDsrStatus(NFS4ERR_BADSESSION)
		return NFS4ERR_BADSESSION
	}
	if !st.connection.associated(id) {
		r.SetDsrStatus(NFS4ERR_CONN_NOT_BOUND_TO_SESSION)
		return NFS4ERR_CONN_NOT_BOUND_TO_SESSION
	}
	if !s.sessions.destroySession(id) {
		r.SetDsrStatus(NFS4ERR_BADSESSION)
		return NFS4ERR_BADSESSION
	}
	r.SetDsrStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opDestroyClientID(args DESTROYCLIENTID4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_DestroyClientid()
	s.sessions.mu.Lock()
	c := s.sessions.wireRecordLocked(args.DcaClientid())
	s.sessions.mu.Unlock()
	stale := func() uint32 {
		r.SetDcrStatus(NFS4ERR_STALE_CLIENTID)
		return NFS4ERR_STALE_CLIENTID
	}
	if c == nil {
		return stale()
	}
	// Ordered with the owner's registrations: no EXCHANGE_ID can replace
	// this record between the checks below and its destruction.
	unlockOwner := s.sessions.ownerLocks.lock(c.owner)
	defer unlockOwner()
	s.sessions.mu.Lock()
	if c.retired {
		s.sessions.mu.Unlock()
		return stale()
	}
	incarnation := c.incarnation
	g := s.sessions.groups[incarnation]
	// A CREATE_SESSION in progress on the record also makes it busy.
	busy := c.csBusy || g != nil && g.members > 0
	s.sessions.mu.Unlock()
	// Opens still make the client busy after its last session is destroyed.
	busy = busy || len(s.opens.clientStates(incarnation)) != 0
	if busy {
		r.SetDcrStatus(NFS4ERR_CLIENTID_BUSY)
		return NFS4ERR_CLIENTID_BUSY
	}
	// The checks above can be overtaken by a CREATE_SESSION, which does not
	// take the owner lock; the final step repeats them under the retirement
	// lock.
	if status := s.sessions.destroyClientRecord(c); status != NFS4_OK {
		r.SetDcrStatus(status)
		return status
	}
	s.expireClientState(incarnation)
	r.SetDcrStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opBindConnToSession(args BINDCONNTOSESSION4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_BindConnToSession()
	id := readSessionID(args.BctsaSessid())
	if s.sessions.lookup(id) == nil {
		// Unknown, including a session from before an nfsd restart: the
		// client creates a new one.
		ew.SetValue_Default(NFS4ERR_BADSESSION)
		w.Resume(ew.Finish())
		return NFS4ERR_BADSESSION
	}
	dir := args.BctsaDir()
	if args.BctsaUseConnInRdmaMode() != 0 || dir != CDFC4_FORE && dir != CDFC4_BACK &&
		dir != CDFC4_FORE_OR_BOTH && dir != CDFC4_BACK_OR_BOTH {
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	sess := s.sessions.lookup(id)
	if sess == nil || sess.destroyed.Load() || sess.fatal.Load() {
		ew.SetValue_Default(NFS4ERR_BADSESSION)
		w.Resume(ew.Finish())
		return NFS4ERR_BADSESSION
	}
	if status := s.sessions.admissionStatus(sess.clientID); status != NFS4_OK {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	fore, back, status := s.bindDirections(st, sess, dir)
	if status != NFS4_OK {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	// SP4_NONE permits any supported effective user; no registration-principal
	// restriction applies here.
	result := uint32(CDFS4_FORE)
	switch {
	case fore && back:
		result = CDFS4_BOTH
	case back:
		result = CDFS4_BACK
	}
	ok := ew.SetValue_Nfs4Ok()
	writeSessionID(ok.BctsrSessid(), id)
	ok.SetBctsrDir(result)
	ok.SetBctsrUseConnInRdmaMode(0)
	w.Resume(ew.Finish())
	return NFS4_OK
}

// bindDirections decides and applies a BIND_CONN_TO_SESSION (RFC 8881
// section 18.34.3, canonical r3 §12). Each request accepts these channel
// sets: FORE {fore}; BACK {back}; FORE_OR_BOTH {fore} or {fore, back};
// BACK_OR_BOTH {back} or {fore, back}. An existing association that is
// acceptable is returned unchanged, and one that is not gets INVAL: a BIND
// never changes an existing association. With none, the preferred set is
// established, both directions when the session asked for a backchannel and
// the transport can carry one; the back direction needs both. The decision
// and both quotas are taken with the publication, in one connection
// section, so concurrent BINDs are ordered and RESOURCE changes nothing.
func (s *Server) bindDirections(st *compoundState, sess *session, dir uint32) (fore, back bool, status uint32) {
	conn := st.connection
	if conn == nil {
		// No transport: only the fore channel, which needs no association.
		if !bindAcceptable(dir, true, false) {
			return false, false, NFS4ERR_INVAL
		}
		return true, false, NFS4_OK
	}
	canBack := sess.backRequested && conn.carriesCallbacks() && sess.cb != nil
	fore, back, added, status := conn.bindDirections(s, sess.id, dir, canBack)
	if status == NFS4_OK && added {
		// The new back connection is probed after this compound's reply.
		if probe := sess.cb.addConn(conn); probe != nil {
			st.later(probe)
		}
	}
	return fore, back, status
}

// bindAcceptable reports whether the channel set (fore, back) satisfies a
// BIND_CONN_TO_SESSION direction.
func bindAcceptable(dir uint32, fore, back bool) bool {
	switch dir {
	case CDFC4_FORE:
		return fore && !back
	case CDFC4_BACK:
		return !fore && back
	case CDFC4_FORE_OR_BOTH:
		return fore
	default: // CDFC4_BACK_OR_BOTH
		return back
	}
}

// commitBackChannel commits a reserved back association for sess. The new
// connection is probed after this compound's reply, so the client sees the
// reply that bound it before any callback.
func (s *Server) commitBackChannel(st *compoundState, bt *backToken, sess *session) {
	if !bt.commit() || sess.cb == nil {
		return
	}
	if probe := sess.cb.addConn(bt.conn); probe != nil {
		st.later(probe)
	}
}

// later runs f after the compound's reply is written, or at once for a
// compound with no transport.
func (st *compoundState) later(f func()) {
	if st.afterReply == nil {
		f()
		return
	}
	*st.afterReply = append(*st.afterReply, f)
}

// opBackchannelCtl replaces the session's callback program and credential.
func (s *Server) opBackchannelCtl(args BACKCHANNELCTL4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_BackchannelCtl()
	sec, status := parseCallbackSec(args.BcaSecParms())
	if status == NFS4_OK && (st.session == nil || st.session.cb == nil) {
		status = NFS4ERR_INVAL
	}
	if status == NFS4_OK {
		st.session.cb.setProgram(args.BcaCbProgram(), sec)
	}
	r.SetBcrStatus(status)
	return status
}

func (s *Server) opReclaimComplete(args RECLAIMCOMPLETE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_ReclaimComplete()
	if args.RcaOneFs() != 0 {
		// Per-filesystem completion is only meaningful for migration.
		r.SetRcrStatus(NFS4_OK)
		return NFS4_OK
	}
	// nfsd offers no grace period, so there is never anything to reclaim.
	s.sessions.mu.Lock()
	g := s.sessions.groups[st.clientID]
	already := g != nil && g.reclaimComplete
	if g != nil {
		g.reclaimComplete = true
	}
	s.sessions.mu.Unlock()
	if already {
		r.SetRcrStatus(NFS4ERR_COMPLETE_ALREADY)
		return NFS4ERR_COMPLETE_ALREADY
	}
	r.SetRcrStatus(NFS4_OK)
	return NFS4_OK
}

// stateIDStatus is clientID's status for a stateid. Another client's
// stateid, live, recovered or expired, is BAD_STATEID (canonical r3 §10).
func (s *Server) stateIDStatus(clientID uint64, stateid Stateid4) uint32 {
	id := extractStateID(stateid)
	if status := s.opens.status(clientID, id, stateid.Seqid()); status != NFS4ERR_BAD_STATEID {
		return status
	}
	// A write open from before a restart remains usable through its staging.
	if s.opens.canRecover(id) {
		for _, meta := range s.stagingStore.Entries() {
			if meta.NFSStateID == id && meta.ClientID == clientID {
				return NFS4_OK
			}
		}
	}
	return NFS4ERR_BAD_STATEID
}

func (s *Server) opTestStateID(args TESTSTATEID4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_TestStateid()
	ok := ew.SetValue_Nfs4Ok()
	for i := 0; i < int(args.TsStateidsCount()); i++ {
		ok.AppendTsrStatusCodes(s.stateIDStatus(st.clientID, args.TsStateids(i)))
	}
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opFreeStateID(args FREESTATEID4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_FreeStateid()
	stateid, ok := st.resolveStateID(args.FsaStateid())
	if !ok {
		r.SetFsrStatus(NFS4ERR_BAD_STATEID)
		return NFS4ERR_BAD_STATEID
	}
	// Live: open stateids are released by CLOSE. Expired and the caller's:
	// forget it. Unknown or another client's: BAD_STATEID, with no effect.
	status := s.stateIDStatus(st.clientID, stateid)
	switch status {
	case NFS4_OK:
		status = NFS4ERR_LOCKS_HELD
	case NFS4ERR_EXPIRED:
		status = NFS4_OK
		if !s.opens.forget(st.clientID, extractStateID(stateid)) {
			status = NFS4ERR_BAD_STATEID
		}
	case NFS4ERR_OLD_STATEID:
	default:
		status = NFS4ERR_BAD_STATEID
	}
	r.SetFsrStatus(status)
	return status
}

func (s *Server) opSecinfoNoName(style uint32, st *compoundState, w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_SecinfoNoName()
	if !st.currentIDSet {
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if style == SECINFO_STYLE4_PARENT && st.currentID == s.fs.RootID() {
		ew.SetValue_Default(NFS4ERR_NOENT)
		w.Resume(ew.Finish())
		return NFS4ERR_NOENT
	}
	okW := ew.SetValue_Nfs4Ok()
	okW.AppendData_Default(authSys)
	okW.AppendData_Default(authNone)
	ew.Resume(okW.Finish())
	w.Resume(ew.Finish())
	// SECINFO consumes the current filehandle (RFC 8881 section 2.6.3.1.1.8).
	st.currentIDSet = false
	return NFS4_OK
}
