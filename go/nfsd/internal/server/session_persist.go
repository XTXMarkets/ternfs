// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/sessionstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
)

// Session files (RFC 8881 §2.10.6.5; README.md#persistent-sessions).
//
// A file a restart restores from must hold, for every slot, the newest
// record this process wrote, and the intent of every protected effect
// (RENAME, REMOVE, CREATE of a directory or symlink) this process permitted.
// A record write that fails breaks the first, so the file is unlinked and the
// session goes on in memory only. If the unlink fails too, the file is
// poisoned: no protected effect is permitted after that (SERVERFAULT), and the
// session is retired. An effect permitted before, whose intent the file holds,
// may still start or finish. A poisoned file lacks only replies, as a file
// does after a kill before the reply write, and a write that failed or was
// torn left the slot's newest record in place (sessionstore), so a restart
// from it is as safe as one after a kill.
//
// The file records the callback program and credential CREATE_SESSION gave,
// and not the callback sequence. So the file also goes before the session's
// first CB_COMPOUND and before BACKCHANNEL_CTL changes them: a file on disk
// is one whose session has done neither. If it cannot go, the callback or
// the change is refused and the session retired.

// sessionFile is a session's file and whether a restart may restore the
// session from it. The zero value is a session with no file, which no
// restart restores. A record write holds mu for reading and invalidation
// holds it for writing across the unlink, so no request finds the file gone
// while it is still on disk, and none writes, or is permitted a protected
// effect, until the unlink's outcome is known. Lock order: slot.mu, then
// mu; with mu held nothing is called but the file's own I/O.
type sessionFile struct {
	mu    sync.RWMutex
	f     *sessionstore.File // nil before attach and after close
	state sessionFileState
	srv   *Server
	// onDisk and poisoned mirror state for readers that never wait for mu:
	// metrics, diagnostics, and finish deciding to retire the session.
	onDisk, poisoned atomic.Bool
}

type sessionFileState uint8

const (
	fileNone     sessionFileState = iota // no file: in memory only
	fileLive                             // records are written
	fileGone                             // unlinked: in memory only
	filePoisoned                         // the unlink failed: no protected effect is permitted
)

// attach gives the session its file, before any request can use it.
func (sf *sessionFile) attach(f *sessionstore.File, srv *Server) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.f, sf.state, sf.srv = f, fileLive, srv
	sf.onDisk.Store(true)
}

// write writes rec to a live file. It returns the state it found and the
// write's error.
func (sf *sessionFile) write(slot uint32, rec sessionstore.Record) (sessionFileState, error) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if sf.state != fileLive {
		return sf.state, nil
	}
	return fileLive, sf.f.Write(slot, rec)
}

// permits reports whether a protected effect is permitted now: not once the
// file is poisoned. The permission is given under the file's lock, and an
// effect permitted before the file was poisoned may still start or finish
// after. An intent written earlier stands for it.
func (sf *sessionFile) permits() bool {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.state != filePoisoned
}

// invalidate unlinks a live file that no longer holds its slots' newest
// records, or whose session is about to change what it does not record.
// The descriptor stays open until close. It returns the state it leaves:
// gone, or poisoned if the unlink failed now or before.
func (sf *sessionFile) invalidate() (sessionFileState, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.state != fileLive {
		return sf.state, nil
	}
	if err := sf.srv.unlinkSessionFile(sf.f); err != nil {
		sf.state = filePoisoned
		sf.poisoned.Store(true)
		return sf.state, err
	}
	sf.state = fileGone
	sf.onDisk.Store(false)
	return sf.state, nil
}

// close ends the file with the session's last use. The session is gone, so
// its file goes too, a poisoned one included.
func (sf *sessionFile) close() {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.f == nil {
		return
	}
	if sf.state == fileLive || sf.state == filePoisoned {
		if err := sf.srv.unlinkSessionFile(sf.f); err != nil {
			sf.srv.log.Error("session file not removed; a restart restores the session from it",
				"path", sf.f.Path(), "err", err)
		} else {
			sf.state = fileGone
			sf.onDisk.Store(false)
		}
	}
	sf.f.Close()
	sf.f = nil
}

