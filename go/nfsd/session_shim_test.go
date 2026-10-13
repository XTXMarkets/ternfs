// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The protocol tests build COMPOUNDs as an NFSv4.0 client would. sendRPC
// gives each one a session: it inserts SEQUENCE as the first operation and
// removes SEQUENCE's result from the reply, so tests index results from
// their own first operation.
//
// A connection uses the session of the client it last registered. A
// compound containing OPEN uses the session of the open owner's clientid,
// and one presenting a stateid an OPEN returned uses that OPEN's session,
// so a test can drive several clients over one connection: a stateid is
// only valid under its own client's sessions (canonical r3 §10).

type testSession struct {
	id       SessionID
	clientID uint64

	mu   sync.Mutex
	free []uint32
	seqs []uint32
	cond *sync.Cond

	lastSlot, lastSeq uint32
	retransmit        bool
	// warm is set by the session's first admitted SEQUENCE. Until then
	// SEQUENCE may get DELAY while the client's first lease is published,
	// and the shim retries, as Linux does (canonical r3 §9.1).
	warm atomic.Bool

	// For recovery after BADSESSION.
	identity  string
	verifier  [8]byte
	nextCSSeq uint32
}

func newTestSession(id SessionID, clientID uint64, slots uint32) *testSession {
	ts := &testSession{id: id, clientID: clientID, seqs: make([]uint32, slots)}
	ts.cond = sync.NewCond(&ts.mu)
	for i := int(slots) - 1; i >= 0; i-- {
		ts.free = append(ts.free, uint32(i))
	}
	return ts
}

func (ts *testSession) acquire() (slot, seq uint32) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for len(ts.free) == 0 {
		ts.cond.Wait()
	}
	slot = ts.free[len(ts.free)-1]
	ts.free = ts.free[:len(ts.free)-1]
	ts.seqs[slot]++
	return slot, ts.seqs[slot]
}

func (ts *testSession) release(slot uint32) {
	ts.mu.Lock()
	ts.free = append(ts.free, slot)
	ts.mu.Unlock()
	ts.cond.Broadcast()
}

var testSessions = struct {
	mu       sync.Mutex
	byClient map[string]*testSession
	byConn   map[net.Conn]*testSession
	// pending holds EXCHANGE_ID sequence ids for confirmClientID.
	pending map[string]uint32
	// nextCSSeq is each clientid's next CREATE_SESSION sequence. A client
	// keeps one sequence per clientid across all its connections.
	nextCSSeq map[uint64]uint32
	// stateids maps the stateids OPEN returned on a connection to their
	// clientid.
	stateids map[net.Conn]map[StateID]uint64
}{
	byClient:  make(map[string]*testSession),
	byConn:    make(map[net.Conn]*testSession),
	pending:   make(map[string]uint32),
	nextCSSeq: make(map[uint64]uint32),
	stateids:  make(map[net.Conn]map[StateID]uint64),
}

// noteOpenStateids records the stateids of a reply's successful OPENs.
func noteOpenStateids(conn net.Conn, clientID uint64, reply []byte) {
	res, err := parseCompoundReply(reply)
	if err != nil {
		return
	}
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	iter := res.Resarray()
	for iter.Next() {
		entry := iter.Resarray()
		if entry.Disc() != nfs4.OP_OPEN {
			continue
		}
		open := entry.Value().AsOPEN4resEntry()
		if open.Disc() != nfs4.NFS4_OK {
			continue
		}
		owners := testSessions.stateids[conn]
		if owners == nil {
			owners = make(map[StateID]uint64)
			testSessions.stateids[conn] = owners
		}
		owners[extractStateID(open.Value().AsOPEN4resok().Stateid())] = clientID
	}
}

