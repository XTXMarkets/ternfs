// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"os"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

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
	stateid nfs4.Stateid4,
	fileID vfs.InodeID,
) (openState, uint32) {
	state, status := s.opens.lookup(
		clientID,
		extractStateID(stateid),
		stateid.Seqid(),
		fileID,
	)
	if status == nfs4.NFS4_OK {
		fileID = state.dataID
	} else if dataID, ok := s.recoveredDataID(clientID, extractStateID(stateid), fileID); ok {
		fileID = dataID
	} else if status == nfs4.NFS4ERR_BAD_STATEID {
		return openState{}, status
	}
	// A failed writer cannot recover from its old sidecar. Preserve ordinary
	// stateid errors, then reject it locally without expiring healthy siblings.
	if s.stagingStore.Failed(fileID) {
		if status != nfs4.NFS4_OK {
			return openState{}, status
		}
		return openState{}, nfs4.NFS4ERR_IO
	}
	if status != nfs4.NFS4_OK {
		id := extractStateID(stateid)
		meta, hasMeta := s.stagingStore.GetMeta(fileID)
		if !hasMeta || meta.NFSStateID != id || meta.ClientID != clientID || !s.opens.canRecover(id) {
			return openState{}, status
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, id, func() {
				s.retireStaging(fileID, meta)
			}); status != nfs4.NFS4_OK {
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
		}, nfs4.NFS4_OK
	}
	// CLOSE can remove a failed writer's marker after the local lookup. Do
	// not let that race expire the client's unrelated writers.
	if status := s.expireInactiveOpen(state, func() bool { return s.stagingStore.Failed(fileID) }); status != nfs4.NFS4_OK {
		return openState{}, status
	}
	return state, nfs4.NFS4_OK
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
	if status != nfs4.NFS4ERR_EXPIRED {
		if status != nfs4.NFS4_OK && failed != nil && failed() {
			return nfs4.NFS4ERR_IO
		}
		return status
	}
	end, ok := s.beginDestroy(state.owner.clientID)
	if !ok {
		return nfs4.NFS4ERR_DELAY // Not while confirming.
	}
	defer end()
	if status = s.requireActiveOpen(state); status != nfs4.NFS4_OK && failed != nil && failed() {
		return nfs4.NFS4ERR_IO
	}
	if status == nfs4.NFS4ERR_EXPIRED && s.clients.MarkerGone(state.owner.clientID, state.id) {
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
		return nfs4.NFS4_OK
	}
	end, ok := s.beginDestroy(clientID)
	if !ok {
		return nfs4.NFS4ERR_DELAY // Not while confirming.
	}
	defer end()
	if active, err = s.clients.HasOpen(clientID, id); err != nil {
		return clientStoreErrToNFS(err)
	}
	if active {
		return nfs4.NFS4_OK
	}
	retire()
	return nfs4.NFS4ERR_EXPIRED
}

// recoveredDataID finds the staging of clientID's write open issued by an
// earlier nfsd process. Its client may present the staging inode as the
// handle (as before this process), or the base inode the staging replaces.
// Another client's staging is not found.
func (s *Server) recoveredDataID(clientID uint64, id StateID, fileID vfs.InodeID) (vfs.InodeID, bool) {
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
		return nfs4.NFS4ERR_EXPIRED
	}
	return nfs4.NFS4_OK
}

