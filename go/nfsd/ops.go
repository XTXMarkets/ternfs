// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"time"
)

// Stable write modes.
const (
	unstable4 = 0
	dataSync4 = 1
	fileSync4 = 2

	maxTernBytesLength = 255
	maxTernNameLength  = maxTernBytesLength
)

func finishDefaultResponse(w *COMPOUND4resWriter, buf []byte, status uint32) uint32 {
	w.Resume(binary.BigEndian.AppendUint32(buf, status))
	return status
}

type statusResponseWriter interface {
	SetStatus(uint32)
}

func finishStatusResponse(r statusResponseWriter, status uint32) uint32 {
	r.SetStatus(status)
	return status
}

func toTernFSName(data []byte) (string, uint32) {
	switch {
	case len(data) == 0:
		return "", NFS4ERR_INVAL
	case len(data) > maxTernNameLength:
		return "", NFS4ERR_NAMETOOLONG
	case bytes.Equal(data, []byte(".")),
		bytes.Equal(data, []byte("..")),
		bytes.IndexByte(data, '/') >= 0,
		bytes.IndexByte(data, 0) >= 0:
		return "", NFS4ERR_BADNAME
	default:
		return string(data), NFS4_OK
	}
}

func requireDirectory(id InodeID) uint32 {
	switch id.Type() {
	case InodeTypeDir:
		return NFS4_OK
	case InodeTypeSymlink:
		return NFS4ERR_SYMLINK
	default:
		return NFS4ERR_NOTDIR
	}
}

func requireRegularFile(id InodeID) uint32 {
	switch id.Type() {
	case InodeTypeFile:
		return NFS4_OK
	case InodeTypeDir:
		return NFS4ERR_ISDIR
	default:
		return NFS4ERR_INVAL
	}
}

// lookupDurableOpen requires valid local protocol state and a fleet-wide
// lease. The lease invalidates stale local state after a reboot is confirmed.
// After an nfsd restart there is no local state; the staging sidecar is then
// the durable proof that this stateid is the confirmed write-open for this
// transient file.
// lookupDurableOpen validates a stateid that clientID presented with
// filehandle fileID. The returned state's dataID is the inode the
// operation's I/O uses. Another client's stateid, live, recovered or
// expired, gets BAD_STATEID before any effect (canonical r3 §10).
func (s *Server) lookupDurableOpen(
	clientID uint64,
	stateid Stateid4,
	fileID InodeID,
) (openState, uint32) {
	state, status := s.opens.lookup(
		clientID,
		extractStateID(stateid),
		stateid.Seqid(),
		fileID,
	)
	if status == NFS4_OK {
		fileID = state.dataID
	} else if dataID, ok := s.recoveredDataID(clientID, extractStateID(stateid), fileID); ok {
		fileID = dataID
	} else if status == NFS4ERR_BAD_STATEID {
		return openState{}, status
	}
	// A failed writer cannot recover from its old sidecar. Preserve ordinary
	// stateid errors, then reject it locally without expiring healthy siblings.
	if s.stagingStore.Failed(fileID) {
		if status != NFS4_OK {
			return openState{}, status
		}
		return openState{}, NFS4ERR_IO
	}
	if status != NFS4_OK {
		id := extractStateID(stateid)
		meta, hasMeta := s.stagingStore.GetMeta(fileID)
		if !hasMeta || meta.NFSStateID != id || meta.ClientID != clientID || !s.opens.canRecover(id) {
			return openState{}, status
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, id, func() {
				s.retireStaging(fileID, meta)
			}); status != NFS4_OK {
				return openState{}, status
			}
		}
		return openState{
			id:         id,
			fileID:     fileID,
			dataID:     fileID,
			owner:      openOwnerKey{clientID: meta.ClientID},
			access:     recoveredAccess(meta),
			generation: stateid.Seqid(),
		}, NFS4_OK
	}
	// CLOSE can remove a failed writer's marker after the local lookup. Do
	// not let that race expire the client's unrelated writers.
	if status := s.expireInactiveOpen(state, func() bool { return s.stagingStore.Failed(fileID) }); status != NFS4_OK {
		return openState{}, status
	}
	return state, NFS4_OK
}

// expireInactiveOpen requires state's open to be active. When it is not, the
// operation gets EXPIRED, and the client's other state is untouched: local
// expiry is the lease gate's decision, which the sweeper drives (canonical
// r3 §8 caller table). An open whose own marker is gone loses its entry.
// The decisive check runs under a destruction hold, so while a confirmation
// protects the client it gives DELAY. failed reports a failed writer, which
// gives IO.
func (s *Server) expireInactiveOpen(state openState, failed func() bool) uint32 {
	status := s.requireActiveOpen(state)
	if status != NFS4ERR_EXPIRED {
		if status != NFS4_OK && failed != nil && failed() {
			return NFS4ERR_IO
		}
		return status
	}
	end, ok := s.beginDestroy(state.owner.clientID)
	if !ok {
		return NFS4ERR_DELAY // Not while confirming.
	}
	defer end()
	if status = s.requireActiveOpen(state); status != NFS4_OK && failed != nil && failed() {
		return NFS4ERR_IO
	}
	if status == NFS4ERR_EXPIRED && s.clients.MarkerGone(state.owner.clientID, state.id) {
		// This open itself is gone, as after a CLOSE that discarded its
		// writer: only its own local entry goes.
		s.opens.expire(state.id)
	}
	return status
}

// retireInactiveRecovered checks a recovered writer's open marker and, when
// it is gone, retires its staging. The decisive check and the retirement run
// under one destruction hold; while a confirmation protects the client it
// gives DELAY and retires nothing.
func (s *Server) retireInactiveRecovered(clientID uint64, id StateID, retire func()) uint32 {
	active, err := s.clients.HasOpen(clientID, id)
	if err != nil {
		return clientStoreErrToNFS(err)
	}
	if active {
		return NFS4_OK
	}
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return NFS4ERR_DELAY // Not while confirming.
	}
	defer end()
	if active, err = s.clients.HasOpen(clientID, id); err != nil {
		return clientStoreErrToNFS(err)
	}
	if active {
		return NFS4_OK
	}
	retire()
	return NFS4ERR_EXPIRED
}

// recoveredDataID finds the staging of clientID's write open issued by an
// earlier nfsd process. Its client may present the staging inode as the
// handle (as before this process), or the base inode the staging replaces.
// Another client's staging is not found.
func (s *Server) recoveredDataID(clientID uint64, id StateID, fileID InodeID) (InodeID, bool) {
	if !s.opens.canRecover(id) {
		return 0, false
	}
	if meta, ok := s.stagingStore.GetMeta(fileID); ok && meta.NFSStateID == id {
		return fileID, meta.ClientID == clientID
	}
	for stagingID, meta := range s.stagingStore.Entries() {
		if meta.NFSStateID == id && meta.BaseID == fileID {
			return stagingID, meta.ClientID == clientID
		}
	}
	return 0, false
}

func (s *Server) requireActiveOpen(state openState) uint32 {
	active, err := s.clients.HasOpen(state.owner.clientID, state.id)
	if err != nil {
		return clientStoreErrToNFS(err)
	}
	if !active {
		return NFS4ERR_EXPIRED
	}
	return NFS4_OK
}

// expireClientState drops local state without revoking a confirmed clientid.
func (s *Server) expireClientState(clientID uint64) {
	for _, state := range s.opens.clientStates(clientID) {
		if err := s.clients.RemoveOpen(clientID, state.id); err != nil &&
			nfsErrCode(err) != NFS4ERR_STALE_CLIENTID {
			s.log.Error("expire client: remove active-open record",
				"clientid", clientID, "stateid", state.id, "err", err)
			return
		}
	}
	for _, state := range s.opens.expireClient(clientID) {
		meta, ok := s.stagingStore.GetMeta(state.dataID)
		if ok && meta.ClientID == clientID && meta.NFSStateID == state.id {
			s.retireStaging(state.dataID, meta)
		}
	}
}

func (s *Server) opAccess(args ACCESS4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Access()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	meaningful := uint32(ACCESS4_READ | ACCESS4_MODIFY | ACCESS4_EXTEND)
	if st.currentID.Type() == InodeTypeDir {
		meaningful |= ACCESS4_LOOKUP | ACCESS4_DELETE
	} else {
		meaningful |= ACCESS4_EXECUTE
	}
	requested := args.Access() & meaningful
	ew := w.AppendResarray_Access()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetSupported(requested)
	ok.SetAccess(requested)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opClose(args CLOSE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		return writeCloseError(w, NFS4ERR_NOFILEHANDLE)
	}
	state, status := s.releaseWrite(args.OpenStateid(), st, true, nil)
	if status != NFS4_OK {
		return writeCloseError(w, status)
	}
	st.setCurrentStateID(state.id, state.generation)
	return writeCloseResponse(w, state)
}

