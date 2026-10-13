// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// peekCompoundHeader extracts the tag and minor version from COMPOUND4args
// without fully parsing the argarray. The wire format is: tag (XDR opaque:
// u32 len + data padded to 4 bytes), then u32 minorversion.
func peekCompoundHeader(body []byte) (tag []byte, minor uint32, ok bool) {
	if len(body) < 4 {
		return nil, 0, false
	}
	tagLen := uint64(binary.BigEndian.Uint32(body[0:4]))
	padded := (tagLen + 3) &^ 3 // XDR pad to 4-byte boundary
	off := 4 + padded
	if 4+tagLen > uint64(len(body)) || off+4 > uint64(len(body)) {
		return nil, 0, false
	}
	return body[4 : 4+tagLen], binary.BigEndian.Uint32(body[off : off+4]), true
}

// Server is the NFSv4 server.
type Server struct {
	fs                 TernVFS
	clients            *ClientStore
	opens              *openStateStore
	sessions           *sessionStore
	backchannel        backchannelStats
	cbXIDs             atomic.Uint32
	allowMinorVersion2 bool
	views              viewStore
	names              nameCache
	stagingStore       StagingStore
	writeVerifier      [8]byte       // random per server instance, changes on restart
	idleTimeout        time.Duration // timeout reading the next request
	writeTimeout       time.Duration // timeout writing one reply
	maxInFlightPerConn int
	log                *slog.Logger
	startedAt          time.Time

	foregroundReads atomic.Int64
	hydrationSlots  chan struct{}
	mutationLocks   keyedLocker[mutationTarget]

	clientGCMu      sync.Mutex
	clientGCPending map[InodeID]struct{}
	clientGCRecheck map[InodeID]struct{}
	clientGCRunning bool
	clientGCDone    chan struct{}

	// Confirmations in progress or unresolved (server_confirm.go), by N and
	// P, by identity and by EXCHANGE_ID owner. At most one per identity.
	// confirmMu is taken after any other lock, sessions.mu included, and
	// held only for map access.
	confirmMu          sync.Mutex
	confirming         map[uint64]*confirmation
	confirmingIdentity map[InodeID]*confirmation
	confirmingOwners   map[string]*confirmation
	// Destruction holds in flight, by incarnation and by identity
	// (server_confirm.go), under confirmMu.
	destroying         map[uint64]int
	destroyingIdentity map[InodeID]int
	destroyEnded       *sync.Cond // on confirmMu
	// Incarnations whose replacement purge is known to be unfinished, a
	// hint for warm EXCHANGE_ID (server_confirm.go), under confirmMu.
	purgePending map[uint64]struct{}
	purgeLimit   int
}

type keyedLockEntry struct {
	mu   sync.Mutex
	refs int
}

type keyedLocker[K comparable] struct {
	mu      sync.Mutex
	entries map[K]*keyedLockEntry
}

func (l *keyedLocker[K]) ref(key K) *keyedLockEntry {
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[K]*keyedLockEntry)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &keyedLockEntry{}
		l.entries[key] = entry
	}
	entry.refs++
	l.mu.Unlock()
	return entry
}

func (l *keyedLocker[K]) unref(key K, entry *keyedLockEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry.refs--
	if entry.refs == 0 {
		delete(l.entries, key)
	}
}

func (l *keyedLocker[K]) lock(key K) func() {
	entry := l.ref(key)
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		l.unref(key, entry)
	}
}

func (l *keyedLocker[K]) tryLock(key K) (func(), bool) {
	entry := l.ref(key)
	if !entry.mu.TryLock() {
		l.unref(key, entry)
		return nil, false
	}
	return func() {
		entry.mu.Unlock()
		l.unref(key, entry)
	}, true
}

type mutationTarget struct {
	dirID InodeID
	name  string
}

func (s *Server) lockMutationTargets(targets ...mutationTarget) func() {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].dirID != targets[j].dirID {
			return targets[i].dirID < targets[j].dirID
		}
		return targets[i].name < targets[j].name
	})
	unique := targets[:0]
	for _, target := range targets {
		if len(unique) == 0 || unique[len(unique)-1] != target {
			unique = append(unique, target)
		}
	}
	unlocks := make([]func(), 0, len(unique))
	for _, target := range unique {
		unlocks = append(unlocks, s.mutationLocks.lock(target))
	}
	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}
}

