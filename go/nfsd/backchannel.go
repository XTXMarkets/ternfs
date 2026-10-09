// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"sync/atomic"
)

// The backchannel (canonical r3 §12): callbacks the server sends on a
// connection the client also uses for requests. This file holds the
// connection side: the sessions whose callbacks a connection carries, and
// the callbacks waiting for a REPLY. A REPLY nothing waits for is dropped
// and counted.

// callbackSec is a session's callback credential: the first AUTH_NONE or
// AUTH_SYS entry the client offered. An AUTH_SYS body is sent verbatim.
type callbackSec struct {
	flavor uint32
	body   []byte
}

// parseCallbackSec chooses the callback credential from a CREATE_SESSION or
// BACKCHANNEL_CTL list, in the client's order. A list offering only
// RPCSEC_GSS gets NOENT, and one with no usable entry INVAL. CREATE_SESSION
// checks it before its slot.
func parseCallbackSec(it CallbackSecParms4EntryIter) (callbackSec, uint32) {
	gss := false
	for it.Next() {
		entry := it.BcaSecParm()
		switch entry.Disc() {
		case AUTH_NONE:
			return callbackSec{flavor: authNone}, NFS4_OK
		case AUTH_SYS:
			return callbackSec{flavor: authSys, body: append([]byte(nil), entry.Value().b...)}, NFS4_OK
		case RPCSEC_GSS:
			gss = true
		}
	}
	if gss {
		return callbackSec{}, NFS4ERR_NOENT
	}
	return callbackSec{}, NFS4ERR_INVAL
}

// cbCall is one outstanding callback on a connection, waiting for its
// REPLY. session is zero for CB_NULL. It belongs to the goroutine that sent
// it until a reply, a disconnect or a cancellation removes it; done is
// closed when the connection removes it without a reply, so that goroutine
// exits.
type cbCall struct {
	xid     uint32
	session SessionID
	null    bool
	reply   chan []byte // buffered: delivery never blocks the reader
	done    chan struct{}
}

func newCBCall(xid uint32, session SessionID, null bool) *cbCall {
	return &cbCall{xid: xid, session: session, null: null,
		reply: make(chan []byte, 1), done: make(chan struct{})}
}

// backchannelStats counts what the connection reader drops.
type backchannelStats struct {
	strayReplies atomic.Uint64
}

// isRPCReply reports an RPC REPLY frame: the answer to a callback.
func isRPCReply(frame []byte) bool {
	return len(frame) >= 8 && binary.BigEndian.Uint32(frame[4:8]) == rpcReply
}

// carriesCallbacks reports a transport that can carry callbacks; direct
// internal calls have none.
func (c *rpcConnection) carriesCallbacks() bool {
	return c != nil && c.send != nil
}

// backToken is one request's reservation of a back association, like
// associationToken for the fore channel: counted against the connection's
// back quota from reservation until commit or abort, exactly once.
type backToken struct {
	conn *rpcConnection
	s    *Server
	id   SessionID
	new  bool
	done bool
}