// stateidOwner returns the clientid of a stateid recorded on conn that one
// of body's operations presents, read from the operations' stateid fields
// rather than from the raw bytes.
func stateidOwner(conn net.Conn, body []byte) (uint64, bool) {
	args, ok := nfs4.ReadCOMPOUND4args(body)
	if !ok {
		return 0, false
	}
	var presented []StateID
	it := args.Argarray()
	for it.Next() {
		op := it.Argarray().Value()
		switch op.Disc() {
		case nfs4.OP_READ:
			presented = append(presented, extractStateID(op.AsREAD4args().Stateid()))
		case nfs4.OP_WRITE:
			presented = append(presented, extractStateID(op.AsWRITE4args().Stateid()))
		case nfs4.OP_SETATTR:
			presented = append(presented, extractStateID(op.AsSETATTR4args().Stateid()))
		case nfs4.OP_CLOSE:
			presented = append(presented, extractStateID(op.AsCLOSE4args().OpenStateid()))
		case nfs4.OP_OPEN_DOWNGRADE:
			presented = append(presented, extractStateID(op.AsOPENDOWNGRADE4args().OpenStateid()))
		case nfs4.OP_FREE_STATEID:
			presented = append(presented, extractStateID(op.AsFREESTATEID4args().FsaStateid()))
		case nfs4.OP_TEST_STATEID:
			ts := op.AsTESTSTATEID4args()
			for i := 0; i < int(ts.TsStateidsCount()); i++ {
				presented = append(presented, extractStateID(ts.TsStateids(i)))
			}
		}
	}
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	for _, id := range presented {
		if clientID, ok := testSessions.stateids[conn][id]; ok {
			return clientID, true
		}
	}
	return 0, false
}

func noteCreateSessionSeq(clientID uint64, next uint32) {
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	testSessions.nextCSSeq[clientID] = next
}

func nextCreateSessionSeq(clientID uint64, fallback uint32) uint32 {
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	if next, ok := testSessions.nextCSSeq[clientID]; ok {
		return next
	}
	return fallback
}

func testClientKey(conn net.Conn, clientID uint64) string {
	return fmt.Sprintf("%s/%d", conn.RemoteAddr(), clientID)
}

func registerTestSession(conn net.Conn, ts *testSession) {
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	testSessions.byClient[testClientKey(conn, ts.clientID)] = ts
	testSessions.byConn[conn] = ts
}

func testSessionForClient(conn net.Conn, clientID uint64) *testSession {
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	return testSessions.byClient[testClientKey(conn, clientID)]
}

func testSessionForConn(conn net.Conn) *testSession {
	testSessions.mu.Lock()
	defer testSessions.mu.Unlock()
	return testSessions.byConn[conn]
}

// compoundFirstOp returns the offset of the argarray count and the first
// operation's opnum.
func compoundArgLayout(body []byte) (countOff int, firstOp uint32, ok bool) {
	if len(body) < 4 {
		return 0, 0, false
	}
	tagLen := int(binary.BigEndian.Uint32(body))
	countOff = 4 + (tagLen+3)&^3 + 4
	if countOff+4 > len(body) {
		return 0, 0, false
	}
	if binary.BigEndian.Uint32(body[countOff:]) == 0 || countOff+8 > len(body) {
		return countOff, 0, true
	}
	return countOff, binary.BigEndian.Uint32(body[countOff+4:]), true
}

// openOwnerClientID finds the clientid named by an OPEN in the compound.
func openOwnerClientID(body []byte) (uint64, bool) {
	args, ok := nfs4.ReadCOMPOUND4args(body)
	if !ok {
		return 0, false
	}
	iter := args.Argarray()
	for iter.Next() {
		entry := iter.Argarray()
		if entry.Disc() == nfs4.OP_OPEN {
			return entry.Value().AsOPEN4args().Owner().Clientid(), true
		}
	}
	return 0, false
}

func withSequence(body []byte, ts *testSession, slot, seq uint32) []byte {
	countOff, _, _ := compoundArgLayout(body)
	var op []byte
	op = binary.BigEndian.AppendUint32(op, nfs4.OP_SEQUENCE)
	op = append(op, ts.id[:]...)
	op = binary.BigEndian.AppendUint32(op, seq)
	op = binary.BigEndian.AppendUint32(op, slot)
	op = binary.BigEndian.AppendUint32(op, uint32(len(ts.seqs)-1))
	op = binary.BigEndian.AppendUint32(op, nfs4.TRUE) // cachethis
	out := make([]byte, 0, len(body)+len(op))
	out = append(out, body[:countOff]...)
	out = binary.BigEndian.AppendUint32(out, binary.BigEndian.Uint32(body[countOff:])+1)
	out = append(out, op...)
	return append(out, body[countOff+4:]...)
}

