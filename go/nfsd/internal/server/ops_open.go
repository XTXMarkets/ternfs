// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"os"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// lookupDurableOpen requires valid local protocol state and a live client.
// After an nfsd restart there is no local state; the staging sidecar is then
// the durable proof that this stateid is the confirmed write-open for this
// transient file.
// lookupDurableOpen validates a stateid that clientID presented with
// filehandle fileID. The returned state's dataID is the inode the
// operation's I/O uses. Another client's stateid, live, recovered or
// expired, gets BAD_STATEID before any effect (README.md#stateid-checks).
func (s *Server) lookupDurableOpen(
	clientID uint64,
	stateid nfs4.Stateid4,
	fileID vfs.InodeID,
) (openstate.State, uint32) {
	handleID := fileID
	state, status := s.opens.Lookup(
		clientID,
		extractStateID(stateid),
		stateid.Seqid(),
		fileID,
	)
	if status == nfs4.NFS4_OK {
		fileID = state.DataID
	} else if dataID, ok := s.recoveredDataID(clientID, extractStateID(stateid), fileID); ok {
		fileID = dataID
	} else if status == nfs4.NFS4ERR_BAD_STATEID {
		return openstate.State{}, status
	}
	// A failed writer cannot recover from its old sidecar. Preserve ordinary
	// stateid errors, then reject it locally without expiring healthy siblings.
	if s.stagingStore.Failed(fileID) {
		if status != nfs4.NFS4_OK {
			return openstate.State{}, status
		}
		return openstate.State{}, nfs4.NFS4ERR_IO
	}
	if status != nfs4.NFS4_OK {
		id := extractStateID(stateid)
		meta, hasMeta := s.stagingStore.GetMeta(fileID)
		if !hasMeta || meta.NFSStateID != id || meta.ClientID != clientID || !s.opens.CanRecover(id) {
			return openstate.State{}, status
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, func() {
				s.retireStaging(fileID, meta)
			}); status != nfs4.NFS4_OK {
				return openstate.State{}, status
			}
		}
		// The client goes on using the open it had: make it a live one here,
		// so COMMIT, reads and CLOSE find it as they find any other. When
		// adoption is refused (read-only staging, or an owner that already
		// has a live open of the file) the request runs on a state built
		// from the sidecar alone, and COMMIT finds such writers through
		// recoveredWritersOf. When another request adopted it meanwhile, the
		// live open decides, as for any request that finds one.
		if adopted, live, status := s.adoptRecoveredWriter(clientID, handleID, fileID, id, stateid.Seqid()); live {
			return adopted, status
		}
		return openstate.State{
			ID:         id,
			FileID:     fileID,
			DataID:     fileID,
			Owner:      openstate.OwnerKey{ClientID: meta.ClientID},
			Access:     recoveredAccess(meta),
			Generation: stateid.Seqid(),
		}, nfs4.NFS4_OK
	}
	if status := s.expireInactiveOpen(state, func() bool { return s.stagingStore.Failed(fileID) }); status != nfs4.NFS4_OK {
		return openstate.State{}, status
	}
	return state, nfs4.NFS4_OK
}

// expireInactiveOpen requires state's client to be live. When it is not, the
// operation gets EXPIRED, and the client's other state is untouched: local
// expiry is the lease gate's decision, which the sweeper drives
// (README.md#expiry-decision). failed reports a failed writer, which gives
// IO.
func (s *Server) expireInactiveOpen(state openstate.State, failed func() bool) uint32 {
	status := s.requireActiveOpen(state)
	if status != nfs4.NFS4_OK && failed != nil && failed() {
		return nfs4.NFS4ERR_IO
	}
	return status
}

// retireInactiveRecovered checks that a recovered writer's client is still
// live and, when it is not, retires its staging. The synced sidecar is the
// record that the open was not closed.
func (s *Server) retireInactiveRecovered(clientID uint64, retire func()) uint32 {
	if s.clientLive(clientID) {
		return nfs4.NFS4_OK
	}
	retire()
	return nfs4.NFS4ERR_EXPIRED
}