// openSessionFiles prepares the session store under the staging directory.
// Without a staging directory nothing is persisted.
func (s *Server) openSessionFiles(stagingDir string) {
	store, err := sessionstore.Open(filepath.Join(stagingDir, staging.SessionsDirName))
	if err != nil {
		s.log.Error("session store unavailable; sessions will not survive a restart", "err", err)
		return
	}
	s.sessionFiles = store
	s.newSessionFile = store.Create
}

// sessionRecord is what the session file's header holds about a session,
// beyond its ID: enough to rebuild it and bind it to its incarnation.
//
// Encoding (big-endian): [1] version=1, [8] incarnation, [8] wire clientid,
// fore and back channel attributes as 6 x [4] each (headerPad, maxReq,
// maxResp, maxCached, maxOps, maxReqs), [4] callback program, [1] flags
// (bit 0: back channel requested), [4] callback credential flavor,
// [2] credential body length, [N] body.
type sessionRecord struct {
	incarnation   uint64
	wireClientID  uint64
	fore, back    channelAttrs
	cbProgram     uint32
	backRequested bool
	cbSec         callbackSec
}

const sessionRecordFixedSize = 1 + 8 + 8 + 2*6*4 + 4 + 1 + 4 + 2

var errBadSessionRecord = errors.New("malformed session record")

func (r sessionRecord) encode() []byte {
	b := make([]byte, 0, sessionRecordFixedSize+len(r.cbSec.body))
	b = append(b, 1)
	b = binary.BigEndian.AppendUint64(b, r.incarnation)
	b = binary.BigEndian.AppendUint64(b, r.wireClientID)
	for _, a := range []channelAttrs{r.fore, r.back} {
		for _, v := range []uint32{a.headerPad, a.maxReq, a.maxResp, a.maxCached, a.maxOps, a.maxReqs} {
			b = binary.BigEndian.AppendUint32(b, v)
		}
	}
	b = binary.BigEndian.AppendUint32(b, r.cbProgram)
	var flags byte
	if r.backRequested {
		flags |= 1
	}
	b = append(b, flags)
	b = binary.BigEndian.AppendUint32(b, r.cbSec.flavor)
	b = binary.BigEndian.AppendUint16(b, uint16(len(r.cbSec.body)))
	return append(b, r.cbSec.body...)
}

func decodeSessionRecord(b []byte) (sessionRecord, error) {
	if len(b) < sessionRecordFixedSize || b[0] != 1 {
		return sessionRecord{}, errBadSessionRecord
	}
	r := sessionRecord{
		incarnation:  binary.BigEndian.Uint64(b[1:9]),
		wireClientID: binary.BigEndian.Uint64(b[9:17]),
	}
	off := 17
	for _, a := range []*channelAttrs{&r.fore, &r.back} {
		for _, v := range []*uint32{&a.headerPad, &a.maxReq, &a.maxResp, &a.maxCached, &a.maxOps, &a.maxReqs} {
			*v = binary.BigEndian.Uint32(b[off : off+4])
			off += 4
		}
	}
	r.cbProgram = binary.BigEndian.Uint32(b[off : off+4])
	r.backRequested = b[off+4]&1 != 0
	off += 5
	r.cbSec.flavor = binary.BigEndian.Uint32(b[off : off+4])
	bodyLen := int(binary.BigEndian.Uint16(b[off+4 : off+6]))
	off += 6
	if len(b) != off+bodyLen {
		return sessionRecord{}, errBadSessionRecord
	}
	if bodyLen > 0 {
		r.cbSec.body = append([]byte(nil), b[off:]...)
	}
	return r, nil
}

