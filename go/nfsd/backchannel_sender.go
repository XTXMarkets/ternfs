// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"sync"
	"time"
)

// The backchannel sender (canonical r3 §12). nfsd sends only CB_NULL, to
// probe a back connection when it appears, and CB_COMPOUND carrying
// CB_SEQUENCE and CB_RECALL_SLOT, to ask a session to use fewer slots
// while the reply cache is under pressure.
//
// Each session has one sender, which runs a goroutine only while it has a
// recall to deliver. It uses slot 0, with its sequence starting at 1, and
// has one CB_COMPOUND outstanding at a time. The goroutine holds a use of
// the session, so the session's record credit returns only after it exits;
// detaching the session (DESTROY_SESSION, expiry, dropClient) cancels it,
// and it removes its outstanding call before exiting. A late reply is then
// an unknown XID.
//
// An unanswered call is never resent while its connection is up: a
// timeout only marks the path unresponsive. After a disconnect the same
// slot and sequence go out on another back connection that is up, or wait
// for a BIND. nfsd never closes a connection to retry.

const (
	// cbProgramVersion is the callback RPC version: NFS_CB = 1 in the
	// official NFSv4.1 XDR (nfs.x), which Linux sends and accepts. RFC 8881
	// section 18.36.3's prose says 4; the XDR and implementations use 1.
	cbProgramVersion = 1
	cbProcNull       = 0
	cbProcCompound   = 1
	cbMaxDelays      = 3
)

// Variables so tests can shorten them.
var (
	cbTimeout      = 10 * time.Second
	cbDelayBackoff = 100 * time.Millisecond
)

type cbPath uint8

const (
	// cbPathProbing: the CB_NULL has not been answered yet. Until it is,
	// or its timeout ends this grace, it counts as up for the status flags
	// only, so a client is not told to BIND again while the first probe is
	// in flight. Nothing is sent on it.
	cbPathProbing cbPath = iota
	cbPathUp
	cbPathUnresponsive
)

// cbSender is a session's backchannel state. mu is never held across a
// send or a wait, and is taken after sessionStore.mu.
type cbSender struct {
	s    *Server
	sess *session

	mu       sync.Mutex
	program  uint32
	sec      callbackSec
	conns    []*rpcConnection // back connections, in BIND order
	seq      uint32           // the next CB_SEQUENCE sequence id
	fault    bool             // BACKCHANNEL_FAULT: nothing more is sent
	recall   bool             // a CB_RECALL_SLOT is pending
	target   uint32           // its target highest slot id
	inFlight bool             // a CB_COMPOUND is outstanding
	// lastRecallStatus is the CB_RECALL_SLOT status of the last validated
	// reply, for diagnostics: an error there is the client refusing the
	// recall, not a delivered recall.
	lastRecallStatus uint32
	running          bool
	cancelled        bool
	cancel           chan struct{}
	wake             chan struct{}
}

func newCBSender(s *Server, sess *session, program uint32, sec callbackSec) *cbSender {
	return &cbSender{s: s, sess: sess, program: program, sec: sec, seq: 1,
		cancel: make(chan struct{}), wake: make(chan struct{}, 1)}
}

func (cb *cbSender) kick() {
	select {
	case cb.wake <- struct{}{}:
	default:
	}
}

// cancelLocked stops the sender for good. The caller holds
// sessionStore.mu, while it detaches the session.
func (cb *cbSender) cancelLocked() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if !cb.cancelled {
		cb.cancelled = true
		close(cb.cancel)
	}
}

// setProgram replaces the callback program and credential
// (BACKCHANNEL_CTL).
func (cb *cbSender) setProgram(program uint32, sec callbackSec) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.program, cb.sec = program, sec
}

