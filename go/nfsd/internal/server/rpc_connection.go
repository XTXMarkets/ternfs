// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"sync"
	"sync/atomic"
	"time"
)

// Forechannel association belongs to the live TCP connection, not its remote
// address or registration principal.
//
// A request that may associate the connection with a session reserves a
// token first (canonical r3 §7.2). sessions holds committed associations,
// which associated() sees; pending counts provisional owners. The quota
// counts each ID once, in either map. mu is held only for map operations,
// never across backend or renewal work; lock order is slot.mu, mu, then
// sessionStore.mu.
type rpcConnection struct {
	mu       sync.Mutex
	sessions map[SessionID]struct{}
	pending  map[SessionID]int
	// The backchannel side (backchannel.go): sessions whose callbacks this
	// connection carries, and the callbacks waiting for a REPLY.
	back        map[SessionID]struct{}
	backPending map[SessionID]int
	calls       map[uint32]*cbCall
	nullCall    bool
	path        cbPath
	remote      string
	// send writes one frame through the connection's writer; closed is
	// closed when the connection ends. Both are set by handleConn.
	send   func([]byte) bool
	closed <-chan struct{}
	// For diagnostics: when the connection was accepted, the requests
	// admitted and not yet replied to, and the requests admitted in total.
	connectedAt time.Time
	inFlight    atomic.Int32
	requests    atomic.Uint64
}

// isClosed reports that the connection has ended, so a request waiting
// for resources on its behalf should stop.
func (c *rpcConnection) isClosed() bool {
	if c.closed == nil {
		return false
	}
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *rpcConnection) associated(id SessionID) bool {
	if c == nil {
		return true
	} // Direct internal calls have no transport to bind.
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.sessions[id]
	return ok
}

// associationToken is one request's reservation of an association. A held
// token found the association already committed; a new one holds a pending
// count until commit or abort, exactly once.
type associationToken struct {
	conn *rpcConnection
	id   SessionID
	new  bool
	done bool
}

// reserve takes a token for id, or fails when the quota is full.
func (c *rpcConnection) reserve(s *Server, id SessionID) (*associationToken, bool) {
	if c == nil {
		return &associationToken{}, true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reserveLocked(s, id)
}

func (c *rpcConnection) reserveLocked(s *Server, id SessionID) (*associationToken, bool) {
	c.pruneLocked(s)
	if _, ok := c.sessions[id]; ok {
		return &associationToken{conn: c, id: id}, true
	}
	if c.pending[id] == 0 && c.associationsLocked() >= maxConnectionSessions {
		return nil, false
	}
	c.pending[id]++
	return &associationToken{conn: c, id: id, new: true}, true
}

// associationsLocked counts the distinct IDs in either map: an ID another
// token committed while this one is still pending counts once.
func (c *rpcConnection) associationsLocked() int {
	n := len(c.sessions)
	for id := range c.pending {
		if _, committed := c.sessions[id]; !committed {
			n++
		}
	}
	return n
}

// commit publishes a new token's association; it is idempotent for the ID.
func (t *associationToken) commit() {
	if t == nil || t.done || t.conn == nil {
		return
	}
	t.done = true
	if !t.new {
		return
	}
	c := t.conn
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unpendLocked(t.id)
	c.sessions[t.id] = struct{}{}
}

// abort drops a new token's pending count. It never removes a committed
// association, whoever committed it.
func (t *associationToken) abort() {
	if t == nil || t.done || t.conn == nil {
		return
	}
	t.done = true
	if !t.new {
		return
	}
	c := t.conn
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unpendLocked(t.id)
}

func (c *rpcConnection) unpendLocked(id SessionID) {
	if c.pending[id]--; c.pending[id] <= 0 {
		delete(c.pending, id)
	}
}

// bind reserves and commits in one section, for BIND_CONN_TO_SESSION.
func (c *rpcConnection) bind(s *Server, id SessionID) bool {
	if c == nil {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.reserveLocked(s, id)
	if !ok {
		return false
	}
	if t.new {
		c.unpendLocked(id)
		c.sessions[id] = struct{}{}
	}
	t.done = true
	return true
}

func (c *rpcConnection) pruneLocked(s *Server) {
	if c.sessions == nil {
		c.sessions = make(map[SessionID]struct{})
		c.pending = make(map[SessionID]int)
	}
	for old := range c.sessions {
		sess := s.sessions.lookup(old)
		if sess == nil || sess.destroyed.Load() {
			delete(c.sessions, old)
		}
	}
}