// encodeUser and decodeUser carry the effective user in slot records, for
// the false-retry check after a restart. Encoding: [4] flavor, [4] uid,
// [4] gid, groups string.
func encodeUser(u effectiveUser) []byte {
	b := make([]byte, 0, 12+len(u.groups))
	b = binary.BigEndian.AppendUint32(b, u.flavor)
	b = binary.BigEndian.AppendUint32(b, u.uid)
	b = binary.BigEndian.AppendUint32(b, u.gid)
	return append(b, u.groups...)
}

func decodeUser(b []byte) (effectiveUser, bool) {
	if len(b) < 12 {
		return effectiveUser{}, false
	}
	return effectiveUser{
		flavor: binary.BigEndian.Uint32(b[0:4]),
		uid:    binary.BigEndian.Uint32(b[4:8]),
		gid:    binary.BigEndian.Uint32(b[8:12]),
		groups: string(b[12:]),
	}, true
}

// slotAreaSize is the record area a session's slots need: the largest
// reply the session may cache or the largest intent, whichever is larger,
// its user, and the record overhead. A session may cache no reply at all
// and still rename between two names of the longest length.
func slotAreaSize(maxCached uint32) uint32 {
	return max(maxCached, maxIntentRecord) + maxUserRecord + sessionstore.RecordOverhead
}

// maxIntentRecord bounds an encoded intent (replay_intent.go): a RENAME's,
// with both names of maxTernNameLength, is the largest.
const maxIntentRecord = 1 + 2 + 8 + 2 + maxTernNameLength + 8 + 8 + 8 + 2 + maxTernNameLength + 8

// maxUserRecord bounds the encoded effective user in a record. AUTH_SYS
// allows 16 groups; the string form is well under this.
const maxUserRecord = 512

// createSessionFile makes the file for a creation's session, once the
// creation holds its slot and has negotiated the fore channel, and before
// it is committed, directly or by a confirmation. It runs without the
// store's lock: the file is the creation's until commitLocked gives it to
// the session, and release removes it on every other outcome. A file that
// cannot be made leaves the session in memory only, unless the failure left
// something at its path that cannot be removed, which a restart could
// find: then the creation fails with SERVERFAULT.
func (s *Server) createSessionFile(cr *creation) uint32 {
	if s.sessionFiles == nil {
		return nfs4.NFS4_OK
	}
	rec := sessionRecord{
		incarnation: cr.c.incarnation, wireClientID: cr.c.clientID, fore: cr.fore, back: cr.back,
		cbProgram: cr.cbProgram, backRequested: cr.backRequested, cbSec: cr.cbSec,
	}
	id := sessionstore.ID(cr.id)
	cr.fileAttempted = true
	f, err := s.newSessionFile(id, sessionstore.Header{
		SlotCount: cr.fore.maxReqs,
		AreaSize:  slotAreaSize(cr.fore.maxCached),
		Session:   rec.encode(),
	})
	if err == nil {
		cr.file = f
		return nfs4.NFS4_OK
	}
	s.metrics.replayWriteFailures.Add(1)
	if rmErr := s.sessionFiles.Remove(id); rmErr != nil {
		s.log.Error("session file not created, and what the attempt left cannot be removed; CREATE_SESSION fails",
			"session", sessionText(cr.id), "err", err, "remove_err", rmErr)
		return nfs4.NFS4ERR_SERVERFAULT
	}
	cr.fileAttempted = false
	s.log.Error("session file not created; the session will not survive a restart",
		"session", sessionText(cr.id), "err", err)
	return nfs4.NFS4_OK
}

// removeCreationFile removes the file of a creation that inserted no
// session. Its session never existed, so a restart could do nothing with
// the file, and would not restore it: it holds no request.
func (s *Server) removeCreationFile(cr *creation) {
	if cr.file != nil {
		if err := s.unlinkSessionFile(cr.file); err != nil {
			s.log.Warn("uncommitted session file not removed", "path", cr.file.Path(), "err", err)
		}
		cr.file.Close()
		cr.file = nil
	} else if cr.fileAttempted {
		if err := s.sessionFiles.Remove(sessionstore.ID(cr.id)); err != nil {
			s.log.Warn("uncommitted session file not removed", "session", sessionText(cr.id), "err", err)
		}
	}
	cr.fileAttempted = false
}