func (s *Server) opOpenDowngrade(args OPENDOWNGRADE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_OpenDowngrade()
	fail := func(status uint32) uint32 {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if !st.currentIDSet {
		return fail(NFS4ERR_NOFILEHANDLE)
	}
	// The target must be a valid share value, a subset of the open's
	// grants, and the union of the values recorded inside it (canonical r3
	// §11). Deny is validated, not enforced. Narrowing BOTH to READ ends the
	// open's writer as CLOSE would; narrowing BOTH to WRITE drops READ and
	// keeps the writer.
	g := grant{access: args.ShareAccess() &^ OPEN4_SHARE_ACCESS_WANT_DELEG_MASK, deny: args.ShareDeny()}
	if g.access == 0 || g.access > OPEN4_SHARE_ACCESS_BOTH || g.deny > OPEN4_SHARE_DENY_BOTH {
		return fail(NFS4ERR_INVAL)
	}
	stateid, resolved := st.resolveStateID(args.OpenStateid())
	if !resolved {
		return fail(NFS4ERR_BAD_STATEID)
	}
	state, status := s.lookupDurableOpen(st.clientID, stateid, st.currentID)
	if status != NFS4_OK {
		return fail(status)
	}
	recovered := !s.opens.live(state.id)
	if status := state.downgradeStatus(g, recovered); status != NFS4_OK {
		return fail(status)
	}
	switch {
	case state.writes() && g.access == OPEN4_SHARE_ACCESS_READ:
		state, status = s.releaseWrite(args.OpenStateid(), st, false, &g)
	case recovered:
		state, status = s.adoptRecovered(st, stateid, g)
	default:
		state, status = s.regrant(st, stateid, g)
	}
	if status != NFS4_OK {
		return fail(status)
	}
	ok := ew.SetValue_Nfs4Ok()
	writeStateID(ok.OpenStateid(), state.generation, state.id)
	w.Resume(ew.Finish())
	st.setCurrentStateID(state.id, state.generation)
	return NFS4_OK
}

// releaseWrite ends an open's writer: the last writer of a staging file
// publishes it. With closing set the open is removed (CLOSE); otherwise it
// continues as a read open of the published version (OPEN_DOWNGRADE).
// to is OPEN_DOWNGRADE's target, checked again under the file lock.
func (s *Server) releaseWrite(openStateid Stateid4, st *compoundState, closing bool, to *grant) (openState, uint32) {
	stateid, ok := st.resolveStateID(openStateid)
	if !ok {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	sid := extractStateID(stateid)
	generation := stateid.Seqid()
	state, status := s.opens.lookup(st.clientID, sid, generation, st.currentID)
	recovered := false
	if status != NFS4_OK {
		// A write open issued by an earlier nfsd process is closed through
		// its staging sidecar.
		dataID, ok := s.recoveredDataID(st.clientID, sid, st.currentID)
		if !ok {
			return openState{}, status
		}
		meta, _ := s.stagingStore.GetMeta(dataID)
		recovered = true
		state = openState{
			id:         sid,
			fileID:     st.currentID,
			dataID:     dataID,
			owner:      openOwnerKey{clientID: meta.ClientID},
			access:     recoveredAccess(meta),
			generation: generation,
		}
	}
	// Lock order is pathname, then file. A writer's pathname comes from its
	// staging; an upgrade while we wait can give the open a writer, so
	// check again once both are held.
	dataID := state.dataID
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	unlockOpen := s.opens.lockFile(st.currentID)
	defer func() {
		unlockOpen()
		unlockTarget()
	}()
	if !recovered {
		current, status := s.opens.lookup(st.clientID, sid, generation, st.currentID)
		if status != NFS4_OK {
			return openState{}, status
		}
		if current.dataID != dataID {
			unlockOpen()
			unlockTarget()
			unlockOpen, unlockTarget = func() {}, func() {}
			return s.releaseWrite(openStateid, st, closing, to)
		}
		state = current
		if to != nil {
			if status := state.downgradeStatus(*to, false); status != NFS4_OK {
				return openState{}, status
			}
		}
	}
	fail := func(status uint32) (openState, uint32) {
		return openState{}, status
	}
	if !recovered {
		if status := s.expireInactiveOpen(state, nil); status != NFS4_OK {
			return fail(status)
		}
	}

	if recovered {
		if !hasMeta {
			return fail(NFS4ERR_EXPIRED)
		}
		if !closing && !s.opens.adoptable(recoveredOwner(meta), st.currentID, sid) {
			// The owner already has a live open of the file: the downgrade
			// could not continue as a read open, so it waits, before any
			// publication. Both opens are kept.
			return fail(NFS4ERR_DELAY)
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, sid, func() {
				s.retireStagingLocked(dataID, meta)
			}); status != NFS4_OK {
				return fail(status)
			}
		}
	}
	ownsStaging := hasMeta && (recovered ||
		state.owner.clientID == meta.ClientID &&
			(sid == meta.NFSStateID || state.dataID == dataID))
	if !ownsStaging {
		hasMeta = false
		meta = StagingMeta{}
	}

	sf := s.stagingStore.Get(dataID)
	if sf == nil {
		hasMeta = false
	}
	// published is the version this open's writer leaves at the name.
	published := InodeID(0)
	expiredClose := false
	if !closing && !state.writes() && !recovered {
		// Already read-only.
		state, _ = s.opens.downgrade(sid, state.dataID, *to)
		return state, NFS4_OK
	}
	if hasMeta && !recovered {
		if next, ok := s.opens.otherWriter(sid, dataID); ok {
			// Another open of this client still writes the shared staging.
			// The final write CLOSE publishes it; hand the sidecar's
			// ownership to that open so recovery and expiry follow it.
			if meta.NFSStateID == sid {
				if err := s.stagingStore.Rebind(dataID, next.owner.clientID, next.id); err != nil {
					return fail(NFS4ERR_IO)
				}
			}
			hasMeta = false
			sf = nil
			state.access &^= OPEN4_SHARE_ACCESS_WRITE
		}
	}
	// Check if there's a staging file for the current filehandle.
	if hasMeta {
		// Final write CLOSE: link the transient file.
		stagingID := dataID
		publish := !meta.Unlinked && (meta.BaseID == 0 || sf.Dirty())
		if publish && meta.BaseID != 0 && !sf.DataChanged() {
			// A timestamp-only CLOSE updates the current published version,
			// preserving any data published by another writer in the meantime.
			currentID, err := s.fs.Lookup(meta.DirID, meta.FileName)
			if err != nil {
				return fail(s.errToNFS(err))
			}
			info := sf.Stat()
			if err := s.fs.SetTime(currentID, &info.Mtime, &info.Atime); err != nil {
				st.observeMutation(s.fs, OP_CLOSE, err)
				return fail(s.errToNFS(err))
			}
			published = currentID
			s.opens.retarget(dataID)
			s.discardStaging(dataID)
		} else if publish {
			if meta.BaseID != 0 {
				s.startHydration(sf)
				if err := sf.FinishHydration(s.readBaseForeground); err != nil {
					s.failForReaders(dataID, sid)
					s.discardStaging(dataID)
					status := s.errToNFS(err)
					s.log.Error("close: hydrate replacement", "err", err,
						"status", Nfsstat4Name(status))
					return fail(status)
				}
			}
			r, rErr := sf.Reader()
			if rErr != nil {
				// Seek on an open regular file cannot fail; broken invariant.
				panic("close: staging Reader failed: " + rErr.Error())
			}
			linkErr := s.fs.LinkFile(stagingID, meta.TernCookie, meta.DirID, meta.FileName, r)
			if linkErr != nil {
				st.observeMutation(s.fs, OP_CLOSE, linkErr)
				// The write phase is not resumable after a partial LinkFile.
				s.failForReaders(dataID, sid)
				s.discardStaging(dataID)
				status := s.errToNFS(linkErr)
				s.log.Error("close: link file", "err", linkErr, "status", Nfsstat4Name(status))
				return fail(status)
			}
			info := sf.Stat()
			// LinkFile stamps the immutable inode with publication time.
			// Restore the writer's times, including explicit SETATTR values.
			timeErr := s.fs.SetTime(stagingID, &info.Mtime, &info.Atime)
			s.stagingStore.Remove(dataID)
			published = stagingID
			if timeErr != nil {
				// Publication already committed. Complete CLOSE so a retry
				// cannot turn the successful write into an expired open.
				s.log.Warn("close: published file but could not restore timestamps", "inode", stagingID, "err", timeErr)
			}
		} else {
			if !meta.Unlinked && meta.BaseID != 0 {
				// A clean CLOSE: readers of the staging read the unchanged
				// published file again.
				s.opens.retarget(dataID)
			}
			s.discardStaging(dataID)
		}
	} else {
		// Read CLOSE does not need the file to remain linked. A write CLOSE
		// without staging has lost the data it was meant to publish.
		if !recovered && state.writes() {
			s.log.Error("close: write staging missing",
				"file_id", dataID, "stateid", sid)
			expiredClose = true
		}
	}

	if published != 0 && state.owner.clientID != 0 {
		s.publishedAs(state.owner.clientID, st.currentID, published)
	}
	if !closing && !expiredClose {
		readID := published
		if readID == 0 {
			readID = s.clientView(state.owner.clientID, st.currentID)
		}
		if !recovered {
			state, _ = s.opens.downgrade(sid, readID, *to)
			return state, NFS4_OK
		}
		// A recovered writer downgraded to READ continues as a live read
		// open under its stateid, keeping its marker. Adoption was checked
		// before publication, under the same file lock.
		if adopted, ok := s.opens.adopt(recoveredOwner(meta), st.currentID, readID, *to, sid, nextStateidGeneration(generation)); ok {
			return adopted, NFS4_OK
		}
		panic("recovered downgrade: adoption checked under lockFile failed")
	}
	if recovered {
		state.generation = nextStateidGeneration(generation)
	} else if expiredClose {
		s.opens.expire(sid)
	} else {
		state, _ = s.opens.close(sid)
	}
	if state.owner.clientID != 0 {
		if err := s.clients.RemoveOpen(state.owner.clientID, sid); err != nil {
			s.log.Error("close: remove active-open record", "err", err)
		}
	}
	if expiredClose {
		return fail(NFS4ERR_EXPIRED)
	}
	return state, NFS4_OK
}

// failForReaders marks a staging failed, when opens other than the closing
// one read it, before a failed CLOSE discards it: its readers then get IO,
// never the old bytes or NOENT (canonical r3 §11).
func (s *Server) failForReaders(dataID InodeID, closing StateID) {
	if !s.opens.readers(dataID, closing) {
		return
	}
	if err := s.stagingStore.Quarantine(dataID); err != nil {
		s.log.Warn("close: mark staging failed for its readers", "inode", dataID, "err", err)
	}
}

// readSource is the inode a read OPEN by clientID of fileID uses, chosen
// under the file lock: this client's view, unless that is a staging which
// is gone or failed.
func (s *Server) readSource(clientID uint64, fileID InodeID) InodeID {
	id := s.clientView(clientID, fileID)
	if id == fileID {
		return id
	}
	if s.stagingStore.Failed(id) {
		return fileID
	}
	if _, ok := s.stagingStore.GetMeta(id); ok {
		return id
	}
	// A version this client published, or a staging since discarded.
	if _, err := s.fs.Stat(id); err != nil {
		return fileID
	}
	return id
}

// foreignStateID reports a stateid known to belong to another client
// incarnation: a live or expired open of another owner, or another
// client's recovered staging.
func (s *Server) foreignStateID(clientID uint64, id StateID) bool {
	if owner, known := s.opens.ownerOf(id); known {
		return owner != clientID
	}
	for _, meta := range s.stagingStore.Entries() {
		if meta.NFSStateID == id {
			return meta.ClientID != clientID
		}
	}
	return false
}

// recoveredOwner is the open owner a recovered writer's sidecar records.
func recoveredOwner(meta StagingMeta) openOwnerKey {
	owner := openOwnerKey{clientID: meta.ClientID}
	if meta.OwnerKnown {
		owner.owner = meta.OpenOwner
	}
	return owner
}

// adoptRecovered completes an OPEN_DOWNGRADE of a recovered writer that
// keeps WRITE: the narrowed grant and the next generation are installed as
// a live open under the same stateid, so later operations see them rather
// than a writer rebuilt from its sidecar. If the owner already has a live
// open of the file, it gets DELAY and nothing changes. Lock order is
// pathname, then file.
func (s *Server) adoptRecovered(st *compoundState, stateid Stateid4, g grant) (openState, uint32) {
	sid := extractStateID(stateid)
	dataID, ok := s.recoveredDataID(st.clientID, sid, st.currentID)
	if !ok {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	defer unlockTarget()
	unlockOpen := s.opens.lockFile(st.currentID)
	defer unlockOpen()
	if !hasMeta || meta.NFSStateID != sid || meta.ClientID != st.clientID {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	if s.opens.live(sid) {
		// Adopted meanwhile: the stateid is now ordinary live state.
		return openState{}, NFS4ERR_DELAY
	}
	owner := recoveredOwner(meta)
	state, ok := s.opens.adopt(owner, st.currentID, dataID, g, sid, nextStateidGeneration(stateid.Seqid()))
	if !ok {
		return openState{}, NFS4ERR_DELAY
	}
	s.views.addWriter(viewKey{st.clientID, st.currentID}, dataID)
	return state, NFS4_OK
}

// recoveredAccess is a recovered open's access: its writer's original
// grant is unknown, so a writer may keep or drop READ.
func recoveredAccess(meta StagingMeta) uint32 {
	if meta.ReadOnly {
		return OPEN4_SHARE_ACCESS_READ
	}
	return OPEN4_SHARE_ACCESS_BOTH
}

// regrant applies an OPEN_DOWNGRADE that keeps the open's writer, or
// narrows a read open, under the file lock: it checks the target again
// against the current grants and advances the generation.
func (s *Server) regrant(st *compoundState, stateid Stateid4, g grant) (openState, uint32) {
	unlock := s.opens.lockFile(st.currentID)
	defer unlock()
	sid := extractStateID(stateid)
	current, status := s.opens.lookup(st.clientID, sid, stateid.Seqid(), st.currentID)
	if status != NFS4_OK {
		return openState{}, status
	}
	if status := current.downgradeStatus(g, false); status != NFS4_OK {
		return openState{}, status
	}
	if current.writes() && g.access == OPEN4_SHARE_ACCESS_READ {
		// An upgrade raced in; the caller's path no longer applies.
		return openState{}, NFS4ERR_DELAY
	}
	state, ok := s.opens.downgrade(sid, current.dataID, g)
	if !ok {
		return openState{}, NFS4ERR_BAD_STATEID
	}
	return state, NFS4_OK
}

func writeCloseError(w *COMPOUND4resWriter, status uint32) uint32 {
	ew := w.AppendResarray_Close()
	ew.SetValue_Default(status)
	w.Resume(ew.Finish())
	return status
}

func writeCloseResponse(w *COMPOUND4resWriter, state openState) uint32 {
	ew := w.AppendResarray_Close()
	stid := ew.SetValue_Nfs4Ok()
	writeStateID(stid, state.generation, state.id)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opCommit(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Commit()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Commit()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	// COMMIT carries no stateid. Sync every writer this client has open
	// through the handle, and the handle's own staging after a restart.
	writers := append([]InodeID{st.currentID},
		s.views.writersOf(viewKey{st.clientID, st.currentID})...)
	failed := func() bool {
		for _, id := range writers {
			if s.stagingStore.Failed(id) {
				return true
			}
		}
		return false
	}
	if failed() {
		ew := w.AppendResarray_Commit()
		ew.SetValue_Default(NFS4ERR_IO)
		w.Resume(ew.Finish())
		return NFS4ERR_IO
	}
	for _, id := range writers {
		sf := s.stagingStore.Get(id)
		if sf == nil {
			continue
		}
		if err := sf.Sync(); err != nil {
			st.effectUnknown = true
			ew := w.AppendResarray_Commit()
			return finishDefaultResponse(w, ew.Finish(), NFS4ERR_IO)
		}
	}

	if failed() {
		ew := w.AppendResarray_Commit()
		ew.SetValue_Default(NFS4ERR_IO)
		w.Resume(ew.Finish())
		return NFS4ERR_IO
	}

	ew := w.AppendResarray_Commit()
	okW := ew.SetValue_Nfs4Ok()
	setData64(okW.Writeverf(), s.writeVerifier)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opCreate(args CREATE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if st.currentID.Type() != InodeTypeDir {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOTDIR)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_ROFS)
	}

	name, status := toTernFSName(args.Objname().Data())
	if status != NFS4_OK {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	objType := args.ObjtypeType()
	switch objType {
	case NF4DIR, NF4LNK:
	case NF4REG:
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_BADTYPE)
	default:
		// We don't support creating block/char/socket/fifo devices.
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOTSUPP)
	}
	if status := validateCreateAttrs(args.Createattrs()); status != NFS4_OK {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	unlock := s.lockMutationTargets(mutationTarget{
		dirID: st.currentID,
		name:  name,
	})
	defer unlock()
	busy, err := s.stagingTargetBusy(st.currentID, name)
	if err != nil {
		ew := w.AppendResarray_Create()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if busy {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_FILE_OPEN)
	}
	if _, err := s.fs.Lookup(st.currentID, name); err == nil {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_EXIST)
	} else if status := s.errToNFS(err); status != NFS4ERR_NOENT {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	var newID InodeID

	switch objType {
	case NF4DIR:
		newID, err = s.fs.Mkdir(st.currentID, name)
	case NF4LNK:
		targetData := args.Objtype().AsLinktext4().Data()
		if len(targetData) == 0 {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), NFS4ERR_INVAL)
		}
		if len(targetData) > maxTernBytesLength {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NAMETOOLONG)
		}
		target := string(targetData)
		newID, err = s.fs.Symlink(st.currentID, name, target)
	}

	if err != nil {
		st.observeMutation(s.fs, OP_CREATE, err)
		ew := w.AppendResarray_Create()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	st.setCurrentFH(newID)

	ew := w.AppendResarray_Create()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(cinfo, TRUE, now-1, now)

	// attrset bitmap: empty (we don't apply createattrs).
	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opGetattr(args GETATTR4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getattr()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	ni, err := s.stat(s.clientView(st.clientID, st.currentID))
	if err != nil {
		ew := w.AppendResarray_Getattr()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	reqMask := parseBitmap(args.AttrRequest())
	if status := validateGetattrMask(reqMask); status != NFS4_OK {
		ew := w.AppendResarray_Getattr()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	ew := w.AppendResarray_Getattr()
	okW := ew.SetValue_Nfs4Ok()

	// Compute response bitmap: intersection of requested and supported.
	var respMask attrMask
	respMask[0] = reqMask[0] & supportedAttrs0
	respMask[1] = reqMask[1] & supportedAttrs1
	respMask[2] = reqMask[2] & supportedAttrs2

	faw := okW.StartObjAttributes()
	attrBuf := encodeAttrs(respMask, st.currentID, ni)
	okW.Resume(writeFattr(&faw, respMask[:], attrBuf))
	ew.Resume(okW.Finish())
	w.Resume(ew.Finish())

	return NFS4_OK
}

func (s *Server) opGetfh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getfh()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	ew := w.AppendResarray_Getfh()
	okW := ew.SetValue_Nfs4Ok()
	fhW := okW.StartObject()
	buf := fhW.SetData(inodeIDToFH(st.currentID)).Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opLookup(args LOOKUP4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, status)
	}
	name, status := toTernFSName(args.Objname().Data())
	if status != NFS4_OK {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, status)
	}
	// Hide the internal .nfs directory from client access.
	if name == nfsDirName && st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, NFS4ERR_NOENT)
	}
	id, err := s.fs.Lookup(st.currentID, name)
	if err != nil {
		r := w.AppendResarray_Lookup()
		status := s.errToNFS(err)
		return finishStatusResponse(&r, status)
	}
	s.names.remember(id, st.currentID, name)
	st.setCurrentFH(id)
	r := w.AppendResarray_Lookup()
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opLookupp(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, status)
	}
	if st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, NFS4ERR_NOENT)
	}
	id, err := s.fs.LookupParent(st.currentID)
	if err != nil {
		r := w.AppendResarray_Lookupp()
		status := s.errToNFS(err)
		return finishStatusResponse(&r, status)
	}
	st.setCurrentFH(id)
	r := w.AppendResarray_Lookupp()
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opNverify(args NVERIFY4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Nverify()
		return finishStatusResponse(&r, NFS4ERR_NOFILEHANDLE)
	}
	same, status := s.verifyAttrs(st.currentID, s.clientView(st.clientID, st.currentID), args.ObjAttributes())
	if status != NFS4_OK {
		r := w.AppendResarray_Nverify()
		return finishStatusResponse(&r, status)
	}
	r := w.AppendResarray_Nverify()
	if same {
		// NVERIFY: fail if attributes are the same.
		return finishStatusResponse(&r, NFS4ERR_SAME)
	}
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opOpen(args OPEN4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	return s.openRetry(args, st, w, 0)
}

