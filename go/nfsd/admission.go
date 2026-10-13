// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func resultErrorSize(op uint32) uint32 {
	if op == nfs4.OP_SETATTR {
		return 12
	} // attrsset exists even on error.
	return 8
}

func appendResultError(body []byte, op, status uint32) []byte {
	if !knownOperation(op) {
		op = nfs4.OP_ILLEGAL
	}
	body = binary.BigEndian.AppendUint32(body, op)
	body = binary.BigEndian.AppendUint32(body, status)
	if op == nfs4.OP_SETATTR {
		body = binary.BigEndian.AppendUint32(body, 0)
	}
	return body
}

func writeOperationError(w *nfs4.COMPOUND4resWriter, op, status uint32) {
	w.AppendEncodedResult(func(buf []byte) []byte {
		return appendResultError(buf, op, status)
	})
}

func resultLimitStatus(seq *sequenceResult) uint32 {
	if seq.cache && seq.sess.maxCached <= seq.sess.maxResp {
		return nfs4.NFS4ERR_REP_TOO_BIG_TO_CACHE
	}
	return nfs4.NFS4ERR_REP_TOO_BIG
}

// Bounds include discriminant/status and XDR padding. Attribute encodings in
// the current profile have fixed lengths; values/IDs do not change those lengths.
// Read-only variable operations may compute a bounded result and then fail that
// operation, preserving earlier effects. Mutations have fixed pre-effect bounds.
func (s *Server) operationReplyBound(op nfs4.NfsArgop4) uint32 {
	switch op.Disc() {
	case nfs4.OP_ACCESS:
		return 16
	case nfs4.OP_CLOSE, nfs4.OP_OPEN_DOWNGRADE, nfs4.OP_WRITE:
		return 24
	case nfs4.OP_COMMIT:
		return 16
	case nfs4.OP_CREATE:
		return 32
	case nfs4.OP_OPEN:
		return 56
	case nfs4.OP_GETFH:
		return 20 // Current native projection is an eight-byte handle.
	case nfs4.OP_GETATTR:
		mask := parseBitmap(op.AsGETATTR4args().AttrRequest())
		mask[0] &= supportedAttrs0
		mask[1] &= supportedAttrs1
		mask[2] &= supportedAttrs2
		return uint32(28 + len(encodeAttrs(mask, 0, vfs.NodeInfo{})))
	case nfs4.OP_READ:
		count := min(op.AsREAD4args().Count(), uint32(maxReadWrite))
		return 16 + (count+3)&^3
	case nfs4.OP_READDIR:
		return 8 + readdirResokOverhead // Actual page uses remaining negotiated budget.
	case nfs4.OP_READLINK:
		return 12 // Actual read-only result checked before later operations.
	case nfs4.OP_REMOVE:
		return 28
	case nfs4.OP_RENAME:
		return 48
	case nfs4.OP_SETATTR:
		return 20
	case nfs4.OP_SECINFO, nfs4.OP_SECINFO_NO_NAME:
		return 20
	case nfs4.OP_EXCHANGE_ID:
		// fixed client/seq/flags/protection + server owner/scope/empty impl array
		return uint32(8 + 8 + 4 + 4 + 4 + 8 + 4 + ((len(s.serverOwnerMajor()) + 3) &^ 3) + 4 + ((len(serverScope) + 3) &^ 3) + 4)
	case nfs4.OP_CREATE_SESSION:
		return 88
	case nfs4.OP_BIND_CONN_TO_SESSION:
		return 32
	case nfs4.OP_TEST_STATEID:
		return 12 + 4*op.AsTESTSTATEID4args().TsStateidsCount()
	default:
		return resultErrorSize(op.Disc())
	}
}

func changesHandle(op uint32) bool {
	switch op {
	case nfs4.OP_OPEN, nfs4.OP_CREATE, nfs4.OP_PUTFH, nfs4.OP_PUTROOTFH, nfs4.OP_PUTPUBFH,
		nfs4.OP_LOOKUP, nfs4.OP_LOOKUPP, nfs4.OP_RESTOREFH:
		return true
	}
	return false
}

func operationMutates(op uint32) bool {
	switch op {
	case nfs4.OP_CLOSE, nfs4.OP_COMMIT, nfs4.OP_CREATE, nfs4.OP_OPEN, nfs4.OP_OPEN_DOWNGRADE, nfs4.OP_REMOVE,
		nfs4.OP_RENAME, nfs4.OP_SETATTR, nfs4.OP_WRITE, nfs4.OP_EXCHANGE_ID, nfs4.OP_CREATE_SESSION,
		nfs4.OP_DESTROY_SESSION, nfs4.OP_DESTROY_CLIENTID, nfs4.OP_BIND_CONN_TO_SESSION,
		nfs4.OP_RECLAIM_COMPLETE, nfs4.OP_FREE_STATEID:
		return true
	}
	return false
}

// OPEN/CREATE must have room to report a following handle before their effects.
// Intervening results are bounded up to the GETFH or another handle change.
func (s *Server) handleReplyObligation(ops []nfs4.NfsArgop4, index int) (uint32, uint32) {
	if ops[index].Disc() != nfs4.OP_OPEN && ops[index].Disc() != nfs4.OP_CREATE {
		return 0, nfs4.NFS4_OK
	}
	end := -1
	for i := index + 1; i < len(ops); i++ {
		if changesHandle(ops[i].Disc()) {
			break
		}
		if ops[i].Disc() == nfs4.OP_GETFH {
			end = i
			break
		}
	}
	if end < 0 {
		return 0, nfs4.NFS4_OK
	}
	var bytes uint32
	for _, op := range ops[index+1 : end+1] {
		if op.Disc() == nfs4.OP_READLINK || op.Disc() == nfs4.OP_READDIR {
			// These currently consume remaining budget or have only a minimum
			// result bound. Refuse before creation rather than lose its handle.
			return 0, nfs4.NFS4ERR_UNSAFE_COMPOUND
		}
		bytes += s.operationReplyBound(op)
	}
	if end+1 < len(ops) {
		bytes += resultErrorSize(ops[end+1].Disc())
	}
	return bytes, nfs4.NFS4_OK
}

func mandatoryUnsupported(op uint32) bool {
	switch op {
	case nfs4.OP_SETCLIENTID, nfs4.OP_SETCLIENTID_CONFIRM, nfs4.OP_OPEN_CONFIRM,
		nfs4.OP_RELEASE_LOCKOWNER, nfs4.OP_RENEW:
		return true
	}
	return false
}

func maintenancePosition(op nfs4.NfsArgop4, st *compoundState, index, count int) uint32 {
	switch op.Disc() {
	case nfs4.OP_BIND_CONN_TO_SESSION:
		if count != 1 {
			return nfs4.NFS4ERR_NOT_ONLY_OP
		}
	case nfs4.OP_EXCHANGE_ID, nfs4.OP_CREATE_SESSION, nfs4.OP_DESTROY_CLIENTID:
		if st.session == nil && count != 1 {
			return nfs4.NFS4ERR_NOT_ONLY_OP
		}
	case nfs4.OP_DESTROY_SESSION:
		if st.session == nil && count != 1 {
			return nfs4.NFS4ERR_NOT_ONLY_OP
		}
		id := readSessionID(op.AsDESTROYSESSION4args().DsaSessionid())
		if st.session != nil && id == st.session.id && index != count-1 {
			return nfs4.NFS4ERR_NOT_ONLY_OP
		}
	}
	return nfs4.NFS4_OK
}
