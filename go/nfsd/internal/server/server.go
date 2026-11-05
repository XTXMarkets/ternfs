// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Package server is the NFSv4.1 server: connections and compounds,
// sessions and their persistence, client registration and the logical
// lease, opens, the staging lifecycle and the backchannel. README.md
// describes how they work.
package server

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
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
	fs vfs.TernVFS
	// records are the confirmed clients, kept beside staging (clients.go).
	records            *clientrecords.Records
	opens              *openstate.Store
	sessions           *sessionStore
	backchannel        backchannelStats
	cbXIDs             atomic.Uint32
	AllowMinorVersion2 bool
	views              openstate.ViewStore
	names              nameCache
	stagingStore       staging.Store
	// serverID is the staging directory's server-id: a moved staging
	// directory is the same server and a wiped one a new server.
	serverID           staging.ServerID
	writeVerifier      [8]byte       // random per server instance, changes on restart
	idleTimeout        time.Duration // timeout reading the next request
	writeTimeout       time.Duration // timeout writing one reply
	MaxInFlightPerConn int
	log                *slog.Logger
	startedAt          time.Time
	NFSAddr            string // the -addr flag, for diagnostics
	// clock is read by now; tests replace it with SetClock.
	clock atomic.Pointer[func() time.Time]

	// Live connections, for diagnostics only; connsMu guards the map.
	connsMu sync.Mutex
	conns   map[*rpcConnection]struct{}

	foregroundReads atomic.Int64
	hydrationSlots  chan struct{}
	mutationLocks   keyedlock.Locker[mutationTarget]

	// aliasMu orders each publication alias set and its append to aliases
	// with the sweep's snapshot and rewrite of that log (alias_log.go).
	aliasMu sync.Mutex
	aliases *aliasLog
	// unlinkSessionFile deletes a session's file from the staging disk;
	// tests make it block or fail.
	unlinkSessionFile func(*sessionstore.File) error
	// newSessionFile creates a session's file, sessionFiles.Create; tests
	// make it block or fail.
	newSessionFile func(sessionstore.ID, sessionstore.Header) (*sessionstore.File, error)
	// removeClientRecord removes an incarnation's record, records.Remove;
	// tests make it block.
	removeClientRecord func(clientrecords.Incarnation) error
	// publishedAtStartup maps the stateid of each writer that startup found
	// already published to its client, so that a late CLOSE of it answers
	// OK. Written only in NewServer.
	publishedAtStartup map[nfs4.StateID]uint64

	metrics      *Metrics
	sessionFiles *sessionstore.Store // nil if the session store could not be opened
}

type mutationTarget struct {
	dirID vfs.InodeID
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
		unlocks = append(unlocks, s.mutationLocks.Lock(target))
	}
	return func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}
}

const DefaultMaxInFlightPerConn = 64

// SetReplyWorkBudget bounds the reply work admitted at once across all
// connections. Requests beyond it wait for releases.
func (s *Server) SetReplyWorkBudget(bytes int) {
	ss := s.sessions
	ss.cacheMu.Lock()
	ss.workLimit = bytes
	ss.workFreed.Broadcast()
	ss.cacheMu.Unlock()
}