// addConn records a back connection. It returns the CB_NULL probe to start
// once the reply that bound the connection has been written, or nil.
func (cb *cbSender) addConn(c *rpcConnection) func() {
	cb.mu.Lock()
	for _, have := range cb.conns {
		if have == c {
			cb.mu.Unlock()
			return nil
		}
	}
	cb.conns = append(cb.conns, c)
	program, sec := cb.program, cb.sec
	cb.mu.Unlock()
	return func() {
		cb.kick()
		go cb.s.probe(c, program, sec)
	}
}

// removeConn forgets a connection that has gone.
func (cb *cbSender) removeConn(c *rpcConnection) {
	cb.mu.Lock()
	for i, have := range cb.conns {
		if have == c {
			cb.conns = append(cb.conns[:i], cb.conns[i+1:]...)
			break
		}
	}
	cb.mu.Unlock()
	cb.kick()
}

// pickConn returns a back connection whose path is up: one that has
// answered a validated callback. Callbacks, resends included, go only there.
func (cb *cbSender) pickConn() *rpcConnection {
	cb.mu.Lock()
	conns := append([]*rpcConnection(nil), cb.conns...)
	cb.mu.Unlock()
	for _, c := range conns {
		if c.backPath() == cbPathUp {
			return c
		}
	}
	return nil
}

// pathUp reports, for the status flags only, a back connection that is up
// or still within its first probe's grace.
func (cb *cbSender) pathUp() bool {
	cb.mu.Lock()
	conns := append([]*rpcConnection(nil), cb.conns...)
	cb.mu.Unlock()
	for _, c := range conns {
		if path := c.backPath(); path == cbPathUp || path == cbPathProbing {
			return true
		}
	}
	return false
}

// flags is the sender's part of SEQUENCE's status flags.
func (cb *cbSender) flags() (fault, pending bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.fault, cb.recall || cb.inFlight
}

// requestRecall asks the session to use slots up to target. A sender that
// is idle starts a goroutine, holding a use of the session.
func (cb *cbSender) requestRecall(target uint32) {
	cb.mu.Lock()
	if cb.cancelled || cb.fault {
		cb.mu.Unlock()
		return
	}
	cb.recall, cb.target = true, target
	start := !cb.running
	if start {
		cb.running = true
	}
	cb.mu.Unlock()
	if !start {
		cb.kick()
		return
	}
	if !cb.s.sessions.useSession(cb.sess) {
		cb.mu.Lock()
		cb.running = false
		cb.mu.Unlock()
		return
	}
	go cb.run()
}

func (cb *cbSender) run() {
	defer cb.s.sessions.releaseSessionUse(cb.sess)
	for {
		cb.mu.Lock()
		if cb.cancelled || cb.fault || !cb.recall {
			cb.running = false
			cb.mu.Unlock()
			return
		}
		target, seq := cb.target, cb.seq
		cb.inFlight = true
		cb.mu.Unlock()
		done := cb.deliver(seq, target)
		cb.mu.Lock()
		cb.inFlight = false
		if done && cb.target == target {
			cb.recall = false
		}
		cb.mu.Unlock()
	}
}

// cbOutcome is what a CB_COMPOUND's reply decided.
type cbOutcome uint8

const (
	cbDone          cbOutcome = iota // CB_SEQUENCE and CB_RECALL_SLOT succeeded
	cbRecallRefused                  // CB_SEQUENCE succeeded, CB_RECALL_SLOT returned an error
	cbRetry                          // an actual CB_SEQUENCE DELAY
	cbFault                          // any other error, or a malformed reply
	cbUnfit                          // not sent: the frame exceeds the back channel's limits
)