// persistSlot records a completed request on its slot. The caller holds
// the slot's mutex, which serialises writes to the slot's areas, and
// retires the session once it has let go of it if the file is poisoned.
func (s *Server) persistSlot(sess *session, slotID uint32, rec sessionstore.Record) {
	s.writeRecord(sess, slotID, rec)
}

// persistIntent records, before a protected effect, what the effect is
// about to do, on the compound's slot. It returns the status that permits
// the effect, or refuses it: SERVERFAULT when the session's file is on disk
// without the intent, for a restart from it would run the effect again
// blind. A retired session's file is still on disk until its last use, so
// its intents are recorded too. A compound without a session (none in
// practice: the operations that call this need one) writes nothing.
//
// A slot holds one intent per request: the session's file is removed before
// a compound's second mutating operation runs (mutationGate), so a second
// intent is never written over the first. An operation run again after a
// restart and found not applied
// has its intent on the slot already. It runs on that intent only if the
// one it would write now is the same: an edge another client replaced since
// the decision is not the one the intent protects, and the replay is
// DEADSESSION.
func (s *Server) persistIntent(st *compoundState, payload []byte) uint32 {
	if st.session == nil || st.slot == nil {
		return nfs4.NFS4_OK
	}
	switch st.intentOp {
	case 0:
	case st.opIndex + 1:
		if !bytes.Equal(payload, st.replay.raw) {
			s.log.Warn("replayed request undecidable: its target changed since the decision",
				"session", sessionText(st.session.id), "slot", st.slotID, "seq", st.seq, "index", st.opIndex)
			s.metrics.replayUndecidable.Add(1)
			return nfs4.NFS4ERR_DEADSESSION
		}
		if !st.session.file.permits() {
			return nfs4.NFS4ERR_SERVERFAULT
		}
		return nfs4.NFS4_OK
	}
	if !s.writeRecord(st.session, st.slotID, sessionstore.Record{
		Seq: st.seq, Kind: sessionstore.KindIntent,
		Digest: st.slot.digest, User: encodeUser(st.slot.user), Payload: payload,
	}) {
		return nfs4.NFS4ERR_SERVERFAULT
	}
	st.intentOp = st.opIndex + 1
	return nfs4.NFS4_OK
}

// dropSessionFile removes a session's file before a change the file does not
// record, named by what. It reports false when the file stays on disk, and
// the change must not be made.
func (s *Server) dropSessionFile(sess *session, what string) bool {
	state, err := sess.file.invalidate()
	if err != nil {
		s.log.Error("session file not removed before "+what+"; refusing it and retiring the session",
			"session", sessionText(sess.id), "err", err)
	}
	return state != filePoisoned
}

// writeRecord writes rec to the session's file. A write that fails leaves
// the file without the slot's newest record, so the file is unlinked and
// the session goes on in memory. It reports false when the file is still on
// disk without the record: the unlink failed, now or before.
func (s *Server) writeRecord(sess *session, slotID uint32, rec sessionstore.Record) bool {
	state, err := sess.file.write(slotID, rec)
	if state == fileLive && err == nil {
		s.metrics.replayRecords.Add(1)
		if rec.Kind == sessionstore.KindIntent {
			s.metrics.replayIntents.Add(1)
		} else {
			s.metrics.replayRecordBytes.Add(uint64(len(rec.Payload)))
		}
		return true
	}
	if state == fileLive {
		s.metrics.replayWriteFailures.Add(1)
		s.log.Warn("session record not written; removing the session file",
			"session", sessionText(sess.id), "slot", slotID, "seq", rec.Seq, "kind", rec.Kind, "err", err)
		if state, err = sess.file.invalidate(); err != nil {
			s.log.Error("session file not removed after a failed write; refusing protected effects and retiring the session",
				"session", sessionText(sess.id), "err", err)
		}
	}
	return state != filePoisoned
}