const maxPendingClientGC = 256
const defaultMaxInFlightPerConn = 64

func NewServer(fs TernVFS, stagingStore StagingStore, logger *slog.Logger) (*Server, error) {
	clients, err := NewClientStore(fs)
	if err != nil {
		return nil, err
	}
	var verf [8]byte
	rand.Read(verf[:])
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		fs:                 fs,
		clients:            clients,
		opens:              newOpenStateStore(),
		stagingStore:       stagingStore,
		writeVerifier:      verf,
		idleTimeout:        5 * time.Minute,
		writeTimeout:       5 * time.Minute,
		maxInFlightPerConn: defaultMaxInFlightPerConn,
		log:                logger,
		startedAt:          clients.now(),
		hydrationSlots:     make(chan struct{}, maxConcurrentHydrations),
		clientGCPending:    make(map[InodeID]struct{}),
		clientGCRecheck:    make(map[InodeID]struct{}),
		confirming:         make(map[uint64]*confirmation),
		confirmingIdentity: make(map[InodeID]*confirmation),
		confirmingOwners:   make(map[string]*confirmation),
		destroying:         make(map[uint64]int),
		destroyingIdentity: make(map[InodeID]int),
		purgePending:       make(map[uint64]struct{}),
		purgeLimit:         maxSessionRecords,
	}
	s.destroyEnded = sync.NewCond(&s.confirmMu)
	s.opens.now = func() time.Time { return s.clients.now() }
	// The client store's expired marker, its client GC and a resumed
	// replacement purge hold destruction holds for their whole action.
	clients.beginMarker = func(id InodeID) (func(), bool) { return s.beginDestroy(uint64(id)) }
	clients.beginCollect = s.beginDestroyIdentity
	clients.beginPurge = s.beginPurge
	s.sessions = newSessionStore(s.opens.epoch)
	s.sessions.now = func() time.Time { return s.clients.now() }
	// The durable lease refresher reads the logical lease's need, and
	// retires an incarnation whose record turns stale or fleet-expired.
	clients.leaseNeed = s.sessions.leaseNeed
	clients.leaseTerminal = s.leaseTerminal
	clients.leaseSessions = s.sessions.hasSessions
	// The nfsd ID names this process's lease and confirming slots in the
	// client store. The epoch is the first four bytes of every stateid this
	// process issues. Both are needed to attribute persisted state and wire
	// stateids to an nfsd instance.
	logger.Info("nfsd instance",
		"nfsd_id", clients.nfsdID,
		"stateid_epoch", fmt.Sprintf("%08x", s.opens.epoch))
	return s, nil
}

func (s *Server) scheduleClientGC(clientID uint64) {
	id := InodeID(clientID)
	s.clientGCMu.Lock()
	if _, pending := s.clientGCPending[id]; !pending {
		if len(s.clientGCPending) == maxPendingClientGC {
			s.clientGCMu.Unlock()
			// Collection is opportunistic; a later login will enqueue it again.
			s.log.Warn("client GC queue is full")
			return
		}
		s.clientGCPending[id] = struct{}{}
	}
	if s.clientGCRunning {
		s.clientGCMu.Unlock()
		return
	}
	s.clientGCRunning = true
	s.clientGCDone = make(chan struct{})
	s.clientGCMu.Unlock()
	go s.runClientGC()
}

func (s *Server) runClientGC() {
	for {
		s.clientGCMu.Lock()
		var clientID InodeID
		for clientID = range s.clientGCPending {
			delete(s.clientGCPending, clientID)
			break
		}
		if clientID == 0 {
			s.clientGCRunning = false
			close(s.clientGCDone)
			s.clientGCMu.Unlock()
			return
		}
		s.clientGCMu.Unlock()

		recheck, err := s.collectClientSafely(clientID)
		if err != nil {
			s.log.Warn("client GC failed", "clientid", uint64(clientID), "err", err)
			recheck = true
		}
		if recheck {
			s.clientGCMu.Lock()
			_, alreadyQueued := s.clientGCRecheck[clientID]
			dropped := !alreadyQueued &&
				len(s.clientGCRecheck) == maxPendingClientGC
			if !dropped {
				s.clientGCRecheck[clientID] = struct{}{}
			}
			s.clientGCMu.Unlock()
			if dropped {
				s.log.Warn("client GC recheck queue is full")
			}
		}
	}
}

