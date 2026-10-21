// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func (s *Server) opAccess(args nfs4.ACCESS4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Access()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	meaningful := uint32(nfs4.ACCESS4_READ | nfs4.ACCESS4_MODIFY | nfs4.ACCESS4_EXTEND)
	if st.currentID.Type() == vfs.InodeTypeDir {
		meaningful |= nfs4.ACCESS4_LOOKUP | nfs4.ACCESS4_DELETE
	} else {
		meaningful |= nfs4.ACCESS4_EXECUTE
	}
	requested := args.Access() & meaningful
	ew := w.AppendResarray_Access()
	ok := ew.SetValue_Nfs4Ok()
	ok.SetSupported(requested)
	ok.SetAccess(requested)
	w.Resume(ew.Finish())
	return nfs4.NFS4_OK
}

func (s *Server) opGetattr(args nfs4.GETATTR4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		ew := w.AppendResarray_Getattr()
		return finishDefaultResponse(w, ew.Finish(), nfs4.NFS4ERR_NOFILEHANDLE)
	}
	ni, err := s.stat(s.clientView(st.clientID, st.currentID))
	if err != nil {
		ew := w.AppendResarray_Getattr()
		status := s.errToNFS(err)
		return finishDefaultResponse(w, ew.Finish(), status)
	}

	reqMask := parseBitmap(args.AttrRequest())
	if status := validateGetattrMask(reqMask); status != nfs4.NFS4_OK {
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

	return nfs4.NFS4_OK
}