// restoreSessions rebuilds, at startup, every session whose file is on the
// staging disk, that recorded a request and whose incarnation is still the
// confirmed one, with its slots as the earlier process recorded them. A
// file that cannot be used is removed: its session is lost, and a client
// that retransmits to it gets BADSESSION as it would from any unknown
// session.
func (s *Server) restoreSessions() {
	if s.sessionFiles == nil {
		return
	}
	files, problems, err := s.sessionFiles.Load()
	if err != nil {
		s.log.Error("session files not read; sessions did not survive this restart", "err", err)
		return
	}
	// Every ID a file names is known from here on, restored or not, and is
	// never given to a new session.
	s.sessions.mu.Lock()
	for _, f := range files {
		s.sessions.observeSessionIDLocked(SessionID(f.ID()))
	}
	for _, p := range problems {
		s.sessions.observeSessionIDLocked(SessionID(p.ID))
	}
	s.sessions.mu.Unlock()
	for _, p := range problems {
		s.log.Warn("session file unusable, removed", "path", p.Path, "err", p.Err)
		os.Remove(p.Path)
	}
	restored := 0
	// Each incarnation needs a lease refresher record, as CREATE_SESSION
	// reserves one before it creates a session; without it every SEQUENCE
	// would get DELAY. An incarnation refused a record loses its sessions.
	leaseRecords := map[uint64]func(){}
	withSession := map[uint64]bool{}
	for _, f := range files {
		if !recordedRequest(f) {
			// Nothing to answer a retransmission with, and perhaps a
			// session the client never learnt of: its CREATE_SESSION
			// reply may have been lost with the process. Restored, it
			// would hold a session record until its incarnation ends.
			s.log.Info("session not restored: it recorded no request", "session", sessionText(SessionID(f.ID())))
			f.Remove()
			continue
		}
		rec, err := decodeSessionRecord(f.Header().Session)
		if err != nil {
			s.log.Warn("session not restored", "session", sessionText(SessionID(f.ID())), "err", err)
			f.Remove()
			continue
		}
		release, reserved := leaseRecords[rec.incarnation]
		if !reserved {
			if release, reserved = s.clients.ReserveLeaseRecord(rec.incarnation); !reserved {
				s.log.Warn("session not restored: no lease record credit", "session", sessionText(SessionID(f.ID())))
				f.Remove()
				continue
			}
			leaseRecords[rec.incarnation] = release
		}
		if s.restoreSession(f, rec) {
			restored++
			withSession[rec.incarnation] = true
		} else {
			f.Remove()
		}
	}
	// An incarnation whose sessions were all refused is not served here,
	// and this process must not publish a lease for it.
	for inc, release := range leaseRecords {
		if !withSession[inc] {
			release()
			delete(leaseRecords, inc)
		}
	}
	// A restored incarnation has no lease from this process yet. Publish
	// one now, as CREATE_SESSION does, and wait for it as CREATE_SESSION
	// does: a SEQUENCE that found it missing would get NFS4ERR_DELAY, and
	// the Linux client answers DELAY on a lone SEQUENCE, which its state
	// manager sends after a reconnect, by waiting its 15 s maximum with
	// every other request held behind it.
	var wg sync.WaitGroup
	for inc, release := range leaseRecords {
		s.clients.StartLeaseBootstrap(inc)
		wg.Add(1)
		go func(inc uint64, release func()) {
			defer wg.Done()
			defer release()
			if !s.clients.AwaitLeasePublished(inc, clientstore.BootstrapWait) {
				s.log.Warn("restored incarnation's lease not published in time", "incarnation", incarnationText(inc))
			}
		}(inc, release)
	}
	wg.Wait()
	if len(files) > 0 || len(problems) > 0 {
		s.log.Info("sessions restored", "restored", restored, "incarnations", len(leaseRecords), "files", len(files), "unusable", len(problems))
	}
	s.metrics.replaySessionsRestored.Add(uint64(restored))
}