func (s *Server) collectClientSafely(
	clientID InodeID,
) (recheck bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			recheck = true
			s.log.Error("panic in client GC",
				"clientid", uint64(clientID), "panic", recovered)
		}
	}()
	return s.clients.collectStaleForClientResult(clientID)
}

func (s *Server) scheduleClientGCRechecks() {
	s.clientGCMu.Lock()
	pending := s.clientGCRecheck
	s.clientGCRecheck = make(map[InodeID]struct{})
	s.clientGCMu.Unlock()
	for clientID := range pending {
		s.scheduleClientGC(uint64(clientID))
	}
}

func (s *Server) removeExpiredStaging() {
	if s.clients.now().Before(s.startedAt.Add(nfsLeaseTime)) {
		return
	}
	byClient := make(map[uint64]map[InodeID]StagingMeta)
	for fileID, meta := range s.stagingStore.Entries() {
		if meta.ClientID == 0 {
			continue
		}
		if byClient[meta.ClientID] == nil {
			byClient[meta.ClientID] = make(map[InodeID]StagingMeta)
		}
		byClient[meta.ClientID][fileID] = meta
	}
	for clientID, fileIDs := range byClient {
		s.removeClientExpiredStaging(clientID, fileIDs)
	}
}

// removeClientExpiredStaging retires an expired client's staging, holding a
// destruction hold from the lease check through the last retirement. A
// confirmation protecting the client keeps its staging.
func (s *Server) removeClientExpiredStaging(clientID uint64, fileIDs map[InodeID]StagingMeta) {
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return
	}
	defer end()
	expired, err := s.decideExpired(clientID, true)
	if err != nil {
		s.log.Warn("staging lease check failed",
			"clientid", clientID, "err", err)
		return
	}
	if expired {
		for fileID, meta := range fileIDs {
			s.retireStaging(fileID, meta)
		}
	}
}

func (s *Server) sweepExpiredClientState() {
	clients := make(map[uint64]struct{})
	for _, clientID := range s.opens.activeClientIDs() {
		clients[clientID] = struct{}{}
	}
	for _, clientID := range s.clients.cachedClientIDs() {
		clients[clientID] = struct{}{}
	}
	s.sessions.mu.Lock()
	for clientID := range s.sessions.groups {
		clients[clientID] = struct{}{}
	}
	s.sessions.mu.Unlock()
	for clientID := range clients {
		s.expireClientIfDead(clientID)
	}
}

// expireClientIfDead expires one client whose lease is dead, holding a
// destruction hold from the decision through the last local effect. Expiry
// waits while a confirmation protects the client.
func (s *Server) expireClientIfDead(clientID uint64) {
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return
	}
	defer end()
	if _, err := s.decideExpired(clientID, true); err != nil {
		s.log.Warn("client lease sweep failed",
			"clientid", clientID, "err", err)
	}
}

// runLeaseSweep is the sweeper's tick. It writes no lease: every durable
// lease write belongs to the refresher (canonical r3 §8).
func (s *Server) runLeaseSweep() {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic in lease sweeper", "panic", recovered)
		}
	}()
	s.settleConfirmations()
	s.scheduleClientGCRechecks()
	s.removeExpiredStaging()
	s.sweepExpiredClientState()
	s.sweepViews()
	s.sessions.mu.Lock()
	s.sessions.pruneGroupsLocked()
	s.sessions.mu.Unlock()
	s.clients.sweepLeaseRecords()
}

func (s *Server) runLeaseSweeper() {
	ticker := time.NewTicker(nfsLeaseRenewAfter)
	defer ticker.Stop()
	for range ticker.C {
		s.runLeaseSweep()
	}
}