// deliver sends one CB_COMPOUND until it is answered, the sender is
// cancelled, or it faults. It reports whether the recall is finished with.
func (cb *cbSender) deliver(seq, target uint32) bool {
	delays := 0
	for {
		conn := cb.pickConn()
		if conn == nil {
			// No back connection: wait for a BIND.
			select {
			case <-cb.wake:
				continue
			case <-cb.cancel:
				return true
			}
		}
		outcome, ok := cb.call(conn, seq, target)
		if !ok {
			// Cancelled, or the connection went: in that case the same
			// slot and sequence go out again elsewhere.
			select {
			case <-cb.cancel:
				return true
			default:
				continue
			}
		}
		switch outcome {
		case cbDone, cbRecallRefused, cbUnfit:
			return true
		case cbFault:
			cb.mu.Lock()
			cb.fault = true
			cb.mu.Unlock()
			return true
		}
		// An actual DELAY reply: a new request, with the same sequence,
		// since CB_SEQUENCE errors leave the slot unchanged.
		if delays++; delays > cbMaxDelays {
			return true
		}
		select {
		case <-time.After(cbDelayBackoff << (delays - 1)):
		case <-cb.cancel:
			return true
		}
	}
}

// call sends one CB_COMPOUND on conn and waits for its reply. ok is false
// when the sender was cancelled or the connection went before a reply.
func (cb *cbSender) call(conn *rpcConnection, seq, target uint32) (cbOutcome, bool) {
	cb.mu.Lock()
	program, sec := cb.program, cb.sec
	cb.mu.Unlock()
	call := newCBCall(cb.s.nextCBXID(), cb.sess.id, false)
	frame := cbCallFrame(call.xid, program, cbProcCompound, sec, cbRecallSlotArgs(cb.sess.id, seq, target))
	if !cbFrameFits(cb.sess, len(frame)) {
		// The credential grew past the negotiated request size since the
		// recall was queued: it is dropped, unsent, consuming nothing.
		return cbUnfit, true
	}
	if !conn.registerCall(call) {
		return cbRetry, false
	}
	if !conn.send(frame) {
		conn.cancelCall(call)
		return cbRetry, false
	}
	timer := time.NewTimer(cbTimeout)
	defer timer.Stop()
	timeout := timer.C
	for {
		select {
		case reply := <-call.reply:
			r := parseRecallReply(reply, cb.sess.id, seq)
			if r.malformed {
				// Not lease evidence, not a delivered recall, and no
				// evidence the path works.
				return cbFault, true
			}
			// A complete, validated reply: the connection answers.
			conn.setBackPath(cbPathUp)
			cb.s.kickBackSessions(conn)
			outcome := r.outcome()
			if outcome == cbDone || outcome == cbRecallRefused {
				cb.mu.Lock()
				cb.seq = seq + 1
				cb.lastRecallStatus = r.recallStatus
				cb.mu.Unlock()
				// Renew while this goroutine still holds its session use.
				cb.s.sessions.callbackRenew(cb.sess.clientID, cb.s.sessions.now())
				cb.s.clients.noteLeaseNeed(cb.sess.clientID)
			}
			return outcome, true
		case <-timeout:
			// Not resent while the connection is up.
			conn.setBackPath(cbPathUnresponsive)
			timeout = nil
		case <-conn.closed:
			conn.cancelCall(call)
			return cbRetry, false
		case <-call.done:
			return cbRetry, false
		case <-cb.cancel:
			conn.cancelCall(call)
			return cbRetry, false
		}
	}
}

// probe sends CB_NULL on a new back connection. A validated reply (RPC
// accepted, SUCCESS, an empty result) marks the path up. A timeout only ends
// the probing grace: the path becomes unresponsive and the call stays
// outstanding, with this goroutine, until a reply, a disconnect, or the
// connection losing its last back association removes it.
func (s *Server) probe(conn *rpcConnection, program uint32, sec callbackSec) {
	call := newCBCall(s.nextCBXID(), SessionID{}, true)
	if !conn.registerCall(call) {
		return // One is already outstanding.
	}
	if !conn.send(cbCallFrame(call.xid, program, cbProcNull, sec, nil)) {
		conn.cancelCall(call)
		return
	}
	timer := time.NewTimer(cbTimeout)
	defer timer.Stop()
	timeout := timer.C
	for {
		select {
		case reply := <-call.reply:
			if body, ok := acceptedReplyBody(reply); ok && len(body) == 0 {
				conn.setBackPath(cbPathUp)
				s.log.Info("backchannel path up", "remote", conn.remote, "xid", call.xid,
					"program", program, "version", cbProgramVersion, "procedure", "CB_NULL")
			} else {
				conn.setBackPath(cbPathUnresponsive)
				s.log.Info("backchannel path unresponsive", "remote", conn.remote, "xid", call.xid,
					"reason", "CB_NULL reply not accepted")
			}
			s.kickBackSessions(conn)
			return
		case <-timeout:
			// A call already removed, answered or cancelled, no longer
			// speaks for the path.
			if conn.timeoutPath(call) {
				s.log.Info("backchannel path unresponsive", "remote", conn.remote, "xid", call.xid,
					"reason", "CB_NULL timed out")
			}
			timeout = nil
		case <-call.done:
			return
		case <-conn.closed:
			conn.cancelCall(call)
			return
		}
	}
}

