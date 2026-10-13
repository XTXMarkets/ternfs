// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"errors"
)

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