func (s *Server) openRetry(args OPEN4args, st *compoundState, w *COMPOUND4resWriter, retries int) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Open()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}

	claim := args.Claim()
	claimType := args.ClaimType()
	owner := args.Owner()
	// NFSv4.1 ignores the clientid in open_owner4 and uses the session's.
	clientID := st.clientID
	ownerKey := openOwnerKey{
		clientID: clientID,
		owner:    string(owner.Owner()),
	}
	fail := func(status uint32) uint32 {
		return writeOpenError(w, status)
	}

	access := args.ShareAccess() & OPEN4_SHARE_ACCESS_BOTH
	if s.stagingStore.ReadOnly() && access&OPEN4_SHARE_ACCESS_WRITE != 0 {
		return fail(NFS4ERR_ROFS)
	}

	// targetID is the inode the open's I/O uses; publishedID is the inode
	// the name resolves to, which is the filehandle OPEN returns.
	var targetID, publishedID InodeID
	var nfsSID StateID
	joined := false
	created := false
	staged := false
	reusedStaging := false
	var openSize *uint64
	var exclusiveVerifier *[8]byte
	var recoveredStagingID InodeID
	var recoveredStagingMeta StagingMeta
	var hasRecoveredStaging bool
	var placeholderID, placeholderDir InodeID
	var placeholderCookie Cookie
	var placeholderName string
	placeholderPublished := false
	defer func() {
		if placeholderID != 0 && !placeholderPublished {
			if err := s.fs.ScrapFile(placeholderID, placeholderCookie); err != nil &&
				!os.IsNotExist(err) {
				s.log.Warn("scrap unpublished empty file", "inode", placeholderID, "err", err)
			}
		}
	}()
	adoptRecoveredStaging := func() {
		nfsSID = s.opens.newStateID()
		reusedStaging = true
		staged = true
	}

	// openDir and openName are where the file is opened, remembered for
	// later opens by filehandle.
	var openDir InodeID
	var openName string
	// A writable CLAIM_FH this nfsd cannot name may recover a staged writer
	// (canonical r3 §6): the preselected staging, which the pathname lock
	// must confirm.
	var recovering bool
	var recoverID InodeID
	var recoverMeta StagingMeta
	switch claimType {
	case CLAIM_NULL, CLAIM_FH:
		var dirID InodeID
		var fileName string
		if claimType == CLAIM_FH {
			// Open the file the client already holds. A read open needs
			// no name. A write publishes at the file's name, which TernFS
			// cannot derive from the inode.
			fileID := st.currentID
			if status := requireRegularFile(fileID); status != NFS4_OK {
				if fileID.Type() == InodeTypeSymlink {
					status = NFS4ERR_SYMLINK
				} else if fileID.Type() == InodeTypeDir {
					status = NFS4ERR_ISDIR
				}
				return fail(status)
			}
			if args.OpenhowType() == OPEN4_CREATE {
				return fail(NFS4ERR_INVAL)
			}
			if access&OPEN4_SHARE_ACCESS_WRITE == 0 {
				if _, err := s.fs.Stat(fileID); err != nil {
					return fail(s.errToNFS(err))
				}
				targetID, publishedID = s.clientView(clientID, fileID), fileID
				break
			}
			var ok bool
			dirID, fileName, ok = s.nameOf(fileID)
			if !ok {
				// Neither name nor current: the file was replaced or this
				// nfsd never saw its name, as after a restart. Only a staged
				// writer this client can recover names it; otherwise the
				// client looks the name up again on STALE. The claim stays
				// CLAIM_FH.
				var status uint32
				recoverID, recoverMeta, status = s.claimFHRecoveryCandidate(fileID, clientID, ownerKey.owner)
				if status != NFS4_OK {
					return fail(status)
				}
				dirID, fileName = recoverMeta.DirID, recoverMeta.FileName
				recovering = true
			}
		} else {
			dirID = st.currentID
			if status := requireDirectory(dirID); status != NFS4_OK {
				return fail(status)
			}
			var status uint32
			fileName, status = toTernFSName(claim.AsNull().Data())
			if status != NFS4_OK {
				return fail(status)
			}
		}
		openDir, openName = dirID, fileName
		if access&OPEN4_SHARE_ACCESS_WRITE != 0 || args.OpenhowType() == OPEN4_CREATE {
			unlock := s.lockMutationTargets(mutationTarget{
				dirID: dirID,
				name:  fileName,
			})
			defer unlock()
			var recoveryErr error
			var releaseRecovery func()
			recoveredStagingID, recoveredStagingMeta,
				hasRecoveredStaging, releaseRecovery, recoveryErr = s.recoveredStagingTarget(
				dirID, fileName, clientID, ownerKey.owner,
				access&OPEN4_SHARE_ACCESS_WRITE != 0,
			)
			// Taking an expired incarnation's staging is destructive for it:
			// its hold lasts, with the pathname lock, until OPEN returns.
			defer releaseRecovery()
			if recovering && errors.Is(recoveryErr, nfsError(NFS4ERR_EXPIRED)) {
				// Another staging became selectable since preselection.
				return fail(NFS4ERR_STALE)
			}
			if recoveryErr != nil {
				return fail(clientStoreErrToNFS(recoveryErr))
			}
			if recovering && !s.sameRecoveryCandidate(recoverID, recoverMeta,
				recoveredStagingID, recoveredStagingMeta, hasRecoveredStaging) {
				// The preselected staging was rebound, removed or failed, or
				// another one is selected now: adopt nothing.
				return fail(NFS4ERR_STALE)
			}
		}

		// Check if this is a create.
		if args.OpenhowType() == OPEN4_CREATE {
			createHow := args.Openhow().AsCreatehow4Entry()
			var createAttrs Fattr4
			switch createHow.Disc() {
			case UNCHECKED4:
				createAttrs = createHow.Value().AsUnchecked4()
			case GUARDED4:
				createAttrs = createHow.Value().AsGuarded4()
			case EXCLUSIVE4:
				exclusiveVerifier = new([8]byte)
				*exclusiveVerifier = verifierBytes(createHow.Value().AsVerifier4())
			case EXCLUSIVE4_1:
				// SUPPATTR_EXCLCREAT is empty: nfsd sets none of the
				// attributes and reports an empty attrset, so the client
				// sets them with a later SETATTR.
				va := createHow.Value().AsCreatverfattr()
				exclusiveVerifier = new([8]byte)
				*exclusiveVerifier = verifierBytes(va.CvaVerf())
			}
			// An exclusive create carries a verifier, not attributes.
			if exclusiveVerifier == nil {
				if status := validateCreateAttrs(createAttrs); status != NFS4_OK {
					return fail(status)
				}
				var status uint32
				openSize, status = createAttrSize(createAttrs)
				if status != NFS4_OK {
					return fail(status)
				}
			}
			// Try lookup first.
			id, err := s.fs.Lookup(dirID, fileName)
			publishedID = id
			if err != nil {
				lookupStatus := s.errToNFS(err)
				if lookupStatus != NFS4ERR_NOENT {
					return fail(lookupStatus)
				}
				if s.stagingStore.ReadOnly() {
					return fail(NFS4ERR_ROFS)
				}
				if hasRecoveredStaging {
					if recoveredStagingMeta.BaseID != 0 {
						s.discardStaging(recoveredStagingID)
						hasRecoveredStaging = false
					}
				}
				// Keep the writer's transient distinct from the empty inode
				// that LOOKUP and READDIR will expose when OPEN succeeds.
				placeholderID, placeholderCookie, err = s.fs.ConstructFile(dirID)
				if err != nil {
					st.observeMutation(s.fs, OP_OPEN, err)
					return fail(s.errToNFS(err))
				}
				id = placeholderID
				publishedID = placeholderID
				if hasRecoveredStaging {
					id = recoveredStagingID
					adoptRecoveredStaging()
				}
				placeholderDir, placeholderName = dirID, fileName
				created = true
			} else if createHow.Disc() == GUARDED4 {
				return fail(NFS4ERR_EXIST)
			} else if exclusiveVerifier != nil {
				stagingID, meta, found := s.exclusiveStagingTarget(
					dirID, fileName, id, ownerKey, *exclusiveVerifier,
				)
				if !found || meta.ReadOnly != (access&OPEN4_SHARE_ACCESS_WRITE == 0) {
					return fail(NFS4ERR_EXIST)
				}
				existing, local := s.opens.existingOpen(ownerKey, publishedID)
				if local && existing.id == meta.NFSStateID {
					nfsSID = existing.id
					reusedStaging, staged = true, true
				} else if s.opens.canRecover(meta.NFSStateID) {
					adoptRecoveredStaging()
				} else {
					return fail(NFS4ERR_EXIST)
				}
				recoveredStagingID, recoveredStagingMeta = stagingID, meta
				id = stagingID
			}
			targetID = id
		} else {
			id, err := s.fs.Lookup(dirID, fileName)
			publishedID = id
			if err != nil {
				lookupStatus := s.errToNFS(err)
				if recovering && lookupStatus == NFS4ERR_NOENT {
					// The name no longer holds the client's handle.
					return fail(NFS4ERR_STALE)
				}
				if lookupStatus == NFS4ERR_NOENT &&
					hasRecoveredStaging &&
					recoveredStagingMeta.BaseID == 0 {
					adoptRecoveredStaging()
					targetID = recoveredStagingID
					created = true
					placeholderID, placeholderCookie, err = s.fs.ConstructFile(dirID)
					if err != nil {
						st.observeMutation(s.fs, OP_OPEN, err)
						return fail(s.errToNFS(err))
					}
					publishedID = placeholderID
					placeholderDir, placeholderName = dirID, fileName
				} else {
					if lookupStatus == NFS4ERR_NOENT &&
						hasRecoveredStaging {
						s.discardStaging(recoveredStagingID)
					}
					return fail(lookupStatus)
				}
			} else if claimType == CLAIM_FH && id != st.currentID {
				// Replaced since the name was checked.
				return fail(NFS4ERR_STALE)
			} else {
				// A read sees this client's own writes to the file.
				targetID = s.clientView(clientID, id)
			}
		}

		if !staged && access&OPEN4_SHARE_ACCESS_WRITE != 0 {
			// Join this client's writer of the same file.
			if writer, ok := s.opens.writerOf(clientID, publishedID); ok {
				targetID = writer.dataID
				staged, joined = true, true
				if writer.owner == ownerKey {
					nfsSID = writer.id
				}
			}
		}
		if !staged && (created || access&OPEN4_SHARE_ACCESS_WRITE != 0) {
			if status := requireRegularFile(targetID); status != NFS4_OK {
				if targetID.Type() == InodeTypeSymlink {
					status = NFS4ERR_SYMLINK
				}
				return fail(status)
			}
			baseID := targetID
			if hasRecoveredStaging {
				// Another writer may have published since this session opened.
				// Its immutable base and private edits still belong to this owner.
				adoptRecoveredStaging()
				targetID = recoveredStagingID
			} else {
				var baseInfo NodeInfo
				if baseID != placeholderID {
					var err error
					baseInfo, err = s.fs.Stat(baseID)
					if err != nil {
						return fail(s.errToNFS(err))
					}
				}
				var status uint32
				targetID, nfsSID, status = s.constructStaging(
					st, dirID, fileName, clientID, ownerKey.owner,
					access&OPEN4_SHARE_ACCESS_WRITE != 0, baseID, baseInfo,
					exclusiveVerifier,
				)
				if status != NFS4_OK {
					return fail(status)
				}
				staged = true
			}
		}
	case CLAIM_PREVIOUS:
		// No grace period — there is no lock/open state to reclaim.
		return fail(NFS4ERR_NO_GRACE)
	default:
		return fail(NFS4ERR_NOTSUPP)
	}

	stagingRebound := false
	cleanupStagedOpen := func() {
		if joined {
			return
		}
		if reusedStaging {
			if stagingRebound {
				if err := s.stagingStore.Rebind(
					recoveredStagingID,
					recoveredStagingMeta.ClientID,
					recoveredStagingMeta.NFSStateID,
				); err != nil {
					s.log.Error("restore recovered staging owner", "err", err)
				} else if recoveredStagingMeta.Retired {
					s.retireStagingLocked(recoveredStagingID, recoveredStagingMeta)
				}
			}
			return
		}
		s.discardStaging(targetID)
	}

	applyOpenSize := func() error {
		sf := s.stagingStore.Get(targetID)
		if sf == nil {
			panic("open: staged file has no staging data")
		}
		err := s.setStagingSize(sf, *openSize)
		if err != nil {
			st.effectUnknown = true
		}
		return err
	}
	if staged && openSize != nil && !reusedStaging {
		// A joined writer applies the size to the shared staging, as a
		// truncating open through the client's cache would.
		if err := applyOpenSize(); err != nil {
			cleanupStagedOpen()
			return fail(NFS4ERR_IO)
		}
	}

	if status := requireRegularFile(targetID); status != NFS4_OK {
		if targetID.Type() == InodeTypeSymlink {
			status = NFS4ERR_SYMLINK
		}
		return fail(status)
	}

	unlockOpen := s.opens.lockFile(publishedID)
	if !staged && !created && access&OPEN4_SHARE_ACCESS_WRITE == 0 {
		// A writer's CLOSE may have ended the staging this read selected
		// while it waited for the file (canonical r3 §11).
		targetID = s.readSource(clientID, publishedID)
	}
	if !staged && claimType == CLAIM_NULL && openDir != 0 && retries < 3 {
		// A CLOSE may have published over the name while we waited. Open
		// what the name holds now. Writers hold the pathname lock already.
		if current, err := s.fs.Lookup(openDir, openName); err == nil &&
			current != publishedID {
			unlockOpen()
			return s.openRetry(args, st, w, retries+1)
		}
	}
	defer unlockOpen()
	markID := nfsSID
	if existing, ok := s.opens.existingOpen(ownerKey, publishedID); ok {
		markID = existing.id
		if staged && !reusedStaging && !joined {
			// An upgrade keeps the existing stateid, so the new writer's
			// staging records it.
			if err := s.stagingStore.Rebind(targetID, clientID, markID); err != nil {
				cleanupStagedOpen()
				return fail(NFS4ERR_IO)
			}
			nfsSID = markID
		}
	}
	if markID == (StateID{}) {
		markID = s.opens.newStateID()
		nfsSID = markID
	}
	if err := s.clients.MarkOpen(clientID, markID); err != nil {
		if staged {
			cleanupStagedOpen()
		}
		return fail(clientStoreErrToNFS(err))
	}
	if reusedStaging && recoveredStagingMeta.NFSStateID != markID {
		if err := s.stagingStore.Rebind(
			recoveredStagingID, clientID, markID,
		); err != nil {
			_ = s.clients.RemoveOpen(clientID, markID)
			return fail(NFS4ERR_IO)
		}
		stagingRebound = true
		// Retired files release their descriptors; Rebind reopens them after
		// the new durable marker exists. Apply CREATE size only then.
		if openSize != nil {
			if err := applyOpenSize(); err != nil {
				_ = s.clients.RemoveOpen(clientID, markID)
				cleanupStagedOpen()
				return fail(NFS4ERR_IO)
			}
		}
	}
	if placeholderID != 0 {
		if err := s.fs.LinkFile(
			placeholderID, placeholderCookie, placeholderDir, placeholderName, nil,
		); err != nil {
			st.observeMutation(s.fs, OP_OPEN, err)
			_ = s.clients.RemoveOpen(clientID, markID)
			if staged {
				cleanupStagedOpen()
			}
			return fail(s.errToNFS(err))
		}
		placeholderPublished = true
	}
	state, err := s.opens.open(
		ownerKey,
		publishedID,
		targetID,
		grant{access: access, deny: args.ShareDeny()},
		nfsSID,
	)
	if err != nil {
		_ = s.clients.RemoveOpen(clientID, markID)
		if staged {
			cleanupStagedOpen()
		}
		return fail(NFS4ERR_SERVERFAULT)
	}
	if state.writes() {
		s.views.addWriter(viewKey{clientID, publishedID}, state.dataID)
	}
	if placeholderDir != 0 {
		s.names.remember(publishedID, placeholderDir, placeholderName)
	} else if openDir != 0 {
		s.names.remember(publishedID, openDir, openName)
	}
	response := openResponse{state: state, created: created}
	if reusedStaging &&
		recoveredStagingMeta.NFSStateID != response.state.id {
		if err := s.clients.RemoveOpen(
			recoveredStagingMeta.ClientID,
			recoveredStagingMeta.NFSStateID,
		); err != nil && !os.IsNotExist(err) {
			s.log.Warn("remove recovered staging open", "err", err)
		}
	}
	return writeOpenResponse(w, st, response)
}