func (s *Server) ListenAndServe(addr string) error {
	ln, err := s.listen(addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	go s.runLeaseSweeper()
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Error("accept failed", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

// listen binds addr, then chooses the wire clientid epoch. A successful bind
// makes this process the endpoint's only owner, so any earlier owner chose
// its epoch no later than the bind second. A process that cannot bind
// chooses no epoch.
func (s *Server) listen(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	epoch := wireEpochAfter(time.Now(), time.Now, time.Sleep)
	if err := s.sessions.wireIDs.start(epoch); err != nil {
		ln.Close()
		return nil, err
	}
	s.log.Info("wire clientid epoch", "epoch", epoch)
	return ln, nil
}

func (s *Server) handleConn(conn net.Conn) {
	remote := conn.RemoteAddr().String()
	association := &rpcConnection{remote: remote}
	defer s.disconnectBack(association)
	s.log.Info("client connected", "remote", remote)
	var closed atomic.Bool
	done := make(chan struct{})
	closeConnection := func() {
		if closed.CompareAndSwap(false, true) {
			close(done)
			conn.Close()
		}
	}
	defer closeConnection()
	var workers sync.WaitGroup
	// A read EOF may be a TCP half-close. Drain accepted replies before
	// closing the write side. Backend operations are not cancelled.
	defer workers.Wait()
	slots := make(chan struct{}, s.maxInFlightPerConn)
	var writeMu, requestsMu sync.Mutex
	inFlight := make(map[uint32][]byte)
	writeReply := func(reply []byte) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if closed.Load() {
			return
		}
		var err error
		if s.writeTimeout > 0 {
			err = conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
		}
		if err == nil {
			err = writeFrame(conn, reply)
		}
		if err != nil {
			s.log.Debug("RPC reply failed", "remote", remote, "err", err)
			// Wake a reader blocked either on the socket or on admission.
			closeConnection()
		}
	}
	association.closed = done
	association.send = func(frame []byte) bool {
		writeReply(frame)
		return !closed.Load()
	}
	run := func(req *rpcRequest) {
		defer workers.Done()
		defer req.releaseReply()
		defer func() {
			requestsMu.Lock()
			delete(inFlight, req.xid)
			requestsMu.Unlock()
			<-slots
			if recovered := recover(); recovered != nil {
				closeConnection()
				s.log.Error("panic handling RPC", "remote", remote, "xid", req.xid, "panic", recovered)
			}
		}()
		reply := s.handleRPC(req, remote)
		if reply == nil {
			closeConnection()
			return
		}
		writeReply(reply)
		reply = nil // Drop the output allocation before releasing its obligation.
	}
	for {
		// Reserve memory and execution capacity before reading a full frame.
		select {
		case slots <- struct{}{}:
		case <-done:
			return
		}
		if closed.Load() {
			<-slots
			return
		}
		if s.idleTimeout > 0 {
			if err := conn.SetReadDeadline(time.Now().Add(s.idleTimeout)); err != nil {
				<-slots
				return
			}
		}
		frame, err := readFrame(conn)
		if err != nil {
			<-slots
			if closed.Load() {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.log.Info("client idle timeout", "remote", remote)
			} else if errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) {
				s.log.Info("client disconnected", "remote", remote)
			} else {
				s.log.Info("client disconnected", "remote", remote, "err", err)
			}
			return
		}
		if isRPCReply(frame) {
			// A callback's REPLY: handed to its waiting sender, or dropped
			// and counted. It takes no admission slot.
			<-slots
			if !association.deliverReply(frame) {
				s.backchannel.strayReplies.Add(1)
			}
			continue
		}
		req, err := parseRPCCall(frame)
		if err != nil {
			<-slots
			s.log.Warn("RPC parse error", "remote", remote, "err", err)
			return
		}
		req.connection = association
		requestsMu.Lock()
		previous, duplicate := inFlight[req.xid]
		if !duplicate {
			inFlight[req.xid] = frame
		}
		requestsMu.Unlock()
		if duplicate {
			<-slots
			if !bytes.Equal(previous, frame) {
				s.log.Warn("conflicting in-flight RPC xid", "remote", remote, "xid", req.xid)
				closeConnection()
				return
			}
			// The original request will send the one reply for this XID.
			continue
		}
		s.log.Debug("RPC call", "remote", remote, "xid", req.xid,
			"prog", req.prog, "vers", req.vers, "proc", req.proc)
		workers.Add(1)
		if req.prog == nfsProg && req.vers == nfsVersion && req.proc == procCompound {
			go run(req)
		} else {
			run(req)
		}
	}
}

func (s *Server) handleRPC(req *rpcRequest, remote string) []byte {
	if !req.acquireReplyWork(s) {
		return nil
	} // Transport overload, before effects.
	if req.prog != nfsProg || req.vers != nfsVersion {
		reply := buildRPCReply(req.xid, acceptProgMismatch)
		reply = binary.BigEndian.AppendUint32(reply, nfsVersion)
		return binary.BigEndian.AppendUint32(reply, nfsVersion)
	}
	switch req.proc {
	case procNull:
		return buildRPCReply(req.xid, acceptSuccess)
	case procCompound:
		return s.safeHandleCompound(req, remote)
	default:
		return buildRPCReply(req.xid, acceptProcUnavail)
	}
}

// compoundState tracks the current/saved filehandle during COMPOUND execution.
type compoundState struct {
	currentID     InodeID
	currentIDSet  bool
	savedID       InodeID
	savedIDSet    bool
	principal     rpcPrincipal
	connection    *rpcConnection
	replyBudget   uint32
	effectUnknown bool // Submitted mutation lacks whole-call outcome proof.

	// session and clientID come from the compound's SEQUENCE.
	session  *session
	clientID uint64

	// afterReply collects work for after the reply is written.
	afterReply *[]func()

	// currentStateID is the stateid returned by the last OPEN or CLOSE,
	// substituted for the special current stateid (RFC 8881 section
	// 16.2.3.1.2). Changing the current filehandle clears it.
	currentStateID    StateID
	currentGeneration uint32
	currentStateIDSet bool
	savedStateID      StateID
	savedGeneration   uint32
	savedStateIDSet   bool
}

// setCurrentFH changes the current filehandle. Operations which do so
// without producing a stateid invalidate the current stateid.
func (st *compoundState) setCurrentFH(id InodeID) {
	st.currentID = id
	st.currentIDSet = true
	st.currentStateIDSet = false
}

func (st *compoundState) setCurrentStateID(id StateID, generation uint32) {
	st.currentStateID = id
	st.currentGeneration = generation
	st.currentStateIDSet = true
}

func (st *compoundState) observeMutation(fs TernVFS, op uint32, err error) {
	if err != nil && fs.ClassifyMutation(op, err) == mutationUnknown {
		st.effectUnknown = true
	}
}

const maxCompoundOperations = 128

func (s *Server) safeHandleCompound(req *rpcRequest, remote string) (reply []byte) {
	if !req.acquireReplyWork(s) {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic handling COMPOUND", "remote", remote, "xid", req.xid, "panic", r)
			reply = buildRPCReply(req.xid, acceptSuccess)
			reply = binary.BigEndian.AppendUint32(reply, NFS4ERR_SERVERFAULT)
			reply = binary.BigEndian.AppendUint32(reply, 0) // empty tag
			reply = binary.BigEndian.AppendUint32(reply, 0) // resarray count
		}
	}()
	return s.handleCompound(req)
}