func (s *Server) opNverify(args nfs4.NVERIFY4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Nverify()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	same, status := s.verifyAttrs(st.currentID, s.clientView(st.clientID, st.currentID), args.ObjAttributes())
	if status != nfs4.NFS4_OK {
		r := w.AppendResarray_Nverify()
		return finishStatusResponse(&r, status)
	}
	r := w.AppendResarray_Nverify()
	if same {
		// NVERIFY: fail if attributes are the same.
		return finishStatusResponse(&r, nfs4.NFS4ERR_SAME)
	}
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

func (s *Server) opSetattr(args nfs4.SETATTR4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
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
		return setattrReply(nfs4.NFS4ERR_NOFILEHANDLE, attrMask{})
	}

	fa := args.ObjAttributes()
	mask := parseBitmap(fa.Attrmask())
	attrData := fa.AttrVals().Data()

	// Supported writable attrs.
	const supportedSet0 = 1 << nfs4.FATTR4_SIZE
	supportedSet1 := uint32((1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32)) |
		(1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32)))

	// Validate fixed-width MODE data before reporting that MODE itself is not
	// supported. RFC 7530 requires malformed attribute XDR to take precedence.
	const modeMask = 1 << (nfs4.FATTR4_MODE - 32)
	if mask[0] == 0 && mask[1] == modeMask && mask[2] == 0 && len(attrData) != 4 {
		return setattrReply(nfs4.NFS4ERR_BADXDR, attrMask{})
	}
	// EXCLUSIVE4 clients send mode in a follow-up SETATTR. Modes are synthetic.
	if writer := s.clientWriter(st.clientID, st.currentID); s.directStaging(writer) != nil ||
		s.stagingStore.Failed(writer) {
		supportedSet1 |= modeMask
	}

	if mask[0]&^writableAttrs0 != 0 || mask[1]&^writableAttrs1 != 0 || mask[2] != 0 {
		return setattrReply(nfs4.NFS4ERR_INVAL, attrMask{})
	}
	if mask[0]&^uint32(supportedSet0) != 0 ||
		mask[1]&^uint32(supportedSet1) != 0 {
		return setattrReply(nfs4.NFS4ERR_ATTRNOTSUPP, attrMask{})
	}

	var resultMask attrMask
	var newSize *uint64
	attrOff := 0
	if mask[0]&(1<<nfs4.FATTR4_SIZE) != 0 {
		if attrOff+8 > len(attrData) {
			return setattrReply(nfs4.NFS4ERR_BADXDR, attrMask{})
		}
		size := binary.BigEndian.Uint64(attrData[attrOff : attrOff+8])
		attrOff += 8
		if size > 1<<63-1 {
			return setattrReply(nfs4.NFS4ERR_FBIG, attrMask{})
		}
		newSize = &size
	}
	if mask[1]&modeMask != 0 {
		if attrOff+4 > len(attrData) {
			return setattrReply(nfs4.NFS4ERR_BADXDR, attrMask{})
		}
		attrOff += 4
		resultMask[1] |= modeMask
	}

	// parseTimeSet reads a SET_TO_CLIENT_TIME4 or SET_TO_SERVER_TIME4
	// value from attrData at the current offset.
	parseTimeSet := func() (*time.Time, uint32) {
		if attrOff+4 > len(attrData) {
			return nil, nfs4.NFS4ERR_BADXDR
		}
		how := binary.BigEndian.Uint32(attrData[attrOff : attrOff+4])
		attrOff += 4
		if how == nfs4.SET_TO_CLIENT_TIME4 {
			if attrOff+12 > len(attrData) {
				return nil, nfs4.NFS4ERR_BADXDR
			}
			sec := int64(binary.BigEndian.Uint64(attrData[attrOff : attrOff+8]))
			nsec := binary.BigEndian.Uint32(attrData[attrOff+8 : attrOff+12])
			attrOff += 12
			if nsec >= 1_000_000_000 {
				return nil, nfs4.NFS4ERR_INVAL
			}
			t := time.Unix(sec, int64(nsec))
			return &t, nfs4.NFS4_OK
		}
		if how == nfs4.SET_TO_SERVER_TIME4 {
			t := time.Now()
			return &t, nfs4.NFS4_OK
		}
		return nil, nfs4.NFS4ERR_INVAL
	}

	var setAtime, setMtime *time.Time
	if mask[1]&(1<<(nfs4.FATTR4_TIME_ACCESS_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != nfs4.NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		setAtime = t
	}
	if mask[1]&(1<<(nfs4.FATTR4_TIME_MODIFY_SET-32)) != 0 {
		t, status := parseTimeSet()
		if status != nfs4.NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		setMtime = t
	}

	if attrOff != len(attrData) {
		return setattrReply(nfs4.NFS4ERR_BADXDR, attrMask{})
	}

	var sf staging.File
	var state openstate.State
	dataID := st.currentID
	stateid, ok := st.resolveStateID(args.Stateid())
	if !ok {
		return setattrReply(nfs4.NFS4ERR_BAD_STATEID, attrMask{})
	}
	if !isSpecialStateID(stateid) {
		var status uint32
		state, status = s.lookupDurableOpen(st.clientID, stateid, st.currentID)
		switch {
		case status == nfs4.NFS4_OK:
			dataID = state.DataID
			sf = s.stagingOwnedBy(dataID, state)
		case newSize != nil || s.foreignStateID(st.clientID, extractStateID(stateid)):
			// Another client's stateid is rejected before any effect, times
			// included (README.md#stateid-checks).
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
		return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
	}
	if newSize != nil {
		if status := requireRegularFile(st.currentID); status != nfs4.NFS4_OK {
			return setattrReply(status, attrMask{})
		}
		if !isSpecialStateID(stateid) && !state.Writes() {
			return setattrReply(nfs4.NFS4ERR_OPENMODE, attrMask{})
		}
		if sf == nil {
			return setattrReply(nfs4.NFS4ERR_OPENMODE, attrMask{})
		}
		if err := s.setStagingSize(sf, *newSize); err != nil {
			st.effectUnknown = true
			return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
		}
		resultMask[0] |= 1 << nfs4.FATTR4_SIZE
	}
	if setAtime != nil || setMtime != nil {
		if sf != nil {
			if err := sf.SetTime(setMtime, setAtime); err != nil {
				st.effectUnknown = true
				return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
			}
			if err := sf.Sync(); err != nil {
				st.effectUnknown = true
				return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
			}
		} else if err := s.fs.SetTime(
			s.clientView(st.clientID, st.currentID), setMtime, setAtime,
		); err != nil {
			st.observeMutation(s.fs, nfs4.OP_SETATTR, err)
			return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
		}
		if setAtime != nil {
			resultMask[1] |= 1 << (nfs4.FATTR4_TIME_ACCESS_SET - 32)
		}
		if setMtime != nil {
			resultMask[1] |= 1 << (nfs4.FATTR4_TIME_MODIFY_SET - 32)
		}
	}

	if s.stagingStore.Failed(dataID) {
		return setattrReply(nfs4.NFS4ERR_IO, attrMask{})
	}

	return setattrReply(nfs4.NFS4_OK, resultMask)
}

func (s *Server) opVerify(args nfs4.VERIFY4args, st *compoundState, w *nfs4.COMPOUND4resWriter) uint32 {
	if !st.currentIDSet {
		r := w.AppendResarray_Verify()
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOFILEHANDLE)
	}
	same, status := s.verifyAttrs(st.currentID, s.clientView(st.clientID, st.currentID), args.ObjAttributes())
	if status != nfs4.NFS4_OK {
		r := w.AppendResarray_Verify()
		return finishStatusResponse(&r, status)
	}
	r := w.AppendResarray_Verify()
	if !same {
		// VERIFY: fail if attributes differ.
		return finishStatusResponse(&r, nfs4.NFS4ERR_NOT_SAME)
	}
	return finishStatusResponse(&r, nfs4.NFS4_OK)
}

// verifyAttrs compares the supplied fattr4 against the current file's attributes.
// Returns (same bool, status uint32). If status != NFS4_OK, comparison failed.
// verifyAttrs compares supplied against the attributes of handle id as
// seen through dataID.
func (s *Server) verifyAttrs(id, dataID vfs.InodeID, supplied nfs4.Fattr4) (bool, uint32) {
	ni, err := s.stat(dataID)
	if err != nil {
		return false, s.errToNFS(err)
	}

	mask := parseBitmap(supplied.Attrmask())
	if status := validateVerifyMask(mask); status != nfs4.NFS4_OK {
		return false, status
	}

	// Encode what we would return for these attributes.
	expected := encodeAttrs(mask, id, ni)

	// Get the supplied attribute values.
	suppliedData := supplied.AttrVals().Data()

	return bytes.Equal(expected, suppliedData), nfs4.NFS4_OK
}