func (s *Server) nextCBXID() uint32 {
	return s.cbXIDs.Add(1)
}

// kickBackSessions wakes the senders of the sessions conn carries.
func (s *Server) kickBackSessions(conn *rpcConnection) {
	for _, id := range conn.backSessions() {
		if sess := s.sessions.lookup(id); sess != nil && sess.cb != nil {
			sess.cb.kick()
		}
	}
}

// disconnectBack runs when a connection ends: its sessions forget it, and
// their senders resend on another connection or wait for a BIND.
func (s *Server) disconnectBack(conn *rpcConnection) {
	for _, id := range conn.backSessions() {
		if sess := s.sessions.lookup(id); sess != nil && sess.cb != nil {
			sess.cb.removeConn(conn)
		}
	}
}

// cbCallFrame is an ONC RPC CALL to the client's callback program.
func cbCallFrame(xid, program, proc uint32, sec callbackSec, args []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, xid)
	b = binary.BigEndian.AppendUint32(b, rpcCall)
	b = binary.BigEndian.AppendUint32(b, rpcVersion)
	b = binary.BigEndian.AppendUint32(b, program)
	b = binary.BigEndian.AppendUint32(b, cbProgramVersion)
	b = binary.BigEndian.AppendUint32(b, proc)
	b = binary.BigEndian.AppendUint32(b, sec.flavor)
	b = binary.BigEndian.AppendUint32(b, uint32(len(sec.body)))
	b = append(b, sec.body...)
	b = binary.BigEndian.AppendUint32(b, authNone)
	b = binary.BigEndian.AppendUint32(b, 0)
	return append(b, args...)
}

// cbRecallSlotArgs is CB_COMPOUND (minor version 1) carrying CB_SEQUENCE
// on slot 0 (highest 0, not cached, no referring calls) and
// CB_RECALL_SLOT.
func cbRecallSlotArgs(id SessionID, seq, target uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, 0) // empty tag
	b = binary.BigEndian.AppendUint32(b, 1)    // minor version
	b = binary.BigEndian.AppendUint32(b, 0)    // callback_ident, unused in 4.1
	b = binary.BigEndian.AppendUint32(b, 2)
	b = binary.BigEndian.AppendUint32(b, OP_CB_SEQUENCE)
	b = append(b, id[:]...)
	b = binary.BigEndian.AppendUint32(b, seq)
	b = binary.BigEndian.AppendUint32(b, 0) // slot
	b = binary.BigEndian.AppendUint32(b, 0) // highest slot
	b = binary.BigEndian.AppendUint32(b, 0) // cachethis
	b = binary.BigEndian.AppendUint32(b, 0) // referring call lists
	b = binary.BigEndian.AppendUint32(b, OP_CB_RECALL_SLOT)
	return binary.BigEndian.AppendUint32(b, target)
}

// cbRecallSlotFits reports whether the CB_COMPOUND fits the session's
// negotiated back channel.
func cbRecallSlotFits(sess *session) bool {
	cb := sess.cb
	cb.mu.Lock()
	sec := cb.sec
	cb.mu.Unlock()
	return cbFrameFits(sess, len(cbCallFrame(0, 0, cbProcCompound, sec, cbRecallSlotArgs(sess.id, 0, 0))))
}

