// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
)

// RPC constants.
const (
	rpcCall  = 0
	rpcReply = 1

	rpcVersion = 2

	nfsProg    = 100003
	nfsVersion = 4

	// nfsd implements NFSv4.1 only.
	nfsMinorVersion = 1

	// RPC procedures.
	procNull     = 0
	procCompound = 1

	// maxRPCMessage caps total reassembled RPC message size: a max WRITE
	// (maxReadWrite) plus headroom for COMPOUND/RPC framing.
	maxRPCMessage = maxReadWrite + 1<<20

	// Reply status.
	msgAccepted = 0
	msgDenied   = 1

	// Accept status.
	acceptSuccess         = 0
	acceptProgUnavail     = 1
	acceptProgMismatch    = 2
	acceptProcUnavail     = 3
	acceptGarbageArgs     = 4
	rpcAcceptedHeaderSize = 24
)

// readFrame reads one RPC record-marking frame from a TCP connection.
// Reassembles multi-fragment messages (bit 31 = last fragment flag).
func readFrame(r io.Reader) ([]byte, error) {
	var buf []byte
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, err
		}
		fragLen := binary.BigEndian.Uint32(hdr[:])
		last := fragLen&0x80000000 != 0
		fragLen &= 0x7FFFFFFF

		// Bound total memory per message. A single fragment can legitimately
		// carry a full maxReadWrite WRITE plus RPC/COMPOUND framing, so the
		// only limit is the overall message size (which has headroom for it).
		if uint64(len(buf))+uint64(fragLen) > maxRPCMessage {
			return nil, fmt.Errorf("RPC message too large: exceeds %d bytes", maxRPCMessage)
		}

		frag := make([]byte, fragLen)
		if _, err := io.ReadFull(r, frag); err != nil {
			return nil, err
		}
		buf = append(buf, frag...)
		if last {
			return buf, nil
		}
	}
}

// writeFrame writes an RPC record-marking frame (single fragment, last=true).
func writeFrame(w net.Conn, data []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data))|0x80000000)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// rpcRequest holds the parsed ONC RPC call header.
type rpcRequest struct {
	xid        uint32
	prog       uint32
	vers       uint32
	proc       uint32
	credFlavor uint32
	credBody   []byte
	verfFlavor uint32
	verfBody   []byte
	body       []byte // remaining bytes after header
	wireSize   int    // full reassembled XDR call, excluding TCP record markers
	connection *rpcConnection
	replyWork  *replyWork
	hold       *requestHold // an admitted SEQUENCE's hold, until releaseReply
	// afterReply runs once the reply is written or will never be: a new
	// back connection is probed only after the reply that created it.
	afterReply []func()
}

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
}

const maxConnectionSessions = 64

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

// Replay identity is the effective supported user, independently of the
// registration principal. AUTH_SYS host/time and group order are not identity.
type effectiveUser struct {
	flavor   uint32
	uid, gid uint32
	groups   string
}