// withoutSequenceResult removes SEQUENCE's result from an RPC reply.
func withoutSequenceResult(reply []byte) []byte {
	if len(reply) < 24+8 {
		return reply
	}
	res := reply[24:]
	tagLen := int(binary.BigEndian.Uint32(res[4:]))
	countOff := 8 + (tagLen+3)&^3
	if countOff+12 > len(res) {
		return reply
	}
	count := binary.BigEndian.Uint32(res[countOff:])
	if count == 0 || binary.BigEndian.Uint32(res[countOff+4:]) != nfs4.OP_SEQUENCE {
		return reply
	}
	size := 8
	if binary.BigEndian.Uint32(res[countOff+8:]) == nfs4.NFS4_OK {
		size += 36 // SEQUENCE4resok
	}
	out := make([]byte, 0, len(reply)-size)
	out = append(out, reply[:24+countOff]...)
	out = binary.BigEndian.AppendUint32(out, count-1)
	return append(out, res[countOff+4+size:]...)
}

// sendSessionRPC sends a COMPOUND under the appropriate test session.
func sendSessionRPC(conn net.Conn, xid uint32, body []byte) ([]byte, error) {
	countOff, firstOp, ok := compoundArgLayout(body)
	if !ok || firstOp == nfs4.OP_SEQUENCE || sessionlessOp(firstOp) ||
		binary.BigEndian.Uint32(body[countOff-4:]) != nfsMinorVersion {
		return sendRawRPC(conn, xid, procCompound, body)
	}
	var ts *testSession
	if clientID, ok := openOwnerClientID(body); ok {
		ts = testSessionForClient(conn, clientID)
	} else if clientID, ok := stateidOwner(conn, body); ok {
		ts = testSessionForClient(conn, clientID)
	}
	if ts == nil {
		ts = testSessionForConn(conn)
	}
	if ts == nil {
		var err error
		if ts, err = createTestSession(conn, shimXIDs.Add(1), "test-client", [8]byte{}); err != nil {
			return nil, err
		}
	}
	ts.mu.Lock()
	retransmit := ts.retransmit
	ts.retransmit = false
	ts.mu.Unlock()
	var slot, seq uint32
	if retransmit {
		slot, seq = ts.acquireLast()
	} else {
		slot, seq = ts.acquire()
	}
	ts.mu.Lock()
	ts.lastSlot, ts.lastSeq = slot, seq
	ts.mu.Unlock()
	reply, err := sendWarming(conn, xid, body, ts, slot, seq)
	ts.release(slot)
	if err != nil {
		return nil, err
	}
	noteOpenStateids(conn, ts.clientID, withoutSequenceResult(reply))
	if sequenceStatus(reply) == nfs4.NFS4ERR_BADSESSION {
		// The server restarted or the lease expired. Recover as the Linux
		// client does: a new session for the clientid, or else a new
		// clientid, then resend.
		replacement, err := recoverTestSession(conn, xid, ts)
		if err != nil {
			return nil, err
		}
		if replacement == nil {
			return withoutSequenceResult(reply), nil
		}
		slot, seq := replacement.acquire()
		defer replacement.release(slot)
		reply, err = sendWarming(conn, shimXIDs.Add(1), body, replacement, slot, seq)
		if err != nil {
			return nil, err
		}
		noteOpenStateids(conn, replacement.clientID, withoutSequenceResult(reply))
	}
	return withoutSequenceResult(reply), nil
}