// cbFrameFits checks a CB_COMPOUND of size bytes, and its reply, against the
// session's negotiated back channel: at least one slot, two operations, the
// request size and the reply size.
func cbFrameFits(sess *session, size int) bool {
	const reply = 24 + 4 + 4 + 4 + (4 + 4 + 16 + 16) + (4 + 4)
	return sess.backMaxReqs >= 1 && sess.backMaxOps >= 2 &&
		uint32(size) <= sess.backMaxReq && reply <= sess.backMaxResp
}

// acceptedReplyBody returns the results of an accepted, successful REPLY.
func acceptedReplyBody(frame []byte) ([]byte, bool) {
	if len(frame) < 24 || binary.BigEndian.Uint32(frame[8:12]) != 0 { // MSG_ACCEPTED
		return nil, false
	}
	verfLen := binary.BigEndian.Uint32(frame[16:20])
	off := 20 + int((verfLen+3)&^3)
	if verfLen > 400 || len(frame) < off+4 || binary.BigEndian.Uint32(frame[off:]) != acceptSuccess {
		return nil, false
	}
	return frame[off+4:], true
}

// recallReply is a parsed CB_COMPOUND reply to CB_SEQUENCE plus
// CB_RECALL_SLOT.
type recallReply struct {
	malformed    bool
	seqStatus    uint32
	recallStatus uint32
}

// parseRecallReply validates a whole CB_COMPOUND reply before any of it is
// used: an accepted RPC reply; a compound whose status is that of its first
// failing operation, or OK; CB_SEQUENCE first, answering for this session,
// sequence and slot 0 when OK; when CB_SEQUENCE succeeded, a CB_RECALL_SLOT
// result after it; nothing more. Anything else is malformed.
func parseRecallReply(frame []byte, id SessionID, seq uint32) recallReply {
	bad := recallReply{malformed: true}
	body, ok := acceptedReplyBody(frame)
	if !ok || len(body) < 8 {
		return bad
	}
	u32 := func(off int) (uint32, bool) {
		if off < 0 || len(body) < off+4 {
			return 0, false
		}
		return binary.BigEndian.Uint32(body[off:]), true
	}
	compound, _ := u32(0)
	tagLen, _ := u32(4)
	if tagLen > 1024 {
		return bad
	}
	off := 8 + int((tagLen+3)&^3)
	count, ok := u32(off)
	if !ok || count < 1 || count > 2 {
		return bad
	}
	op, ok1 := u32(off + 4)
	seqStatus, ok2 := u32(off + 8)
	if !ok1 || !ok2 || op != OP_CB_SEQUENCE {
		return bad
	}
	off += 12
	r := recallReply{seqStatus: seqStatus}
	if seqStatus != NFS4_OK {
		// Processing stops at the failing operation.
		if count != 1 || compound != seqStatus || len(body) != off {
			return bad
		}
		return r
	}
	if len(body) < off+32 {
		return bad
	}
	var got SessionID
	copy(got[:], body[off:off+16])
	gotSeq, _ := u32(off + 16)
	slot, _ := u32(off + 20)
	if got != id || gotSeq != seq || slot != 0 {
		return bad
	}
	off += 32
	if count != 2 {
		return bad
	}
	op, ok1 = u32(off)
	recallStatus, ok2 := u32(off + 4)
	if !ok1 || !ok2 || op != OP_CB_RECALL_SLOT || len(body) != off+8 || compound != recallStatus {
		return bad
	}
	r.recallStatus = recallStatus
	return r
}

// outcome maps a validated reply to what the sender does next.
func (r recallReply) outcome() cbOutcome {
	switch {
	case r.malformed:
		return cbFault
	case r.seqStatus == NFS4ERR_DELAY:
		return cbRetry
	case r.seqStatus != NFS4_OK:
		return cbFault
	case r.recallStatus != NFS4_OK:
		return cbRecallRefused
	}
	return cbDone
}