func (r *rpcRequest) effectiveUser() (effectiveUser, error) {
	switch r.credFlavor {
	case nfs4.AUTH_NONE:
		if len(r.credBody) != 0 {
			return effectiveUser{}, fmt.Errorf("AUTH_NONE body")
		}
		return effectiveUser{flavor: nfs4.AUTH_NONE, uid: 65534, gid: 65534}, nil
	case nfs4.AUTH_SYS:
		body := r.credBody
		if len(body) < 8 {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS header")
		}
		name := uint64(binary.BigEndian.Uint32(body[4:8]))
		if name > 255 {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS machine name")
		}
		off := uint64(8) + (name+3)&^3
		if off+12 > uint64(len(body)) {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS user")
		}
		uid := binary.BigEndian.Uint32(body[off:])
		gid := binary.BigEndian.Uint32(body[off+4:])
		count := uint64(binary.BigEndian.Uint32(body[off+8:]))
		off += 12
		if count > 16 || off+4*count != uint64(len(body)) {
			return effectiveUser{}, fmt.Errorf("AUTH_SYS groups")
		}
		groups := make([]uint32, 0, count)
		for i := uint64(0); i < count; i++ {
			groups = append(groups, binary.BigEndian.Uint32(body[off+4*i:]))
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
		var canonical []byte
		var prev uint32
		first := true
		for _, group := range groups {
			if group == gid || !first && group == prev {
				continue
			}
			canonical = binary.BigEndian.AppendUint32(canonical, group)
			prev, first = group, false
		}
		return effectiveUser{r.credFlavor, uid, gid, string(canonical)}, nil
	default:
		return effectiveUser{}, fmt.Errorf("unsupported credential flavor %d", r.credFlavor)
	}
}

func (r *rpcRequest) fullSize() int {
	if r.wireSize != 0 {
		return r.wireSize
	}
	// Direct in-process tests also get honest header/opaque padding accounting.
	return 40 + ((len(r.credBody) + 3) &^ 3) + ((len(r.verfBody) + 3) &^ 3) + len(r.body)
}

func (r *rpcRequest) principal() rpcPrincipal {
	body := r.credBody
	// AUTH_SYS starts with a caller-supplied timestamp. It is not part of
	// the principal and may change between requests from the same caller.
	if r.credFlavor == nfs4.AUTH_SYS && len(body) >= 4 {
		body = body[4:]
		// The machine name is caller-supplied too. Linux nfsd identifies
		// AUTH_SYS callers by uid, gid and auxiliary groups instead.
		if len(body) >= 4 {
			nameLen := int(binary.BigEndian.Uint32(body[:4]))
			nameEnd := 4 + (nameLen+3)&^3
			if nameEnd <= len(body) {
				body = body[nameEnd:]
			}
		}
	}
	return rpcPrincipal{flavor: r.credFlavor, body: string(body)}
}

// parseRPCCall parses an ONC RPC CALL message.
func parseRPCCall(data []byte) (*rpcRequest, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("RPC message too short: %d bytes", len(data))
	}
	xid := binary.BigEndian.Uint32(data[0:4])
	msgType := binary.BigEndian.Uint32(data[4:8])
	if msgType != rpcCall {
		return nil, fmt.Errorf("expected RPC CALL (0), got %d", msgType)
	}
	rpcVers := binary.BigEndian.Uint32(data[8:12])
	if rpcVers != rpcVersion {
		return nil, fmt.Errorf("expected RPC version 2, got %d", rpcVers)
	}
	prog := binary.BigEndian.Uint32(data[12:16])
	vers := binary.BigEndian.Uint32(data[16:20])
	proc := binary.BigEndian.Uint32(data[20:24])

	off := 24

	// Parse credential.
	if off+8 > len(data) {
		return nil, fmt.Errorf("truncated credential")
	}
	credFlavor := binary.BigEndian.Uint32(data[off : off+4])
	credLen := binary.BigEndian.Uint32(data[off+4 : off+8])
	if credLen > 400 {
		return nil, fmt.Errorf("oversized RPC credential")
	}
	off += 8
	if uint64(off)+uint64(credLen) > uint64(len(data)) {
		return nil, fmt.Errorf("truncated credential body")
	}
	credBody := data[off : off+int(credLen)]
	off += int((uint64(credLen) + 3) &^ 3)

	// Parse verifier.
	if off+8 > len(data) {
		return nil, fmt.Errorf("truncated verifier")
	}
	verfFlavor := binary.BigEndian.Uint32(data[off : off+4])
	verfLen := binary.BigEndian.Uint32(data[off+4 : off+8])
	if verfLen > 400 {
		return nil, fmt.Errorf("oversized RPC verifier")
	}
	off += 8
	if uint64(off)+uint64(verfLen) > uint64(len(data)) {
		return nil, fmt.Errorf("truncated verifier body")
	}
	verfBody := data[off : off+int(verfLen)]
	off += int((uint64(verfLen) + 3) &^ 3)
	if off > len(data) {
		return nil, fmt.Errorf("truncated verifier padding")
	}

	return &rpcRequest{
		xid:        xid,
		prog:       prog,
		vers:       vers,
		proc:       proc,
		credFlavor: credFlavor,
		credBody:   credBody,
		verfFlavor: verfFlavor,
		verfBody:   verfBody,
		body:       data[off:],
		wireSize:   len(data),
	}, nil
}

// buildRPCReply builds an RPC reply header for an accepted, successful call.
// Returns a buffer that the caller appends procedure-specific data to.
func buildRPCReply(xid uint32, acceptStat uint32) []byte {
	buf := make([]byte, 0, 128)
	buf = binary.BigEndian.AppendUint32(buf, xid)
	buf = binary.BigEndian.AppendUint32(buf, rpcReply)
	buf = binary.BigEndian.AppendUint32(buf, msgAccepted)
	// Verifier: AUTH_NONE, length 0.
	buf = binary.BigEndian.AppendUint32(buf, nfs4.AUTH_NONE)
	buf = binary.BigEndian.AppendUint32(buf, 0)
	buf = binary.BigEndian.AppendUint32(buf, acceptStat)
	return buf
}