func (s *Server) constructStaging(
	st *compoundState,
	dirID InodeID,
	fileName string,
	clientID uint64,
	openOwner string,
	write bool,
	baseID InodeID,
	baseInfo NodeInfo,
	exclusiveVerifier *[8]byte,
) (InodeID, StateID, uint32) {
	recoveryKey, err := s.clients.stagingRecoveryKey(clientID)
	if err != nil {
		return 0, StateID{}, clientStoreErrToNFS(err)
	}
	id, cookie, err := s.fs.ConstructFile(dirID)
	if err != nil {
		st.observeMutation(s.fs, OP_OPEN, err)
		return 0, StateID{}, s.errToNFS(err)
	}
	stateID := s.opens.newStateID()
	meta := StagingMeta{
		DirID:       dirID,
		FileName:    fileName,
		TernCookie:  cookie,
		NFSStateID:  stateID,
		ClientID:    clientID,
		BaseID:      baseID,
		BaseSize:    baseInfo.Size,
		Attrs:       baseInfo,
		OpenOwner:   openOwner,
		OwnerKnown:  true,
		ReadOnly:    !write,
		RecoveryKey: recoveryKey,
	}
	if exclusiveVerifier != nil {
		meta.Exclusive = true
		meta.Verifier = *exclusiveVerifier
	}
	if _, err := s.stagingStore.Create(id, meta); err != nil {
		_ = s.fs.ScrapFile(id, cookie)
		s.log.Error("staging create error", "err", err)
		return 0, StateID{}, NFS4ERR_IO
	}
	return id, stateID, NFS4_OK
}