// callbackRenew renews the logical lease for an accepted CB_SEQUENCE
// reply, under the gate (RFC 8881 §20.9.3): while live, and while expiry
// is still deciding, when the renewal makes decideExpired give way; not
// once the marker is submitted or the client expired.
func (ss *sessionStore) callbackRenew(clientID uint64, received time.Time) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[clientID]
	if g == nil {
		return
	}
	switch {
	case g.lease == gateLive:
	case g.lease == gateChecking && g.markerPhase == markerDeciding:
		g.renewedDuringCheck = true
	default:
		return
	}
	if quiet := received.Add(nfsLeaseTime); quiet.After(g.quietUntil) {
		g.quietUntil = quiet
	}
}

// sequenceFlagsFrom combines the observations into SEQUENCE's status flags.
// fault, pending and up are the session's own sender and path state;
// clientDown is clientPathDown for its incarnation. The request path and
// the diagnostics snapshot share it, so both report the same meanings.
func sequenceFlagsFrom(fault, pending, up, backRequested, clientDown bool) uint32 {
	var flags uint32
	if fault {
		flags |= SEQ4_STATUS_BACKCHANNEL_FAULT
	}
	if (backRequested || pending) && !up {
		flags |= SEQ4_STATUS_CB_PATH_DOWN_SESSION
	}
	if !up && clientDown {
		flags |= SEQ4_STATUS_CB_PATH_DOWN
	}
	return flags
}

// sequenceFlags is SEQUENCE's status flags for sess, from memory.
func (s *Server) sequenceFlags(sess *session) uint32 {
	cb := sess.cb
	if cb == nil {
		return 0
	}
	fault, pending := cb.flags()
	up := cb.pathUp()
	return sequenceFlagsFrom(fault, pending, up, sess.backRequested, !up && s.sessions.clientPathDown(sess.clientID))
}

// clientPathDown reports that some session of the client asked for a
// backchannel and none has one up.
func (ss *sessionStore) clientPathDown(clientID uint64) bool {
	ss.mu.Lock()
	var mine []*session
	for _, sess := range ss.sessions {
		if sess.clientID == clientID && sess.cb != nil {
			mine = append(mine, sess)
		}
	}
	ss.mu.Unlock()
	requested := false
	for _, sess := range mine {
		if sess.cb.pathUp() {
			return false
		}
		requested = requested || sess.backRequested
	}
	return requested
}

// slotTarget is SEQUENCE's target highest slot id. While the reply cache
// is at least three quarters full, a session charged more than its share
// is asked to use half its slots, once by CB_RECALL_SLOT if its back
// channel can carry one; it keeps that target until the cache is below
// half full.
func (s *Server) slotTarget(sess *session) uint32 {
	highest := uint32(len(sess.slots) - 1)
	ss := s.sessions
	ss.cacheMu.Lock()
	cached, limit := ss.cachedBytes, ss.cacheLimit
	ss.cacheMu.Unlock()
	switch {
	case cached < limit/2:
		sess.recalled.Store(false)
		return highest
	case cached < limit*3/4:
		if sess.recalled.Load() {
			return highest / 2
		}
		return highest
	}
	if !sess.recalled.Load() {
		ss.mu.Lock()
		share := limit / max(1, len(ss.sessions))
		ss.mu.Unlock()
		if ss.sessionCharge(sess) <= share {
			return highest
		}
		if !sess.recalled.Swap(true) && sess.cb != nil && sess.backRequested && cbRecallSlotFits(sess) {
			sess.cb.requestRecall(highest / 2)
		}
	}
	return highest / 2
}

// sessionCharge is the reply cache charge of sess's slots.
func (ss *sessionStore) sessionCharge(sess *session) int {
	total := 0
	for i := range sess.slots {
		slot := &sess.slots[i]
		slot.mu.Lock()
		total += slot.reserved
		slot.mu.Unlock()
	}
	return total
}