func (s *Server) handleCompound(req *rpcRequest) []byte {
	// Pre-check minor version before full parse, since an unknown version
	// may use operations this XDR does not describe.
	if tag, minor, ok := peekCompoundHeader(req.body); ok && !s.acceptsMinorVersion(minor) {
		s.log.Debug("minor version not supported", "minor", minor)
		reply := buildRPCReply(req.xid, acceptSuccess)
		// Build COMPOUND4res: status + echoed tag + 0 ops.
		reply = binary.BigEndian.AppendUint32(reply, NFS4ERR_MINOR_VERS_MISMATCH)
		// Echo tag as XDR opaque (u32 len + data + padding).
		reply = binary.BigEndian.AppendUint32(reply, uint32(len(tag)))
		reply = append(reply, tag...)
		if pad := (4 - len(tag)%4) % 4; pad > 0 {
			reply = append(reply, make([]byte, pad)...)
		}
		reply = binary.BigEndian.AppendUint32(reply, 0) // resarray count
		return reply
	}

	args, ok := ReadCOMPOUND4args(req.body)
	if !ok {
		s.log.Debug("COMPOUND: garbage args")
		reply := buildRPCReply(req.xid, acceptGarbageArgs)
		return reply
	}

	body := s.compoundBody(req, args)
	if body == nil {
		return nil
	}
	return req.joinRPCBody(body)
}

