// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"os"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func (s *Server) opCreate(args nfs4.CREATE4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if st.currentID.Type() != vfs.InodeTypeDir {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOTDIR)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_ROFS)
	}

	name, status := toTernFSName(args.Objname().Data())
	if status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	objType := args.ObjtypeType()
	switch objType {
	case nfs4.NF4DIR, nfs4.NF4LNK:
	case nfs4.NF4REG:
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_BADTYPE)
	default:
		// We don't support creating block/char/socket/fifo devices.
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOTSUPP)
	}
	if status := validateCreateAttrs(args.Createattrs()); status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	unlock := s.lockMutationTargets(mutationTarget{
		dirID: st.currentID,
		name:  name,
	})
	defer unlock()
	if verdict, status := s.decideCreateReplay(st, st.currentID, name); verdict != replayNone {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	busy, err := s.stagingTargetBusy(st.currentID, name)
	if err != nil {
		ew := w.AppendResarray_Create()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if busy {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_FILE_OPEN)
	}
	if _, err := s.fs.Lookup(st.currentID, name); err == nil {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_EXIST)
	} else if status := s.errToNFS(err); status != nfs4.NFS4ERR_NOENT {
		ew := w.AppendResarray_Create()
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	var newID vfs.InodeID

	switch objType {
	case nfs4.NF4DIR:
		if status := s.persistIntent(st, createIntent{dir: st.currentID, name: name, objType: nfs4.NF4DIR}.encode(st.opIndex)); status != nfs4.NFS4_OK {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), status)
		}
		newID, err = s.fs.Mkdir(st.currentID, name)
	case nfs4.NF4LNK:
		targetData := args.Objtype().AsLinktext4().Data()
		if len(targetData) == 0 {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_INVAL)
		}
		if len(targetData) > maxTernBytesLength {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NAMETOOLONG)
		}
		target := string(targetData)
		if status := s.persistIntent(st, createIntent{dir: st.currentID, name: name, objType: nfs4.NF4LNK, target: target}.encode(st.opIndex)); status != nfs4.NFS4_OK {
			ew := w.AppendResarray_Create()
			return finishDefaultResponse(w, ew.Finish(), status)
		}
		newID, err = s.fs.Symlink(st.currentID, name, target)
	}

	if err != nil {
		st.observeMutation(s.fs, nfs4.OP_CREATE, err)
		ew := w.AppendResarray_Create()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	return writeCreateOK(w, st, newID)
}