func NewServer(fs vfs.TernVFS, stagingStore staging.Store, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	stagingDir := stagingStore.Dir()
	records, err := clientrecords.Open(stagingDir)
	if err != nil {
		return nil, err
	}
	var verf [8]byte
	rand.Read(verf[:])
	s := &Server{
		fs:                 fs,
		records:            records,
		opens:              openstate.NewStore(),
		serverID:           stagingStore.ServerID(),
		stagingStore:       stagingStore,
		writeVerifier:      verf,
		idleTimeout:        5 * time.Minute,
		writeTimeout:       5 * time.Minute,
		MaxInFlightPerConn: DefaultMaxInFlightPerConn,
		log:                logger,
		hydrationSlots:     make(chan struct{}, maxConcurrentHydrations),
		metrics:            newMetrics(),
		unlinkSessionFile:  (*sessionstore.File).Unlink,
		removeClientRecord: records.Remove,
	}
	s.SetClock(time.Now)
	s.opens.Now = s.now
	s.sessions = newSessionStore(s.opens.Epoch)
	s.sessions.now = s.now
	s.sessions.persistSlot = s.persistSlot
	// The server id is the server owner clients see. The epoch is the first
	// four bytes of every stateid this process issues, which attributes a
	// wire stateid to an nfsd process.
	logger.Info("nfsd instance",
		"server_id", s.serverID.String(),
		"stateid_epoch", fmt.Sprintf("%08x", s.opens.Epoch))
	s.aliases = newAliasLog(stagingDir)
	s.loadAliases()
	s.openSessionFiles(stagingDir)
	s.restoreSessions()
	s.dropPublishedStaging()
	// The startup lease is the restored clients' time to reconnect, so it
	// starts once startup's backend calls are done.
	s.startedAt = s.now()
	return s, nil
}

// dropPublishedStaging removes each recovered staging whose candidate is
// linked at its checkpointed data version: its CLOSE published it and did
// not get to remove it. Left in place, the client's next OPEN would adopt
// it and publish its old bytes over later versions. A late CLOSE of its
// stateid still answers OK, through publishedAtStartup. The first backend
// error other than not-exist ends the pass, so a backend that does not
// answer costs one timeout rather than one per entry.
func (s *Server) dropPublishedStaging() {
	s.publishedAtStartup = make(map[nfs4.StateID]uint64)
	for id, meta := range s.stagingStore.Entries() {
		sf := s.stagingStore.Get(id)
		if meta.Unlinked || sf == nil {
			continue
		}
		c := sf.Candidate()
		if c.Version != sf.DataVersion() {
			// Writes after the link still need publishing.
			continue
		}
		if _, err := s.fs.Stat(c.ID); os.IsNotExist(err) {
			continue
		} else if err != nil {
			s.log.Warn("staging recover: cannot stat candidate, keeping the rest unchecked", "inode", id, "candidate", c.ID, "err", err)
			return
		}
		// CLOSE restores the writer's times after the link; this one may
		// not have got that far.
		info := sf.Stat()
		if err := s.fs.SetTime(c.ID, &info.Mtime, &info.Atime); err != nil {
			s.log.Warn("staging recover: cannot restore times of a published candidate, keeping the rest unchecked", "inode", id, "candidate", c.ID, "err", err)
			return
		}
		s.log.Info("staging recover: already published", "inode", id, "name", meta.FileName, "candidate", c.ID)
		if meta.ClientID != 0 && meta.BaseID != 0 {
			s.publishedAs(meta.ClientID, meta.BaseID, c.ID)
		}
		s.publishedAtStartup[meta.NFSStateID] = meta.ClientID
		s.stagingStore.Remove(id)
	}
}

func (s *Server) removeExpiredStaging() {
	if s.inStartupLease() {
		return
	}
	byClient := make(map[uint64]map[vfs.InodeID]staging.Meta)
	for fileID, meta := range s.stagingStore.Entries() {
		if meta.ClientID == 0 {
			continue
		}
		if byClient[meta.ClientID] == nil {
			byClient[meta.ClientID] = make(map[vfs.InodeID]staging.Meta)
		}
		byClient[meta.ClientID][fileID] = meta
	}
	for clientID, fileIDs := range byClient {
		s.removeClientExpiredStaging(clientID, fileIDs)
	}
}

// removeClientExpiredStaging retires an expired client's staging.
func (s *Server) removeClientExpiredStaging(clientID uint64, fileIDs map[vfs.InodeID]staging.Meta) {
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
	for _, clientID := range s.opens.ActiveClientIDs() {
		clients[clientID] = struct{}{}
	}
	for _, rec := range s.records.List() {
		clients[uint64(rec.Incarnation)] = struct{}{}
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

// expireClientIfDead expires one client whose lease is dead.
func (s *Server) expireClientIfDead(clientID uint64) {
	if _, err := s.decideExpired(clientID, true); err != nil {
		s.log.Warn("client lease sweep failed",
			"clientid", clientID, "err", err)
	}
}

// runLeaseSweep is the sweeper's tick.
func (s *Server) runLeaseSweep() {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic in lease sweeper", "panic", recovered)
		}
	}()
	s.removeExpiredStaging()
	s.sweepExpiredClientState()
	s.sweepViews()
	s.sessions.mu.Lock()
	s.sessions.pruneGroupsLocked()
	s.sessions.mu.Unlock()
}