// recoveredDataID finds the staging of clientID's write open issued by an
// earlier nfsd process. Its client may present the staging inode as the
// handle (as before this process), or the base inode the staging replaces.
// A file created by OPEN has the empty placeholder published at its name as
// its base, which is the handle its client holds. Another client's staging
// is not found.
func (s *Server) recoveredDataID(clientID uint64, id nfs4.StateID, fileID vfs.InodeID) (vfs.InodeID, bool) {
	if !s.opens.CanRecover(id) {
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

func (s *Server) requireActiveOpen(state openstate.State) uint32 {
	if !s.clientLive(state.Owner.ClientID) {
		return nfs4.NFS4ERR_EXPIRED
	}
	return nfs4.NFS4_OK
}

// expireClientState drops local state without revoking a confirmed clientid.
func (s *Server) expireClientState(clientID uint64) {
	for _, state := range s.opens.ExpireClient(clientID) {
		meta, ok := s.stagingStore.GetMeta(state.DataID)
		if ok && meta.ClientID == clientID && meta.NFSStateID == state.ID {
			s.retireStaging(state.DataID, meta)
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
	st.setCurrentStateID(state.ID, state.Generation)
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
	// grants, and the union of the values recorded inside it
	// (../openstate/README.md#downgrades). Deny is validated, not enforced.
	// Narrowing BOTH to READ ends the
	// open's writer as CLOSE would; narrowing BOTH to WRITE drops READ and
	// keeps the writer.
	g := openstate.Grant{Access: args.ShareAccess() &^ nfs4.OPEN4_SHARE_ACCESS_WANT_DELEG_MASK, Deny: args.ShareDeny()}
	if g.Access == 0 || g.Access > nfs4.OPEN4_SHARE_ACCESS_BOTH || g.Deny > nfs4.OPEN4_SHARE_DENY_BOTH {
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
	recovered := !s.opens.Live(state.ID)
	if status := state.DowngradeStatus(g, recovered || state.Recovered); status != nfs4.NFS4_OK {
		return fail(status)
	}
	switch {
	case state.Writes() && g.Access == nfs4.OPEN4_SHARE_ACCESS_READ:
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
	writeStateID(ok.OpenStateid(), state.Generation, state.ID)
	w.Resume(ew.Finish())
	st.setCurrentStateID(state.ID, state.Generation)
	return nfs4.NFS4_OK
}

// releaseWrite ends an open's writer: the last writer of a staging file
// publishes it. With closing set the open is removed (CLOSE); otherwise it
// continues as a read open of the published version (OPEN_DOWNGRADE).
// to is OPEN_DOWNGRADE's target, checked again under the file lock.
func (s *Server) releaseWrite(openStateid nfs4.Stateid4, st *compoundState, closing bool, to *openstate.Grant) (openstate.State, uint32) {
	stateid, ok := st.resolveStateID(openStateid)
	if !ok {
		return openstate.State{}, nfs4.NFS4ERR_BAD_STATEID
	}
	sid := extractStateID(stateid)
	generation := stateid.Seqid()
	state, status := s.opens.Lookup(st.clientID, sid, generation, st.currentID)
	recovered := false
	if status != nfs4.NFS4_OK {
		// A write open issued by an earlier nfsd process is closed through
		// its staging sidecar.
		dataID, ok := s.recoveredDataID(st.clientID, sid, st.currentID)
		if !ok {
			return openstate.State{}, status
		}
		meta, _ := s.stagingStore.GetMeta(dataID)
		recovered = true
		state = openstate.State{
			ID:         sid,
			FileID:     st.currentID,
			DataID:     dataID,
			Owner:      openstate.OwnerKey{ClientID: meta.ClientID},
			Access:     recoveredAccess(meta),
			Generation: generation,
		}
	}
	// Lock order is pathname, then file. A writer's pathname comes from its
	// staging; an upgrade while we wait can give the open a writer, so
	// check again once both are held.
	dataID := state.DataID
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	releaseTarget := st.hold(unlockTarget)
	releaseOpen := st.hold(s.opens.LockFile(st.currentID))
	if recovered && s.opens.Live(sid) {
		// Adopted since the lookup above, by a request that took these
		// locks first. No adoption can start while they are held, so the
		// live open answers, as for any CLOSE or OPEN_DOWNGRADE of one.
		recovered = false
	}
	if !recovered {
		current, status := s.opens.Lookup(st.clientID, sid, generation, st.currentID)
		if status != nfs4.NFS4_OK {
			return openstate.State{}, status
		}
		if current.DataID != dataID {
			releaseOpen()
			releaseTarget()
			return s.releaseWrite(openStateid, st, closing, to)
		}
		state = current
		if to != nil {
			if status := state.DowngradeStatus(*to, state.Recovered); status != nfs4.NFS4_OK {
				return openstate.State{}, status
			}
		}
	}
	fail := func(status uint32) (openstate.State, uint32) {
		if closing && status == nfs4.NFS4ERR_DELAY {
			// libnfs gives the open up after DELAY: hand it no staging.
			s.opens.MarkClosing(sid)
		}
		return openstate.State{}, status
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
		if !closing && !s.opens.Adoptable(recoveredOwner(meta), st.currentID, sid) {
			// The owner already has a live open of the file: the downgrade
			// could not continue as a read open, so it waits, before any
			// publication. Both opens are kept.
			return fail(nfs4.NFS4ERR_DELAY)
		}
		if meta.ClientID != 0 {
			if status := s.retireInactiveRecovered(meta.ClientID, func() {
				s.retireStagingLocked(dataID, meta)
			}); status != nfs4.NFS4_OK {
				return fail(status)
			}
		}
	}
	// The sidecar's open owns the staging, a read-only creator's included,
	// and so does any writer of its client; a borrowing read open does not.
	ownsStaging := hasMeta && (recovered ||
		state.Owner.ClientID == meta.ClientID && (sid == meta.NFSStateID || state.Writes()))
	if !ownsStaging {
		hasMeta = false
		meta = staging.Meta{}
	}

	sf := s.stagingStore.Get(dataID)
	if sf == nil {
		hasMeta = false
	}
	// published is the version this open's writer leaves at the name.
	published := vfs.InodeID(0)
	// kept: the staging stays, as the data of a read open (endStaging).
	kept := false
	expiredClose := false
	if !closing && !state.Writes() && !recovered {
		// Already read-only.
		state, _ = s.opens.Downgrade(sid, state.DataID, *to)
		return state, nfs4.NFS4_OK
	}
	if hasMeta && !recovered {
		if next, ok := s.opens.OtherWriter(sid, dataID); ok {
			// Another open of this client still writes the shared staging.
			// The final write CLOSE publishes it; hand the sidecar's
			// ownership to that open so recovery and expiry follow it.
			s.log.Info("close: staging handed to another open of this client", "inode", dataID, "stateid", sid, "next", next.ID)
			if meta.NFSStateID == sid {
				if err := s.stagingStore.Bind(dataID, next.Owner.ClientID, next.ID, next.Owner.Owner, false); err != nil {
					return fail(nfs4.NFS4ERR_IO)
				}
			}
			hasMeta = false
			sf = nil
			state.Access &^= nfs4.OPEN4_SHARE_ACCESS_WRITE
		}
	}
	if hasMeta {
		owner := state.Owner
		if recovered {
			owner = recoveredOwner(meta)
		}
		var status uint32
		published, kept, status = s.endStaging(st, sid, owner, closing, dataID, meta, sf)
		if status != nfs4.NFS4_OK {
			if status != nfs4.NFS4ERR_DELAY {
				s.abandonFailedOpen(sid, recovered)
			}
			return fail(status)
		}
	} else {
		// Read CLOSE does not need the file to remain linked. A write CLOSE
		// without staging has lost the data it was meant to publish.
		if !recovered && state.Writes() {
			s.log.Error("close: write staging missing",
				"file_id", dataID, "stateid", sid)
			expiredClose = true
		}
	}

	if recovered {
		s.log.Info("recovered close", "inode", dataID, "name", meta.FileName, "stateid", sid, "published", published, "discarded", published == 0)
	}
	if published != 0 && state.Owner.ClientID != 0 {
		s.metrics.publishes.Add(1)
	}
	if !closing && !expiredClose {
		readID := published
		if kept {
			readID = dataID
		} else if readID == 0 {
			readID = s.clientView(state.Owner.ClientID, st.currentID)
		}
		if !recovered {
			state, _ = s.opens.Downgrade(sid, readID, *to)
			return state, nfs4.NFS4_OK
		}
		// A recovered writer downgraded to READ continues as a live read
		// open under its stateid, keeping its marker. Adoption was checked
		// before publication, under the same file lock.
		if adopted, ok := s.opens.Adopt(recoveredOwner(meta), st.currentID, readID, *to, sid, openstate.NextGeneration(generation)); ok {
			return adopted, nfs4.NFS4_OK
		}
		panic("recovered downgrade: adoption failed with the file lock held")
	}
	if recovered {
		state.Generation = openstate.NextGeneration(generation)
	} else if expiredClose {
		s.opens.Expire(sid)
	} else {
		state, _ = s.opens.Close(sid)
	}
	if expiredClose {
		return fail(nfs4.NFS4ERR_EXPIRED)
	}
	return state, nfs4.NFS4_OK
}

// endStaging publishes, discards or fails the staging of its last writer,
// open sid, for a CLOSE or OPEN_DOWNGRADE. Every step that can answer DELAY
// precedes ending the open or discarding staging. kept reports staging that
// stays as a read open's data; for an error other than DELAY the writer has ended.
func (s *Server) endStaging(
	st *compoundState,
	sid nfs4.StateID,
	owner openstate.OwnerKey,
	closing bool,
	dataID vfs.InodeID,
	meta staging.Meta,
	sf staging.File,
) (vfs.InodeID, bool, uint32) {
	version := sf.DataVersion()
	detached := meta.Unlinked
	publish := !detached && (meta.BaseID == 0 || sf.Dirty())
	timesOnly := publish && meta.BaseID != 0 && !sf.DataChanged()
	upload := publish && !timesOnly
	delay := func(err error) (vfs.InodeID, bool, uint32) {
		s.metrics.publishDelays.Add(1)
		s.log.Warn("close: publication delayed; the client retries",
			"inode", dataID, "name", meta.FileName, "err", err)
		return 0, false, nfs4.NFS4ERR_DELAY
	}
	var published vfs.InodeID
	fenced := false
	var endErr error
	switch {
	case timesOnly:
		// A timestamp-only CLOSE updates the current published version,
		// preserving any data published by another writer in the meantime.
		currentID, err := s.fs.Lookup(meta.DirID, meta.FileName)
		if err == nil {
			info := sf.Stat()
			err = s.fs.SetTime(currentID, &info.Mtime, &info.Atime)
		}
		if vfs.IsTransientBackendError(err) {
			return delay(err)
		}
		endErr = err
		if err == nil {
			published = currentID
			if owner.ClientID != 0 {
				s.publishedAs(owner.ClientID, st.currentID, currentID)
			}
		}
	case upload:
		id, dv, err := s.publishStaging(dataID, meta, sf)
		if err != nil && publicationDelayed(err) {
			// Linux ignores a CLOSE's error, so an error would lose the save.
			return delay(err)
		}
		if err != nil {
			var resolved resolvedError
			if errors.As(err, &resolved) {
				fenced, err = true, resolved.error
			}
			endErr = err
			break
		}
		info := sf.Stat()
		// LinkFile stamps the inode with publication time. Restore the
		// writer's times; the publication stands whatever this answers.
		if err := s.fs.SetTime(id, &info.Mtime, &info.Atime); err != nil {
			s.log.Warn("close: published file but could not restore timestamps", "inode", id, "err", err)
		}
		// Before the staging goes, so the client's view never falls back.
		if owner.ClientID != 0 {
			s.publishedAs(owner.ClientID, st.currentID, id)
		}
		version = dv
		published = id
	}
	uploaded := upload && endErr == nil
	// A staging that will not be published fences its candidate now.
	candidate := sf.Candidate()
	if !fenced && !uploaded && (detached || upload) && candidate.ID != 0 {
		if err := s.scrapTransient(dataID, candidate); err != nil {
			return delay(err)
		}
		fenced = true
	}

	if endErr == nil && !detached && !sf.Seal(version) {
		// Check the version and stop new writes under the staging lock.
		// A WRITE that already selected the file can otherwise succeed
		// after the final check and lose its bytes at Remove below.
		if uploaded {
			if err := sf.SetCandidate(sf.Candidate(), true); err != nil {
				s.log.Warn("close: save publication candidate", "inode", dataID, "err", err)
			}
		}
		return delay(errStagingChanged)
	}

	for _, other := range s.opens.ExpireWriters(dataID, sid) {
		s.log.Info("close: writer whose CLOSE was delayed expired", "inode", dataID, "stateid", other.ID)
	}
	switch {
	case upload && endErr != nil:
		s.failForReaders(dataID, sid)
	case detached:
		// Never published, and kept while an open reads it: bound ReadOnly
		// to that open, so a restart does not bring back a writer.
		reader, ok := openstate.State{ID: sid, Owner: owner}, !closing
		if closing {
			reader, ok = s.opens.ReaderOf(dataID, sid, owner.ClientID)
		}
		if ok {
			err := s.stagingStore.Bind(dataID, reader.Owner.ClientID, reader.ID, reader.Owner.Owner, true)
			if err == nil {
				return 0, true, nfs4.NFS4_OK
			}
			s.log.Warn("close: bind detached staging to its reader", "inode", dataID, "err", err)
			if !closing {
				// The downgraded open reads the failed staging: IO, not NOENT.
				if err := s.stagingStore.Quarantine(dataID); err != nil {
					s.log.Warn("close: fail detached staging", "inode", dataID, "err", err)
				}
				return 0, true, nfs4.NFS4_OK
			}
			s.failForReaders(dataID, sid)
		}
	case uploaded:
		s.opens.Retarget(dataID, published)
	default:
		// Clean, timestamp-only or ended: back to the published file.
		s.opens.Retarget(dataID, 0)
	}
	s.stagingStore.Remove(dataID)
	if !uploaded && !fenced {
		c := unpublished(dataID, meta, candidate)
		st.later(func() { _ = s.scrapTransient(dataID, c) })
	}
	if endErr != nil {
		status := s.errToNFS(endErr)
		s.log.Error("close: publication failed", "inode", dataID, "name", meta.FileName,
			"err", endErr, "status", nfs4.Nfsstat4Name(status))
		return 0, false, status
	}
	return published, false, nfs4.NFS4_OK
}

// failForReaders quarantines a staging, when opens other than the closing
// one read it, before a failed CLOSE would discard it: its files are kept,
// the entry stays failed, and the CLOSE's later discard leaves it alone. Its
// readers then get IO, never the old bytes or NOENT
// (../openstate/README.md#reader-visibility).
func (s *Server) failForReaders(dataID vfs.InodeID, closing nfs4.StateID) {
	if !s.opens.Readers(dataID, closing) {
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
func (s *Server) foreignStateID(clientID uint64, id nfs4.StateID) bool {
	if owner, known := s.opens.OwnerOf(id); known {
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
func recoveredOwner(meta staging.Meta) openstate.OwnerKey {
	owner := openstate.OwnerKey{ClientID: meta.ClientID}
	if meta.OwnerKnown {
		owner.Owner = meta.OpenOwner
	}
	return owner
}

// adoptRecovered completes an OPEN_DOWNGRADE of a recovered writer that
// keeps WRITE: the narrowed grant and the next generation are installed as
// a live open under the same stateid, so later operations see them rather
// than a writer rebuilt from its sidecar. If the owner already has a live
// open of the file, it gets DELAY and nothing changes. Lock order is
// pathname, then file.
func (s *Server) adoptRecovered(st *compoundState, stateid nfs4.Stateid4, g openstate.Grant) (openstate.State, uint32) {
	sid := extractStateID(stateid)
	dataID, ok := s.recoveredDataID(st.clientID, sid, st.currentID)
	if !ok {
		return openstate.State{}, nfs4.NFS4ERR_BAD_STATEID
	}
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	st.hold(unlockTarget)
	st.hold(s.opens.LockFile(st.currentID))
	if !hasMeta || meta.NFSStateID != sid || meta.ClientID != st.clientID {
		return openstate.State{}, nfs4.NFS4ERR_BAD_STATEID
	}
	if s.opens.Live(sid) {
		// Adopted meanwhile: the stateid is now ordinary live state.
		return openstate.State{}, nfs4.NFS4ERR_DELAY
	}
	owner := recoveredOwner(meta)
	state, ok := s.opens.Adopt(owner, st.currentID, dataID, g, sid, openstate.NextGeneration(stateid.Seqid()))
	if !ok {
		return openstate.State{}, nfs4.NFS4ERR_DELAY
	}
	s.views.AddWriter(openstate.ViewKey{ClientID: st.clientID, FileID: st.currentID}, dataID)
	return state, nfs4.NFS4_OK
}

// adoptRecoveredWriter installs a recovered writer, presented by its old
// stateid on the handle the client holds, as a live open of this process at
// the generation the client is using (README.md#writer-recovery-mechanics).
// The caller holds no staging or file lock.
//
// live reports that a live open answers for the stateid: the one adopted
// here, or one another request adopted after the caller's own lookup, whose
// status, OLD_STATEID or BAD_STATEID for a generation it does not have, is
// the request's. Otherwise adoption was refused, and the caller may run the
// request on the sidecar.
func (s *Server) adoptRecoveredWriter(clientID uint64, handleID, dataID vfs.InodeID, id nfs4.StateID, generation uint32) (state openstate.State, live bool, status uint32) {
	meta, hasMeta, unlockTarget := s.lockStagingTarget(dataID)
	defer unlockTarget()
	unlockOpen := s.opens.LockFile(handleID)
	defer unlockOpen()
	if s.opens.Live(id) {
		// Adopted meanwhile by another request of this client. Its live
		// open answers, whatever the staging says now.
		state, status = s.opens.Lookup(clientID, id, generation, handleID)
		return state, true, status
	}
	if !hasMeta || meta.NFSStateID != id || meta.ClientID != clientID || meta.ReadOnly {
		return openstate.State{}, false, nfs4.NFS4_OK
	}
	owner := recoveredOwner(meta)
	state, ok := s.opens.Adopt(owner, handleID, dataID, openstate.Grant{Access: recoveredAccess(meta)}, id, generation)
	if !ok {
		return openstate.State{}, false, nfs4.NFS4_OK
	}
	s.views.AddWriter(openstate.ViewKey{ClientID: clientID, FileID: handleID}, dataID)
	if !meta.Unlinked {
		s.names.remember(handleID, meta.DirID, meta.FileName)
	}
	s.log.Info("recovered writer adopted", "inode", dataID, "name", meta.FileName, "stateid", id, "handle", handleID)
	return state, true, nfs4.NFS4_OK
}

// recoveredWritersOf lists the client's staged writers for a handle that no
// request of this process has adopted yet: those an earlier process issued
// against the handle as base or placeholder, or with the handle as their
// own inode. COMMIT carries no stateid, so it finds them here.
func (s *Server) recoveredWritersOf(clientID uint64, handleID vfs.InodeID) []vfs.InodeID {
	var ids []vfs.InodeID
	for _, id := range s.stagingStore.RecoveredWriters(handleID) {
		meta, ok := s.stagingStore.GetMeta(id)
		if !ok || meta.ClientID != clientID || meta.ReadOnly || !s.opens.CanRecover(meta.NFSStateID) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// abandonFailedOpen ends the open state of a CLOSE whose publication
// failed and whose staging was discarded. The Linux client treats a failed
// close() as closed and never retries it, so an open kept here would last
// until the client's lease ended, which for a live client is never; the
// lease-expiry run on QA left one such open with no staging behind it. A
// retried CLOSE gets EXPIRED, as for a writer whose staging was lost.
func (s *Server) abandonFailedOpen(sid nfs4.StateID, recovered bool) {
	if !recovered {
		s.opens.Expire(sid)
	}
}

// errStagingChanged: a WRITE landed during the publication.
var errStagingChanged = errors.New("staging changed during its publication")

var errCandidateCheckpoint = errors.New("publication candidate not checkpointed")

// publicationDelayed reports a publication error a retried CLOSE gets
// past.
func publicationDelayed(err error) bool {
	return vfs.IsTransientBackendError(err) || errors.Is(err, nfs4.Error(nfs4.NFS4ERR_LOCKED)) ||
		errors.Is(err, errStagingChanged) || errors.Is(err, errCandidateCheckpoint)
}

// publishStaging returns the inode now published for a staging and its
// data version. TernFS appends spans at a file's current size, so a
// transient that may hold some is never written again: the candidate, the
// transient last submitted, is resolved first, and a linked one with the
// current data version is the publication; otherwise one fresh transient
// is linked.
func (s *Server) publishStaging(stagingID vfs.InodeID, meta staging.Meta, sf staging.File) (vfs.InodeID, uint64, error) {
	c := sf.Candidate()
	// linkErr is a first submission's error that another copy of the
	// request may hide: it stands only if the candidate did not link.
	var linkErr error
	if c.ID == 0 {
		if err := s.hydrateForPublication(sf); err != nil {
			return 0, 0, err
		}
		// Set before the submission, so that a checkpoint during the upload
		// records it beside the newer data version.
		c = staging.Candidate{ID: stagingID, Cookie: meta.TernCookie, Version: sf.DataVersion()}
		if err := sf.SetCandidate(c, false); err != nil {
			return 0, 0, err
		}
		err := s.linkCandidate(c, meta, sf)
		switch {
		case err == nil && sf.DataVersion() == c.Version:
			return c.ID, c.Version, nil
		case err == nil:
			// A WRITE, which takes no file lock, landed during the upload.
			return s.publishFresh(meta, sf)
		case publicationDelayed(err):
			if err := sf.SetCandidate(c, true); err != nil {
				s.log.Warn("close: save publication candidate", "inode", stagingID, "err", err)
			}
			return 0, 0, err
		case !os.IsNotExist(err):
			linkErr = err
		}
	}
	linked, err := s.resolveCandidate(c)
	switch {
	case err != nil:
		return 0, 0, err
	case linked && c.Version == sf.DataVersion():
		s.log.Info("close: an earlier attempt had published", "inode", c.ID, "staging", stagingID, "name", meta.FileName)
		return c.ID, c.Version, nil
	case !linked && linkErr != nil:
		return 0, 0, resolvedError{linkErr}
	}
	return s.publishFresh(meta, sf)
}

// resolvedError is a failure whose candidate cannot link: no fence needed.
type resolvedError struct{ error }

func (e resolvedError) Unwrap() error { return e.error }

// publishFresh links the staging's bytes through a fresh transient, recorded
// as the candidate before it is submitted.
func (s *Server) publishFresh(meta staging.Meta, sf staging.File) (vfs.InodeID, uint64, error) {
	if err := s.hydrateForPublication(sf); err != nil {
		return 0, 0, resolvedError{err}
	}
	version := sf.DataVersion()
	id, cookie, err := s.fs.ConstructFile(meta.DirID)
	if err != nil {
		return 0, 0, resolvedError{err}
	}
	c := staging.Candidate{ID: id, Cookie: cookie, Version: version}
	if err := sf.SetCandidate(c, true); err != nil {
		// Recovery must know this transient before it can link. Keep the
		// staging and delay without submitting; a retry resolves this
		// unused candidate and checkpoints a fresh one.
		return 0, 0, errors.Join(errCandidateCheckpoint, err)
	}
	err = s.linkCandidate(c, meta, sf)
	if err != nil && !publicationDelayed(err) {
		// Not-exist from a retransmitted LinkFileReq of a link that
		// applied, or any error another copy of the request may hide.
		linked, resolveErr := s.resolveCandidate(c)
		switch {
		case resolveErr != nil:
			return 0, 0, resolveErr
		case !linked:
			return 0, 0, resolvedError{err}
		}
		err = nil
	}
	if err != nil {
		return 0, 0, err
	}
	if sf.DataVersion() != version {
		// The candidate keeps its older version: the retry publishes again.
		return 0, 0, errStagingChanged
	}
	s.log.Info("close: published through a fresh transient", "inode", id, "name", meta.FileName)
	return id, version, nil
}

// hydrateForPublication completes a replacement's copy of its base.
func (s *Server) hydrateForPublication(sf staging.File) error {
	s.startHydration(sf)
	return sf.FinishHydration(s.readBaseForeground)
}

// linkCandidate uploads the staging into the candidate and links it.
func (s *Server) linkCandidate(c staging.Candidate, meta staging.Meta, sf staging.File) error {
	r, err := sf.Reader()
	if err != nil {
		// Seek on an open regular file cannot fail; broken invariant.
		panic("close: staging Reader failed: " + err.Error())
	}
	return s.fs.LinkFile(c.ID, c.Cookie, meta.DirID, meta.FileName, r)
}

// resolveCandidate reports whether a candidate was linked: a successful
// ScrapFile proves it transient, and it can then never link; not-exist
// with Stat succeeding proves it linked, even if since replaced.
func (s *Server) resolveCandidate(c staging.Candidate) (bool, error) {
	err := s.fs.ScrapFile(c.ID, c.Cookie)
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	if _, err := s.fs.Stat(c.ID); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	return false, nil
}

// fenceCandidate scraps a staging's candidate, so that an unanswered
// LinkFile of it cannot land later; it returns only a transient error.
func (s *Server) fenceCandidate(id vfs.InodeID) error {
	if sf := s.stagingStore.Get(id); sf != nil && sf.Candidate().ID != 0 {
		return s.scrapTransient(id, sf.Candidate())
	}
	return nil
}

// scrapTransient scraps a transient; it returns only a transient error.
func (s *Server) scrapTransient(stagingID vfs.InodeID, c staging.Candidate) error {
	err := s.fs.ScrapFile(c.ID, c.Cookie)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	s.log.Warn("scrap transient staging file", "inode", stagingID, "transient", c.ID, "err", err)
	if vfs.IsTransientBackendError(err) {
		return err
	}
	return nil
}

// unpublished is the candidate, or the staging's own transient.
func unpublished(id vfs.InodeID, meta staging.Meta, c staging.Candidate) staging.Candidate {
	if c.ID != 0 {
		return c
	}
	return staging.Candidate{ID: id, Cookie: meta.TernCookie}
}

// recoveredAccess is a recovered open's access: its writer's original
// grant is unknown, so a writer may keep or drop READ.
func recoveredAccess(meta staging.Meta) uint32 {
	if meta.ReadOnly {
		return nfs4.OPEN4_SHARE_ACCESS_READ
	}
	return nfs4.OPEN4_SHARE_ACCESS_BOTH
}

// regrant applies an OPEN_DOWNGRADE that keeps the open's writer, or
// narrows a read open, under the file lock: it checks the target again
// against the current grants and advances the generation.
func (s *Server) regrant(st *compoundState, stateid nfs4.Stateid4, g openstate.Grant) (openstate.State, uint32) {
	st.hold(s.opens.LockFile(st.currentID))
	sid := extractStateID(stateid)
	current, status := s.opens.Lookup(st.clientID, sid, stateid.Seqid(), st.currentID)
	if status != nfs4.NFS4_OK {
		return openstate.State{}, status
	}
	if status := current.DowngradeStatus(g, current.Recovered); status != nfs4.NFS4_OK {
		return openstate.State{}, status
	}
	if current.Writes() && g.Access == nfs4.OPEN4_SHARE_ACCESS_READ {
		// An upgrade raced in; the caller's path no longer applies.
		return openstate.State{}, nfs4.NFS4ERR_DELAY
	}
	state, ok := s.opens.Downgrade(sid, current.DataID, g)
	if !ok {
		return openstate.State{}, nfs4.NFS4ERR_BAD_STATEID
	}
	return state, nfs4.NFS4_OK
}

func writeCloseError(w *nfs4.COMPOUND4resWriter, status uint32) uint32 {
	ew := w.AppendResarray_Close()
	ew.SetValue_Default(status)
	w.Resume(ew.Finish())
	return status
}

func writeCloseResponse(w *nfs4.COMPOUND4resWriter, state openstate.State) uint32 {
	ew := w.AppendResarray_Close()
	stid := ew.SetValue_Nfs4Ok()
	writeStateID(stid, state.Generation, state.ID)
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
	ownerKey := openstate.OwnerKey{
		ClientID: clientID,
		Owner:    string(owner.Owner()),
	}
	fail := func(status uint32) uint32 {
		return writeOpenError(w, status)
	}

	access := args.ShareAccess() & nfs4.OPEN4_SHARE_ACCESS_BOTH

	// targetID is the inode the open's I/O uses; publishedID is the inode
	// the name resolves to, which is the filehandle OPEN returns.
	var targetID, publishedID vfs.InodeID
	var nfsSID nfs4.StateID
	joined := false
	created := false
	staged := false
	reusedStaging := false
	var openSize *uint64
	var exclusiveVerifier *[8]byte
	var recoveredStagingID vfs.InodeID
	var recoveredStagingMeta staging.Meta
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
	// The pathname lock, which a re-open drops before it starts again.
	releaseName := func() {}
	adoptRecoveredStaging := func() {
		nfsSID = s.opens.NewStateID()
		reusedStaging = true
		staged = true
	}

	// openDir and openName are where the file is opened, remembered for
	// later opens by filehandle.
	var openDir vfs.InodeID
	var openName string
	// A writable CLAIM_FH this nfsd cannot name may recover a staged writer
	// (README.md#writer-recovery-mechanics): the preselected staging, which the
	// pathname lock
	// must confirm.
	var recovering bool
	var recoverID vfs.InodeID
	var recoverMeta staging.Meta
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
				recoverID, recoverMeta, status = s.claimFHRecoveryCandidate(fileID, clientID, ownerKey.Owner)
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
			releaseName = st.hold(s.lockMutationTargets(mutationTarget{
				dirID: dirID,
				name:  fileName,
			}))
			var recoveryErr error
			recoveredStagingID, recoveredStagingMeta,
				hasRecoveredStaging, recoveryErr = s.recoveredStagingTarget(
				dirID, fileName, clientID, ownerKey.Owner,
				access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0,
			)
			if recovering && errors.Is(recoveryErr, nfs4.Error(nfs4.NFS4ERR_EXPIRED)) {
				// Another staging became selectable since preselection.
				return fail(nfs4.NFS4ERR_STALE)
			}
			if recoveryErr != nil {
				return fail(statusOf(recoveryErr))
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
				if hasRecoveredStaging {
					if recoveredStagingMeta.BaseID != 0 {
						if s.fenceAndDiscardStaging(recoveredStagingID) != nil {
							return fail(nfs4.NFS4ERR_DELAY)
						}
						hasRecoveredStaging = false
					}
				}
				// Keep the writer's transient distinct from the empty inode
				// that LOOKUP and READDIR will expose when OPEN succeeds.
				placeholderID, placeholderCookie, err = s.fs.ConstructFile(dirID)
				if err != nil {
					return fail(s.constructErrToNFS(err))
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
				existing, local := s.opens.ExistingOpen(ownerKey, publishedID)
				if local && existing.ID == meta.NFSStateID {
					nfsSID = existing.ID
					reusedStaging, staged = true, true
				} else if s.opens.CanRecover(meta.NFSStateID) {
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
						return fail(s.constructErrToNFS(err))
					}
					publishedID = placeholderID
					placeholderDir, placeholderName = dirID, fileName
				} else {
					if lookupStatus == nfs4.NFS4ERR_NOENT &&
						hasRecoveredStaging {
						// Fenced first: a link of its candidate the backend
						// has not answered must not recreate the name.
						if s.fenceAndDiscardStaging(recoveredStagingID) != nil {
							return fail(nfs4.NFS4ERR_DELAY)
						}
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
			if writer, ok := s.opens.WriterOf(clientID, publishedID); ok {
				if s.stagingStore.Failed(writer.DataID) {
					// As for any operation on that writer.
					return fail(nfs4.NFS4ERR_IO)
				}
				targetID = writer.DataID
				staged, joined = true, true
				if writer.Owner == ownerKey {
					nfsSID = writer.ID
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
					dirID, fileName, clientID, ownerKey.Owner,
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
		return s.setStagingSize(sf, *openSize)
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

	releaseOpen := st.hold(s.opens.LockFile(publishedID))
	if !staged && !created && access&nfs4.OPEN4_SHARE_ACCESS_WRITE == 0 {
		// A writer's CLOSE may have ended the staging this read selected
		// while it waited for the file (../openstate/README.md#reader-visibility).
		targetID = s.readSource(clientID, publishedID)
	}
	if !staged && claimType == nfs4.CLAIM_NULL && openDir != 0 && retries < 3 {
		// A CLOSE may have published over the name while we waited. Open
		// what the name holds now. Writers hold the pathname lock already.
		if current, err := s.fs.Lookup(openDir, openName); err == nil &&
			current != publishedID {
			releaseOpen()
			releaseName()
			return s.openRetry(args, st, w, retries+1)
		}
	}
	markID := nfsSID
	if existing, ok := s.opens.ExistingOpen(ownerKey, publishedID); ok {
		markID = existing.ID
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
	if markID == (nfs4.StateID{}) {
		markID = s.opens.NewStateID()
		nfsSID = markID
	}
	// The open's client must be confirmed with a live lease. The open is
	// recorded in this process only; a writer's sidecar is what a restart
	// recovers from.
	if status := s.clientStatus(clientID); status != nfs4.NFS4_OK {
		if staged {
			cleanupStagedOpen()
		}
		return fail(status)
	}
	// A joined staging bound to a closing open is bound to this one, so a
	// restart recovers the writer under the owner still using it.
	var joinedFrom staging.Meta
	if joined {
		if meta, ok := s.stagingStore.GetMeta(targetID); ok && meta.NFSStateID != markID && s.opens.Closing(meta.NFSStateID) {
			if err := s.stagingStore.Bind(targetID, clientID, markID, ownerKey.Owner, false); err != nil {
				return fail(nfs4.NFS4ERR_IO)
			}
			joinedFrom = meta
		}
	}
	if reusedStaging && recoveredStagingMeta.NFSStateID != markID {
		if err := s.stagingStore.Rebind(
			recoveredStagingID, clientID, markID,
		); err != nil {
			return fail(nfs4.NFS4ERR_IO)
		}
		stagingRebound = true
		s.log.Info("recovered staging adopted", "inode", recoveredStagingID, "name", recoveredStagingMeta.FileName,
			"old_stateid", recoveredStagingMeta.NFSStateID, "stateid", markID, "claim", claimType)
		// Retired files release their descriptors; Rebind reopens them.
		// Apply CREATE size only then.
		if openSize != nil {
			if err := applyOpenSize(); err != nil {
				cleanupStagedOpen()
				return fail(nfs4.NFS4ERR_IO)
			}
		}
	}
	if placeholderID != 0 {
		if err := s.fs.LinkFile(
			placeholderID, placeholderCookie, placeholderDir, placeholderName, nil,
		); err != nil {
			if staged {
				cleanupStagedOpen()
			}
			return fail(s.errToNFS(err))
		}
		placeholderPublished = true
	}
	state, err := s.opens.Open(
		ownerKey,
		publishedID,
		targetID,
		openstate.Grant{Access: access, Deny: args.ShareDeny()},
		nfsSID,
	)
	if err != nil {
		if joinedFrom.ClientID != 0 {
			if err := s.stagingStore.Bind(targetID, joinedFrom.ClientID, joinedFrom.NFSStateID,
				joinedFrom.OpenOwner, joinedFrom.ReadOnly); err != nil {
				s.log.Error("restore joined staging owner", "err", err)
			}
		}
		if staged {
			cleanupStagedOpen()
		}
		return fail(nfs4.NFS4ERR_SERVERFAULT)
	}
	if state.Writes() {
		s.views.AddWriter(openstate.ViewKey{ClientID: clientID, FileID: publishedID}, state.DataID)
	}
	if placeholderDir != 0 {
		s.names.remember(publishedID, placeholderDir, placeholderName)
	} else if openDir != 0 {
		s.names.remember(publishedID, openDir, openName)
	}
	response := openResponse{state: state, created: created}
	return writeOpenResponse(w, st, response)
}

func (s *Server) constructStaging(
	dirID vfs.InodeID,
	fileName string,
	clientID uint64,
	openOwner string,
	write bool,
	baseID vfs.InodeID,
	baseInfo vfs.NodeInfo,
	exclusiveVerifier *[8]byte,
) (vfs.InodeID, nfs4.StateID, uint32) {
	recoveryKey, status := s.recoveryKey(clientID)
	if status != nfs4.NFS4_OK {
		return 0, nfs4.StateID{}, status
	}
	id, cookie, err := s.fs.ConstructFile(dirID)
	if err != nil {
		return 0, nfs4.StateID{}, s.constructErrToNFS(err)
	}
	stateID := s.opens.NewStateID()
	meta := staging.Meta{
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
		return 0, nfs4.StateID{}, nfs4.NFS4ERR_IO
	}
	return id, stateID, nfs4.NFS4_OK
}

// constructErrToNFS answers DELAY for an unanswered ConstructFile, which has
// no visible effect.
func (s *Server) constructErrToNFS(err error) uint32 {
	if vfs.IsTransientBackendError(err) {
		return nfs4.NFS4ERR_DELAY
	}
	return s.errToNFS(err)
}

type openResponse struct {
	state   openstate.State
	created bool
}

func writeOpenResponse(
	w *nfs4.COMPOUND4resWriter,
	st *compoundState,
	response openResponse,
) uint32 {
	ew := w.AppendResarray_Open()
	state := response.state
	st.setCurrentFH(state.FileID)
	st.setCurrentStateID(state.ID, state.Generation)
	okW := ew.SetValue_Nfs4Ok()

	writeStateID(okW.Stateid(), state.Generation, state.ID)

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