// writeCreateOK finishes a successful CREATE of newID.
func writeCreateOK(w *nfs4.COMPOUND4resWriter, st *compoundState, newID vfs.InodeID) uint32 {
	st.setCurrentFH(newID)

	ew := w.AppendResarray_Create()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(cinfo, nfs4.TRUE, now-1, now)

	// attrset bitmap: empty (we don't apply createattrs).
	bmW := okW.StartAttrset()
	buf := bmW.Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opGetfh(st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getfh()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	ew := w.AppendResarray_Getfh()
	okW := ew.SetValue_Nfs4Ok()
	fhW := okW.StartObject()
	buf := fhW.SetData(vfs.InodeIDToFH(st.currentID)).Finish()
	okW.Resume(buf)
	buf = okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opLookup(args nfs4.LOOKUP4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != nfs4.NFS4_OK {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, status)
	}
	name, status := toTernFSName(args.Objname().Data())
	if status != nfs4.NFS4_OK {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, status)
	}
	// Hide the internal .nfs directory from client access.
	if name == vfs.NFSDirName && st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookup()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOENT)
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
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opLookupp(st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != nfs4.NFS4_OK {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, status)
	}
	if st.currentID == s.fs.RootID() {
		r := w.AppendResarray_Lookupp()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOENT)
	}
	id, err := s.fs.LookupParent(st.currentID)
	if err != nil {
		r := w.AppendResarray_Lookupp()
		status := s.errToNFS(err)
		return finishStatusResponse(&r, status)
	}
	st.setCurrentFH(id)
	r := w.AppendResarray_Lookupp()
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opPutfh(args nfs4.PUTFH4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	fhData := args.Object().Data()
	id, ok := vfs.FHToInodeID(fhData)
	if !ok {
		r := w.AppendResarray_Putfh()
		return finishStatusResponse(&r, nfs4.NFS4ERR_BADHANDLE)
	}
	internal, err := s.clients.IsInternalFilehandle(id)
	if err != nil {
		r := w.AppendResarray_Putfh()
		status := clientstore.ErrToNFS(err)
		return finishStatusResponse(&r, status)
	}
	if internal {
		r := w.AppendResarray_Putfh()
		return finishStatusResponse(&r, nfs4.NFS4ERR_BADHANDLE)
	}
	st.setCurrentFH(id)
	r := w.AppendResarray_Putfh()
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opPutrootfh(st *compoundState, w *nfs4.COMPOUND4resWriter, isPub bool) uint32 {
	st.setCurrentFH(s.fs.RootID())
	if isPub {
		r := w.AppendResarray_Putpubfh()
		return finishStatusResponse(&r, nfs4.NFS4_OK)
	} else {
		r := w.AppendResarray_Putrootfh()
		return finishStatusResponse(&r, nfs4.NFS4_OK)
	}
}

func (s *Server) opRemove(args nfs4.REMOVE4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Remove()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_ROFS)
	}

	name, status := toTernFSName(args.Target().Data())
	if status != nfs4.NFS4_OK {
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
	switch verdict, status := s.decideRemoveReplay(st, st.currentID, name); verdict {
	case replayApplied:
		return writeRemoveOK(w)
	case replayUndecidable:
		return fail(status)
	}
	if _, err := s.retireInactiveStagingTarget(st.currentID, name); err != nil {
		return fail(s.errToNFS(err))
	}
	writers := s.namespaceWriters(st.currentID, name, nil)
	edge, err := s.fs.LookupEdge(st.currentID, name)
	if err != nil {
		return fail(s.errToNFS(err))
	}
	// The intent goes before the writers' guard, so that a REMOVE refused
	// here has nothing to undo; one whose guard then fails left an intent
	// that a replay finds not applied.
	if status := s.persistIntent(st, removeIntent{dir: st.currentID, name: name, edge: edge}.encode(st.opIndex)); status != nfs4.NFS4_OK {
		return fail(status)
	}
	if err := s.guardNamespaceWriters(writers); err != nil {
		s.log.Warn("cannot guard REMOVE writers", "err", err)
		return fail(nfs4.NFS4ERR_IO)
	}
	err = s.fs.RemoveEdge(st.currentID, name, edge)
	st.observeMutation(s.fs, nfs4.OP_REMOVE, err)
	if status := s.finishNamespaceWriters(writers, nfs4.OP_REMOVE, err); status != nfs4.NFS4_OK {
		return fail(status)
	}
	return writeRemoveOK(w)
}