// sendWarming sends a compound on slot and seq. Until the session's first
// admitted SEQUENCE, a DELAY is retried on the same slot and sequence: the
// failed admission consumed nothing.
func sendWarming(conn net.Conn, xid uint32, body []byte, ts *testSession, slot, seq uint32) ([]byte, error) {
	for deadline := time.Now().Add(10 * time.Second); ; {
		reply, err := sendRawRPC(conn, xid, procCompound, withSequence(body, ts, slot, seq))
		if err != nil {
			return nil, err
		}
		status := sequenceStatus(reply)
		if status != nfs4.NFS4ERR_DELAY || ts.warm.Load() || time.Now().After(deadline) {
			if status == nfs4.NFS4_OK {
				ts.warm.Store(true)
			}
			return reply, nil
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// shimXIDs numbers the shim's own calls apart from the tests' xids.
var shimXIDs atomic.Uint32

func init() { shimXIDs.Store(0x80000000) }

func recoverTestSession(conn net.Conn, xid uint32, ts *testSession) (*testSession, error) {
	replacement, status, err := createSession(conn, shimXIDs.Add(1), ts.clientID,
		nextCreateSessionSeq(ts.clientID, ts.nextCSSeq))
	if err != nil {
		return nil, err
	}
	switch status {
	case nfs4.NFS4_OK:
		replacement.identity, replacement.verifier = ts.identity, ts.verifier
		return replacement, nil
	case nfs4.NFS4ERR_STALE_CLIENTID, nfs4.NFS4ERR_EXPIRED, nfs4.NFS4ERR_SEQ_MISORDERED:
	default:
		return nil, nil
	}
	if ts.identity == "" {
		return nil, nil
	}
	return createTestSession(conn, shimXIDs.Add(1), ts.identity, ts.verifier)
}

// sequenceStatus returns the status of the SEQUENCE result in a reply.
func sequenceStatus(reply []byte) uint32 {
	if len(reply) < 24+8 {
		return nfs4.NFS4_OK
	}
	res := reply[24:]
	tagLen := int(binary.BigEndian.Uint32(res[4:]))
	countOff := 8 + (tagLen+3)&^3
	if countOff+12 > len(res) || binary.BigEndian.Uint32(res[countOff+4:]) != nfs4.OP_SEQUENCE {
		return nfs4.NFS4_OK
	}
	return binary.BigEndian.Uint32(res[countOff+8:])
}

// retransmitNext makes the connection's next compound reuse the slot and
// sequence id of its previous one, as a client retransmission does.
func retransmitNext(conn net.Conn) {
	ts := testSessionForConn(conn)
	ts.mu.Lock()
	ts.retransmit = true
	ts.mu.Unlock()
}

// acquireLast waits for the last slot used and reuses its sequence id.
func (ts *testSession) acquireLast() (slot, seq uint32) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for {
		for i, free := range ts.free {
			if free == ts.lastSlot {
				ts.free = append(ts.free[:i], ts.free[i+1:]...)
				return ts.lastSlot, ts.lastSeq
			}
		}
		ts.cond.Wait()
	}
}

func exchangeIDBody(identity string, verifier [8]byte) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		ew := w.AppendArgarray_ExchangeId()
		cw := ew.StartEiaClientowner()
		setVerifier(cw.CoVerifier(), verifier)
		ew.Resume(cw.SetCoOwnerid([]byte(identity)).Finish())
		ew.SetEiaFlags(0)
		ew.SetEiaStateProtect_None()
		w.Resume(ew.Finish())
	})
}

func createSessionBody(clientID uint64, seq uint32) []byte {
	return buildCompoundBody(nil, func(w *nfs4.COMPOUND4argsWriter) {
		cw := w.AppendArgarray_CreateSession()
		cw.SetCsaClientid(clientID)
		cw.SetCsaSequence(seq)
		cw.SetCsaFlags(0)
		attrs := func(a nfs4.ChannelAttrs4Writer) []byte {
			a.SetCaMaxrequestsize(maxRPCMessage)
			a.SetCaMaxresponsesize(maxRPCMessage)
			a.SetCaMaxresponsesizeCached(maxSessionReplyCached)
			a.SetCaMaxoperations(maxCompoundOperations)
			a.SetCaMaxrequests(16)
			return a.Finish()
		}
		cw.Resume(attrs(cw.StartCsaForeChanAttrs()))
		cw.Resume(attrs(cw.StartCsaBackChanAttrs()))
		cw.SetCsaCbProgram(0x40000000)
		cw.AppendCsaSecParms_None()
		w.Resume(cw.Finish())
	})
}

func parseCompoundReply(reply []byte) (nfs4.COMPOUND4res, error) {
	body, err := parseRPCReply(reply)
	if err != nil {
		return nfs4.COMPOUND4res{}, err
	}
	res, ok := nfs4.ReadCOMPOUND4res(body)
	if !ok {
		return nfs4.COMPOUND4res{}, errors.New("failed to parse COMPOUND4res")
	}
	return res, nil
}

// exchangeID returns the clientid and the CREATE_SESSION sequence id.
func exchangeID(conn net.Conn, xid uint32, identity string, verifier [8]byte) (uint64, uint32, uint32, error) {
	reply, err := sendRawRPC(conn, xid, procCompound, exchangeIDBody(identity, verifier))
	if err != nil {
		return 0, 0, 0, err
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		return 0, 0, 0, err
	}
	if res.Status() != nfs4.NFS4_OK {
		return 0, 0, res.Status(), nil
	}
	iter := res.Resarray()
	iter.Next()
	ok := iter.Resarray().Value().AsEXCHANGEID4resEntry().Value().AsEXCHANGEID4resok()
	return ok.EirClientid(), ok.EirSequenceid(), nfs4.NFS4_OK, nil
}