// reserveBack reserves a back association with id. The connection carries
// callbacks for at most maxConnectionSessions distinct sessions, pending
// reservations included; retired sessions are pruned first. A connection
// with no transport reserves nothing, successfully.
func (c *rpcConnection) reserveBack(s *Server, id SessionID) (*backToken, bool) {
	if !c.carriesCallbacks() {
		return &backToken{}, true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneBackLocked(s)
	if _, ok := c.back[id]; ok {
		return &backToken{conn: c, id: id}, true
	}
	if c.backPending[id] == 0 && c.backAssociationsLocked() >= maxConnectionSessions {
		return nil, false
	}
	if c.backPending == nil {
		c.backPending = make(map[SessionID]int)
	}
	c.backPending[id]++
	return &backToken{conn: c, s: s, id: id, new: true}, true
}

func (c *rpcConnection) backAssociationsLocked() int {
	n := len(c.back)
	for id := range c.backPending {
		if _, committed := c.back[id]; !committed {
			n++
		}
	}
	return n
}

func (c *rpcConnection) unpendBackLocked(id SessionID) {
	if c.backPending[id]--; c.backPending[id] <= 0 {
		delete(c.backPending, id)
	}
}

// commit publishes the back association. It reports whether the
// connection carries id's callbacks now.
func (t *backToken) commit() bool {
	if t == nil || t.done || t.conn == nil {
		return false
	}
	t.done = true
	c := t.conn
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.new {
		c.unpendBackLocked(t.id)
		c.pruneBackLocked(t.s)
		c.publishBackLocked(t.id)
	}
	return true
}

// publishBackLocked adds a back association. A connection whose earlier
// back associations all went is probed afresh; pruneBackLocked has already
// cancelled their probe. The caller holds c.mu.
func (c *rpcConnection) publishBackLocked(id SessionID) {
	if len(c.back) == 0 {
		c.path = cbPathProbing
	}
	if c.back == nil {
		c.back = make(map[SessionID]struct{})
	}
	c.back[id] = struct{}{}
}

// abort drops a new token's reservation; a no-op once committed.
func (t *backToken) abort() {
	if t == nil || t.done || t.conn == nil {
		return
	}
	t.done = true
	if t.new {
		c := t.conn
		c.mu.Lock()
		c.unpendBackLocked(t.id)
		c.mu.Unlock()
	}
}

// pruneBackLocked forgets back associations of sessions that have been
// retired. Their senders were cancelled at detach and remove their own
// calls. When no committed back association is left, an unanswered CB_NULL
// has no owner to answer for any more, whatever reservations are pending:
// it is removed and its probe exits, so the connection is probed again for
// the next backchannel committed on it. The caller holds c.mu; lock order
// is c.mu, then sessionStore.mu.
func (c *rpcConnection) pruneBackLocked(s *Server) {
	for id := range c.back {
		if sess := s.sessions.lookup(id); sess == nil || sess.destroyed.Load() || sess.fatal.Load() {
			delete(c.back, id)
		}
	}
	if len(c.back) != 0 {
		return
	}
	for _, call := range c.calls {
		if call.null {
			c.forgetCallLocked(call)
			close(call.done)
		}
	}
}

// bindDirections decides and publishes a BIND_CONN_TO_SESSION on the
// connection (Server.bindDirections) in one section: it observes the
// committed directions together, decides against them, checks both quotas
// and publishes both maps, or nothing. Another request's pending token is
// not an association here, and keeps its own commit or abort. It does no
// I/O and sends nothing; added reports a new back association, which the
// caller adds to the session's sender once the section ends.
func (c *rpcConnection) bindDirections(s *Server, id SessionID, dir uint32, canBack bool) (fore, back, added bool, status uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked(s)
	c.pruneBackLocked(s)
	_, hasFore := c.sessions[id]
	_, hasBack := c.back[id]
	if hasFore || hasBack {
		if !bindAcceptable(dir, hasFore, hasBack) {
			return false, false, false, NFS4ERR_INVAL
		}
		return hasFore, hasBack, false, NFS4_OK
	}
	switch dir {
	case CDFC4_FORE:
		fore = true
	case CDFC4_BACK:
		back = true
	case CDFC4_FORE_OR_BOTH:
		fore, back = true, canBack
	default:
		fore, back = canBack, true
	}
	if back && !canBack {
		return false, false, false, NFS4ERR_INVAL
	}
	if fore && c.pending[id] == 0 && c.associationsLocked() >= maxConnectionSessions ||
		back && c.backPending[id] == 0 && c.backAssociationsLocked() >= maxConnectionSessions {
		return false, false, false, NFS4ERR_RESOURCE
	}
	if fore {
		c.sessions[id] = struct{}{}
	}
	if back {
		c.publishBackLocked(id)
	}
	return fore, back, back, NFS4_OK
}

// bindBack reserves and commits a back association in one step.
func (c *rpcConnection) bindBack(s *Server, id SessionID) bool {
	t, ok := c.reserveBack(s, id)
	return ok && t.commit()
}

// carriesBack reports a back association with id.
func (c *rpcConnection) carriesBack(id SessionID) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.back[id]
	return ok
}

// carriesFore reports a committed fore association with id, without
// treating a missing transport as associated.
func (c *rpcConnection) carriesFore(id SessionID) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.sessions[id]
	return ok
}

// registerCall records an outstanding callback. A connection holds at most
// one CB_COMPOUND per session it carries callbacks for, and one CB_NULL, so
// the map is bounded by its back associations.
func (c *rpcConnection) registerCall(call *cbCall) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if call.null {
		if c.nullCall {
			return false
		}
	} else {
		if _, ok := c.back[call.session]; !ok {
			return false
		}
		for _, other := range c.calls {
			if !other.null && other.session == call.session {
				return false
			}
		}
	}
	if c.calls == nil {
		c.calls = make(map[uint32]*cbCall)
	}
	if _, taken := c.calls[call.xid]; taken {
		return false
	}
	c.calls[call.xid] = call
	if call.null {
		c.nullCall = true
	}
	return true
}

// forgetCallLocked removes an outstanding callback. The caller holds c.mu.
func (c *rpcConnection) forgetCallLocked(call *cbCall) {
	if c.calls[call.xid] == call {
		delete(c.calls, call.xid)
		if call.null {
			c.nullCall = false
		}
	}
}

// cancelCall removes an outstanding callback whose sender gave up on it; a
// late reply is then an unknown XID.
func (c *rpcConnection) cancelCall(call *cbCall) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forgetCallLocked(call)
}

// deliverReply hands a REPLY to the callback waiting for its XID. It
// reports false for an unknown XID or a connection with no back
// association; the reader drops those.
func (c *rpcConnection) deliverReply(frame []byte) bool {
	xid := binary.BigEndian.Uint32(frame[0:4])
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.back) == 0 {
		return false
	}
	call := c.calls[xid]
	if call == nil {
		return false
	}
	c.forgetCallLocked(call)
	call.reply <- frame
	return true
}

// backSessions lists the sessions whose callbacks the connection carries.
func (c *rpcConnection) backSessions() []SessionID {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]SessionID, 0, len(c.back))
	for id := range c.back {
		ids = append(ids, id)
	}
	return ids
}

// backPath is the connection's backchannel state; a closed connection's is
// unresponsive.
func (c *rpcConnection) backPath() cbPath {
	select {
	case <-c.closed:
		return cbPathUnresponsive
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.path
}

func (c *rpcConnection) setBackPath(path cbPath) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = path
}

// timeoutPath marks the path unresponsive for a CB_NULL that timed out,
// if the connection still holds that call.
func (c *rpcConnection) timeoutPath(call *cbCall) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls[call.xid] != call {
		return false
	}
	c.path = cbPathUnresponsive
	return true
}

// outstandingCalls is the number of callbacks waiting for a reply.
func (c *rpcConnection) outstandingCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}