type openResponse struct {
	state   openState
	created bool
}

func writeOpenResponse(
	w *COMPOUND4resWriter,
	st *compoundState,
	response openResponse,
) uint32 {
	ew := w.AppendResarray_Open()
	state := response.state
	st.setCurrentFH(state.fileID)
	st.setCurrentStateID(state.id, state.generation)
	okW := ew.SetValue_Nfs4Ok()

	writeStateID(okW.Stateid(), state.generation, state.id)

	now := uint64(time.Now().UnixNano())
	cinfo := okW.Cinfo()
	cinfo.SetAtomic(TRUE)
	if response.created {
		cinfo.SetBefore(now - 1)
	} else {
		cinfo.SetBefore(now)
	}
	cinfo.SetAfter(now)

	okW.SetRflags(OPEN4_RESULT_LOCKTYPE_POSIX)

	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)

	okW.SetDelegation_None()

	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func writeOpenError(w *COMPOUND4resWriter, status uint32) uint32 {
	ew := w.AppendResarray_Open()
	return finishDefaultResponse(w, ew.Finish(), status)
}

func (s *Server) opPutfh(args PUTFH4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	fhData := args.Object().Data()
	id, ok := fhToInodeID(fhData)
	if !ok {
		r := w.AppendResarray_Putfh()
		return finishStatusResponse(&r, NFS4ERR_BADHANDLE)
	}
	internal, err := s.clients.isInternalFilehandle(id)
	if err != nil {
		r := w.AppendResarray_Putfh()
		status := clientStoreErrToNFS(err)
		return finishStatusResponse(&r, status)
	}
	if internal {
		r := w.AppendResarray_Putfh()
		return finishStatusResponse(&r, NFS4ERR_BADHANDLE)
	}
	st.setCurrentFH(id)
	r := w.AppendResarray_Putfh()
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opPutrootfh(st *compoundState, w *COMPOUND4resWriter, isPub bool) uint32 {
	st.setCurrentFH(s.fs.RootID())
	if isPub {
		r := w.AppendResarray_Putpubfh()
		return finishStatusResponse(&r, NFS4_OK)
	} else {
		r := w.AppendResarray_Putrootfh()
		return finishStatusResponse(&r, NFS4_OK)
	}
}

func (s *Server) opRead(args READ4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Read()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Read()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	stateid, ok := st.resolveStateID(args.Stateid())
	if !ok {
		ew := w.AppendResarray_Read()
		ew.SetValue_Default(NFS4ERR_BAD_STATEID)
		w.Resume(ew.Finish())
		return NFS4ERR_BAD_STATEID
	}
	// dataID is the version this read sees: the reader's own staging, the
	// version it opened, or for a special stateid the client's latest view
	// of the handle.
	dataID := s.clientView(st.clientID, st.currentID)
	if !isSpecialStateID(stateid) {
		state, status := s.lookupDurableOpen(st.clientID, stateid, st.currentID)
		if status != NFS4_OK {
			ew := w.AppendResarray_Read()
			return finishDefaultResponse(w, ew.Finish(), status)
		}
		dataID = state.dataID
	}
	sf := s.stagingStore.Get(dataID)
	if s.stagingStore.Failed(dataID) {
		ew := w.AppendResarray_Read()
		ew.SetValue_Default(NFS4ERR_IO)
		w.Resume(ew.Finish())
		return NFS4ERR_IO
	}

	offset := args.Offset()
	count := min(args.Count(), maxReadWrite)

	buf := make([]byte, count)

	var n int
	var eof bool
	var err error

	if sf != nil {
		n, eof, err = sf.Read(offset, buf, s.readBaseForeground)
		if errors.Is(err, errStagingRemoved) && !s.stagingStore.Failed(dataID) {
			n, eof, err = s.readBaseForeground(
				s.clientView(st.clientID, st.currentID), offset, buf,
			)
		}
	} else {
		n, eof, err = s.readBaseForeground(dataID, offset, buf)
	}

	if s.stagingStore.Failed(dataID) {
		ew := w.AppendResarray_Read()
		ew.SetValue_Default(NFS4ERR_IO)
		w.Resume(ew.Finish())
		return NFS4ERR_IO
	}
	if err != nil {
		ew := w.AppendResarray_Read()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	ew := w.AppendResarray_Read()
	okW := ew.SetValue_Nfs4Ok()
	if eof {
		okW = okW.SetEof(TRUE)
	} else {
		okW = okW.SetEof(FALSE)
	}
	okW = okW.SetData(buf[:n])
	rbuf := okW.Finish()
	ew.Resume(rbuf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opReaddir(args READDIR4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Readdir()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Readdir()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	cookie := args.Cookie()
	if cookie == 1 || cookie == 2 {
		ew := w.AppendResarray_Readdir()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_BAD_COOKIE)
	}
	maxCount := args.Maxcount()
	if maxCount > 1<<20 {
		maxCount = 1 << 20
	}
	if st.replyBudget != 0 {
		// maxcount counts READDIR4resok, excluding its discriminant/status.
		maxCount = min(maxCount, st.replyBudget-8)
	}

	// Validate cookieverf on continuation requests (RFC 7530 §16.24.4).
	// cookieverf = directory mtime; if it changed, the listing is stale.
	// Some clients (e.g. libnfs) send all-zero cookieverf on continuations
	// ("should" echo it back per RFC, not "MUST"), so only check non-zero.
	if cookie != 0 {
		clientVerf := getData64(args.Cookieverf())
		if clientVerf != [8]byte{} {
			ni, err := s.fs.Stat(st.currentID)
			if err != nil {
				ew := w.AppendResarray_Readdir()
				status := s.errToNFS(err)
				return finishDefaultResponse(w, ew.Finish(), status)
			}
			var expectedVerf [8]byte
			binary.BigEndian.PutUint64(expectedVerf[:], uint64(ni.Mtime.UnixNano()))
			if clientVerf != expectedVerf {
				ew := w.AppendResarray_Readdir()
				return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOT_SAME)
			}
		}
	}

	reqMask := parseBitmap(args.AttrRequest())
	if status := validateGetattrMask(reqMask); status != NFS4_OK {
		ew := w.AppendResarray_Readdir()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	dirCount := args.Dircount()
	if dirCount > 1<<20 {
		dirCount = 1 << 20
	}

	// Stream one backend page at a time into the response writer. Only the
	// page being consumed and two candidate entries are held besides the
	// writer; the work reservation charges both (reply_resources.go).
	//
	// NFS cookie semantics: cookie=X means "I already have entries up to
	// and including X". Skip entries with NameHash <= cookie on continuations,
	// since VFS Readdir returns entries with hash >= startHash (inclusive).
	// Also hide the internal .nfs directory from client directory listings.
	var page []DirEntry
	startHash := cookie
	lastPage := false
	nextEntry := func() (DirEntry, bool, error) {
		for {
			for len(page) > 0 {
				e := page[0]
				page = page[1:]
				if cookie != 0 && e.NameHash <= cookie {
					continue
				}
				if st.currentID == s.fs.RootID() && e.Name == nfsDirName {
					continue
				}
				return e, true, nil
			}
			if lastPage {
				return DirEntry{}, false, nil
			}
			page = nil // Drop the consumed page before fetching the next.
			entries, nextHash, err := s.fs.Readdir(st.currentID, startHash)
			if err != nil {
				return DirEntry{}, false, err
			}
			page = entries
			lastPage = nextHash == 0
			startHash = nextHash
		}
	}
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_Readdir()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	var respMask attrMask
	respMask[0] = reqMask[0] & supportedAttrs0
	respMask[1] = reqMask[1] & supportedAttrs1
	respMask[2] = reqMask[2] & supportedAttrs2
	attrSize := len(encodeAttrs(respMask, 0, NodeInfo{}))
	// Two candidates alternate: the one being written and its successor,
	// whose size decides the written entry's nextentry discriminant.
	candidates := [2]readdirEntry{
		{respMask: respMask, attrData: make([]byte, 0, attrSize)},
		{respMask: respMask, attrData: make([]byte, 0, attrSize)},
	}
	scratch := [2][]byte{candidates[0].attrData, candidates[1].attrData}
	prepare := func(i int, e DirEntry) {
		re := &candidates[i]
		re.DirEntry = e
		re.attrData = nil // Stat failed: encode empty attrs.
		if ni, err := s.stat(e.ID); err == nil {
			re.attrData = appendAttrs(scratch[i][:0], respMask, e.ID, ni)
			scratch[i] = re.attrData
		}
	}

	// Enforce maxcount and dircount with exact XDR sizes.
	// maxcount covers the entire READDIR4resok: cookieverf(8) +
	// entries_present(4) + entry chain + eof(4). The entry chain
	// ends with a terminal FALSE(4) (either entries_present=FALSE
	// when empty, or the last entry's nextentry=FALSE).
	// dircount limits directory-information bytes: cookie + name per entry.
	maxBudget := int(maxCount) - readdirResokOverhead
	dirBudget := int(dirCount)
	usedEntryBytes, usedDirBytes := 0, 0
	fits := func(re *readdirEntry) bool {
		return usedEntryBytes+readdirEntryXDRSize(re) <= maxBudget &&
			usedDirBytes+readdirEntryDirSize(re) <= dirBudget
	}

	e, more, err := nextEntry()
	if err != nil {
		return fail(s.errToNFS(err))
	}
	if maxBudget < 0 {
		return fail(NFS4ERR_TOOSMALL)
	}
	cur := 0
	if more {
		prepare(cur, e)
		if !fits(&candidates[cur]) {
			return fail(NFS4ERR_TOOSMALL)
		}
	}

	// A later backend error discards the partial page, as if it had
	// happened before any entry was written.
	start := *w
	ew := w.AppendResarray_Readdir()
	okW := ew.SetValue_Nfs4Ok()

	// Set cookieverf to directory mtime.
	ni, _ := s.fs.Stat(st.currentID)
	verf := okW.Cookieverf()
	var mtimeBytes [8]byte
	binary.BigEndian.PutUint64(mtimeBytes[:], uint64(ni.Mtime.UnixNano()))
	setData64(verf, mtimeBytes)

	dirW := dirlistWriter{dir: okW.StartReply(), name: make([]byte, 0, maxTernNameLength)}
	eof := !more
	for more {
		re := &candidates[cur]
		usedEntryBytes += readdirEntryXDRSize(re)
		usedDirBytes += readdirEntryDirSize(re)
		e, more, err = nextEntry()
		if err != nil {
			*w = start
			return fail(s.errToNFS(err))
		}
		next := 1 - cur
		if more {
			prepare(next, e)
		}
		eof = !more
		follows := more && fits(&candidates[next])
		dirW.entry(re, follows)
		if !follows {
			break
		}
		cur = next
	}
	okW.Resume(dirW.finish(eof))
	ew.Resume(okW.Finish())
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opReadlink(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Readlink()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if st.currentID.Type() != InodeTypeSymlink {
		ew := w.AppendResarray_Readlink()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_INVAL)
	}
	target, err := s.fs.Readlink(st.currentID)
	if err != nil {
		ew := w.AppendResarray_Readlink()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if len(target) > 64<<10 {
		// The work envelope includes bounded variable-result scratch. Backend
		// strings beyond it cannot be copied into a reply.
		writeOperationError(w, OP_READLINK, NFS4ERR_REP_TOO_BIG)
		return NFS4ERR_REP_TOO_BIG
	}
	ew := w.AppendResarray_Readlink()
	okW := ew.SetValue_Nfs4Ok()
	lw := okW.StartLink()
	buf := lw.SetData([]byte(target)).Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opRemove(args REMOVE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_ROFS)
	}

	name, status := toTernFSName(args.Target().Data())
	if status != NFS4_OK {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	unlock := s.lockMutationTargets(mutationTarget{
		dirID: st.currentID,
		name:  name,
	})
	defer unlock()
	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if _, err := s.retireInactiveStagingTarget(st.currentID, name); err != nil {
		return fail(s.errToNFS(err))
	}
	writers := s.namespaceWriters(st.currentID, name, nil)
	edge, err := s.fs.LookupEdge(st.currentID, name)
	if err != nil {
		return fail(s.errToNFS(err))
	}
	if err := s.guardNamespaceWriters(writers); err != nil {
		s.log.Warn("cannot guard REMOVE writers", "err", err)
		return fail(NFS4ERR_IO)
	}
	err = s.fs.RemoveEdge(st.currentID, name, edge)
	st.observeMutation(s.fs, OP_REMOVE, err)
	if status := s.finishNamespaceWriters(writers, OP_REMOVE, err); status != NFS4_OK {
		return fail(status)
	}

	ew := w.AppendResarray_Remove()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(cinfo, TRUE, now-1, now)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opRename(args RENAME4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	// RENAME uses savedFH as source directory and currentFH as target directory.
	if !st.currentIDSet || !st.savedIDSet {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.savedID); status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_ROFS)
	}

	oldName, status := toTernFSName(args.Oldname().Data())
	if status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	newName, status := toTernFSName(args.Newname().Data())
	if status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if st.savedID == st.currentID && oldName == newName {
		ew := w.AppendResarray_Rename()
		okW := ew.SetValue_Nfs4Ok()
		now := uint64(time.Now().UnixNano())
		setChangeInfo(okW.SourceCinfo(), TRUE, now, now)
		setChangeInfo(okW.TargetCinfo(), TRUE, now, now)
		w.Resume(ew.Finish())
		return NFS4_OK
	}

	unlock := s.lockMutationTargets(
		mutationTarget{dirID: st.savedID, name: oldName},
		mutationTarget{dirID: st.currentID, name: newName},
	)
	defer unlock()

	fail := func(status uint32) uint32 {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	sourceActive, err := s.retireInactiveStagingTarget(st.savedID, oldName)
	if err != nil {
		return fail(s.errToNFS(err))
	}
	if _, err := s.retireInactiveStagingTarget(st.currentID, newName); err != nil {
		return fail(s.errToNFS(err))
	}
	if sourceActive && st.savedID != st.currentID {
		// The transient inode belongs to its construction shard. Moving a live
		// writer needs a copy into another inode, which this design leaves out.
		return fail(NFS4ERR_FILE_OPEN)
	}
	var sourceTarget *mutationTarget
	if st.savedID == st.currentID {
		sourceTarget = &mutationTarget{dirID: st.currentID, name: newName}
	}
	writers := s.namespaceWriters(st.savedID, oldName, sourceTarget)
	writers = append(writers, s.namespaceWriters(st.currentID, newName, nil)...)
	edge, err := s.fs.LookupEdge(st.savedID, oldName)
	if err != nil {
		return fail(s.errToNFS(err))
	}
	dst, err := s.fs.Lookup(st.currentID, newName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(s.errToNFS(err))
	}
	if err == nil && (edge.ID.Type() == InodeTypeDir) != (dst.Type() == InodeTypeDir) {
		return fail(NFS4ERR_EXIST)
	}
	// Preflight cannot freeze the destination. A later type race is a
	// submitted failure and can quarantine writers on the remote backend.
	if err := s.guardNamespaceWriters(writers); err != nil {
		s.log.Warn("cannot guard RENAME writers", "err", err)
		return fail(NFS4ERR_IO)
	}
	err = s.fs.RenameEdge(st.savedID, oldName, edge, st.currentID, newName)
	st.observeMutation(s.fs, OP_RENAME, err)
	if status := s.finishNamespaceWriters(writers, OP_RENAME, err); status != NFS4_OK {
		return fail(status)
	}

	ew := w.AppendResarray_Rename()
	okW := ew.SetValue_Nfs4Ok()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(okW.SourceCinfo(), TRUE, now-1, now)
	setChangeInfo(okW.TargetCinfo(), TRUE, now-1, now)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opSavefh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Savefh()
		return finishStatusResponse(&r, NFS4ERR_NOFILEHANDLE)
	}
	st.savedID = st.currentID
	st.savedIDSet = true
	st.savedStateID, st.savedGeneration, st.savedStateIDSet =
		st.currentStateID, st.currentGeneration, st.currentStateIDSet
	r := w.AppendResarray_Savefh()
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opRestorefh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.savedIDSet {
		r := w.AppendResarray_Restorefh()
		return finishStatusResponse(&r, NFS4ERR_RESTOREFH)
	}
	st.setCurrentFH(st.savedID)
	st.currentStateID, st.currentGeneration, st.currentStateIDSet =
		st.savedStateID, st.savedGeneration, st.savedStateIDSet
	r := w.AppendResarray_Restorefh()
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opSecinfo(args SECINFO4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	name, status := toTernFSName(args.Name().Data())
	if status != NFS4_OK {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if name == nfsDirName && st.currentID == s.fs.RootID() {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOENT)
	}
	if _, err := s.fs.Lookup(st.currentID, name); err != nil {
		ew := w.AppendResarray_Secinfo()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	// Return AUTH_SYS (flavor 1) and AUTH_NONE (flavor 0).
	ew := w.AppendResarray_Secinfo()
	okW := ew.SetValue_Nfs4Ok()
	okW.AppendData_Default(authSys)
	okW.AppendData_Default(authNone)
	buf := okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	// SECINFO consumes the current filehandle (RFC 8881 section 2.6.3.1.1.8).
	st.currentIDSet = false
	return NFS4_OK
}

func (s *Server) opSetattr(args SETATTR4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	// setattrReply writes a SETATTR response with the given status and
	// optional result bitmap. Factored out because the response always
	// requires an attrsset bitmap, even on error.
	setattrReply := func(status uint32, resultMask attrMask) uint32 {
		saw := w.AppendResarray_Setattr()
		saw.SetStatus(status)
		bmW := saw.StartAttrsset()
		if resultMask[0] != 0 || resultMask[1] != 0 {
			bmW.AppendData(resultMask[0])
			bmW.AppendData(resultMask[1])
		}
		buf := bmW.Finish()
		saw.Resume(buf)
		w.Resume(saw.Finish())
		return status
	}

	if !st.currentIDSet {
		return setattrReply(NFS4ERR_NOFILEHANDLE, attrMask{})
	}

	fa := args.ObjAttributes()
	mask := parseBitmap(fa.Attrmask())
	attrData := fa.AttrVals().Data()

	// Supported writable attrs.
	const supportedSet0 = 1 << FATTR4_SIZE
	supportedSet1 := uint32((1 << (FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (FATTR4_TIME_MODIFY_SET - 32)))

	// Validate fixed-width MODE data before reporting that MODE itself is not
	// supported. RFC 7530 requires malformed attribute XDR to take precedence.
	const modeMask = 1 << (FATTR4_MODE - 32)
	if mask[0] == 0 && mask[1] == modeMask && mask[2] == 0 && len(attrData) != 4 {
		return setattrReply(NFS4ERR_BADXDR, attrMask{})
	}
	// EXCLUSIVE4 clients send mode in a follow-up SETATTR. Modes are synthetic.
	if writer := s.clientWriter(st.clientID, st.currentID); s.directStaging(writer) != nil ||
		s.stagingStore.Failed(writer) {
		supportedSet1 |= modeMask
	}

	if mask[0]&^writableAttrs0 != 0 || mask[1]&^writableAttrs1 != 0 || mask[2] != 0 {
		return setattrReply(NFS4ERR_INVAL, attrMask{})
	}
	if mask[0]&^uint32(supportedSet0) != 0 ||
		mask[1]&^uint32(supportedSet1) != 0 {
		return setattrReply(NFS4ERR_ATTRNOTSUPP, attrMask{})
	}

	var resultMask attrMask
	var newSize *uint64
	attrOff := 0
	if mask[0]&(1<<FATTR4_SIZE) != 0 {
		if attrOff+8 > len(attrData) {
			return setattrReply(NFS4ERR_BADXDR, attrMask{})
		}
		size := binary.BigEndian.Uint64(attrData[attrOff : attrOff+8])
		attrOff += 8
		if size > 1<<63-1 {
			return setattrReply(NFS4ERR_FBIG, attrMask{})
		}
		newSize = &size
	}
	if mask[1]&modeMask != 0 {
		if attrOff+4 > len(attrData) {
			return setattrReply(NFS4ERR_BADXDR, attrMask{})
		}
		attrOff += 4
		resultMask[1] |= modeMask
	}

	// parseTimeSet reads a SET_TO_CLIENT_TIME4 or SET_TO_SERVER_TIME4
	// value from attrData at the current offset.
	parseTimeSet := func() (*time.Time, uint32) {
		if attrOff+4 > len(attrData) {
			return nil, NFS4ERR_BADXDR
		}
		how := binary.BigEndian.Uint32(attrData[attrOff : attrOff+4])
		attrOff += 4
		if how == SET_TO_CLIENT_TIME4 {
			if attrOff+12 > len(attrData) {
				return nil, NFS4ERR_BADXDR
			}
			sec := int64(binary.BigEndian.Uint64(attrData[attrOff : attrOff+8]))
			nsec := binary.BigEndian.Uint32(attrData[attrOff+8 : attrOff+12])
			attrOff += 12
			if nsec >= 1_000_000_000 {
				return nil, NFS4ERR_INVAL
			}
			t := time.Unix(sec, int64(nsec))
			return &t, NFS4_OK
		}
		if how == SET_TO_SERVER_TIME4 {
			t := time.Now()
			return &t, NFS4_OK
		}
		return nil, NFS4ERR_INVAL
	}

	var setAtime, setMtime *time.Time
	if mask[1]&(1<<(FATTR4_TIME_ACCESS_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		setAtime = t
	}
	if mask[1]&(1<<(FATTR4_TIME_MODIFY_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		setMtime = t
	}

	if attrOff != len(attrData) {
		return setattrReply(NFS4ERR_BADXDR, attrMask{})
	}

	var sf StagingFile
	var state openState
	dataID := st.currentID
	stateid, ok := st.resolveStateID(args.Stateid())
	if !ok {
		return setattrReply(NFS4ERR_BAD_STATEID, attrMask{})
	}
	if !isSpecialStateID(stateid) {
		var status uint32
		state, status = s.lookupDurableOpen(st.clientID, stateid, st.currentID)
		switch {
		case status == NFS4_OK:
			dataID = state.dataID
			sf = s.stagingOwnedBy(dataID, state)
		case newSize != nil || s.foreignStateID(st.clientID, extractStateID(stateid)):
			// Another client's stateid is rejected before any effect, times
			// included (canonical r3 §10).
			return setattrReply(status, attrMask{})
		default:
			// Times need no open; apply them as this client sees the file.
			dataID = s.clientWriter(st.clientID, st.currentID)
		}
	} else {
		dataID = s.clientWriter(st.clientID, st.currentID)
		sf = s.directStaging(dataID)
	}
	if s.stagingStore.Failed(dataID) || s.failedWriter(st.clientID, st.currentID) {
		return setattrReply(NFS4ERR_IO, attrMask{})
	}
	if newSize != nil {
		if status := requireRegularFile(st.currentID); status != NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		if !isSpecialStateID(stateid) && !state.writes() {
			return setattrReply(NFS4ERR_OPENMODE, attrMask{})
		}
		if sf == nil {
			return setattrReply(NFS4ERR_OPENMODE, attrMask{})
		}
		if err := s.setStagingSize(sf, *newSize); err != nil {
			st.effectUnknown = true
			return setattrReply(NFS4ERR_IO, attrMask{})
		}
		resultMask[0] |= 1 << FATTR4_SIZE
	}
	if setAtime != nil || setMtime != nil {
		if sf != nil {
			if err := sf.SetTime(setMtime, setAtime); err != nil {
				st.effectUnknown = true
				return setattrReply(NFS4ERR_IO, attrMask{})
			}
			if err := sf.Sync(); err != nil {
				st.effectUnknown = true
				return setattrReply(NFS4ERR_IO, attrMask{})
			}
		} else if err := s.fs.SetTime(
			s.clientView(st.clientID, st.currentID), setMtime, setAtime,
		); err != nil {
			st.observeMutation(s.fs, OP_SETATTR, err)
			return setattrReply(NFS4ERR_IO, attrMask{})
		}
		if setAtime != nil {
			resultMask[1] |= 1 << (FATTR4_TIME_ACCESS_SET - 32)
		}
		if setMtime != nil {
			resultMask[1] |= 1 << (FATTR4_TIME_MODIFY_SET - 32)
		}
	}

	if s.stagingStore.Failed(dataID) {
		return setattrReply(NFS4ERR_IO, attrMask{})
	}

	return setattrReply(NFS4_OK, resultMask)
}

func (s *Server) opVerify(args VERIFY4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Verify()
		return finishStatusResponse(&r, NFS4ERR_NOFILEHANDLE)
	}
	same, status := s.verifyAttrs(st.currentID, s.clientView(st.clientID, st.currentID), args.ObjAttributes())
	if status != NFS4_OK {
		r := w.AppendResarray_Verify()
		return finishStatusResponse(&r, status)
	}
	r := w.AppendResarray_Verify()
	if !same {
		// VERIFY: fail if attributes differ.
		return finishStatusResponse(&r, NFS4ERR_NOT_SAME)
	}
	return finishStatusResponse(&r, NFS4_OK)
}

func (s *Server) opWrite(args WRITE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_NOFILEHANDLE)
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	var sf StagingFile
	dataID := st.currentID
	stateid, ok := st.resolveStateID(args.Stateid())
	if !ok {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_BAD_STATEID)
	}
	if !isSpecialStateID(stateid) {
		state, status := s.lookupDurableOpen(
			st.clientID, stateid, st.currentID)
		if status != NFS4_OK {
			ew := w.AppendResarray_Write()
			return finishDefaultResponse(w, ew.Finish(), status)
		}
		if !state.writes() {
			ew := w.AppendResarray_Write()
			return finishDefaultResponse(w, ew.Finish(), NFS4ERR_OPENMODE)
		}
		sf = s.stagingOwnedBy(state.dataID, state)
		dataID = state.dataID
	} else {
		dataID = s.clientWriter(st.clientID, st.currentID)
		sf = s.directStaging(dataID)
	}

	if s.stagingStore.Failed(dataID) {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_IO)
	}

	// Find the staging buffer for this file.
	if sf == nil {
		// No staging buffer — not opened for write.
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_OPENMODE)
	}

	offset := args.Offset()
	data := args.Data()
	stable := args.Stable()
	if stable > fileSync4 {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_INVAL)
	}

	if err := sf.Write(offset, data); err != nil {
		st.effectUnknown = true
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_IO)
	}
	if sf.Dirty() {
		s.startHydration(sf)
	}
	committed := uint32(unstable4)
	if stable != unstable4 {
		if err := sf.Sync(); err != nil {
			st.effectUnknown = true
			ew := w.AppendResarray_Write()
			return finishDefaultResponse(w, ew.Finish(), NFS4ERR_IO)
		}
		committed = stable
	}

	if s.stagingStore.Failed(dataID) {
		ew := w.AppendResarray_Write()
		return finishDefaultResponse(w, ew.Finish(), NFS4ERR_IO)
	}

	ew := w.AppendResarray_Write()
	okW := ew.SetValue_Nfs4Ok()
	okW.SetCount(uint32(len(data)))
	okW.SetCommitted(committed)
	verf := okW.Writeverf()
	for i := 0; i < 8; i++ {
		verf.SetData(i, s.writeVerifier[i])
	}
	w.Resume(ew.Finish())
	return NFS4_OK
}

// verifyAttrs compares the supplied fattr4 against the current file's attributes.
// Returns (same bool, status uint32). If status != NFS4_OK, comparison failed.
// verifyAttrs compares supplied against the attributes of handle id as
// seen through dataID.
func (s *Server) verifyAttrs(id, dataID InodeID, supplied Fattr4) (bool, uint32) {
	ni, err := s.stat(dataID)
	if err != nil {
		return false, s.errToNFS(err)
	}

	mask := parseBitmap(supplied.Attrmask())
	if status := validateVerifyMask(mask); status != NFS4_OK {
		return false, status
	}

	// Encode what we would return for these attributes.
	expected := encodeAttrs(mask, id, ni)

	// Get the supplied attribute values.
	suppliedData := supplied.AttrVals().Data()

	return bytes.Equal(expected, suppliedData), NFS4_OK
}

// extractStateID reads the 12-byte "other" field from a Stateid4 into a StateID.
func extractStateID(s Stateid4) StateID {
	var sid StateID
	for i := 0; i < 12; i++ {
		sid[i] = s.Other(i)
	}
	return sid
}

// writeStateID writes the generation and opaque ID into a Stateid4.
func writeStateID(s Stateid4, generation uint32, sid StateID) {
	s.SetSeqid(generation)
	for i := 0; i < 12; i++ {
		s.SetOther(i, sid[i])
	}
}

func getData64(v Verifier4) (data [8]byte) {
	for i := range data {
		data[i] = v.Data(i)
	}
	return data
}

func setData64(v Verifier4, data [8]byte) {
	for i := range data {
		v.SetData(i, data[i])
	}
}

func setChangeInfo(info ChangeInfo4, atomic uint32, before, after uint64) {
	info.SetAtomic(atomic)
	info.SetBefore(before)
	info.SetAfter(after)
}

// errToNFS converts a Go error to an NFS status code.
func (s *Server) errToNFS(err error) uint32 {
	if err == nil {
		return NFS4_OK
	}
	err = ternToOSError(err)
	switch {
	case errors.Is(err, syscall.ENOTDIR):
		return NFS4ERR_NOTDIR
	case errors.Is(err, syscall.EISDIR):
		return NFS4ERR_ISDIR
	case errors.Is(err, syscall.ENOTEMPTY):
		return NFS4ERR_NOTEMPTY
	}
	if e, ok := err.(nfsError); ok {
		return uint32(e)
	}
	if os.IsNotExist(err) {
		return NFS4ERR_NOENT
	}
	if os.IsPermission(err) {
		return NFS4ERR_ACCESS
	}
	if os.IsExist(err) {
		return NFS4ERR_EXIST
	}
	s.log.Warn("VFS error", "err", err)
	return NFS4ERR_IO
}

// Time helper.
func timeToNFS(t time.Time) (int64, uint32) {
	return t.Unix(), uint32(t.Nanosecond())
}