// compoundBody runs a COMPOUND and returns the COMPOUND4res body. Replies to
// requests on a session slot are cached by body, so a retransmission with a
// new XID returns the same results.
func (s *Server) compoundBody(req *rpcRequest, args COMPOUND4args) (result []byte) {
	// Bound the initial writer allocation; the work lease also covers variable
	// result temporaries and checkpoints retained through result construction.
	w := StartCOMPOUND4res(make([]byte, 0, req.replyWork.capacity))
	w.SetStatus(NFS4_OK)

	// Echo the tag.
	tagW := w.StartTag()
	tagData := args.Tag().Data()
	reply := tagW.SetData(tagData).Finish()
	w.Resume(reply)

	w.Finish() // Materialize the count so every prefix has an exact byte size.
	ops := make([]NfsArgop4, 0, min(args.ArgarrayCount(), maxCompoundOperations+1))
	iter := args.Argarray()
	for iter.Next() {
		ops = append(ops, iter.Argarray().Value())
		if len(ops) > maxCompoundOperations {
			break
		}
	}
	st := &compoundState{principal: req.principal(), connection: req.connection, afterReply: &req.afterReply}
	overallStatus := uint32(NFS4_OK)
	opCount := 0
	var seq sequenceResult
	checkpoint := w
	currentOp := uint32(OP_ILLEGAL)
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic handling admitted compound", "xid", req.xid, "panic", recovered)
			if seq.slot != nil {
				s.sessions.retireSession(seq.sess, true)
			}
			w = checkpoint
			writeOperationError(&w, currentOp, NFS4ERR_SERVERFAULT)
			w.SetStatus(NFS4ERR_SERVERFAULT)
			result = w.Finish()
			if seq.slot != nil {
				seq.finish(result)
			}
		}
	}()
	user, userErr := req.effectiveUser()
	for index, op := range ops {
		opnum := op.disc
		currentOp, checkpoint = opnum, w
		s.log.Debug("op", "opnum", NfsOpnum4Name(opnum))
		if userErr != nil {
			writeOperationError(&w, opnum, NFS4ERR_WRONGSEC)
			overallStatus = NFS4ERR_WRONGSEC
			goto done
		}
		if index == 0 {
			switch {
			case opnum == OP_SEQUENCE:
				prefixBytes := uint32(rpcAcceptedHeaderSize + len(w.buf) + 44)
				uncachedBytes := prefixBytes
				if len(ops) > 1 {
					uncachedBytes += resultErrorSize(ops[1].disc)
				}
				seq = s.startSequence(op.AsSEQUENCE4args(), sequenceAdmission{
					digest: sha256.Sum256(req.body), user: user, requestBytes: uint32(req.fullSize()),
					operations: args.ArgarrayCount(), sequenceReplyBytes: prefixBytes,
					uncachedBytes: uncachedBytes, connection: req.connection,
				})
				opCount++
				if seq.status != NFS4_OK {
					writeSequenceError(&w, seq.status)
					overallStatus = seq.status
					goto done
				}
				// The hold lasts until the transport is done with the reply.
				req.hold, seq.hold = seq.hold, nil
				if seq.replay != nil {
					req.replyWork.view = seq.view
					return seq.replay
				}
				st.session, st.clientID = seq.sess, seq.sess.clientID
				s.writeSequenceOK(&w, &seq)
				continue
			case sessionlessOp(opnum):
			default:
				status := uint32(NFS4ERR_OP_NOT_IN_SESSION)
				if !knownOperation(opnum) {
					status = NFS4ERR_OP_ILLEGAL
				}
				writeOperationError(&w, opnum, status)
				overallStatus = status
				opCount++
				goto done
			}
		} else if opnum == OP_SEQUENCE {
			writeOperationError(&w, opnum, NFS4ERR_SEQUENCE_POS)
			overallStatus = NFS4ERR_SEQUENCE_POS
			opCount++
			break
		}
		if status := maintenancePosition(op, st, index, len(ops)); status != NFS4_OK {
			writeOperationError(&w, opnum, status)
			overallStatus = status
			opCount++
			break
		}
		limit := uint32(maxRPCMessage)
		if st.session != nil {
			limit = seq.budget
		}
		used := uint32(rpcAcceptedHeaderSize + len(w.buf))
		suffix := uint32(0)
		if index+1 < len(ops) {
			suffix = resultErrorSize(ops[index+1].disc)
		}
		bound := s.operationReplyBound(op)
		handleBytes, handleStatus := s.handleReplyObligation(ops, index)
		if handleStatus != NFS4_OK {
			writeOperationError(&w, opnum, handleStatus)
			overallStatus = handleStatus
			opCount++
			break
		}
		obligation := max(suffix, handleBytes)
		if used > limit || uint64(used)+uint64(bound)+uint64(obligation) > uint64(limit) {
			status := uint32(NFS4ERR_REP_TOO_BIG)
			if st.session != nil {
				status = resultLimitStatus(&seq)
			}
			writeOperationError(&w, opnum, status)
			overallStatus = status
			opCount++
			break
		}
		st.replyBudget = limit - used - suffix

		var opStatus uint32
		switch opnum {
		case OP_ACCESS:
			opStatus = s.opAccess(op.AsACCESS4args(), st, &w)
		case OP_CLOSE:
			opStatus = s.opClose(op.AsCLOSE4args(), st, &w)
		case OP_COMMIT:
			opStatus = s.opCommit(st, &w)
		case OP_CREATE:
			opStatus = s.opCreate(op.AsCREATE4args(), st, &w)
		case OP_GETATTR:
			opStatus = s.opGetattr(op.AsGETATTR4args(), st, &w)
		case OP_GETFH:
			opStatus = s.opGetfh(st, &w)
		case OP_LOOKUP:
			opStatus = s.opLookup(op.AsLOOKUP4args(), st, &w)
		case OP_LOOKUPP:
			opStatus = s.opLookupp(st, &w)
		case OP_NVERIFY:
			opStatus = s.opNverify(op.AsNVERIFY4args(), st, &w)
		case OP_OPEN:
			opStatus = s.opOpen(op.AsOPEN4args(), st, &w)
		case OP_OPEN_DOWNGRADE:
			opStatus = s.opOpenDowngrade(op.AsOPENDOWNGRADE4args(), st, &w)
		case OP_PUTFH:
			opStatus = s.opPutfh(op.AsPUTFH4args(), st, &w)
		case OP_PUTPUBFH:
			opStatus = s.opPutrootfh(st, &w, true)
		case OP_PUTROOTFH:
			opStatus = s.opPutrootfh(st, &w, false)
		case OP_READ:
			opStatus = s.opRead(op.AsREAD4args(), st, &w)
		case OP_READDIR:
			opStatus = s.opReaddir(op.AsREADDIR4args(), st, &w)
		case OP_READLINK:
			opStatus = s.opReadlink(st, &w)
		case OP_REMOVE:
			opStatus = s.opRemove(op.AsREMOVE4args(), st, &w)
		case OP_RENAME:
			opStatus = s.opRename(op.AsRENAME4args(), st, &w)
		case OP_RESTOREFH:
			opStatus = s.opRestorefh(st, &w)
		case OP_SAVEFH:
			opStatus = s.opSavefh(st, &w)
		case OP_SECINFO:
			opStatus = s.opSecinfo(op.AsSECINFO4args(), st, &w)
		case OP_SETATTR:
			opStatus = s.opSetattr(op.AsSETATTR4args(), st, &w)
		case OP_VERIFY:
			opStatus = s.opVerify(op.AsVERIFY4args(), st, &w)
		case OP_WRITE:
			opStatus = s.opWrite(op.AsWRITE4args(), st, &w)
		case OP_EXCHANGE_ID:
			opStatus = s.opExchangeID(op.AsEXCHANGEID4args(), st, &w)
		case OP_CREATE_SESSION:
			opStatus = s.opCreateSession(op.AsCREATESESSION4args(), st, &w)
		case OP_DESTROY_SESSION:
			opStatus = s.opDestroySession(op.AsDESTROYSESSION4args(), st, &w)
		case OP_DESTROY_CLIENTID:
			opStatus = s.opDestroyClientID(op.AsDESTROYCLIENTID4args(), st, &w)
		case OP_BIND_CONN_TO_SESSION:
			opStatus = s.opBindConnToSession(op.AsBINDCONNTOSESSION4args(), st, &w)
		case OP_RECLAIM_COMPLETE:
			opStatus = s.opReclaimComplete(op.AsRECLAIMCOMPLETE4args(), st, &w)
		case OP_TEST_STATEID:
			opStatus = s.opTestStateID(op.AsTESTSTATEID4args(), st, &w)
		case OP_FREE_STATEID:
			opStatus = s.opFreeStateID(op.AsFREESTATEID4args(), st, &w)
		case OP_BACKCHANNEL_CTL:
			opStatus = s.opBackchannelCtl(op.AsBACKCHANNELCTL4args(), st, &w)
		case OP_SET_SSV:
			// Only SP4_SSV state protection uses an SSV, and nfsd offers
			// SP4_NONE (RFC 8881 section 18.47.3).
			writeOperationError(&w, opnum, NFS4ERR_INVAL)
			opStatus = NFS4ERR_INVAL
		case OP_SECINFO_NO_NAME:
			opStatus = s.opSecinfoNoName(op.AsNfsArgop4OPSECINFONONAME().Value(), st, &w)
		default:
			// Every other operation is removed from NFSv4.1 (RFC 8881
			// section 18), optional (locks, delegations, pNFS, 4.2), or
			// unsupported by TernFS (LINK, OPENATTR). nfsd grants no
			// delegations, so DELEGRETURN is NOTSUPP too.
			status := uint32(NFS4ERR_NOTSUPP)
			if !knownOperation(opnum) {
				status = NFS4ERR_OP_ILLEGAL
			}
			writeOperationError(&w, opnum, status)
			opStatus = status
		}

		opCount++
		if st.effectUnknown && seq.sess != nil {
			s.sessions.retireSession(seq.sess, true)
		}
		if uint64(rpcAcceptedHeaderSize+len(w.buf))+uint64(suffix) > uint64(limit) {
			if operationMutates(opnum) {
				// A violated fixed bound after a known effect cannot be recast as
				// a pre-effect failure. Retain terminal slot authority and close.
				if seq.sess != nil {
					s.sessions.retireSession(seq.sess, true)
				}
				seq.releaseUse()
				return nil
			}
			w = checkpoint
			opStatus = NFS4ERR_REP_TOO_BIG
			if seq.sess != nil {
				opStatus = resultLimitStatus(&seq)
			}
			writeOperationError(&w, opnum, opStatus)
		}

		if opStatus != NFS4_OK {
			s.log.Debug("op failed", "op", NfsOpnum4Name(opnum), "status", Nfsstat4Name(opStatus))
			overallStatus = opStatus
			break
		}
	}

done:
	s.log.Debug("COMPOUND done", "status", Nfsstat4Name(overallStatus), "ops", opCount)

	w.SetStatus(overallStatus)
	body := w.Finish()

	if seq.slot != nil {
		seq.finish(body)
	}
	return body
}

// acceptsMinorVersion reports whether nfsd serves a COMPOUND of this minor
// version. With allowMinorVersion2, NFSv4.2 is served as NFSv4.1: every
// operation new in 4.2 is optional (RFC 7862 section 1.4) and returns
// NFS4ERR_NOTSUPP. This exists for clients, such as libnfs 8, which offer
// sessions only as 4.2.
func (s *Server) acceptsMinorVersion(minor uint32) bool {
	return minor == nfsMinorVersion || minor == 2 && s.allowMinorVersion2
}

// knownOperation reports opcodes defined by the XDR. OP_ILLEGAL is not one:
// a client sends it to provoke NFS4ERR_OP_ILLEGAL.
func knownOperation(opnum uint32) bool {
	return opnum >= OP_ACCESS && opnum <= OP_REMOVEXATTR
}
