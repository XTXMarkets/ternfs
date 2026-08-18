// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"time"
)

// Stable write modes.
const (
	unstable4 = 0
	dataSync4 = 1
	fileSync4 = 2

	maxTernNameLength = 255
)

func ternNameTooLong(name []byte) bool {
	return len(name) > maxTernNameLength
}

func ternNameUnsupported(name []byte) bool {
	return bytes.Equal(name, []byte(".")) ||
		bytes.Equal(name, []byte("..")) ||
		bytes.IndexByte(name, '/') >= 0 ||
		bytes.IndexByte(name, 0) >= 0
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

// lookupDurableOpen requires both valid local protocol state and a fleet-wide
// lease. The lease invalidates stale local state after a reboot is confirmed.
func (s *Server) lookupDurableOpen(
	stateid Stateid4,
	fileID InodeID,
) (openState, uint32) {
	state, status := s.opens.lookup(
		extractStateID(stateid),
		stateid.Seqid(),
		fileID,
	)
	if status != NFS4_OK {
		return openState{}, status
	}
	active, err := s.clients.HasOpen(state.owner.clientID, state.id)
	if err != nil {
		return openState{}, s.errToNFS(err)
	}
	if !active {
		return openState{}, NFS4ERR_EXPIRED
	}
	return state, NFS4_OK
}

func (s *Server) opAccess(args ACCESS4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Access()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
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
		ew := w.AppendResarray_Close()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}

	sid := extractStateID(args.OpenStateid())
	meta, hasMeta := s.stagingStore.GetMeta(st.currentID)
	openState, replay, status := s.opens.validateClose(
		sid,
		args.OpenStateid().Seqid(),
		st.currentID,
		args.Seqid(),
	)
	recovered := status != NFS4_OK && hasMeta &&
		sid == meta.NFSStateID && s.opens.canRecover(sid)
	if status != NFS4_OK && !recovered {
		ew := w.AppendResarray_Close()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if !recovered && !replay {
		// Recovered CLOSE is backed by staging state. A replay has already
		// consumed its lease. Other CLOSEs still require a fleet-wide lease.
		active, err := s.clients.HasOpen(openState.owner.clientID, sid)
		if err != nil {
			ew := w.AppendResarray_Close()
			status := s.errToNFS(err)
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		if !active {
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(NFS4ERR_EXPIRED)
			w.Resume(ew.Finish())
			return NFS4ERR_EXPIRED
		}
	}
	if replay {
		if err := s.clients.RemoveOpen(openState.owner.clientID, sid); err != nil {
			s.log.Warn("close: remove active-open record", "err", err)
		}
		ew := w.AppendResarray_Close()
		stid := ew.SetValue_Nfs4Ok()
		stid.SetSeqid(openState.generation)
		writeStateID(stid, sid)
		w.Resume(ew.Finish())
		return NFS4_OK
	}

	// Check if there's a staging file for the current filehandle.
	if hasMeta {
		// First CLOSE for a write-open: link the transient file.
		if sid != meta.NFSStateID {
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(NFS4ERR_BAD_STATEID)
			w.Resume(ew.Finish())
			return NFS4ERR_BAD_STATEID
		}
		sf := s.stagingStore.Get(st.currentID)
		if sf == nil {
			// Invariant: a staging entry always has both meta and a data file.
			panic("close: staging meta present but no data file")
		}
		r, rErr := sf.Reader()
		if rErr != nil {
			// Seek on an open regular file cannot fail; broken invariant.
			panic("close: staging Reader failed: " + rErr.Error())
		}
		linkErr := s.fs.LinkFile(st.currentID, meta.TernCookie, meta.DirID, meta.FileName, r)
		// Any error here is terminal: transient failures are already retried
		// below us, and the write phase isn't resumable. Drop staging either way.
		s.stagingStore.Remove(st.currentID)
		if linkErr != nil {
			status := s.errToNFS(linkErr)
			s.log.Error("close: link file", "err", linkErr, "status", Nfsstat4Name(status))
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
	} else {
		// No staging: either a read-close or a replay of a write-close.
		// Check if the file still exists. For read opens the file is
		// immutable and always exists. For write-close replays the file
		// exists if LinkFile succeeded previously. If the transient was
		// GC'd without being linked, Stat fails and we report expired.
		if _, err := s.fs.Stat(st.currentID); err != nil {
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(NFS4ERR_EXPIRED)
			w.Resume(ew.Finish())
			return NFS4ERR_EXPIRED
		}
	}

	var openClientID uint64
	if recovered {
		// The sidecar is the only owner record that survives a restart.
		openClientID = meta.ClientID
		openState = s.opens.closeRecovered(
			sid,
			st.currentID,
			args.OpenStateid().Seqid(),
			args.Seqid(),
		)
	} else {
		openState, status = s.opens.close(sid, args.Seqid())
		if status != NFS4_OK {
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		openClientID = openState.owner.clientID
	}
	if openClientID != 0 {
		if err := s.clients.RemoveOpen(openClientID, sid); err != nil {
			s.log.Warn("close: remove active-open record", "err", err)
		}
	}
	ew := w.AppendResarray_Close()
	stid := ew.SetValue_Nfs4Ok()
	stid.SetSeqid(openState.generation)
	writeStateID(stid, sid)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opCommit(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Commit()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Commit()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	// Sync the staging file for the current filehandle.
	if sf := s.stagingStore.Get(st.currentID); sf != nil {
		sf.Sync()
	}

	ew := w.AppendResarray_Commit()
	okW := ew.SetValue_Nfs4Ok()
	verf := okW.Writeverf()
	for i := 0; i < 8; i++ {
		verf.SetData(i, s.writeVerifier[i])
	}
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opCreate(args CREATE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if st.currentID.Type() != InodeTypeDir {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_NOTDIR)
		w.Resume(ew.Finish())
		return NFS4ERR_NOTDIR
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	nameData := args.Objname().Data()
	if len(nameData) == 0 {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	if ternNameTooLong(nameData) {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_NAMETOOLONG)
		w.Resume(ew.Finish())
		return NFS4ERR_NAMETOOLONG
	}
	if ternNameUnsupported(nameData) {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_BADNAME)
		w.Resume(ew.Finish())
		return NFS4ERR_BADNAME
	}
	name := string(nameData)
	objType := args.ObjtypeType()
	switch objType {
	case NF4DIR, NF4LNK:
	case NF4REG:
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_BADTYPE)
		w.Resume(ew.Finish())
		return NFS4ERR_BADTYPE
	default:
		// We don't support creating block/char/socket/fifo devices.
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_NOTSUPP)
		w.Resume(ew.Finish())
		return NFS4ERR_NOTSUPP
	}
	if status := validateCreateAttrs(args.Createattrs()); status != NFS4_OK {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if _, err := s.fs.Lookup(st.currentID, name); err == nil {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_EXIST)
		w.Resume(ew.Finish())
		return NFS4ERR_EXIST
	} else if status := s.errToNFS(err); status != NFS4ERR_NOENT {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	var newID InodeID
	var err error

	switch objType {
	case NF4DIR:
		newID, err = s.fs.Mkdir(st.currentID, name)
	case NF4LNK:
		targetData := args.Objtype().AsLinktext4().Data()
		if len(targetData) == 0 {
			ew := w.AppendResarray_Create()
			ew.SetValue_Default(NFS4ERR_INVAL)
			w.Resume(ew.Finish())
			return NFS4ERR_INVAL
		}
		target := string(targetData)
		newID, err = s.fs.Symlink(st.currentID, name, target)
	}

	if err != nil {
		ew := w.AppendResarray_Create()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	st.currentID = newID
	st.currentIDSet = true

	ew := w.AppendResarray_Create()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	cinfo.SetAtomic(TRUE)
	now := uint64(time.Now().UnixNano())
	cinfo.SetBefore(now - 1)
	cinfo.SetAfter(now)

	// attrset bitmap: empty (we don't apply createattrs).
	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opDelegpurge(w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_Delegpurge()
	r.SetStatus(NFS4ERR_NOTSUPP)
	return NFS4ERR_NOTSUPP
}

func (s *Server) opDelegreturn(st *compoundState, w *COMPOUND4resWriter) uint32 {
	// We never grant delegations, but accept returns gracefully.
	r := w.AppendResarray_Delegreturn()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opGetattr(args GETATTR4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getattr()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	ni, err := s.fs.Stat(st.currentID)
	if err != nil {
		// If Stat fails but the file is being staged (transient file not yet
		// linked), synthesize metadata from the staging store.
		if sz, ok := s.stagingStore.StagedSize(st.currentID); ok {
			now := time.Now()
			ni = NodeInfo{Size: sz, Mtime: now, Atime: now}
		} else {
			ew := w.AppendResarray_Getattr()
			status := s.errToNFS(err)
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
	} else {
		// If the file has an active staging buffer, use its size.
		if sz, ok := s.stagingStore.StagedSize(st.currentID); ok {
			ni.Size = sz
		}
	}

	reqMask := parseBitmap(args.AttrRequest())
	if status := validateGetattrMask(reqMask); status != NFS4_OK {
		ew := w.AppendResarray_Getattr()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	ew := w.AppendResarray_Getattr()
	okW := ew.SetValue_Nfs4Ok()

	// Build fattr4: bitmap + attrlist.
	faw := okW.StartObjAttributes()
	bmW := faw.StartAttrmask()

	// Compute response bitmap: intersection of requested and supported.
	var respMask [2]uint32
	respMask[0] = reqMask[0] & supportedAttrs0
	respMask[1] = reqMask[1] & supportedAttrs1

	bmW.AppendData(respMask[0])
	bmW.AppendData(respMask[1])
	buf := bmW.Finish()
	faw.Resume(buf)

	// Build attribute values.
	alW := faw.StartAttrVals()
	attrBuf := encodeAttrs(respMask, st.currentID, ni)
	buf = alW.SetData(attrBuf).Finish()
	faw.Resume(buf)
	buf = faw.Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())

	return NFS4_OK
}

func (s *Server) opGetfh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getfh()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
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

func (s *Server) opLink(st *compoundState, w *COMPOUND4resWriter) uint32 {
	// TernFS doesn't support hard links.
	ew := w.AppendResarray_Link()
	ew.SetValue_Default(NFS4ERR_NOTSUPP)
	w.Resume(ew.Finish())
	return NFS4ERR_NOTSUPP
}

func (s *Server) opLock(w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_Lock()
	ew.SetValue_Default(NFS4ERR_NOTSUPP)
	w.Resume(ew.Finish())
	return NFS4ERR_NOTSUPP
}

func (s *Server) opLockt(w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_Lockt()
	ew.SetValue_Default(NFS4ERR_NOTSUPP)
	w.Resume(ew.Finish())
	return NFS4ERR_NOTSUPP
}

func (s *Server) opLocku(w *COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_Locku()
	ew.SetValue_Default(NFS4ERR_NOTSUPP)
	w.Resume(ew.Finish())
	return NFS4ERR_NOTSUPP
}

func (s *Server) opLookup(args LOOKUP4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookup()
		r.SetStatus(NFS4ERR_NOFILEHANDLE)
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		r := w.AppendResarray_Lookup()
		r.SetStatus(status)
		return status
	}
	nameData := args.Objname().Data()
	if len(nameData) == 0 {
		r := w.AppendResarray_Lookup()
		r.SetStatus(NFS4ERR_INVAL)
		return NFS4ERR_INVAL
	}
	if ternNameTooLong(nameData) {
		r := w.AppendResarray_Lookup()
		r.SetStatus(NFS4ERR_NAMETOOLONG)
		return NFS4ERR_NAMETOOLONG
	}
	if ternNameUnsupported(nameData) {
		r := w.AppendResarray_Lookup()
		r.SetStatus(NFS4ERR_BADNAME)
		return NFS4ERR_BADNAME
	}
	name := string(nameData)
	// Hide the internal .nfs directory from client access.
	if name == nfsDirName && st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookup()
		r.SetStatus(NFS4ERR_NOENT)
		return NFS4ERR_NOENT
	}
	id, err := s.fs.Lookup(st.currentID, name)
	if err != nil {
		r := w.AppendResarray_Lookup()
		status := s.errToNFS(err)
		r.SetStatus(status)
		return status
	}
	st.currentID = id
	st.currentIDSet = true
	r := w.AppendResarray_Lookup()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opLookupp(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookupp()
		r.SetStatus(NFS4ERR_NOFILEHANDLE)
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		r := w.AppendResarray_Lookupp()
		r.SetStatus(status)
		return status
	}
	if st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookupp()
		r.SetStatus(NFS4ERR_NOENT)
		return NFS4ERR_NOENT
	}
	id, err := s.fs.LookupParent(st.currentID)
	if err != nil {
		r := w.AppendResarray_Lookupp()
		status := s.errToNFS(err)
		r.SetStatus(status)
		return status
	}
	st.currentID = id
	st.currentIDSet = true
	r := w.AppendResarray_Lookupp()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opNverify(args NVERIFY4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Nverify()
		r.SetStatus(NFS4ERR_NOFILEHANDLE)
		return NFS4ERR_NOFILEHANDLE
	}
	same, status := s.verifyAttrs(st.currentID, args.ObjAttributes())
	if status != NFS4_OK {
		r := w.AppendResarray_Nverify()
		r.SetStatus(status)
		return status
	}
	r := w.AppendResarray_Nverify()
	if same {
		// NVERIFY: fail if attributes are the same.
		r.SetStatus(NFS4ERR_SAME)
		return NFS4ERR_SAME
	}
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opOpen(args OPEN4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}

	// Reject all writes if read-only (no staging directory configured).
	access := args.ShareAccess()
	if s.stagingStore.ReadOnly() && access&OPEN4_SHARE_ACCESS_WRITE != 0 {
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	claim := args.Claim()
	claimType := args.ClaimType()
	owner := args.Owner()
	clientID := owner.Clientid()
	confirmed, err := s.clients.IsConfirmed(clientID)
	if err != nil {
		ew := w.AppendResarray_Open()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if !confirmed {
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(NFS4ERR_STALE_CLIENTID)
		w.Resume(ew.Finish())
		return NFS4ERR_STALE_CLIENTID
	}
	ownerKey := openOwnerKey{
		clientID: clientID,
		owner:    string(owner.Owner()),
	}
	replayState, replay, status := s.opens.beginOpen(ownerKey, args.Seqid())
	if status != NFS4_OK {
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if replay {
		if err := s.clients.MarkOpen(clientID, replayState.id); err != nil {
			ew := w.AppendResarray_Open()
			status := s.errToNFS(err)
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		st.currentID = replayState.fileID
		st.currentIDSet = true
		writeOpenResponse(w, replayState, false)
		return NFS4_OK
	}

	var targetID InodeID
	var nfsSID StateID
	created := false

	switch claimType {
	case CLAIM_NULL:
		dirID := st.currentID
		if status := requireDirectory(dirID); status != NFS4_OK {
			ew := w.AppendResarray_Open()
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		fileNameData := claim.AsNull().Data()
		if len(fileNameData) == 0 {
			ew := w.AppendResarray_Open()
			ew.SetValue_Default(NFS4ERR_INVAL)
			w.Resume(ew.Finish())
			return NFS4ERR_INVAL
		}
		if ternNameTooLong(fileNameData) {
			ew := w.AppendResarray_Open()
			ew.SetValue_Default(NFS4ERR_NAMETOOLONG)
			w.Resume(ew.Finish())
			return NFS4ERR_NAMETOOLONG
		}
		if ternNameUnsupported(fileNameData) {
			ew := w.AppendResarray_Open()
			ew.SetValue_Default(NFS4ERR_BADNAME)
			w.Resume(ew.Finish())
			return NFS4ERR_BADNAME
		}
		fileName := string(fileNameData)

		// Check if this is a create.
		if args.OpenhowType() == OPEN4_CREATE {
			createHow := args.Openhow().AsCreatehow4Entry()
			// EXCLUSIVE4 not supported — clients should fall back
			// to GUARDED4 or UNCHECKED4.
			if createHow.Disc() == EXCLUSIVE4 {
				ew := w.AppendResarray_Open()
				ew.SetValue_Default(NFS4ERR_NOTSUPP)
				w.Resume(ew.Finish())
				return NFS4ERR_NOTSUPP
			}
			var createAttrs Fattr4
			switch createHow.Disc() {
			case UNCHECKED4:
				createAttrs = createHow.Value().AsUnchecked4()
			case GUARDED4:
				createAttrs = createHow.Value().AsGuarded4()
			}
			if status := validateCreateAttrs(createAttrs); status != NFS4_OK {
				ew := w.AppendResarray_Open()
				ew.SetValue_Default(status)
				w.Resume(ew.Finish())
				return status
			}
			// Try lookup first.
			id, err := s.fs.Lookup(dirID, fileName)
			if err != nil {
				// File doesn't exist — construct a transient file.
				var fileCookie Cookie
				id, fileCookie, err = s.fs.ConstructFile(dirID)
				if err != nil {
					ew := w.AppendResarray_Open()
					ew.SetValue_Default(s.errToNFS(err))
					w.Resume(ew.Finish())
					return s.errToNFS(err)
				}
				// Generate a random NFS stateid and create staging with metadata.
				nfsSID = s.opens.newStateID()
				meta := StagingMeta{
					DirID:      dirID,
					FileName:   fileName,
					TernCookie: fileCookie,
					NFSStateID: nfsSID,
					ClientID:   clientID,
				}
				if _, sfErr := s.stagingStore.Create(id, meta); sfErr != nil {
					s.log.Error("staging create error", "err", sfErr)
					ew := w.AppendResarray_Open()
					ew.SetValue_Default(NFS4ERR_IO)
					w.Resume(ew.Finish())
					return NFS4ERR_IO
				}
				created = true
			} else if createHow.Disc() == GUARDED4 {
				ew := w.AppendResarray_Open()
				ew.SetValue_Default(NFS4ERR_EXIST)
				w.Resume(ew.Finish())
				return NFS4ERR_EXIST
			}
			targetID = id
		} else {
			id, err := s.fs.Lookup(dirID, fileName)
			if err != nil {
				ew := w.AppendResarray_Open()
				status := s.errToNFS(err)
				ew.SetValue_Default(status)
				w.Resume(ew.Finish())
				return status
			}
			targetID = id
		}

		// Files are immutable: reject write access to existing files.
		// To replace a file, clients must remove + create.
		if !created && access&OPEN4_SHARE_ACCESS_WRITE != 0 {
			ew := w.AppendResarray_Open()
			ew.SetValue_Default(NFS4ERR_PERM)
			w.Resume(ew.Finish())
			return NFS4ERR_PERM
		}
	case CLAIM_PREVIOUS:
		// No grace period — there is no lock/open state to reclaim.
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(NFS4ERR_NO_GRACE)
		w.Resume(ew.Finish())
		return NFS4ERR_NO_GRACE
	default:
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(NFS4ERR_NOTSUPP)
		w.Resume(ew.Finish())
		return NFS4ERR_NOTSUPP
	}

	if status := requireRegularFile(targetID); status != NFS4_OK {
		ew := w.AppendResarray_Open()
		if targetID.Type() == InodeTypeSymlink {
			status = NFS4ERR_SYMLINK
		}
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	if nfsSID == (StateID{}) {
		nfsSID = s.opens.newStateID()
	}
	if err := s.clients.MarkOpen(clientID, nfsSID); err != nil {
		if created {
			s.stagingStore.Remove(targetID)
		}
		ew := w.AppendResarray_Open()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	state, status := s.opens.addOpen(
		ownerKey,
		args.Seqid(),
		targetID,
		access&OPEN4_SHARE_ACCESS_WRITE != 0,
		nfsSID,
	)
	if status != NFS4_OK {
		_ = s.clients.RemoveOpen(clientID, nfsSID)
		if created {
			s.stagingStore.Remove(targetID)
		}
		ew := w.AppendResarray_Open()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	nfsSID = state.id

	st.currentID = targetID
	st.currentIDSet = true

	writeOpenResponse(w, state, created)
	return NFS4_OK
}

func writeOpenResponse(
	w *COMPOUND4resWriter,
	state openState,
	created bool,
) {
	ew := w.AppendResarray_Open()
	okW := ew.SetValue_Nfs4Ok()

	stid := okW.Stateid()
	stid.SetSeqid(state.generation)
	writeStateID(stid, state.id)

	cinfo := okW.Cinfo()
	cinfo.SetAtomic(TRUE)
	now := uint64(time.Now().UnixNano())
	if created {
		cinfo.SetBefore(now - 1)
		cinfo.SetAfter(now)
	} else {
		cinfo.SetBefore(now)
		cinfo.SetAfter(now)
	}

	okW.SetRflags(OPEN4_RESULT_LOCKTYPE_POSIX | OPEN4_RESULT_CONFIRM)

	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)

	okW.SetDelegation_None()

	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
}

func (s *Server) opOpenConfirm(args OPENCONFIRM4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_OpenConfirm()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}

	sid := extractStateID(args.OpenStateid())
	state, status := s.opens.confirm(
		sid,
		args.OpenStateid().Seqid(),
		st.currentID,
		args.Seqid(),
	)
	if status != NFS4_OK {
		ew := w.AppendResarray_OpenConfirm()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	active, err := s.clients.HasOpen(state.owner.clientID, state.id)
	if err != nil {
		ew := w.AppendResarray_OpenConfirm()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if !active {
		ew := w.AppendResarray_OpenConfirm()
		ew.SetValue_Default(NFS4ERR_EXPIRED)
		w.Resume(ew.Finish())
		return NFS4ERR_EXPIRED
	}
	ew := w.AppendResarray_OpenConfirm()
	ok := ew.SetValue_Nfs4Ok()
	stid := ok.OpenStateid()
	stid.SetSeqid(state.generation)
	writeStateID(stid, sid)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opOpenDowngrade(st *compoundState, w *COMPOUND4resWriter) uint32 {
	// No persistent open state — downgrade is a no-op.
	ew := w.AppendResarray_OpenDowngrade()
	ew.SetValue_Default(NFS4ERR_NOTSUPP)
	w.Resume(ew.Finish())
	return NFS4ERR_NOTSUPP
}

func (s *Server) opOpenattr(w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_Openattr()
	r.SetStatus(NFS4ERR_NOTSUPP)
	return NFS4ERR_NOTSUPP
}

func (s *Server) opPutfh(args PUTFH4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	fhData := args.Object().Data()
	id, ok := fhToInodeID(fhData)
	if !ok {
		r := w.AppendResarray_Putfh()
		r.SetStatus(NFS4ERR_BADHANDLE)
		return NFS4ERR_BADHANDLE
	}
	st.currentID = id
	st.currentIDSet = true
	r := w.AppendResarray_Putfh()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opPutrootfh(st *compoundState, w *COMPOUND4resWriter, isPub bool) uint32 {
	st.currentID = s.fs.RootID()
	st.currentIDSet = true
	if isPub {
		r := w.AppendResarray_Putpubfh()
		r.SetStatus(NFS4_OK)
	} else {
		r := w.AppendResarray_Putrootfh()
		r.SetStatus(NFS4_OK)
	}
	return NFS4_OK
}

func (s *Server) opRead(args READ4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Read()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Read()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	if !isSpecialStateID(args.Stateid()) {
		_, status := s.lookupDurableOpen(args.Stateid(), st.currentID)
		if status != NFS4_OK {
			ew := w.AppendResarray_Read()
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
	}

	// Check if we should read from a staging buffer.
	sf := s.stagingStore.Get(st.currentID)

	offset := args.Offset()
	count := min(args.Count(), maxReadWrite)

	buf := make([]byte, count)

	var n int
	var eof bool
	var err error

	if sf != nil {
		n, eof, err = sf.Read(offset, buf)
	} else {
		n, eof, err = s.fs.Read(st.currentID, offset, buf)
	}

	if err != nil {
		ew := w.AppendResarray_Read()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
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
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Readdir()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	cookie := args.Cookie()
	if cookie == 1 || cookie == 2 {
		ew := w.AppendResarray_Readdir()
		ew.SetValue_Default(NFS4ERR_BAD_COOKIE)
		w.Resume(ew.Finish())
		return NFS4ERR_BAD_COOKIE
	}
	maxCount := args.Maxcount()
	if maxCount > 1<<20 {
		maxCount = 1 << 20
	}

	// Validate cookieverf on continuation requests (RFC 7530 §16.24.4).
	// cookieverf = directory mtime; if it changed, the listing is stale.
	// Some clients (e.g. libnfs) send all-zero cookieverf on continuations
	// ("should" echo it back per RFC, not "MUST"), so only check non-zero.
	if cookie != 0 {
		var clientVerf [8]byte
		cv := args.Cookieverf()
		for i := range clientVerf {
			clientVerf[i] = cv.Data(i)
		}
		if clientVerf != [8]byte{} {
			ni, err := s.fs.Stat(st.currentID)
			if err != nil {
				ew := w.AppendResarray_Readdir()
				status := s.errToNFS(err)
				ew.SetValue_Default(status)
				w.Resume(ew.Finish())
				return status
			}
			var expectedVerf [8]byte
			binary.BigEndian.PutUint64(expectedVerf[:], uint64(ni.Mtime.UnixNano()))
			if clientVerf != expectedVerf {
				ew := w.AppendResarray_Readdir()
				ew.SetValue_Default(NFS4ERR_NOT_SAME)
				w.Resume(ew.Finish())
				return NFS4ERR_NOT_SAME
			}
		}
	}

	reqMask := parseBitmap(args.AttrRequest())
	if status := validateGetattrMask(reqMask); status != NFS4_OK {
		ew := w.AppendResarray_Readdir()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	dirCount := args.Dircount()
	if dirCount > 1<<20 {
		dirCount = 1 << 20
	}

	// Batch multiple VFS Readdir calls until we have enough entries
	// and NextHash >= 3 (avoiding NFS reserved cookie values 1 and 2).
	maxEntries := int(maxCount / 100)
	if maxEntries < 32 {
		maxEntries = 32
	}
	var allEntries []DirEntry
	startHash := cookie
	eof := false
	for {
		entries, nextHash, err := s.fs.Readdir(st.currentID, startHash)
		if err != nil {
			ew := w.AppendResarray_Readdir()
			status := s.errToNFS(err)
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		allEntries = append(allEntries, entries...)
		if nextHash == 0 {
			eof = true
			break
		}
		if nextHash >= 3 && len(allEntries) >= maxEntries {
			break
		}
		startHash = nextHash
	}

	// NFS cookie semantics: cookie=X means "I already have entries up to
	// and including X". Skip entries with NameHash <= cookie on continuations,
	// since VFS Readdir returns entries with hash >= startHash (inclusive).
	// Also hide the internal .nfs directory from client directory listings.
	{
		filtered := allEntries[:0]
		for _, e := range allEntries {
			if cookie != 0 && e.NameHash <= cookie {
				continue
			}
			if st.currentID == s.fs.RootID() && e.Name == nfsDirName {
				continue
			}
			filtered = append(filtered, e)
		}
		allEntries = filtered
	}

	// Build staged sizes map for entries being written.
	ss := stagedSizes(s.stagingStore.StagedSizes())

	// Pre-compute attributes for each entry so we can calculate exact
	// XDR sizes before encoding.
	prepared := prepareReaddirEntries(allEntries, reqMask, s.fs, ss)

	// Enforce maxcount and dircount with exact XDR sizes.
	// maxcount covers the entire READDIR4resok: cookieverf(8) +
	// entries_present(4) + entry chain + eof(4). The entry chain
	// ends with a terminal FALSE(4) (either entries_present=FALSE
	// when empty, or the last entry's nextentry=FALSE).
	// dircount limits directory-information bytes: cookie + name per entry.
	maxBudget := int(maxCount) - readdirResokOverhead
	dirBudget := int(dirCount)
	if maxBudget < 0 {
		ew := w.AppendResarray_Readdir()
		ew.SetValue_Default(NFS4ERR_TOOSMALL)
		w.Resume(ew.Finish())
		return NFS4ERR_TOOSMALL
	}
	n := 0
	predictedEntryBytes := 0
	predictedDirBytes := 0
	for i := range prepared {
		eSize := readdirEntryXDRSize(&prepared[i])
		dSize := readdirEntryDirSize(&prepared[i])
		if predictedEntryBytes+eSize > maxBudget || predictedDirBytes+dSize > dirBudget {
			if i == 0 {
				ew := w.AppendResarray_Readdir()
				ew.SetValue_Default(NFS4ERR_TOOSMALL)
				w.Resume(ew.Finish())
				return NFS4ERR_TOOSMALL
			}
			eof = false
			break
		}
		predictedEntryBytes += eSize
		predictedDirBytes += dSize
		n++
	}
	prepared = prepared[:n]

	ew := w.AppendResarray_Readdir()
	okW := ew.SetValue_Nfs4Ok()

	// Set cookieverf to directory mtime.
	ni, _ := s.fs.Stat(st.currentID)
	verf := okW.Cookieverf()
	var mtimeBytes [8]byte
	binary.BigEndian.PutUint64(mtimeBytes[:], uint64(ni.Mtime.UnixNano()))
	for i := 0; i < 8; i++ {
		verf.SetData(i, mtimeBytes[i])
	}

	dirW := okW.StartReply()
	encodeDirEntries(&dirW, prepared, eof)
	buf := dirW.Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opReadlink(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Readlink()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if st.currentID.Type() != InodeTypeSymlink {
		ew := w.AppendResarray_Readlink()
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	target, err := s.fs.Readlink(st.currentID)
	if err != nil {
		ew := w.AppendResarray_Readlink()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
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
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	nameData := args.Target().Data()
	if len(nameData) == 0 {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	if ternNameTooLong(nameData) {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(NFS4ERR_NAMETOOLONG)
		w.Resume(ew.Finish())
		return NFS4ERR_NAMETOOLONG
	}
	if ternNameUnsupported(nameData) {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(NFS4ERR_BADNAME)
		w.Resume(ew.Finish())
		return NFS4ERR_BADNAME
	}
	name := string(nameData)
	err := s.fs.Remove(st.currentID, name)
	if err != nil {
		ew := w.AppendResarray_Remove()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	ew := w.AppendResarray_Remove()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	cinfo.SetAtomic(TRUE)
	now := uint64(time.Now().UnixNano())
	cinfo.SetBefore(now - 1)
	cinfo.SetAfter(now)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opRename(args RENAME4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	// RENAME uses savedFH as source directory and currentFH as target directory.
	if !st.currentIDSet || !st.savedIDSet {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.savedID); status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	oldNameData := args.Oldname().Data()
	newNameData := args.Newname().Data()
	if len(oldNameData) == 0 || len(newNameData) == 0 {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	if ternNameTooLong(oldNameData) || ternNameTooLong(newNameData) {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_NAMETOOLONG)
		w.Resume(ew.Finish())
		return NFS4ERR_NAMETOOLONG
	}
	if ternNameUnsupported(oldNameData) || ternNameUnsupported(newNameData) {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_BADNAME)
		w.Resume(ew.Finish())
		return NFS4ERR_BADNAME
	}
	oldName := string(oldNameData)
	newName := string(newNameData)

	if st.savedID == st.currentID && oldName == newName {
		ew := w.AppendResarray_Rename()
		okW := ew.SetValue_Nfs4Ok()
		now := uint64(time.Now().UnixNano())
		srcInfo := okW.SourceCinfo()
		srcInfo.SetAtomic(TRUE)
		srcInfo.SetBefore(now)
		srcInfo.SetAfter(now)
		tgtInfo := okW.TargetCinfo()
		tgtInfo.SetAtomic(TRUE)
		tgtInfo.SetBefore(now)
		tgtInfo.SetAfter(now)
		w.Resume(ew.Finish())
		return NFS4_OK
	}

	err := s.fs.Rename(st.savedID, oldName, st.currentID, newName)
	if err != nil {
		ew := w.AppendResarray_Rename()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	ew := w.AppendResarray_Rename()
	okW := ew.SetValue_Nfs4Ok()
	now := uint64(time.Now().UnixNano())
	srcInfo := okW.SourceCinfo()
	srcInfo.SetAtomic(TRUE)
	srcInfo.SetBefore(now - 1)
	srcInfo.SetAfter(now)
	tgtInfo := okW.TargetCinfo()
	tgtInfo.SetAtomic(TRUE)
	tgtInfo.SetBefore(now - 1)
	tgtInfo.SetAfter(now)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opRenew(args RENEW4args, w *COMPOUND4resWriter) uint32 {
	r := w.AppendResarray_Renew()
	if err := s.clients.Renew(args.Clientid()); err != nil {
		status := nfsErrCode(err)
		r.SetStatus(status)
		return status
	}
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opSavefh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Savefh()
		r.SetStatus(NFS4ERR_NOFILEHANDLE)
		return NFS4ERR_NOFILEHANDLE
	}
	st.savedID = st.currentID
	st.savedIDSet = true
	r := w.AppendResarray_Savefh()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opRestorefh(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.savedIDSet {
		r := w.AppendResarray_Restorefh()
		r.SetStatus(NFS4ERR_RESTOREFH)
		return NFS4ERR_RESTOREFH
	}
	st.currentID = st.savedID
	st.currentIDSet = true
	r := w.AppendResarray_Restorefh()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opSecinfo(args SECINFO4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireDirectory(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	nameData := args.Name().Data()
	if len(nameData) == 0 {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_INVAL)
		w.Resume(ew.Finish())
		return NFS4ERR_INVAL
	}
	if ternNameTooLong(nameData) {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_NAMETOOLONG)
		w.Resume(ew.Finish())
		return NFS4ERR_NAMETOOLONG
	}
	if ternNameUnsupported(nameData) {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_BADNAME)
		w.Resume(ew.Finish())
		return NFS4ERR_BADNAME
	}
	name := string(nameData)
	if name == nfsDirName && st.currentID == s.fs.RootID() {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_NOENT)
		w.Resume(ew.Finish())
		return NFS4ERR_NOENT
	}
	if _, err := s.fs.Lookup(st.currentID, name); err != nil {
		ew := w.AppendResarray_Secinfo()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}
	// Return AUTH_SYS (flavor 1) and AUTH_NONE (flavor 0).
	ew := w.AppendResarray_Secinfo()
	okW := ew.SetValue_Nfs4Ok()
	okW.AppendData_Default(authSys)
	okW.AppendData_Default(authNone)
	buf := okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opSetattr(args SETATTR4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	// setattrReply writes a SETATTR response with the given status and
	// optional result bitmap. Factored out because the response always
	// requires an attrsset bitmap, even on error.
	setattrReply := func(status uint32, resultMask [2]uint32) uint32 {
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
		return setattrReply(NFS4ERR_NOFILEHANDLE, [2]uint32{})
	}

	fa := args.ObjAttributes()
	mask := parseBitmap(fa.Attrmask())
	attrData := fa.AttrVals().Data()

	// Supported writable attrs.
	const supportedSet0 = 1 << FATTR4_SIZE
	const supportedSet1 = (1 << (FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (FATTR4_TIME_MODIFY_SET - 32))

	// Validate fixed-width MODE data before reporting that MODE itself is not
	// supported. RFC 7530 requires malformed attribute XDR to take precedence.
	const modeMask = 1 << (FATTR4_MODE - 32)
	if mask[0] == 0 && mask[1] == modeMask && len(attrData) != 4 {
		return setattrReply(NFS4ERR_BADXDR, [2]uint32{})
	}

	if mask[0]&^writableAttrs0 != 0 || mask[1]&^writableAttrs1 != 0 {
		return setattrReply(NFS4ERR_INVAL, [2]uint32{})
	}
	if mask[0]&^uint32(supportedSet0) != 0 ||
		mask[1]&^uint32(supportedSet1) != 0 {
		return setattrReply(NFS4ERR_ATTRNOTSUPP, [2]uint32{})
	}

	var resultMask [2]uint32
	var newSize *uint64
	attrOff := 0
	if mask[0]&(1<<FATTR4_SIZE) != 0 {
		if attrOff+8 > len(attrData) {
			return setattrReply(NFS4ERR_BADXDR, [2]uint32{})
		}
		size := binary.BigEndian.Uint64(attrData[attrOff : attrOff+8])
		attrOff += 8
		if size > 1<<63-1 {
			return setattrReply(NFS4ERR_FBIG, [2]uint32{})
		}
		newSize = &size
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
			return setattrReply(status, [2]uint32{})
		}
		setAtime = t
	}
	if mask[1]&(1<<(FATTR4_TIME_MODIFY_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != NFS4_OK {
			return setattrReply(status, [2]uint32{})
		}
		setMtime = t
	}

	if attrOff != len(attrData) {
		return setattrReply(NFS4ERR_BADXDR, [2]uint32{})
	}

	if newSize != nil {
		if !isSpecialStateID(args.Stateid()) {
			state, status := s.lookupDurableOpen(args.Stateid(), st.currentID)
			if status != NFS4_OK {
				return setattrReply(status, [2]uint32{})
			}
			if !state.write {
				return setattrReply(NFS4ERR_OPENMODE, [2]uint32{})
			}
		}
		sf := s.stagingStore.Get(st.currentID)
		if sf == nil {
			return setattrReply(NFS4ERR_BAD_STATEID, [2]uint32{})
		}
		if err := sf.SetSize(*newSize); err != nil {
			return setattrReply(NFS4ERR_IO, [2]uint32{})
		}
		resultMask[0] |= 1 << FATTR4_SIZE
	}
	if setAtime != nil || setMtime != nil {
		if err := s.fs.SetTime(st.currentID, setMtime, setAtime); err != nil {
			return setattrReply(NFS4ERR_IO, [2]uint32{})
		}
		if setAtime != nil {
			resultMask[1] |= 1 << (FATTR4_TIME_ACCESS_SET - 32)
		}
		if setMtime != nil {
			resultMask[1] |= 1 << (FATTR4_TIME_MODIFY_SET - 32)
		}
	}

	return setattrReply(NFS4_OK, resultMask)
}

func (s *Server) opSetclientid(
	args SETCLIENTID4args,
	st *compoundState,
	w *COMPOUND4resWriter,
) uint32 {
	clientID := args.Client()
	verifier := clientID.Verifier()
	idData := clientID.Id()
	location := args.Callback().CbLocation()
	owner := clientOwner{
		principal: st.principal,
		netid:     string(location.RNetid().Data()),
		addr:      string(location.RAddr().Data()),
	}

	var verf [8]byte
	for i := 0; i < 8; i++ {
		verf[i] = verifier.Data(i)
	}

	clid, confirm, err := s.clients.SetClientID(verf, idData, owner)
	if err != nil {
		ew := w.AppendResarray_Setclientid()
		var inUse clientInUseError
		if errors.As(err, &inUse) {
			addrW := ew.SetValue_Nfs4errClidInuse()
			netidW := addrW.StartRNetid()
			buf := netidW.SetData([]byte(inUse.owner.netid)).Finish()
			addrW.Resume(buf)
			rAddrW := addrW.StartRAddr()
			buf = rAddrW.SetData([]byte(inUse.owner.addr)).Finish()
			addrW.Resume(buf)
			buf = addrW.Finish()
			ew.Resume(buf)
			w.Resume(ew.Finish())
			return NFS4ERR_CLID_INUSE
		}
		ew.SetValue_Default(nfsErrCode(err))
		w.Resume(ew.Finish())
		return nfsErrCode(err)
	}

	ew := w.AppendResarray_Setclientid()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetClientid(clid)
	confirmWriter := ok.SetclientidConfirm()
	for i, b := range confirm {
		confirmWriter.SetData(i, b)
	}
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opSetclientidConfirm(args SETCLIENTIDCONFIRM4args, w *COMPOUND4resWriter) uint32 {
	clid := args.Clientid()
	var confirm [8]byte
	confirmReader := args.SetclientidConfirm()
	for i := range confirm {
		confirm[i] = confirmReader.Data(i)
	}
	replacedClientID, err := s.clients.ConfirmClientID(clid, confirm)
	if err != nil {
		r := w.AppendResarray_SetclientidConfirm()
		r.SetStatus(nfsErrCode(err))
		return nfsErrCode(err)
	}
	if replacedClientID != 0 {
		for _, fileID := range s.opens.purgeClient(replacedClientID) {
			s.stagingStore.Remove(fileID)
		}
	}
	r := w.AppendResarray_SetclientidConfirm()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opVerify(args VERIFY4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Verify()
		r.SetStatus(NFS4ERR_NOFILEHANDLE)
		return NFS4ERR_NOFILEHANDLE
	}
	same, status := s.verifyAttrs(st.currentID, args.ObjAttributes())
	if status != NFS4_OK {
		r := w.AppendResarray_Verify()
		r.SetStatus(status)
		return status
	}
	r := w.AppendResarray_Verify()
	if !same {
		// VERIFY: fail if attributes differ.
		r.SetStatus(NFS4ERR_NOT_SAME)
		return NFS4ERR_NOT_SAME
	}
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

func (s *Server) opWrite(args WRITE4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Write()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	if status := requireRegularFile(st.currentID); status != NFS4_OK {
		ew := w.AppendResarray_Write()
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	if !isSpecialStateID(args.Stateid()) {
		state, status := s.lookupDurableOpen(args.Stateid(), st.currentID)
		if status != NFS4_OK {
			ew := w.AppendResarray_Write()
			ew.SetValue_Default(status)
			w.Resume(ew.Finish())
			return status
		}
		if !state.write {
			ew := w.AppendResarray_Write()
			ew.SetValue_Default(NFS4ERR_OPENMODE)
			w.Resume(ew.Finish())
			return NFS4ERR_OPENMODE
		}
	}

	// Find the staging buffer for this file.
	sf := s.stagingStore.Get(st.currentID)
	if sf == nil {
		// No staging buffer — not opened for write.
		ew := w.AppendResarray_Write()
		ew.SetValue_Default(NFS4ERR_OPENMODE)
		w.Resume(ew.Finish())
		return NFS4ERR_OPENMODE
	}

	offset := args.Offset()
	data := args.Data()

	if err := sf.Write(offset, data); err != nil {
		ew := w.AppendResarray_Write()
		ew.SetValue_Default(NFS4ERR_IO)
		w.Resume(ew.Finish())
		return NFS4ERR_IO
	}

	ew := w.AppendResarray_Write()
	okW := ew.SetValue_Nfs4Ok()
	okW.SetCount(uint32(len(data)))
	okW.SetCommitted(unstable4)
	verf := okW.Writeverf()
	for i := 0; i < 8; i++ {
		verf.SetData(i, s.writeVerifier[i])
	}
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opReleaseLockowner(w *COMPOUND4resWriter) uint32 {
	// No lock state — always succeeds.
	r := w.AppendResarray_ReleaseLockowner()
	r.SetStatus(NFS4_OK)
	return NFS4_OK
}

// verifyAttrs compares the supplied fattr4 against the current file's attributes.
// Returns (same bool, status uint32). If status != NFS4_OK, comparison failed.
func (s *Server) verifyAttrs(id InodeID, supplied Fattr4) (bool, uint32) {
	ni, err := s.fs.Stat(id)
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

// writeStateID writes a StateID into a Stateid4's "other" field.
func writeStateID(s Stateid4, sid StateID) {
	for i := 0; i < 12; i++ {
		s.SetOther(i, sid[i])
	}
}

// errToNFS converts a Go error to an NFS status code.
func (s *Server) errToNFS(err error) uint32 {
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
