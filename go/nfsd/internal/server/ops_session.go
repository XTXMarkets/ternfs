// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
)

// NFSv4.1 sessions (RFC 8881 section 2.10; README.md#sessions-and-slots).
// The slot table and its reply cache replace NFSv4.0 open-owner seqids, and
// requests on different slots run concurrently. Sessions are also kept on
// disk (session_persist.go; README.md#persistent-sessions).

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

const maxSessionCacheBytes = 64 << 20

// sessionlessOp reports operations which may start a compound without
// SEQUENCE. Each must then be the only operation (RFC 8881 section 2.10.6.2).
func sessionlessOp(opnum uint32) bool {
	switch opnum {
	case nfs4.OP_EXCHANGE_ID, nfs4.OP_CREATE_SESSION, nfs4.OP_DESTROY_SESSION,
		nfs4.OP_DESTROY_CLIENTID, nfs4.OP_BIND_CONN_TO_SESSION:
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
	// hasIntent: the request is run again after a restart and its slot held
	// an intent, whose payload is intent.
	hasIntent bool
	intent    []byte
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

// replayDigest identifies a compound's request for the false-retry check:
// everything but the SEQUENCE arguments. The slot, sequence number and
// session are checked on their own, and sa_highest_slotid and sa_cachethis
// are not part of the request's identity: the Linux client re-encodes a
// retransmission with its current highest slot in use, so hashing them
// turned every retransmission after a reconnect into SEQ_FALSE_RETRY,
// which the client answers by running the request again as a new one.
func replayDigest(body []byte) [32]byte {
	// COMPOUND4args: tag (length, bytes, padding), minorversion, count,
	// then the first operation: opnum and, for SEQUENCE, 32 bytes of
	// arguments.
	const sequenceArgs = 16 + 4 + 4 + 4 + 4
	if len(body) >= 4 {
		tagLen := int(binary.BigEndian.Uint32(body))
		argsOff := 4 + (tagLen+3)&^3 + 4 + 4 + 4
		if argsOff <= len(body) && argsOff+sequenceArgs <= len(body) &&
			binary.BigEndian.Uint32(body[argsOff-4:]) == nfs4.OP_SEQUENCE {
			h := sha256.New()
			h.Write(body[:argsOff])
			h.Write(body[argsOff+sequenceArgs:])
			var out [32]byte
			h.Sum(out[:0])
			return out
		}
	}
	return sha256.Sum256(body)
}

// startSequence validates SEQUENCE and claims its slot. The caller must
// call sequenceResult.finish once the compound reply is complete.
func (s *Server) startSequence(args nfs4.SEQUENCE4args, admission ...sequenceAdmission) sequenceResult {
	// Without an admission (tests only) the request has no identity; the
	// transport computes replayDigest over the compound.
	a := sequenceAdmission{operations: 1, sequenceReplyBytes: rpcAcceptedHeaderSize + 12 + 44}
	if len(admission) != 0 {
		a = admission[0]
	}
	id := readSessionID(args.SaSessionid())
	sess := s.sessions.lookup(id)
	// An unknown session is never reconstructed on demand (RFC 8881
	// §2.10.13.1.4); sessions with a file were restored at startup.
	if sess == nil || sess.destroyed.Load() || sess.fatal.Load() {
		return sequenceResult{status: nfs4.NFS4ERR_BADSESSION}
	}
	slotID := args.SaSlotid()
	if slotID >= uint32(len(sess.slots)) {
		return sequenceResult{status: nfs4.NFS4ERR_BADSLOT}
	}
	if args.SaHighestSlotid() < slotID || args.SaHighestSlotid() >= uint32(len(sess.slots)) {
		return sequenceResult{status: nfs4.NFS4ERR_BAD_HIGH_SLOT}
	}
	switch {
	case a.operations > min(sess.maxOps, uint32(maxCompoundOperations)):
		return sequenceResult{status: nfs4.NFS4ERR_TOO_MANY_OPS}
	case a.requestBytes > sess.maxReq:
		return sequenceResult{status: nfs4.NFS4ERR_REQ_TOO_BIG}
	case max(a.sequenceReplyBytes, a.uncachedBytes) > sess.maxResp:
		return sequenceResult{status: nfs4.NFS4ERR_REP_TOO_BIG}
	case args.SaCachethis() != 0 && max(a.sequenceReplyBytes, a.uncachedBytes) > sess.maxCached:
		return sequenceResult{status: nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE}
	}
	slot := &sess.slots[slotID]
	seq := args.SaSequenceid()

	// slot.seq is the sequence id of the request running or last completed
	// on the slot.
	if !slot.mu.TryLock() {
		return sequenceResult{status: nfs4.NFS4ERR_DELAY}
	}
	defer slot.mu.Unlock()
	replay := slot.occupied && seq == slot.seq
	// rerun: a retransmission to a restored slot whose reply was never
	// written. The request ran, or was about to, in the earlier process;
	// it runs again, deciding any intent it left (replay_decide.go).
	rerun := false
	switch {
	case replay:
		// Retransmission of the last request on this slot.
		if slot.inUse {
			return sequenceResult{status: nfs4.NFS4ERR_DELAY}
		}
		if slot.digest != a.digest || slot.user != a.user {
			return sequenceResult{status: nfs4.NFS4ERR_SEQ_FALSE_RETRY}
		}
		if slot.buffer == nil {
			if !slot.restored {
				return sequenceResult{status: nfs4.NFS4ERR_BADSESSION}
			}
			rerun = true
		}
	case !slot.inUse && (slot.occupied && seq == slot.seq+1 ||
		!slot.occupied && seq == 1):
	case !slot.inUse && slot.restored && sessionstore.SeqAfter(seq, slot.seq):
		// A later request's record on this slot was not written before the
		// restart. Only a new request can carry a later sequence number,
		// and running it is safe. While the session's file was on disk,
		// each request was permitted at most one mutating operation
		// (mutationGate). RENAME, REMOVE and CREATE of a directory or
		// symlink write an intent before theirs. The others run again as
		// new requests: an exclusive OPEN matches its verifier, a GUARDED
		// one gets EXIST, an upgrade OPEN gets a new stateid, a CLOSE finds
		// its publication through its candidate, and a CLOSE or
		// OPEN_DOWNGRADE whose staging was removed gets BAD_STATEID. The
		// ordinary rules apply from here on.
	default:
		return sequenceResult{status: nfs4.NFS4ERR_SEQ_MISORDERED}
	}

	// Admission (README.md#sequence-admission) holds slot.mu throughout and does no
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
	if status != nfs4.NFS4_OK {
		return sequenceResult{status: status}
	}
	undo = append(undo, hold.release)
	// Pin: while held, the session record cannot be released.
	if !s.sessions.useSession(sess) {
		return sequenceResult{status: nfs4.NFS4ERR_BADSESSION}
	}
	undo = append(undo, func() { s.sessions.releaseSessionUse(sess) })
	// Association, counted against the connection's quota while pending.
	token, ok := a.connection.reserve(s, id)
	if !ok {
		return sequenceResult{status: nfs4.NFS4ERR_RESOURCE}
	}
	undo = append(undo, token.abort)
	// Reply reservation: a new request charges a new buffer while the old
	// one stays attached and replayable.
	budget := sess.maxResp
	if args.SaCachethis() != 0 {
		budget = min(budget, sess.maxCached)
	}
	var reply *cachedReply
	if !replay || rerun {
		reservation := int(max(a.sequenceReplyBytes, a.uncachedBytes))
		if args.SaCachethis() != 0 {
			reservation = int(budget)
		}
		if reply, ok = s.sessions.reserveSlotReply(sess, slot, reservation); !ok {
			return sequenceResult{status: nfs4.NFS4ERR_RESOURCE}
		}
		undo = append(undo, reply.release)
	}
	// The gate again: expiry may have started deciding since the hold.
	if status := hold.gateStatus(); status != nfs4.NFS4_OK {
		return sequenceResult{status: status}
	}
	if sess.destroyed.Load() || sess.fatal.Load() {
		return sequenceResult{status: nfs4.NFS4ERR_BADSESSION}
	}

	// Commit, which cannot fail.
	committed = true
	token.commit()
	hold.activate()
	if replay && !rerun {
		if slot.restored {
			s.metrics.replayServed.Add(1)
		}
		slot.buffer.retain()
		s.sessions.releaseSessionUse(sess)
		return sequenceResult{status: nfs4.NFS4_OK, sess: sess, replay: slot.reply, view: slot.buffer, hold: hold}
	}
	s.sessions.installSlotReply(slot, reply)
	slot.seq, slot.occupied, slot.inUse = seq, true, true
	slot.cached = false
	slot.digest, slot.user = a.digest, a.user
	var intent []byte
	hasIntent := rerun && slot.hasIntent
	if hasIntent {
		intent = slot.intent
	}
	slot.restored, slot.hasIntent, slot.intent = false, false, nil
	return sequenceResult{
		hasIntent: hasIntent,
		intent:    intent,
		hold:      hold,
		status:    nfs4.NFS4_OK,
		sess:      sess,
		slot:      slot,
		slotID:    slotID,
		seq:       seq,
		cache:     args.SaCachethis() != 0,
		budget:    budget,
		store:     s.sessions,
	}
}

// finish records the reply for a request which claimed a slot.
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
	defer func() {
		// A session whose file is out of date and still on disk goes, now
		// that the slot is unlocked, for retiring drains every slot.
		if r.sess.file.poisoned.Load() && r.store != nil {
			r.store.retireSession(r.sess, true)
		}
	}()
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
			status := uint32(nfs4.NFS4ERR_RETRY_UNCACHED_REP)
			if op == nfs4.OP_ILLEGAL || mandatoryUnsupported(op) {
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
	if r.store != nil && r.store.persistSlot != nil {
		r.store.persistSlot(r.sess, r.slotID, sessionstore.Record{
			Seq: r.seq, Kind: sessionstore.KindSlot, CacheThis: r.cache,
			Digest: slot.digest, User: encodeUser(slot.user), Payload: saved,
		})
	}
}

func (s *Server) writeSequenceOK(w *nfs4.COMPOUND4resWriter, seq *sequenceResult) {
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

func writeSequenceError(w *nfs4.COMPOUND4resWriter, status uint32) {
	ew := w.AppendResarray_Sequence()
	ew.SetValue_Default(status)
	w.Resume(ew.Finish())
}

// serverOwner is the server owner major id and the server scope EXCHANGE_ID
// returns. Both come from the server id, so a client sees the same server
// after a restart or a move of the staging directory, and a new one after a
// wipe.
func (s *Server) serverOwner() []byte {
	return []byte("ternfs-nfsd/" + s.serverID.String())
}

func (s *Server) opExchangeID(
	args nfs4.EXCHANGEID4args,
	st *compoundState,
	w *nfs4.COMPOUND4resWriter,
) uint32 {
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_ExchangeId()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	flags := args.EiaFlags()
	if flags&^(nfs4.EXCHGID4_FLAG_SUPP_MOVED_REFER|nfs4.EXCHGID4_FLAG_SUPP_MOVED_MIGR|
		nfs4.EXCHGID4_FLAG_BIND_PRINC_STATEID|nfs4.EXCHGID4_FLAG_MASK_PNFS|
		nfs4.EXCHGID4_FLAG_UPD_CONFIRMED_REC_A) != 0 {
		return fail(nfs4.NFS4ERR_INVAL)
	}
	if args.EiaClientImplIdCount() > 1 {
		return fail(nfs4.NFS4ERR_BADXDR)
	}
	if args.EiaStateProtectType() != nfs4.SP4_NONE {
		return fail(nfs4.NFS4ERR_ENCR_ALG_UNSUPP)
	}
	owner := args.EiaClientowner()
	ownerKey := string(owner.CoOwnerid())
	verifier := verifierBytes(owner.CoVerifier())
	ss := s.sessions
	if flags&nfs4.EXCHGID4_FLAG_UPD_CONFIRMED_REC_A != 0 {
		// An update of a confirmed record (RFC 8881 section 18.35.5,
		// cases 6-9). nfsd has no per-client properties for an update to
		// change, so a valid update returns the record unchanged.
		ss.mu.Lock()
		var c *sessionClient
		if o := ss.owners[ownerKey]; o != nil {
			c = o.confirmed
		}
		ss.mu.Unlock()
		if c == nil {
			return fail(nfs4.NFS4ERR_NOENT)
		}
		// RFC 8881 section 18.35.5 cases 6-8 need a confirmed record
		// with the same verifier and principal.
		rec, ok := s.records.Lookup(clientrecords.HashOwner(owner.CoOwnerid()))
		switch {
		case !ok || uint64(rec.Incarnation) != c.incarnation:
			// This process's confirmed record is no longer the durable one.
			return fail(nfs4.NFS4ERR_NOENT)
		case rec.Verifier != verifier:
			return fail(nfs4.NFS4ERR_NOT_SAME)
		case rec.Principal != st.principal:
			return fail(nfs4.NFS4ERR_PERM)
		}
		return s.writeExchangeIDResult(w, c, true)
	}

	// One registration or confirmation at a time per owner, so the wire
	// record this request publishes is decided from the owner's current
	// record.
	unlockOwner := ss.ownerLocks.Lock(ownerKey)
	defer unlockOwner()

	// Case 2: a retransmission or a client that already holds the record.
	// The record and its creation sequence are unchanged; nothing is written.
	ss.mu.Lock()
	if o := ss.owners[ownerKey]; o != nil && o.confirmed != nil &&
		o.confirmed.verifier == verifier && o.confirmed.principal == st.principal &&
		!ss.expiredLocked(o.confirmed.incarnation) {
		c := o.confirmed
		ss.mu.Unlock()
		return s.writeExchangeIDResult(w, c, true)
	}
	if o := ss.owners[ownerKey]; o != nil && o.confirmed != nil && ss.expiredLocked(o.confirmed.incarnation) {
		// This process expired the record's incarnation: it is not returned
		// as live. The wire record is retired and registration starts again.
		ss.retireWireLocked(o.confirmed)
	}
	if status := s.wireCapacityLocked(0); status != nfs4.NFS4_OK {
		ss.mu.Unlock()
		return fail(status)
	}
	ss.mu.Unlock()

	incarnation, adopted, status := s.register(owner.CoOwnerid(), verifier, st.principal)
	if status != nfs4.NFS4_OK {
		return fail(status)
	}

	ss.mu.Lock()
	if incarnation == 0 {
		incarnation = s.newIncarnationLocked()
	}
	o := ss.owners[ownerKey]
	if adopted && o != nil && o.confirmed != nil && o.confirmed.incarnation == incarnation {
		// A concurrent CREATE_SESSION confirmed the record this request would
		// have adopted; answer as case 2.
		c := o.confirmed
		ss.mu.Unlock()
		return s.writeExchangeIDResult(w, c, true)
	}
	if g := ss.groups[incarnation]; g != nil && g.state == groupClosing {
		// Another client is replacing this incarnation.
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_DELAY)
	}
	// Case 4: a new EXCHANGE_ID supersedes the owner's unconfirmed record,
	// and an incarnation has one live record.
	if o != nil && o.unconfirmed != nil {
		ss.retireWireLocked(o.unconfirmed)
	}
	if g := ss.groups[incarnation]; g != nil && g.wire != nil {
		ss.retireWireLocked(g.wire)
	}
	if status := s.wireCapacityLocked(incarnation); status != nfs4.NFS4_OK {
		ss.mu.Unlock()
		return fail(status)
	}
	clientID, err := ss.wireIDs.next()
	if err != nil {
		ss.mu.Unlock()
		s.log.Error("cannot issue an EXCHANGE_ID clientid", "err", err)
		return fail(nfs4.NFS4ERR_SERVERFAULT)
	}
	// wireCapacityLocked checked the group credit under this lock.
	ss.groupLocked(incarnation)
	c := &sessionClient{
		clientID:    clientID,
		incarnation: incarnation,
		owner:       ownerKey,
		verifier:    verifier,
		principal:   st.principal,
		createdAt:   s.now(),
		adopted:     adopted,
	}
	ss.addWireLocked(c)
	ss.mu.Unlock()
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
		return nfs4.NFS4ERR_SERVERFAULT
	}
	admits := func() bool {
		return len(ss.clients) < ss.wireLimit && ss.groupAvailableLocked(incarnation)
	}
	if !admits() {
		ss.reclaimUnconfirmedLocked(s.now())
	}
	if !admits() {
		return nfs4.NFS4ERR_DELAY
	}
	return nfs4.NFS4_OK
}

func (s *Server) writeExchangeIDResult(w *nfs4.COMPOUND4resWriter, c *sessionClient, confirmed bool) uint32 {
	s.sessions.mu.Lock()
	nextSeq := c.csSeq + 1
	s.sessions.mu.Unlock()
	ew := w.AppendResarray_ExchangeId()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetEirClientid(c.clientID)
	ok.SetEirSequenceid(nextSeq)
	resFlags := uint32(nfs4.EXCHGID4_FLAG_USE_NON_PNFS)
	if confirmed {
		resFlags |= nfs4.EXCHGID4_FLAG_CONFIRMED_R
	}
	ok.SetEirFlags(resFlags)
	ok.SetEirStateProtectType(nfs4.SP4_NONE)
	ok.SetEirStateProtect_None()
	sow := ok.StartEirServerOwner()
	sow.SetSoMinorId(0)
	owner := s.serverOwner()
	ok.Resume(sow.SetSoMajorId(owner).Finish())
	ok.SetEirServerScope(owner)
	impl := ok.StartEirServerImplId()
	ok.Resume(impl.Finish())
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func readChannelAttrs(m nfs4.ChannelAttrs4) channelAttrs {
	return channelAttrs{
		headerPad: m.CaHeaderpadsize(),
		maxReq:    m.CaMaxrequestsize(),
		maxResp:   m.CaMaxresponsesize(),
		maxCached: m.CaMaxresponsesizeCached(),
		maxOps:    m.CaMaxoperations(),
		maxReqs:   m.CaMaxrequests(),
	}
}

func writeChannelAttrs(w nfs4.ChannelAttrs4Writer, a channelAttrs) []byte {
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
	args nfs4.CREATESESSION4args,
	st *compoundState,
	w *nfs4.COMPOUND4resWriter,
) uint32 {
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_CreateSession()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	clientID := args.CsaClientid()
	seq := args.CsaSequence()
	if args.CsaFlags()&^(nfs4.CREATE_SESSION4_FLAG_PERSIST|nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN|
		nfs4.CREATE_SESSION4_FLAG_CONN_RDMA) != 0 {
		return fail(nfs4.NFS4ERR_INVAL)
	}
	fore := readChannelAttrs(args.CsaForeChanAttrs())
	back := readChannelAttrs(args.CsaBackChanAttrs())
	if args.CsaForeChanAttrs().CaRdmaIrdCount() > 1 ||
		args.CsaBackChanAttrs().CaRdmaIrdCount() > 1 {
		return fail(nfs4.NFS4ERR_BADXDR)
	}
	if fore.maxReq < minSessionRequest || fore.maxResp < minSessionResponse ||
		back.maxReq < minSessionRequest || back.maxResp < minSessionResponse ||
		fore.maxOps < 2 {
		return fail(nfs4.NFS4ERR_TOOSMALL)
	}
	// The callback credential is checked before the slot, and only when a
	// backchannel is asked for: clients without one may offer none (libnfs
	// sends an empty list).
	backRequested := args.CsaFlags()&nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN != 0
	var cbSec callbackSec
	if backRequested {
		var status uint32
		if cbSec, status = parseCallbackSec(args.CsaSecParms()); status != nfs4.NFS4_OK {
			return fail(status)
		}
	}

	// This request's reservations are released when it returns. No
	// connection lock is held across record writes: the association is a
	// token, counted against the connection's quota from before the slot
	// (README.md#connection-associations).
	cr := &creation{s: s, seq: seq, back: back, cbProgram: args.CsaCbProgram(), cbSec: cbSec, backRequested: backRequested}
	var token *associationToken
	var backTok *backToken
	defer func() {
		token.abort() // a no-op once committed
		backTok.abort()
		cr.finish()
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
			return nfs4.NFS4_OK
		}
		t, ok := st.connection.reserve(s, reply.id)
		if !ok {
			return nfs4.NFS4ERR_RESOURCE
		}
		var bt *backToken
		if reply.back {
			if bt, ok = st.connection.reserveBack(s, reply.id); !ok {
				t.abort()
				return nfs4.NFS4ERR_RESOURCE
			}
		}
		t.commit()
		if sess := ss.lookup(reply.id); sess != nil && reply.back {
			s.commitBackChannel(st, bt, sess)
		} else {
			bt.abort()
		}
		return nfs4.NFS4_OK
	}
	// The clientid is looked up only among this process's wire records:
	// never issued, from another epoch or retired is STALE_CLIENTID, before
	// any slot or store effect.
	ss.mu.Lock()
	c := ss.wireRecordLocked(clientID)
	if c == nil {
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	// Outcomes that create nothing are decided from the slot before any
	// reservation, so they need no group credit, session record or serial,
	// and work at those limits: a replay of the cached outcome, and the
	// waiting and misordered sequences. A closing or replaced incarnation
	// gives the statuses beginCreate would. Only a new creation reserves,
	// and then checks the slot again below.
	var early, cached bool
	var earlyStatus uint32
	var reply createSessionResult
	switch g := ss.groups[c.incarnation]; {
	case g != nil && g.state == groupClosing:
		early, earlyStatus = true, nfs4.NFS4ERR_DELAY
	case g != nil && g.state == groupReplaced:
		early, earlyStatus = true, nfs4.NFS4ERR_STALE_CLIENTID
	case c.csBusy && seq == c.csBusySeq:
		early, earlyStatus = true, nfs4.NFS4ERR_DELAY
	case c.csCached && seq == c.csSeq:
		early, earlyStatus = true, c.csStatus
		cached, reply = c.csStatus == nfs4.NFS4_OK, c.csReply
	case c.csBusy || seq != c.csSeq+1:
		// Misordering never overwrites the cached outcome.
		early, earlyStatus = true, nfs4.NFS4ERR_SEQ_MISORDERED
	}
	ss.mu.Unlock()
	switch {
	case cached:
		// A cached result never rebuilds its session: once destroyed, the
		// session stays BADSESSION and the connection gains nothing. A
		// session already associated with the connection replays at the
		// quota; only a new association is refused there.
		if status := replayAssociation(reply); status != nfs4.NFS4_OK {
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
	if status != nfs4.NFS4_OK {
		return fail(status)
	}
	cr.endCreate = end
	// The bounded volatile session record and a non-reused local serial.
	id, admitted := ss.reserveSessionRecord(incarnation)
	if !admitted {
		return fail(nfs4.NFS4ERR_DELAY)
	}
	cr.id, cr.record = id, true
	// The new session's association on this connection, and its back
	// association when a backchannel is asked for and this transport can
	// carry one: both before the slot, so a full connection changes nothing.
	if token, admitted = st.connection.reserve(s, id); !admitted {
		return fail(nfs4.NFS4ERR_RESOURCE)
	}
	cr.backGrant = backRequested && st.connection.carriesCallbacks()
	if cr.backGrant {
		if backTok, admitted = st.connection.reserveBack(s, id); !admitted {
			return fail(nfs4.NFS4ERR_RESOURCE)
		}
	}

	// The creation slot, checked again now that the reservations are held:
	// it may have moved while unlocked. While a request owns it, the same
	// sequence waits, the cached sequence replays and any other is
	// misordered.
	ss.mu.Lock()
	if c.retired {
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	switch {
	case c.csBusy && seq == c.csBusySeq:
		// The request owning the slot is running.
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_DELAY)
	case c.csCached && seq == c.csSeq:
		reply, cachedStatus := c.csReply, c.csStatus
		ss.mu.Unlock()
		if cachedStatus != nfs4.NFS4_OK {
			return fail(cachedStatus)
		}
		// This request creates nothing: its own tokens go, and the
		// cached result's association follows the replay rule.
		token.abort()
		backTok.abort()
		if status := replayAssociation(reply); status != nfs4.NFS4_OK {
			return fail(status)
		}
		return writeCreateSessionResult(w, reply)
	case c.csBusy || seq != c.csSeq+1:
		// Misordering never overwrites the cached outcome.
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_SEQ_MISORDERED)
	}
	c.csBusy, c.csBusySeq = true, seq
	cr.slot = true // Any exit that caches no outcome rolls the slot back.
	confirmed, pending, principal := c.confirmed, c.pending(), c.principal
	ss.mu.Unlock()
	cr.fore = s.negotiateForeChannel(fore)
	if status := s.createSessionFile(cr); status != nfs4.NFS4_OK {
		return fail(status)
	}

	if pending {
		// A new registration: the principal check, the record, the purge
		// of the incarnation it replaces, then the session
		// (README.md#client-confirmation).
		status, reply := s.confirmPending(cr, st.principal)
		if status != nfs4.NFS4_OK {
			return fail(status)
		}
		return associate(reply)
	}
	if !confirmed && principal != st.principal {
		// An unconfirmed (adopted) record belongs to the principal that
		// registered it.
		ss.mu.Lock()
		cr.cacheLocked(nfs4.NFS4ERR_CLID_INUSE, createSessionResult{})
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_CLID_INUSE)
	}
	// An adopted or already confirmed record: check, without writing, that
	// the incarnation is still the confirmed one
	// (README.md#create_session-outcomes). If not, STALE_CLIENTID ends the
	// wire record and leaves its slot untouched, so no later EXCHANGE_ID
	// returns it as confirmed.
	if _, live := s.records.Live(clientrecords.Incarnation(incarnation)); !live {
		ss.mu.Lock()
		ss.retireWireLocked(c)
		ss.mu.Unlock()
		return fail(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	ss.mu.Lock()
	// A later EXCHANGE_ID may have retired the record while this request
	// ran (case 4); its delayed CREATE_SESSION joins no record.
	reply, inserted := cr.commitLocked()
	ss.mu.Unlock()
	if !inserted {
		return fail(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	return associate(reply)
}

func writeCreateSessionResult(w *nfs4.COMPOUND4resWriter, r createSessionResult) uint32 {
	ew := w.AppendResarray_CreateSession()
	ok := ew.SetValue_Nfs4Ok()
	writeSessionID(ok.CsrSessionid(), r.id)
	ok.SetCsrSequence(r.seq)
	var flags uint32
	if r.back {
		flags = nfs4.CREATE_SESSION4_FLAG_CONN_BACK_CHAN
	}
	ok.SetCsrFlags(flags)
	ok.Resume(writeChannelAttrs(ok.StartCsrForeChanAttrs(), r.foreAttrs))
	ok.Resume(writeChannelAttrs(ok.StartCsrBackChanAttrs(), r.backAttr))
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opDestroySession(args nfs4.DESTROYSESSION4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_DestroySession()
	id := readSessionID(args.DsaSessionid())
	if s.sessions.lookup(id) == nil {
		r.SetDsrStatus(nfs4.NFS4ERR_BADSESSION)
		return nfs4.NFS4ERR_BADSESSION
	}
	if !st.connection.associated(id) {
		r.SetDsrStatus(nfs4.NFS4ERR_CONN_NOT_BOUND_TO_SESSION)
		return nfs4.NFS4ERR_CONN_NOT_BOUND_TO_SESSION
	}
	if !s.sessions.destroySession(id) {
		r.SetDsrStatus(nfs4.NFS4ERR_BADSESSION)
		return nfs4.NFS4ERR_BADSESSION
	}
	r.SetDsrStatus(nfs4.NFS4_OK)
	return nfs4.NFS4_OK
}

func (s *Server) opDestroyClientID(args nfs4.DESTROYCLIENTID4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_DestroyClientid()
	s.sessions.mu.Lock()
	c := s.sessions.wireRecordLocked(args.DcaClientid())
	s.sessions.mu.Unlock()
	stale := func() uint32 {
		r.SetDcrStatus(nfs4.NFS4ERR_STALE_CLIENTID)
		return nfs4.NFS4ERR_STALE_CLIENTID
	}
	if c == nil {
		return stale()
	}
	// Ordered with the owner's registrations: no EXCHANGE_ID can replace
	// this record between the checks below and its destruction.
	unlockOwner := s.sessions.ownerLocks.Lock(c.owner)
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
	busy = busy || len(s.opens.ClientStates(incarnation)) != 0
	if busy {
		r.SetDcrStatus(nfs4.NFS4ERR_CLIENTID_BUSY)
		return nfs4.NFS4ERR_CLIENTID_BUSY
	}
	// The checks above can be overtaken by a CREATE_SESSION, which does not
	// take the owner lock; the final step repeats them under the retirement
	// lock.
	if status := s.sessions.destroyClientRecord(c); status != nfs4.NFS4_OK {
		r.SetDcrStatus(status)
		return status
	}
	s.expireClientState(incarnation)
	// The client is gone, and so is its record: a later registration of the
	// identity is a new client.
	if err := s.removeRecord(incarnation); err != nil {
		s.log.Error("destroyed client's record not removed", "incarnation", incarnationText(incarnation), "err", err)
		r.SetDcrStatus(nfs4.NFS4ERR_SERVERFAULT)
		return nfs4.NFS4ERR_SERVERFAULT
	}
	r.SetDcrStatus(nfs4.NFS4_OK)
	return nfs4.NFS4_OK
}

func (s *Server) opBindConnToSession(args nfs4.BINDCONNTOSESSION4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_BindConnToSession()
	id := readSessionID(args.BctsaSessid())
	if s.sessions.lookup(id) == nil {
		// Unknown, including a session from before an nfsd restart that
		// has no file: the client creates a new one.
		ew.SetValue_Default(nfs4.NFS4ERR_BADSESSION)
		w.Resume(ew.Finish())
		return nfs4.NFS4ERR_BADSESSION
	}
	dir := args.BctsaDir()
	if args.BctsaUseConnInRdmaMode() != 0 || dir != nfs4.CDFC4_FORE && dir != nfs4.CDFC4_BACK &&
		dir != nfs4.CDFC4_FORE_OR_BOTH && dir != nfs4.CDFC4_BACK_OR_BOTH {
		ew.SetValue_Default(nfs4.NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return nfs4.NFS4ERR_INVAL
	}
	sess := s.sessions.lookup(id)
	if sess == nil || sess.destroyed.Load() || sess.fatal.Load() {
		ew.SetValue_Default(nfs4.NFS4ERR_BADSESSION)
		w.Resume(ew.Finish())
		return nfs4.NFS4ERR_BADSESSION
	}
	if status := s.sessions.admissionStatus(sess.clientID); status != nfs4.NFS4_OK {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	fore, back, status := s.bindDirections(st, sess, dir)
	if status != nfs4.NFS4_OK {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	// SP4_NONE permits any supported effective user; no registration-principal
	// restriction applies here.
	result := uint32(nfs4.CDFS4_FORE)
	switch {
	case fore && back:
		result = nfs4.CDFS4_BOTH
	case back:
		result = nfs4.CDFS4_BACK
	}
	ok := ew.SetValue_Nfs4Ok()
	writeSessionID(ok.BctsrSessid(), id)
	ok.SetBctsrDir(result)
	ok.SetBctsrUseConnInRdmaMode(0)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

// bindDirections decides and applies a BIND_CONN_TO_SESSION (RFC 8881 section
// 18.34.3; README.md#grants-and-binding). Each request accepts these channel
// sets: FORE {fore}; BACK {back}; FORE_OR_BOTH {fore} or {fore, back};
// BACK_OR_BOTH {back} or {fore, back}. An existing association that is
// acceptable is returned unchanged, and one that is not gets INVAL: a BIND
// never changes an existing association. With none, FORE and BACK establish
// their own channel. FORE_OR_BOTH and BACK_OR_BOTH establish both when the
// session asked for a backchannel and the transport can carry one; otherwise
// FORE_OR_BOTH establishes the fore channel. The back direction needs both,
// so BACK and BACK_OR_BOTH get INVAL without them. The decision and both
// quotas are taken with the publication, in one connection section, so
// concurrent BINDs are ordered and RESOURCE changes nothing.
func (s *Server) bindDirections(st *compoundState, sess *session, dir uint32) (fore, back bool, status uint32) {
	conn := st.connection
	if conn == nil {
		// No transport: only the fore channel, which needs no association.
		if !bindAcceptable(dir, true, false) {
			return false, false, nfs4.NFS4ERR_INVAL
		}
		return true, false, nfs4.NFS4_OK
	}
	canBack := sess.backRequested && conn.carriesCallbacks() && sess.cb != nil
	fore, back, added, status := conn.bindDirections(s, sess.id, dir, canBack)
	if status == nfs4.NFS4_OK && added {
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
	case nfs4.CDFC4_FORE:
		return fore && !back
	case nfs4.CDFC4_BACK:
		return !fore && back
	case nfs4.CDFC4_FORE_OR_BOTH:
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

// opBackchannelCtl replaces the session's callback program and credential.
func (s *Server) opBackchannelCtl(args nfs4.BACKCHANNELCTL4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_BackchannelCtl()
	sec, status := parseCallbackSec(args.BcaSecParms())
	if status == nfs4.NFS4_OK && (st.session == nil || st.session.cb == nil) {
		status = nfs4.NFS4ERR_INVAL
	}
	// The session's file records the program and credential CREATE_SESSION
	// gave: it goes before they change, or they do not. A file that stays
	// retires the session when the request finishes.
	if status == nfs4.NFS4_OK && !s.dropSessionFile(st.session, "BACKCHANNEL_CTL") {
		status = nfs4.NFS4ERR_SERVERFAULT
	}
	if status == nfs4.NFS4_OK {
		st.session.cb.setProgram(args.BcaCbProgram(), sec)
	}
	r.SetBcrStatus(status)
	return status
}

func (s *Server) opReclaimComplete(args nfs4.RECLAIMCOMPLETE4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_ReclaimComplete()
	if args.RcaOneFs() != 0 {
		// Per-filesystem completion is only meaningful for migration.
		r.SetRcrStatus(nfs4.NFS4_OK)
		return nfs4.NFS4_OK
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
		r.SetRcrStatus(nfs4.NFS4ERR_COMPLETE_ALREADY)
		return nfs4.NFS4ERR_COMPLETE_ALREADY
	}
	r.SetRcrStatus(nfs4.NFS4_OK)
	return nfs4.NFS4_OK
}

// stateIDStatus is clientID's status for a stateid. Another client's
// stateid, live, recovered or expired, is BAD_STATEID
// (../openstate/README.md#stateid-scope).
func (s *Server) stateIDStatus(clientID uint64, stateid nfs4.Stateid4) uint32 {
	id := extractStateID(stateid)
	if status := s.opens.Status(clientID, id, stateid.Seqid()); status != nfs4.NFS4ERR_BAD_STATEID {
		return status
	}
	// A write open from before a restart remains usable through its staging.
	if s.opens.CanRecover(id) {
		for _, meta := range s.stagingStore.Entries() {
			if meta.NFSStateID == id && meta.ClientID == clientID {
				return nfs4.NFS4_OK
			}
		}
	}
	return nfs4.NFS4ERR_BAD_STATEID
}

func (s *Server) opTestStateID(args nfs4.TESTSTATEID4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_TestStateid()
	ok := ew.SetValue_Nfs4Ok()
	for i := 0; i < int(args.TsStateidsCount()); i++ {
		ok.AppendTsrStatusCodes(s.stateIDStatus(st.clientID, args.TsStateids(i)))
	}
	ew.Resume(ok.Finish())
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opFreeStateID(args nfs4.FREESTATEID4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_FreeStateid()
	stateid, ok := st.resolveStateID(args.FsaStateid())
	if !ok {
		r.SetFsrStatus(nfs4.NFS4ERR_BAD_STATEID)
		return nfs4.NFS4ERR_BAD_STATEID
	}
	// Live: open stateids are released by CLOSE. Expired and the caller's:
	// forget it. Unknown or another client's: BAD_STATEID, with no effect.
	status := s.stateIDStatus(st.clientID, stateid)
	switch status {
	case nfs4.NFS4_OK:
		status = nfs4.NFS4ERR_LOCKS_HELD
	case nfs4.NFS4ERR_EXPIRED:
		status = nfs4.NFS4_OK
		if !s.opens.Forget(st.clientID, extractStateID(stateid)) {
			status = nfs4.NFS4ERR_BAD_STATEID
		}
	case nfs4.NFS4ERR_OLD_STATEID:
	default:
		status = nfs4.NFS4ERR_BAD_STATEID
	}
	r.SetFsrStatus(status)
	return status
}

func (s *Server) opSecinfoNoName(style uint32, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_SecinfoNoName()
	if !st.currentIDSet {
		ew.SetValue_Default(nfs4.NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return nfs4.NFS4ERR_NOFILEHANDLE
	}
	if style == nfs4.SECINFO_STYLE4_PARENT && st.currentID == s.fs.RootID() {
		ew.SetValue_Default(nfs4.NFS4ERR_NOENT)
		w.Resume(ew.Finish())
		return nfs4.NFS4ERR_NOENT
	}
	okW := ew.SetValue_Nfs4Ok()
	okW.AppendData_Default(nfs4.AUTH_SYS)
	okW.AppendData_Default(nfs4.AUTH_NONE)
	ew.Resume(okW.Finish())
	w.Resume(ew.Finish())
	// SECINFO consumes the current filehandle (RFC 8881 section 2.6.3.1.1.8).
	st.currentIDSet = false
	return nfs4.NFS4_OK
}

type createSessionResult struct {
	id                  SessionID
	seq                 uint32
	foreAttrs, backAttr channelAttrs
	back                bool // CONN_BACK_CHAN granted
}