// writeRemoveOK finishes a successful REMOVE.
func writeRemoveOK(w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_Remove()
	okW := ew.SetValue_Nfs4Ok()
	cinfo := okW.Cinfo()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(cinfo, nfs4.TRUE, now-1, now)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opRename(args nfs4.RENAME4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	// RENAME uses savedFH as source directory and currentFH as target directory.
	if !st.currentIDSet || !st.savedIDSet {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.savedID); status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if status := requireDirectory(st.currentID); status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_ROFS)
	}

	oldName, status := toTernFSName(args.Oldname().Data())
	if status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	newName, status := toTernFSName(args.Newname().Data())
	if status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Rename()
		return finishDefaultResponse(w, ew.Finish(), status)
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
	// A replay is decided first, also when the names now resolve to one:
	// the RENAME the intent records may have crossed directories.
	switch verdict, status := s.decideRenameReplay(st, st.savedID, oldName, st.currentID, newName); verdict {
	case replayApplied:
		return writeRenameOK(w)
	case replayUndecidable:
		return fail(status)
	}
	if st.savedID == st.currentID && oldName == newName {
		ew := w.AppendResarray_Rename()
		okW := ew.SetValue_Nfs4Ok()
		now := uint64(time.Now().UnixNano())
		setChangeInfo(okW.SourceCinfo(), nfs4.TRUE, now, now)
		setChangeInfo(okW.TargetCinfo(), nfs4.TRUE, now, now)
		w.Resume(ew.Finish())
		return nfs4.NFS4_OK
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
		return fail(nfs4.NFS4ERR_FILE_OPEN)
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
	if err == nil && (edge.ID.Type() == vfs.InodeTypeDir) != (dst.Type() == vfs.InodeTypeDir) {
		return fail(nfs4.NFS4ERR_EXIST)
	}
	// The intent goes before the writers' guard, as for REMOVE.
	if status := s.persistIntent(st, renameIntent{srcDir: st.savedID, srcName: oldName, src: edge,
		dstDir: st.currentID, dstName: newName, dst: dst}.encode(st.opIndex)); status != nfs4.NFS4_OK {
		return fail(status)
	}
	// Preflight cannot freeze the destination. A later type race is a
	// submitted failure and can quarantine writers on the remote backend.
	if err := s.guardNamespaceWriters(writers); err != nil {
		s.log.Warn("cannot guard RENAME writers", "err", err)
		return fail(nfs4.NFS4ERR_IO)
	}
	err = s.fs.RenameEdge(st.savedID, oldName, edge, st.currentID, newName)
	st.observeMutation(s.fs, nfs4.OP_RENAME, err)
	if status := s.finishNamespaceWriters(writers, nfs4.OP_RENAME, err); status != nfs4.NFS4_OK {
		return fail(status)
	}
	return writeRenameOK(w)
}

// writeRenameOK finishes a successful RENAME.
func writeRenameOK(w *nfs4.COMPOUND4resWriter) uint32 {
	ew := w.AppendResarray_Rename()
	okW := ew.SetValue_Nfs4Ok()
	now := uint64(time.Now().UnixNano())
	setChangeInfo(okW.SourceCinfo(), nfs4.TRUE, now-1, now)
	setChangeInfo(okW.TargetCinfo(), nfs4.TRUE, now-1, now)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opSavefh(st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Savefh()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	st.savedID = st.currentID
	st.savedIDSet = true
	st.savedStateID, st.savedGeneration, st.savedStateIDSet =
		st.currentStateID, st.currentGeneration, st.currentStateIDSet
	r := w.AppendResarray_Savefh()
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opRestorefh(st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.savedIDSet {
		r := w.AppendResarray_Restorefh()
		return finishStatusResponse(&r, nfs4.NFS4ERR_RESTOREFH)
	}
	st.setCurrentFH(st.savedID)
	st.currentStateID, st.currentGeneration, st.currentStateIDSet =
		st.savedStateID, st.savedGeneration, st.savedStateIDSet
	r := w.AppendResarray_Restorefh()
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opSecinfo(args nfs4.SECINFO4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	if status := requireDirectory(st.currentID); status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	name, status := toTernFSName(args.Name().Data())
	if status != nfs4.NFS4_OK {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	if name == vfs.NFSDirName && st.currentID == s.fs.RootID() {
		ew := w.AppendResarray_Secinfo()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOENT)
	}
	if _, err := s.fs.Lookup(st.currentID, name); err != nil {
		ew := w.AppendResarray_Secinfo()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}
	// Return AUTH_SYS (flavor 1) and AUTH_NONE (flavor 0).
	ew := w.AppendResarray_Secinfo()
	okW := ew.SetValue_Nfs4Ok()
	okW.AppendData_Default(nfs4.AUTH_SYS)
	okW.AppendData_Default(nfs4.AUTH_NONE)
	buf := okW.Finish()
	ew.Resume(buf)
	w.Resume(ew.Finish())
	// SECINFO consumes the current filehandle (RFC 8881 section 2.6.3.1.1.8).
	st.currentIDSet = false
	return nfs4.NFS4_OK
}
