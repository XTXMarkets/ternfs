// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"time"
)

// Stable write modes.
const (
	unstable4 = 0
	dataSync4 = 1
	fileSync4 = 2
)

func (s *Server) opAccess(args ACCESS4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Access()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}
	requested := args.Access()
	ew := w.AppendResarray_Access()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetSupported(requested)
	ok.SetAccess(requested) // grant everything requested
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

	// Check if there's a staging file for the current filehandle.
	meta, hasMeta := s.stagingStore.GetMeta(st.currentID)
	if hasMeta {
		// First CLOSE for a write-open: link the transient file.
		if sid != meta.NFSStateID {
			ew := w.AppendResarray_Close()
			ew.SetValue_Default(NFS4ERR_BAD_STATEID)
			w.Resume(ew.Finish())
			return NFS4ERR_BAD_STATEID
		}
		sf := s.stagingStore.Get(st.currentID)
		if sf != nil {
			r, rErr := sf.Reader()
			if rErr == nil {
				_ = s.fs.LinkFile(st.currentID, meta.TernCookie, meta.DirID, meta.FileName, r)
			}
		}
		s.stagingStore.Remove(st.currentID)
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

	ew := w.AppendResarray_Close()
	stid := ew.SetValue_Nfs4Ok()
	stid.SetSeqid(args.OpenStateid().Seqid() + 1)
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
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	name := string(args.Objname().Data())
	objType := args.ObjtypeType()

	var newID InodeID
	var err error

	switch objType {
	case NF4DIR:
		newID, err = s.fs.Mkdir(st.currentID, name)
	case NF4LNK:
		target := string(args.Objtype().AsLinktext4().Data())
		newID, err = s.fs.Symlink(st.currentID, name, target)
	default:
		// We don't support creating block/char/socket/fifo devices.
		ew := w.AppendResarray_Create()
		ew.SetValue_Default(NFS4ERR_NOTSUPP)
		w.Resume(ew.Finish())
		return NFS4ERR_NOTSUPP
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
		ew := w.AppendResarray_Getattr()
		status := s.errToNFS(err)
		ew.SetValue_Default(status)
		w.Resume(ew.Finish())
		return status
	}

	// If the file has an active staging buffer, use its size.
	if sz, ok := s.stagingStore.StagedSize(st.currentID); ok {
		ni.Size = sz
	}

	reqMask := parseBitmap(args.AttrRequest())

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
	name := string(args.Objname().Data())
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

	var targetID InodeID
	var nfsSID StateID
	created := false

	switch claimType {
	case CLAIM_NULL:
		dirID := st.currentID
		fileName := string(claim.AsNull().Data())

		// Check if this is a create.
		if args.OpenhowType() == OPEN4_CREATE {
			// EXCLUSIVE4 not supported — clients should fall back
			// to GUARDED4 or UNCHECKED4.
			if args.Openhow().AsCreatehow4Entry().Disc() == EXCLUSIVE4 {
				ew := w.AppendResarray_Open()
				ew.SetValue_Default(NFS4ERR_NOTSUPP)
				w.Resume(ew.Finish())
				return NFS4ERR_NOTSUPP
			}
			// Try lookup first.
			id, err := s.fs.Lookup(dirID, fileName)
			if err != nil {
				// File doesn't exist — construct a transient file.
				var fileCookie Cookie
				id, fileCookie, err = s.fs.ConstructFile()
				if err != nil {
					ew := w.AppendResarray_Open()
					ew.SetValue_Default(s.errToNFS(err))
					w.Resume(ew.Finish())
					return s.errToNFS(err)
				}
				// Generate a random NFS stateid and create staging with metadata.
				nfsSID = newNFSStateID()
				meta := StagingMeta{
					DirID:      dirID,
					FileName:   fileName,
					TernCookie: fileCookie,
					NFSStateID: nfsSID,
				}
				if _, sfErr := s.stagingStore.Create(id, meta); sfErr != nil {
					s.log.Error("staging create error", "err", sfErr)
					ew := w.AppendResarray_Open()
					ew.SetValue_Default(NFS4ERR_IO)
					w.Resume(ew.Finish())
					return NFS4ERR_IO
				}
				created = true
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

	// For read opens, derive a deterministic stateid from the file and client.
	if !created {
		nfsSID = deriveReadStateID(targetID, clientID)
	}

	st.currentID = targetID
	st.currentIDSet = true

	ew := w.AppendResarray_Open()
	okW := ew.SetValue_Nfs4Ok()

	stid := okW.Stateid()
	stid.SetSeqid(1)
	writeStateID(stid, nfsSID)

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

// deriveReadStateID produces a deterministic stateid for read opens.
// The stateid encodes the InodeID and a hash of the client ID, so any
// server instance can recompute it without persistent state.
func deriveReadStateID(fileID InodeID, clientID uint64) StateID {
	var sid StateID
	binary.BigEndian.PutUint64(sid[0:8], uint64(fileID))
	// Mix in the client ID for basic validation.
	binary.BigEndian.PutUint32(sid[8:12], uint32(clientID^(clientID>>32)))
	return sid
}

func (s *Server) opOpenConfirm(args OPENCONFIRM4args, st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_OpenConfirm()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
	}

	sid := extractStateID(args.OpenStateid())
	seqid := args.OpenStateid().Seqid()

	// No persistent open state to confirm — just echo the stateid
	// with an incremented seqid.
	ew := w.AppendResarray_OpenConfirm()
	ok := ew.SetValue_Nfs4Ok()
	stid := ok.OpenStateid()
	stid.SetSeqid(seqid + 1)
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

	// Check if we should read from a staging buffer.
	sf := s.stagingStore.Get(st.currentID)

	offset := args.Offset()
	count := args.Count()
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

	cookie := args.Cookie()
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
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Remove()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	name := string(args.Target().Data())
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
	if s.stagingStore.ReadOnly() {
		ew := w.AppendResarray_Rename()
		ew.SetValue_Default(NFS4ERR_ROFS)
		w.Resume(ew.Finish())
		return NFS4ERR_ROFS
	}

	oldName := string(args.Oldname().Data())
	newName := string(args.Newname().Data())

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
	// No lease state — just accept the renewal.
	_ = args.Clientid()
	r := w.AppendResarray_Renew()
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

func (s *Server) opSecinfo(st *compoundState, w *COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Secinfo()
		ew.SetValue_Default(NFS4ERR_NOFILEHANDLE)
		w.Resume(ew.Finish())
		return NFS4ERR_NOFILEHANDLE
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

	// Supported writable attrs.
	const supportedSet0 = 1 << FATTR4_SIZE
	const supportedSet1 = (1 << (FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (FATTR4_TIME_MODIFY_SET - 32))

	if mask[0] & ^uint32(supportedSet0) != 0 || mask[1] & ^uint32(supportedSet1) != 0 {
		return setattrReply(NFS4ERR_ATTRNOTSUPP, [2]uint32{})
	}

	var resultMask [2]uint32

	if mask[0]&(1<<FATTR4_SIZE) != 0 {
		attrData := fa.AttrVals().Data()
		if len(attrData) < 8 {
			return setattrReply(NFS4ERR_BADXDR, [2]uint32{})
		}
		newSize := binary.BigEndian.Uint64(attrData[0:8])

		sf := s.stagingStore.Get(st.currentID)
		if sf == nil {
			return setattrReply(NFS4ERR_BAD_STATEID, [2]uint32{})
		}

		if err := sf.SetSize(newSize); err != nil {
			return setattrReply(NFS4ERR_IO, [2]uint32{})
		}
		resultMask[0] |= 1 << FATTR4_SIZE
	}

	// Parse time values from the attribute data (after SIZE if present).
	attrOff := 0
	if mask[0]&(1<<FATTR4_SIZE) != 0 {
		attrOff = 8
	}
	attrData := fa.AttrVals().Data()

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
			t := time.Unix(sec, int64(nsec))
			return &t, NFS4_OK
		}
		t := time.Now()
		return &t, NFS4_OK
	}

	var setAtime, setMtime *time.Time
	if mask[1]&(1<<(FATTR4_TIME_ACCESS_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != NFS4_OK {
			return setattrReply(status, [2]uint32{})
		}
		setAtime = t
		resultMask[1] |= 1 << (FATTR4_TIME_ACCESS_SET - 32)
	}
	if mask[1]&(1<<(FATTR4_TIME_MODIFY_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != NFS4_OK {
			return setattrReply(status, [2]uint32{})
		}
		setMtime = t
		resultMask[1] |= 1 << (FATTR4_TIME_MODIFY_SET - 32)
	}

	if setAtime != nil || setMtime != nil {
		if err := s.fs.SetTime(st.currentID, setMtime, setAtime); err != nil {
			return setattrReply(NFS4ERR_IO, [2]uint32{})
		}
	}

	return setattrReply(NFS4_OK, resultMask)
}

func (s *Server) opSetclientid(args SETCLIENTID4args, w *COMPOUND4resWriter) uint32 {
	clientID := args.Client()
	verifier := clientID.Verifier()
	idData := clientID.Id()

	var verf [8]byte
	for i := 0; i < 8; i++ {
		verf[i] = verifier.Data(i)
	}

	clid, err := s.clients.SetClientID(verf, idData)
	if err != nil {
		ew := w.AppendResarray_Setclientid()
		ew.SetValue_Default(nfsErrCode(err))
		w.Resume(ew.Finish())
		return nfsErrCode(err)
	}

	ew := w.AppendResarray_Setclientid()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetClientid(clid)
	w.Resume(ew.Finish())
	return NFS4_OK
}

func (s *Server) opSetclientidConfirm(args SETCLIENTIDCONFIRM4args, w *COMPOUND4resWriter) uint32 {
	clid := args.Clientid()
	_, err := s.clients.ConfirmClientID(clid)
	if err != nil {
		r := w.AppendResarray_SetclientidConfirm()
		r.SetStatus(nfsErrCode(err))
		return nfsErrCode(err)
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

	// Only compare attributes we support.
	mask[0] &= supportedAttrs0
	mask[1] &= supportedAttrs1

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