// recordedRequest says whether any slot of a session file holds a record,
// a completed request's or an intent.
func recordedRequest(f *sessionstore.File) bool {
	for slot := uint32(0); slot < f.Header().SlotCount; slot++ {
		if _, ok := f.Slot(slot); ok {
			return true
		}
	}
	return false
}

// restoreSession rebuilds one session from its file and record. Its slots
// are rebuilt first, so that a reply the cache cannot hold refuses the whole
// session rather than leaving a slot that would run a completed request
// again; then it is inserted.
func (s *Server) restoreSession(f *sessionstore.File, rec sessionRecord) bool {
	id := SessionID(f.ID())
	h := f.Header()
	confirmed, err := s.clients.IsConfirmed(rec.incarnation)
	if err != nil || !confirmed {
		s.log.Info("session not restored: its incarnation is not the confirmed one",
			"session", sessionText(id), "incarnation", incarnationText(rec.incarnation), "err", err)
		return false
	}
	sess := &session{
		id:        id,
		clientID:  rec.incarnation,
		slots:     make([]sessionSlot, h.SlotCount),
		maxOps:    rec.fore.maxOps,
		maxReq:    rec.fore.maxReq,
		maxResp:   rec.fore.maxResp,
		maxCached: rec.fore.maxCached,

		cbProgram:     rec.cbProgram,
		cbSec:         rec.cbSec,
		backRequested: rec.backRequested,
		backMaxReq:    rec.back.maxReq,
		backMaxResp:   rec.back.maxResp,
		backMaxOps:    rec.back.maxOps,
		backMaxReqs:   rec.back.maxReqs,
	}
	sess.file.attach(f, s)
	sess.cb = newCBSender(s, sess, rec.cbProgram, rec.cbSec)
	sess.restored = true
	ss := s.sessions
	releaseBuffers := func() {
		for i := range sess.slots {
			if b := sess.slots[i].buffer; b != nil {
				b.release()
				sess.slots[i].buffer = nil
			}
		}
	}
	for i := range sess.slots {
		slot := &sess.slots[i]
		slot.restored = true
		rec, ok := f.Slot(uint32(i))
		if !ok {
			continue
		}
		slot.seq, slot.occupied, slot.digest = rec.Seq, true, rec.Digest
		slot.user, _ = decodeUser(rec.User)
		switch rec.Kind {
		case sessionstore.KindIntent:
			slot.hasIntent, slot.intent = true, rec.Payload
		case sessionstore.KindSlot:
			slot.cached = rec.CacheThis
			buf, ok := ss.reserveSlotReply(sess, slot, len(rec.Payload))
			if !ok {
				// A slot without its reply would run a completed request
				// again; the session is lost instead.
				s.log.Warn("session not restored: reply cache full", "session", sessionText(id), "slot", i)
				releaseBuffers()
				return false
			}
			ss.installSlotReply(slot, buf)
			buf.storeBody(rec.Payload)
			slot.reply, slot.reserved = buf.data, buf.charge
		}
	}
	ss.mu.Lock()
	if ss.records >= ss.recordLimit {
		ss.mu.Unlock()
		releaseBuffers()
		s.log.Warn("session not restored: no record credit", "session", sessionText(id))
		return false
	}
	if _, ok := ss.groupLocked(rec.incarnation); !ok {
		ss.mu.Unlock()
		releaseBuffers()
		s.log.Warn("session not restored: no group credit", "session", sessionText(id))
		return false
	}
	ss.insertSessionLocked(sess)
	ss.mu.Unlock()
	s.log.Info("session restored", "session", sessionText(id), "incarnation", incarnationText(rec.incarnation), "slots", len(sess.slots))
	return true
}