func (s *Server) runLeaseSweeper() {
	ticker := time.NewTicker(leaseSweepInterval)
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
	association := &rpcConnection{remote: remote, connectedAt: s.now()}
	defer s.untrackConn(association)
	defer s.disconnectBack(association)
	s.log.Info("client connected", "remote", remote)
	s.metrics.connAccepted.Add(1)
	// Why the connection ended: every exit records a cause, the first one
	// recorded is kept, and it is counted once, after the drain and close.
	var ended atomic.Uint32
	end := func(why connEnd) { ended.CompareAndSwap(uint32(connEndUnset), uint32(why)) }
	defer func() { s.metrics.connectionEnded(connEnd(ended.Load())) }()
	var closed atomic.Bool
	done := make(chan struct{})
	closeConnection := func() {
		if closed.CompareAndSwap(false, true) {
			close(done)
			conn.Close()
			s.sessions.wakeWorkWaiters()
		}
	}
	defer closeConnection()
	var workers sync.WaitGroup
	// A read EOF may be a TCP half-close. Drain accepted replies before
	// closing the write side. Other read failures and parse errors close
	// the connection first, so the drain only waits for requests already
	// executing. Backend operations are not cancelled.
	defer workers.Wait()
	slots := make(chan struct{}, s.MaxInFlightPerConn)
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
			end(connEndServer)
			closeConnection()
		}
	}
	association.closed = done
	association.send = func(frame []byte) bool {
		writeReply(frame)
		return !closed.Load()
	}
	// Published to diagnostics only once closed and send are set, since a
	// snapshot reads them without the connection's lock.
	s.trackConn(association)
	run := func(req *rpcRequest) {
		defer workers.Done()
		defer req.releaseReply()
		defer func() {
			requestsMu.Lock()
			delete(inFlight, req.xid)
			requestsMu.Unlock()
			association.inFlight.Add(-1)
			<-slots
			if recovered := recover(); recovered != nil {
				end(connEndServer)
				closeConnection()
				s.log.Error("panic handling RPC", "remote", remote, "xid", req.xid, "panic", recovered)
			}
		}()
		reply := s.handleRPC(req, remote)
		if reply == nil {
			// Refused or dropped without a reply. When the connection had
			// already closed, its cause was recorded first and is kept.
			end(connEndServer)
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
				end(connEndReadError)
				closeConnection()
				return
			}
		}
		frame, err := readFrame(conn)
		if err != nil {
			<-slots
			if closed.Load() {
				return
			}
			eof := errors.Is(err, io.EOF)
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				end(connEndIdle)
				s.log.Info("client idle timeout", "remote", remote)
			} else if eof || errors.Is(err, syscall.ECONNRESET) {
				end(connEndClient)
				s.log.Info("client disconnected", "remote", remote)
			} else {
				end(connEndReadError)
				s.log.Info("client disconnected", "remote", remote, "err", err)
			}
			// After a clean EOF the client can still read its replies. After
			// any other failure no reply will be read: close now, so requests
			// waiting for reply work give up rather than hold their frames
			// until the budget frees.
			if !eof {
				closeConnection()
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
			end(connEndParseError)
			s.log.Warn("RPC parse error", "remote", remote, "err", err)
			// Unlike EOF, a bad frame says nothing about whether the client
			// still reads, and with the reader gone nothing would notice it
			// leave: close now, so waiting requests give up.
			closeConnection()
			return
		}
		req.connection = association
		requestsMu.Lock()
		previous, duplicate := inFlight[req.xid]
		if !duplicate {
			inFlight[req.xid] = frame
			association.inFlight.Add(1)
			association.requests.Add(1)
		}
		requestsMu.Unlock()
		if duplicate {
			<-slots
			if !bytes.Equal(previous, frame) {
				s.log.Warn("conflicting in-flight RPC xid", "remote", remote, "xid", req.xid)
				end(connEndServer)
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

func (s *Server) trackConn(c *rpcConnection) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	if s.conns == nil {
		s.conns = make(map[*rpcConnection]struct{})
	}
	s.conns[c] = struct{}{}
}

func (s *Server) untrackConn(c *rpcConnection) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	delete(s.conns, c)
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

const maxCompoundOperations = 128

func (s *Server) safeHandleCompound(req *rpcRequest, remote string) (reply []byte) {
	if !req.acquireReplyWork(s) {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic handling COMPOUND", "remote", remote, "xid", req.xid, "panic", r)
			reply = buildRPCReply(req.xid, acceptSuccess)
			reply = binary.BigEndian.AppendUint32(reply, nfs4.NFS4ERR_SERVERFAULT)
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
		reply = binary.BigEndian.AppendUint32(reply, nfs4.NFS4ERR_MINOR_VERS_MISMATCH)
		// Echo tag as XDR opaque (u32 len + data + padding).
		reply = binary.BigEndian.AppendUint32(reply, uint32(len(tag)))
		reply = append(reply, tag...)
		if pad := (4 - len(tag)%4) % 4; pad > 0 {
			reply = append(reply, make([]byte, pad)...)
		}
		reply = binary.BigEndian.AppendUint32(reply, 0) // resarray count
		return reply
	}

	args, ok := nfs4.ReadCOMPOUND4args(req.body)
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
func (s *Server) compoundBody(req *rpcRequest, args nfs4.COMPOUND4args) (result []byte) {
	// Bound the initial writer allocation; the work lease also covers variable
	// result temporaries and checkpoints retained through result construction.
	w := nfs4.StartCOMPOUND4res(make([]byte, 0, req.replyWork.capacity))
	w.SetStatus(nfs4.NFS4_OK)
	s.metrics.compounds.Add(1)

	// Echo the tag.
	tagW := w.StartTag()
	tagData := args.Tag().Data()
	reply := tagW.SetData(tagData).Finish()
	w.Resume(reply)

	w.Finish() // Materialize the count so every prefix has an exact byte size.
	ops := make([]nfs4.NfsArgop4, 0, min(args.ArgarrayCount(), maxCompoundOperations+1))
	iter := args.Argarray()
	for iter.Next() {
		ops = append(ops, iter.Argarray().Value())
		if len(ops) > maxCompoundOperations {
			break
		}
	}
	st := &compoundState{principal: req.principal(), connection: req.connection, afterReply: &req.afterReply}
	// Registered before the recovery below, so that the locks of the
	// operation that ends the compound go only once its reply, whatever it
	// is, has been recorded.
	defer st.releaseHolds()
	overallStatus := uint32(nfs4.NFS4_OK)
	opCount := 0
	var seq sequenceResult
	checkpoint := w
	currentOp := uint32(nfs4.OP_ILLEGAL)
	// Each operation's result is counted once it is final: an earlier one
	// when the next operation installs its checkpoint, the last one when
	// the compound returns, after recovery has chosen what is returned.
	opStart := time.Now()
	var last opResult
	record := func(op, status uint32) {
		last = opResult{op: op, status: status, elapsed: time.Since(opStart), set: true}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("panic handling admitted compound", "xid", req.xid, "panic", recovered)
			if seq.slot != nil {
				s.sessions.retireSession(seq.sess, true)
			}
			w = checkpoint
			writeOperationError(&w, currentOp, nfs4.NFS4ERR_SERVERFAULT)
			record(currentOp, nfs4.NFS4ERR_SERVERFAULT)
			w.SetStatus(nfs4.NFS4ERR_SERVERFAULT)
			result = w.Finish()
			if seq.slot != nil {
				seq.finish(result)
			}
		}
		s.metrics.observeResult(&last)
	}()
	user, userErr := req.effectiveUser()
	for index, op := range ops {
		opnum := op.Disc()
		s.metrics.observeResult(&last)
		currentOp, checkpoint, opStart = opnum, w, time.Now()
		s.log.Debug("op", "opnum", nfs4.NfsOpnum4Name(opnum))
		if userErr != nil {
			writeOperationError(&w, opnum, nfs4.NFS4ERR_WRONGSEC)
			record(opnum, nfs4.NFS4ERR_WRONGSEC)
			overallStatus = nfs4.NFS4ERR_WRONGSEC
			goto done
		}
		if index == 0 {
			switch {
			case opnum == nfs4.OP_SEQUENCE:
				prefixBytes := uint32(rpcAcceptedHeaderSize + w.Len() + sequenceResultBytes)
				uncachedBytes := prefixBytes
				if len(ops) > 1 {
					uncachedBytes += resultErrorSize(ops[1].Disc())
				}
				seq = s.startSequence(op.AsSEQUENCE4args(), sequenceAdmission{
					digest: replayDigest(req.body), user: user, requestBytes: uint32(req.fullSize()),
					operations: args.ArgarrayCount(), sequenceReplyBytes: prefixBytes,
					uncachedBytes: uncachedBytes, connection: req.connection,
				})
				opCount++
				record(nfs4.OP_SEQUENCE, seq.status)
				if seq.status != nfs4.NFS4_OK {
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
				st.slot, st.slotID, st.seq = seq.slot, seq.slotID, seq.seq
				st.replay = newReplayState(seq.hasIntent, seq.intent)
				s.writeSequenceOK(&w, &seq)
				continue
			case sessionlessOp(opnum):
			default:
				status := uint32(nfs4.NFS4ERR_OP_NOT_IN_SESSION)
				if !knownOperation(opnum) {
					status = nfs4.NFS4ERR_OP_ILLEGAL
				}
				writeOperationError(&w, opnum, status)
				record(opnum, status)
				overallStatus = status
				opCount++
				goto done
			}
		} else if opnum == nfs4.OP_SEQUENCE {
			writeOperationError(&w, opnum, nfs4.NFS4ERR_SEQUENCE_POS)
			record(opnum, nfs4.NFS4ERR_SEQUENCE_POS)
			overallStatus = nfs4.NFS4ERR_SEQUENCE_POS
			opCount++
			break
		}
		if status := maintenancePosition(op, st, index, len(ops)); status != nfs4.NFS4_OK {
			writeOperationError(&w, opnum, status)
			record(opnum, status)
			overallStatus = status
			opCount++
			break
		}
		limit := uint32(maxRPCMessage)
		if st.session != nil {
			limit = seq.budget
		}
		// Admission reserved the envelope the compound's operations can
		// fill; the output must stay inside it.
		limit = min(limit, uint32(req.replyWork.capacity))
		used := uint32(rpcAcceptedHeaderSize + w.Len())
		suffix := uint32(0)
		if index+1 < len(ops) {
			suffix = resultErrorSize(ops[index+1].Disc())
		}
		bound := s.operationReplyBound(op)
		handleBytes, handleStatus := s.handleReplyObligation(ops, index)
		if handleStatus != nfs4.NFS4_OK {
			writeOperationError(&w, opnum, handleStatus)
			record(opnum, handleStatus)
			overallStatus = handleStatus
			opCount++
			break
		}
		obligation := max(suffix, handleBytes)
		if used > limit || uint64(used)+uint64(bound)+uint64(obligation) > uint64(limit) {
			status := uint32(nfs4.NFS4ERR_REP_TOO_BIG)
			if st.session != nil {
				status = resultLimitStatus(&seq)
			}
			writeOperationError(&w, opnum, status)
			record(opnum, status)
			overallStatus = status
			opCount++
			break
		}
		st.replyBudget = limit - used - suffix
		st.opIndex = index
		if status := s.replayGate(st, opnum); status != nfs4.NFS4_OK {
			writeOperationError(&w, opnum, status)
			record(opnum, status)
			overallStatus = status
			opCount++
			break
		}
		if status := s.mutationGate(st, opnum); status != nfs4.NFS4_OK {
			writeOperationError(&w, opnum, status)
			record(opnum, status)
			overallStatus = status
			opCount++
			break
		}

		var opStatus uint32
		switch opnum {
		case nfs4.OP_ACCESS:
			opStatus = s.opAccess(op.AsACCESS4args(), st, &w)
		case nfs4.OP_CLOSE:
			opStatus = s.opClose(op.AsCLOSE4args(), st, &w)
		case nfs4.OP_COMMIT:
			opStatus = s.opCommit(st, &w)
		case nfs4.OP_CREATE:
			opStatus = s.opCreate(op.AsCREATE4args(), st, &w)
		case nfs4.OP_GETATTR:
			opStatus = s.opGetattr(op.AsGETATTR4args(), st, &w)
		case nfs4.OP_GETFH:
			opStatus = s.opGetfh(st, &w)
		case nfs4.OP_LOOKUP:
			opStatus = s.opLookup(op.AsLOOKUP4args(), st, &w)
		case nfs4.OP_LOOKUPP:
			opStatus = s.opLookupp(st, &w)
		case nfs4.OP_NVERIFY:
			opStatus = s.opNverify(op.AsNVERIFY4args(), st, &w)
		case nfs4.OP_OPEN:
			opStatus = s.opOpen(op.AsOPEN4args(), st, &w)
		case nfs4.OP_OPEN_DOWNGRADE:
			opStatus = s.opOpenDowngrade(op.AsOPENDOWNGRADE4args(), st, &w)
		case nfs4.OP_PUTFH:
			opStatus = s.opPutfh(op.AsPUTFH4args(), st, &w)
		case nfs4.OP_PUTPUBFH:
			opStatus = s.opPutrootfh(st, &w, true)
		case nfs4.OP_PUTROOTFH:
			opStatus = s.opPutrootfh(st, &w, false)
		case nfs4.OP_READ:
			opStatus = s.opRead(op.AsREAD4args(), st, &w)
		case nfs4.OP_READDIR:
			opStatus = s.opReaddir(op.AsREADDIR4args(), st, &w)
		case nfs4.OP_READLINK:
			opStatus = s.opReadlink(st, &w)
		case nfs4.OP_REMOVE:
			opStatus = s.opRemove(op.AsREMOVE4args(), st, &w)
		case nfs4.OP_RENAME:
			opStatus = s.opRename(op.AsRENAME4args(), st, &w)
		case nfs4.OP_RESTOREFH:
			opStatus = s.opRestorefh(st, &w)
		case nfs4.OP_SAVEFH:
			opStatus = s.opSavefh(st, &w)
		case nfs4.OP_SECINFO:
			opStatus = s.opSecinfo(op.AsSECINFO4args(), st, &w)
		case nfs4.OP_SETATTR:
			opStatus = s.opSetattr(op.AsSETATTR4args(), st, &w)
		case nfs4.OP_VERIFY:
			opStatus = s.opVerify(op.AsVERIFY4args(), st, &w)
		case nfs4.OP_WRITE:
			opStatus = s.opWrite(op.AsWRITE4args(), st, &w)
		case nfs4.OP_EXCHANGE_ID:
			opStatus = s.opExchangeID(op.AsEXCHANGEID4args(), st, &w)
		case nfs4.OP_CREATE_SESSION:
			opStatus = s.opCreateSession(op.AsCREATESESSION4args(), st, &w)
		case nfs4.OP_DESTROY_SESSION:
			opStatus = s.opDestroySession(op.AsDESTROYSESSION4args(), st, &w)
		case nfs4.OP_DESTROY_CLIENTID:
			opStatus = s.opDestroyClientID(op.AsDESTROYCLIENTID4args(), st, &w)
		case nfs4.OP_BIND_CONN_TO_SESSION:
			opStatus = s.opBindConnToSession(op.AsBINDCONNTOSESSION4args(), st, &w)
		case nfs4.OP_RECLAIM_COMPLETE:
			opStatus = s.opReclaimComplete(op.AsRECLAIMCOMPLETE4args(), st, &w)
		case nfs4.OP_TEST_STATEID:
			opStatus = s.opTestStateID(op.AsTESTSTATEID4args(), st, &w)
		case nfs4.OP_FREE_STATEID:
			opStatus = s.opFreeStateID(op.AsFREESTATEID4args(), st, &w)
		case nfs4.OP_BACKCHANNEL_CTL:
			opStatus = s.opBackchannelCtl(op.AsBACKCHANNELCTL4args(), st, &w)
		case nfs4.OP_SET_SSV:
			// Only SP4_SSV state protection uses an SSV, and nfsd offers
			// SP4_NONE (RFC 8881 section 18.47.3).
			writeOperationError(&w, opnum, nfs4.NFS4ERR_INVAL)
			opStatus = nfs4.NFS4ERR_INVAL
		case nfs4.OP_SECINFO_NO_NAME:
			opStatus = s.opSecinfoNoName(op.AsNfsArgop4OPSECINFONONAME().Value(), st, &w)
		default:
			// Every other operation is removed from NFSv4.1 (RFC 8881
			// section 18), optional (locks, delegations, pNFS, 4.2), or
			// unsupported by TernFS (LINK, OPENATTR). nfsd grants no
			// delegations, so DELEGRETURN is NOTSUPP too.
			status := uint32(nfs4.NFS4ERR_NOTSUPP)
			if !knownOperation(opnum) {
				status = nfs4.NFS4ERR_OP_ILLEGAL
			}
			writeOperationError(&w, opnum, status)
			opStatus = status
		}

		opCount++
		if uint64(rpcAcceptedHeaderSize+w.Len())+uint64(suffix) > uint64(limit) {
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
			opStatus = nfs4.NFS4ERR_REP_TOO_BIG
			if seq.sess != nil {
				opStatus = resultLimitStatus(&seq)
			}
			writeOperationError(&w, opnum, opStatus)
		}

		record(opnum, opStatus)
		if opStatus != nfs4.NFS4_OK {
			s.log.Debug("op failed", "op", nfs4.NfsOpnum4Name(opnum), "status", nfs4.Nfsstat4Name(opStatus))
			overallStatus = opStatus
			break
		}
		if index+1 < len(ops) {
			// The compound goes on: this operation's locks go now.
			st.releaseHolds()
		}
	}

done:
	s.log.Debug("COMPOUND done", "status", nfs4.Nfsstat4Name(overallStatus), "ops", opCount)

	w.SetStatus(overallStatus)
	body := w.Finish()

	if seq.slot != nil {
		seq.finish(body)
	}
	return body
}

// acceptsMinorVersion reports whether nfsd serves a COMPOUND of this minor
// version. With AllowMinorVersion2, NFSv4.2 is served as NFSv4.1: every
// operation new in 4.2 is optional (RFC 7862 section 1.4) and returns
// NFS4ERR_NOTSUPP. This exists for clients, such as libnfs 8, which offer
// sessions only as 4.2.
func (s *Server) acceptsMinorVersion(minor uint32) bool {
	return minor == nfsMinorVersion || minor == 2 && s.AllowMinorVersion2
}

// knownOperation reports opcodes defined by the XDR. OP_ILLEGAL is not one:
// a client sends it to provoke NFS4ERR_OP_ILLEGAL.
func knownOperation(opnum uint32) bool {
	return opnum >= nfs4.OP_ACCESS && opnum <= nfs4.OP_REMOVEXATTR
}

// holdsWriteState reports that this process has a write open of the
// incarnation, or a staged writer of it that is not retired: what makes a
// registration by another principal NFS4ERR_CLID_INUSE.
func (s *Server) holdsWriteState(clientID uint64) bool {
	for _, state := range s.opens.ClientStates(clientID) {
		if state.Writes() {
			return true
		}
	}
	for _, meta := range s.stagingStore.Entries() {
		if meta.ClientID == clientID && !meta.Retired {
			return true
		}
	}
	return false
}