// expireClientState drops local state without revoking a confirmed clientid.
func (s *Server) expireClientState(clientID uint64) {
	for _, state := range s.opens.clientStates(clientID) {
		if err := s.clients.RemoveOpen(clientID, state.id); err != nil &&
			nfs4.ErrorCode(err) != nfs4.NFS4ERR_STALE_CLIENTID {
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

func (s *Server) opClose(args nfs4.CLOSE4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		return writeCloseError(w, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	state, status := s.releaseWrite(args.OpenStateid(), st, true, nil)
	if status != nfs4.NFS4_OK {
		return writeCloseError(w, status)
	}
	st.setCurrentStateID(state.id, state.generation)
	return writeCloseResponse(w, state)
}

func (s *Server) opOpenDowngrade(args nfs4.OPENDOWNGRADE4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_OpenDowngrade()
	fail := func(status uint32) uint32 {
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if !st.currentIDSet {
		return fail(nfs4.NFS4ERR_NOFILEHANDLE)
	}
	// The target must be a valid share value, a subset of the open's
	// grants, and the union of the values recorded inside it (canonical r3
	// §11). Deny is validated, not enforced. Narrowing BOTH to READ ends the
	// open's writer as CLOSE would; narrowing BOTH to WRITE drops READ and
	// keeps the writer.
	g := grant{access: args.ShareAccess() &^ nfs4.OPEN4_SHARE_ACCESS_WANT_DELEG_MASK, deny: args.ShareDeny()}
	if g.access == 0 || g.access > nfs4.OPEN4_SHARE_ACCESS_BOTH || g.deny > nfs4.OPEN4_SHARE_DENY_BOTH {
		return fail(nfs4.NFS4ERR_INVAL)
	}
	stateid, resolved := st.resolveStateID(args.OpenStateid())
	if !resolved {
		return fail(nfs4.NFS4ERR_BAD_STATEID)
	}
	state, status := s.lookupDurableOpen(st.clientID, stateid, st.currentID)
	if status != nfs4.NFS4_OK {
		return fail(status)
	}
	recovered := !s.opens.live(state.id)
	if status := state.downgradeStatus(g, recovered); status != nfs4.NFS4_OK {
		return fail(status)
	}
	switch {
	case state.writes() && g.access == nfs4.OPEN4_SHARE_ACCESS_READ:
		state, status = s.releaseWrite(args.OpenStateid(), st, false, &g)
	case recovered:
		state, status = s.adoptRecovered(st, stateid, g)
	default:
		state, status = s.regrant(st, stateid, g)
	}
	if status != nfs4.NFS4_OK {
		return fail(status)
	}
	ok := ew.SetValue_Nfs4Ok()
	writeStateID(ok.OpenStateid(), state.generation, state.id)
	w.Resume(ew.Finish())
	st.setCurrentStateID(state.id, state.generation)
	return nfs4.NFS4_OK
}

// releaseWrite ends an open's writer: the last writer of a staging file
// publishes it. With closing set the open is removed (CLOSE); otherwise it
// continues as a read open of the published version (OPEN_DOWNGRADE).
// to is OPEN_DOWNGRADE's target, checked again under the file lock.
func (s *Server) releaseWrite(openStateid nfs4.Stateid4, st *compoundState, closing bool, to *grant) (openState, uint32) {
	stateid, ok := st.resolveStateID(openStateid)
	if !ok {
		return openState{}, nfs4.NFS4ERR_BAD_STATEID
	}
	sid := extractStateID(stateid)
	generation := stateid.Seqid()
	state, status := s.opens.lookup(st.clientID, sid, generation, st.currentID)
	recovered := false
	if status != nfs4.NFS4_OK {
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
		if status != nfs4.NFS4_OK {
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
			if status := state.downgradeStatus(*to, false); status != nfs4.NFS4_OK {
				return openState{}, status
			}
		}
	}
	fail := func(status uint32) (openState, uint32) {
		return openState{}, status
	}
	if !recovered {
		if status := s.expireInactiveOpen(state, nil); status != nfs4.NFS4_OK {
			return fail(status)
		}
	}

	if recovered {
		if !hasMeta {
			return fail(nfs4.NFS4ERR_EXPIRED)
		}
		if !closing && !s.opens.adoptable(recoveredOwner(meta), st.currentID, sid) {
			// The owner already has a live open of the file: the downgrade
			// could not continue as a read open, so it waits, before any
			// publication. Both opens are kept.
			return fail(nfs4.NFS4ERR_DELAY)
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, sid, func() {
				s.retireStagingLocked(dataID, meta)
			}); status != nfs4.NFS4_OK {
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
	published := vfs.InodeID(0)
	expiredClose := false
	if !closing && !state.writes() && !recovered {
		// Already read-only.
		state, _ = s.opens.downgrade(sid, state.dataID, *to)
		return state, nfs4.NFS4_OK
	}
	if hasMeta && !recovered {
		if next, ok := s.opens.otherWriter(sid, dataID); ok {
			// Another open of this client still writes the shared staging.
			// The final write CLOSE publishes it; hand the sidecar's
			// ownership to that open so recovery and expiry follow it.
			if meta.NFSStateID == sid {
				if err := s.stagingStore.Rebind(dataID, next.owner.clientID, next.id); err != nil {
					return fail(nfs4.NFS4ERR_IO)
				}
			}
			hasMeta = false
			sf = nil
			state.access &^= nfs4.OPEN4_SHARE_ACCESS_WRITE
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
				st.observeMutation(s.fs, nfs4.OP_CLOSE, err)
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
						"status", nfs4.Nfsstat4Name(status))
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
				st.observeMutation(s.fs, nfs4.OP_CLOSE, linkErr)
				// The write phase is not resumable after a partial LinkFile.
				s.failForReaders(dataID, sid)
				s.discardStaging(dataID)
				status := s.errToNFS(linkErr)
				s.log.Error("close: link file", "err", linkErr, "status", nfs4.Nfsstat4Name(status))
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
			return state, nfs4.NFS4_OK
		}
		// A recovered writer downgraded to READ continues as a live read
		// open under its stateid, keeping its marker. Adoption was checked
		// before publication, under the same file lock.
		if adopted, ok := s.opens.adopt(recoveredOwner(meta), st.currentID, readID, *to, sid, nextStateidGeneration(generation)); ok {
			return adopted, nfs4.NFS4_OK
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
		return fail(nfs4.NFS4ERR_EXPIRED)
	}
	return state, nfs4.NFS4_OK
}

// failForReaders marks a staging failed, when opens other than the closing
// one read it, before a failed CLOSE discards it: its readers then get IO,
// never the old bytes or NOENT (canonical r3 §11).
func (s *Server) failForReaders(dataID vfs.InodeID, closing StateID) {
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
func (s *Server) readSource(clientID uint64, fileID vfs.InodeID) vfs.InodeID {
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
func (s *Server) adoptRecovered(st *compoundState, stateid nfs4.Stateid4, g grant) (openState, uint32) {
	sid := extractStateID(stateid)
	dataID, ok := s.recoveredDataID(st.clientID, sid, st.currentID)
	if !ok {
		return openState{}, nfs4.NFS4ERR_BAD_STATEID
	}
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	defer unlockTarget()
	unlockOpen := s.opens.lockFile(st.currentID)
	defer unlockOpen()
	if !hasMeta || meta.NFSStateID != sid || meta.ClientID != st.clientID {
		return openState{}, nfs4.NFS4ERR_BAD_STATEID
	}
	if s.opens.live(sid) {
		// Adopted meanwhile: the stateid is now ordinary live state.
		return openState{}, nfs4.NFS4ERR_DELAY
	}
	owner := recoveredOwner(meta)
	state, ok := s.opens.adopt(owner, st.currentID, dataID, g, sid, nextStateidGeneration(stateid.Seqid()))
	if !ok {
		return openState{}, nfs4.NFS4ERR_DELAY
	}
	s.views.addWriter(viewKey{st.clientID, st.currentID}, dataID)
	return state, nfs4.NFS4_OK
}

// recoveredAccess is a recovered open's access: its writer's original
// grant is unknown, so a writer may keep or drop READ.
func recoveredAccess(meta StagingMeta) uint32 {
	if meta.ReadOnly {
		return nfs4.OPEN4_SHARE_ACCESS_READ
	}
	return nfs4.OPEN4_SHARE_ACCESS_BOTH
}

// regrant applies an OPEN_DOWNGRADE that keeps the open's writer, or
// narrows a read open, under the file lock: it checks the target again
// against the current grants and advances the generation.
func (s *Server) regrant(st *compoundState, stateid nfs4.Stateid4, g grant) (openState, uint32) {
	unlock := s.opens.lockFile(st.currentID)
	defer unlock()
	sid := extractStateID(stateid)
	current, status := s.opens.lookup(st.clientID, sid, stateid.Seqid(), st.currentID)
	if status != nfs4.NFS4_OK {
		return openState{}, status
	}
	if status := current.downgradeStatus(g, false); status != nfs4.NFS4_OK {
		return openState{}, status
	}
	if current.writes() && g.access == nfs4.OPEN4_SHARE_ACCESS_READ {
		// An upgrade raced in; the caller's path no longer applies.
		return openState{}, nfs4.NFS4ERR_DELAY
	}
	state, ok := s.opens.downgrade(sid, current.dataID, g)
	if !ok {
		return openState{}, nfs4.NFS4ERR_BAD_STATEID
	}
	return state, nfs4.NFS4_OK
}

func writeCloseError(w *nfs4.COMPOUND4resWriter, status uint32) uint32 {
	ew := w.AppendResarray_Close()
	ew.SetValue_Default(status)
	w.Resume(ew.Finish())
	return status
}

func writeCloseResponse(w *nfs4.COMPOUND4resWriter, state openState) uint32 {
	ew := w.AppendResarray_Close()
	stid := ew.SetValue_Nfs4Ok()
	writeStateID(stid, state.generation, state.id)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opOpen(args nfs4.OPEN4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	return s.openRetry(args, st, w, 0)
}

func (s *Server) openRetry(args nfs4.OPEN4args, st *compoundState, w *nfs4.COMPOUND4resWriter, retries int) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Open()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
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

	access := args.ShareAccess() & nfs4.OPEN4_SHARE_ACCESS_BOTH
	if s.stagingStore.ReadOnly() && access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 {
		return fail(nfs4.NFS4ERR_ROFS)
	}

	// targetID is the inode the open's I/O uses; publishedID is the inode
	// the name resolves to, which is the filehandle OPEN returns.
	var targetID, publishedID vfs.InodeID
	var nfsSID StateID
	joined := false
	created := false
	staged := false
	reusedStaging := false
	var openSize *uint64
	var exclusiveVerifier *[8]byte
	var recoveredStagingID vfs.InodeID
	var recoveredStagingMeta StagingMeta
	var hasRecoveredStaging bool
	var placeholderID, placeholderDir vfs.InodeID
	var placeholderCookie vfs.Cookie
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
	var openDir vfs.InodeID
	var openName string
	// A writable CLAIM_FH this nfsd cannot name may recover a staged writer
	// (canonical r3 §6): the preselected staging, which the pathname lock
	// must confirm.
	var recovering bool
	var recoverID vfs.InodeID
	var recoverMeta StagingMeta
	switch claimType {
	case nfs4.CLAIM_NULL, nfs4.CLAIM_FH:
		var dirID vfs.InodeID
		var fileName string
		if claimType == nfs4.CLAIM_FH {
			// Open the file the client already holds. A read open needs
			// no name. A write publishes at the file's name, which TernFS
			// cannot derive from the inode.
			fileID := st.currentID
			if status := requireRegularFile(fileID); status != nfs4.NFS4_OK {
				if fileID.Type() == vfs.InodeTypeSymlink {
					status = nfs4.NFS4ERR_SYMLINK
				} else if fileID.Type() == vfs.InodeTypeDir {
					status = nfs4.NFS4ERR_ISDIR
				}
				return fail(status)
			}
			if args.OpenhowType() == nfs4.OPEN4_CREATE {
				return fail(nfs4.NFS4ERR_INVAL)
			}
			if access&nfs4.OPEN4_SHARE_ACCESS_WRITE == 0 {
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
				if status != nfs4.NFS4_OK {
					return fail(status)
				}
				dirID, fileName = recoverMeta.DirID, recoverMeta.FileName
				recovering = true
			}
		} else {
			dirID = st.currentID
			if status := requireDirectory(dirID); status != nfs4.NFS4_OK {
				return fail(status)
			}
			var status uint32
			fileName, status = toTernFSName(claim.AsNull().Data())
			if status != nfs4.NFS4_OK {
				return fail(status)
			}
		}
		openDir, openName = dirID, fileName
		if access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 || args.OpenhowType() == nfs4.OPEN4_CREATE {
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
				access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0,
			)
			// Taking an expired incarnation's staging is destructive for it:
			// its hold lasts, with the pathname lock, until OPEN returns.
			defer releaseRecovery()
			if recovering && errors.Is(recoveryErr, nfs4.Error(nfs4.NFS4ERR_EXPIRED)) {
				// Another staging became selectable since preselection.
				return fail(nfs4.NFS4ERR_STALE)
			}
			if recoveryErr != nil {
				return fail(clientStoreErrToNFS(recoveryErr))
			}
			if recovering && !s.sameRecoveryCandidate(recoverID, recoverMeta,
				recoveredStagingID, recoveredStagingMeta, hasRecoveredStaging) {
				// The preselected staging was rebound, removed or failed, or
				// another one is selected now: adopt nothing.
				return fail(nfs4.NFS4ERR_STALE)
			}
		}

		// Check if this is a create.
		if args.OpenhowType() == nfs4.OPEN4_CREATE {
			createHow := args.Openhow().AsCreatehow4Entry()
			var createAttrs nfs4.Fattr4
			switch createHow.Disc() {
			case nfs4.UNCHECKED4:
				createAttrs = createHow.Value().AsUnchecked4()
			case nfs4.GUARDED4:
				createAttrs = createHow.Value().AsGuarded4()
			case nfs4.EXCLUSIVE4:
				exclusiveVerifier = new([8]byte)
				*exclusiveVerifier = verifierBytes(createHow.Value().AsVerifier4())
			case nfs4.EXCLUSIVE4_1:
				// SUPPATTR_EXCLCREAT is empty: nfsd sets none of the
				// attributes and reports an empty attrset, so the client
				// sets them with a later SETATTR.
				va := createHow.Value().AsCreatverfattr()
				exclusiveVerifier = new([8]byte)
				*exclusiveVerifier = verifierBytes(va.CvaVerf())
			}
			// An exclusive create carries a verifier, not attributes.
			if exclusiveVerifier == nil {
				if status := validateCreateAttrs(createAttrs); status != nfs4.NFS4_OK {
					return fail(status)
				}
				var status uint32
				openSize, status = createAttrSize(createAttrs)
				if status != nfs4.NFS4_OK {
					return fail(status)
				}
			}
			// Try lookup first.
			id, err := s.fs.Lookup(dirID, fileName)
			publishedID = id
			if err != nil {
				lookupStatus := s.errToNFS(err)
				if lookupStatus != nfs4.NFS4ERR_NOENT {
					return fail(lookupStatus)
				}
				if s.stagingStore.ReadOnly() {
					return fail(nfs4.NFS4ERR_ROFS)
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
					st.observeMutation(s.fs, nfs4.OP_OPEN, err)
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
			} else if createHow.Disc() == nfs4.GUARDED4 {
				return fail(nfs4.NFS4ERR_EXIST)
			} else if exclusiveVerifier != nil {
				stagingID, meta, found := s.exclusiveStagingTarget(
					dirID, fileName, id, ownerKey, *exclusiveVerifier,
				)
				if !found || meta.ReadOnly != (access&nfs4.OPEN4_SHARE_ACCESS_WRITE == 0) {
					return fail(nfs4.NFS4ERR_EXIST)
				}
				existing, local := s.opens.existingOpen(ownerKey, publishedID)
				if local && existing.id == meta.NFSStateID {
					nfsSID = existing.id
					reusedStaging, staged = true, true
				} else if s.opens.canRecover(meta.NFSStateID) {
					adoptRecoveredStaging()
				} else {
					return fail(nfs4.NFS4ERR_EXIST)
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
				if recovering && lookupStatus == nfs4.NFS4ERR_NOENT {
					// The name no longer holds the client's handle.
					return fail(nfs4.NFS4ERR_STALE)
				}
				if lookupStatus == nfs4.NFS4ERR_NOENT &&
					hasRecoveredStaging &&
					recoveredStagingMeta.BaseID == 0 {
					adoptRecoveredStaging()
					targetID = recoveredStagingID
					created = true
					placeholderID, placeholderCookie, err = s.fs.ConstructFile(dirID)
					if err != nil {
						st.observeMutation(s.fs, nfs4.OP_OPEN, err)
						return fail(s.errToNFS(err))
					}
					publishedID = placeholderID
					placeholderDir, placeholderName = dirID, fileName
				} else {
					if lookupStatus == nfs4.NFS4ERR_NOENT &&
						hasRecoveredStaging {
						s.discardStaging(recoveredStagingID)
					}
					return fail(lookupStatus)
				}
			} else if claimType == nfs4.CLAIM_FH && id != st.currentID {
				// Replaced since the name was checked.
				return fail(nfs4.NFS4ERR_STALE)
			} else {
				// A read sees this client's own writes to the file.
				targetID = s.clientView(clientID, id)
			}
		}

		if !staged && access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 {
			// Join this client's writer of the same file.
			if writer, ok := s.opens.writerOf(clientID, publishedID); ok {
				targetID = writer.dataID
				staged, joined = true, true
				if writer.owner == ownerKey {
					nfsSID = writer.id
				}
			}
		}
		if !staged && (created || access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0) {
			if status := requireRegularFile(targetID); status != nfs4.NFS4_OK {
				if targetID.Type() == vfs.InodeTypeSymlink {
					status = nfs4.NFS4ERR_SYMLINK
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
				var baseInfo vfs.NodeInfo
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
					access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0, baseID, baseInfo,
					exclusiveVerifier,
				)
				if status != nfs4.NFS4_OK {
					return fail(status)
				}
				staged = true
			}
		}
	case nfs4.CLAIM_PREVIOUS:
		// No grace period — there is no lock/open state to reclaim.
		return fail(nfs4.NFS4ERR_NO_GRACE)
	default:
		return fail(nfs4.NFS4ERR_NOTSUPP)
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
			return fail(nfs4.NFS4ERR_IO)
		}
	}

	if status := requireRegularFile(targetID); status != nfs4.NFS4_OK {
		if targetID.Type() == vfs.InodeTypeSymlink {
			status = nfs4.NFS4ERR_SYMLINK
		}
		return fail(status)
	}

	unlockOpen := s.opens.lockFile(publishedID)
	if !staged && !created && access&nfs4.OPEN4_SHARE_ACCESS_WRITE == 0 {
		// A writer's CLOSE may have ended the staging this read selected
		// while it waited for the file (canonical r3 §11).
		targetID = s.readSource(clientID, publishedID)
	}
	if !staged && claimType == nfs4.CLAIM_NULL && openDir != 0 && retries < 3 {
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
				return fail(nfs4.NFS4ERR_IO)
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
			return fail(nfs4.NFS4ERR_IO)
		}
		stagingRebound = true
		// Retired files release their descriptors; Rebind reopens them after
		// the new durable marker exists. Apply CREATE size only then.
		if openSize != nil {
			if err := applyOpenSize(); err != nil {
				_ = s.clients.RemoveOpen(clientID, markID)
				cleanupStagedOpen()
				return fail(nfs4.NFS4ERR_IO)
			}
		}
	}
	if placeholderID != 0 {
		if err := s.fs.LinkFile(
			placeholderID, placeholderCookie, placeholderDir, placeholderName, nil,
		); err != nil {
			st.observeMutation(s.fs, nfs4.OP_OPEN, err)
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
		return fail(nfs4.NFS4ERR_SERVERFAULT)
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
	dirID vfs.InodeID,
	fileName string,
	clientID uint64,
	openOwner string,
	write bool,
	baseID vfs.InodeID,
	baseInfo vfs.NodeInfo,
	exclusiveVerifier *[8]byte,
) (vfs.InodeID, StateID, uint32) {
	recoveryKey, err := s.clients.stagingRecoveryKey(clientID)
	if err != nil {
		return 0, StateID{}, clientStoreErrToNFS(err)
	}
	id, cookie, err := s.fs.ConstructFile(dirID)
	if err != nil {
		st.observeMutation(s.fs, nfs4.OP_OPEN, err)
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
		return 0, StateID{}, nfs4.NFS4ERR_IO
	}
	return id, stateID, nfs4.NFS4_OK
}

type openResponse struct {
	state   openState
	created bool
}

func writeOpenResponse(
	w *nfs4.COMPOUND4resWriter,
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
	cinfo.SetAtomic(nfs4.TRUE)
	if response.created {
		cinfo.SetBefore(now - 1)
	} else {
		cinfo.SetBefore(now)
	}
	cinfo.SetAfter(now)

	okW.SetRflags(nfs4.OPEN4_RESULT_LOCKTYPE_POSIX)

	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)

	okW.SetDelegation_None()

	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func writeOpenError(w *nfs4.COMPOUND4resWriter, status uint32) uint32 {
	ew := w.AppendResarray_Open()
	return finishDefaultResponse(w, ew.Finish(), status)
}