func createSession(conn net.Conn, xid uint32, clientID uint64, seq uint32) (*testSession, uint32, error) {
	reply, err := sendRawRPC(conn, xid, procCompound, createSessionBody(clientID, seq))
	if err != nil {
		return nil, 0, err
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		return nil, 0, err
	}
	if res.Status() != nfs4.NFS4_OK {
		return nil, res.Status(), nil
	}
	iter := res.Resarray()
	iter.Next()
	ok := iter.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
	ts := newTestSession(readSessionID(ok.CsrSessionid()), clientID,
		ok.CsrForeChanAttrs().CaMaxrequests())
	ts.nextCSSeq = seq + 1
	noteCreateSessionSeq(clientID, seq+1)
	if previous := testSessionForClient(conn, clientID); previous != nil {
		ts.identity, ts.verifier = previous.identity, previous.verifier
	}
	registerTestSession(conn, ts)
	awaitShimLease(conn, clientID)
	return ts, nfs4.NFS4_OK, nil
}

func createTestSession(conn net.Conn, xid uint32, identity string, verifier [8]byte) (*testSession, error) {
	clientID, seq, status, err := exchangeID(conn, xid, identity, verifier)
	if err != nil {
		return nil, err
	}
	if status != nfs4.NFS4_OK {
		return nil, fmt.Errorf("EXCHANGE_ID status = %s", nfs4.Nfsstat4Name(status))
	}
	ts, status, err := createSession(conn, shimXIDs.Add(1), clientID, seq)
	if err != nil {
		return nil, err
	}
	if status != nfs4.NFS4_OK {
		return nil, fmt.Errorf("CREATE_SESSION status = %s", nfs4.Nfsstat4Name(status))
	}
	ts.identity, ts.verifier = identity, verifier
	return ts, nil
}

// encodeConfirm carries the CREATE_SESSION sequence id where NFSv4.0 tests
// held the SETCLIENTID_CONFIRM verifier.
func encodeConfirm(seq uint32) [8]byte {
	var confirm [8]byte
	binary.BigEndian.PutUint32(confirm[:4], seq)
	confirm[4] = 1
	return confirm
}

func decodeConfirm(confirm [8]byte) uint32 {
	return binary.BigEndian.Uint32(confirm[:4])
}

// waitForOpenFileWaiter signals once a second request waits on the file's
// open lock or on a pathname lock.
func (s *Server) waitForOpenFileWaiter(clientID uint64, fh []byte) <-chan struct{} {
	fileID, _ := vfs.FHToInodeID(fh)
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		for {
			s.opens.fileLocks.mu.Lock()
			entry := s.opens.fileLocks.entries[fileID]
			waiting := entry != nil && entry.refs > 1
			s.opens.fileLocks.mu.Unlock()
			// CLOSE takes the writer's pathname lock first.
			s.mutationLocks.mu.Lock()
			for _, e := range s.mutationLocks.entries {
				if e.refs > 1 {
					waiting = true
				}
			}
			s.mutationLocks.mu.Unlock()
			if waiting {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	return ch
}

// testIdentities remembers the EXCHANGE_ID arguments behind each clientid.
var testIdentities = identityMap{m: make(map[uint64]identityEntry)}

type identityEntry struct {
	identity string
	verifier [8]byte
}

type identityMap struct {
	mu sync.Mutex
	m  map[uint64]identityEntry
}

func (im *identityMap) set(clientID uint64, identity string, verifier [8]byte) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.m[clientID] = identityEntry{identity, verifier}
}

func (im *identityMap) get(clientID uint64) (string, [8]byte) {
	im.mu.Lock()
	defer im.mu.Unlock()
	e := im.m[clientID]
	return e.identity, e.verifier
}

// stagingIDForTest returns the staging inode of the writer with stateid. A
// write OPEN returns the published filehandle, not its staging inode.
func stagingIDForTest(t *testing.T, store StagingStore, stateid [16]byte) vfs.InodeID {
	t.Helper()
	var other StateID
	copy(other[:], stateid[4:])
	for id, meta := range store.Entries() {
		if meta.NFSStateID == other {
			return id
		}
	}
	t.Fatalf("no staging for stateid %x", stateid)
	return 0
}
