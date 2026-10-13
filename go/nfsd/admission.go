// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import "encoding/binary"

func resultErrorSize(op uint32) uint32 {
	if op == OP_SETATTR {
		return 12
	} // attrsset exists even on error.
	return 8
}

func appendResultError(body []byte, op, status uint32) []byte {
	if !knownOperation(op) {
		op = OP_ILLEGAL
	}
	body = binary.BigEndian.AppendUint32(body, op)
	body = binary.BigEndian.AppendUint32(body, status)
	if op == OP_SETATTR {
		body = binary.BigEndian.AppendUint32(body, 0)
	}
	return body
}

func writeOperationError(w *COMPOUND4resWriter, op, status uint32) {
	w.AppendEncodedResult(func(buf []byte) []byte {
		return appendResultError(buf, op, status)
	})
}

func resultLimitStatus(seq *sequenceResult) uint32 {
	if seq.cache && seq.sess.maxCached <= seq.sess.maxResp {
		return NFS4ERR_REP_TOO_BIG_TO_CACHE
	}
	return NFS4ERR_REP_TOO_BIG
}

// Bounds include discriminant/status and XDR padding. Attribute encodings in
// the current profile have fixed lengths; values/IDs do not change those lengths.
// Read-only variable operations may compute a bounded result and then fail that
// operation, preserving earlier effects. Mutations have fixed pre-effect bounds.
func (s *Server) operationReplyBound(op NfsArgop4) uint32 {
	switch op.Disc() {
	case OP_ACCESS:
		return 16
	case OP_CLOSE, OP_OPEN_DOWNGRADE, OP_WRITE:
		return 24
	case OP_COMMIT:
		return 16
	case OP_CREATE:
		return 32
	case OP_OPEN:
		return 56
	case OP_GETFH:
		return 20 // Current native projection is an eight-byte handle.
	case OP_GETATTR:
		mask := parseBitmap(op.AsGETATTR4args().AttrRequest())
		mask[0] &= supportedAttrs0
		mask[1] &= supportedAttrs1
		mask[2] &= supportedAttrs2
		return uint32(28 + len(encodeAttrs(mask, 0, NodeInfo{})))
	case OP_READ:
		count := min(op.AsREAD4args().Count(), uint32(maxReadWrite))
		return 16 + (count+3)&^3
	case OP_READDIR:
		return 8 + readdirResokOverhead // Actual page uses remaining negotiated budget.
	case OP_READLINK:
		return 12 // Actual read-only result checked before later operations.
	case OP_REMOVE:
		return 28
	case OP_RENAME:
		return 48
	case OP_SETATTR:
		return 20
	case OP_SECINFO, OP_SECINFO_NO_NAME:
		return 20
	case OP_EXCHANGE_ID:
		// fixed client/seq/flags/protection + server owner/scope/empty impl array
		return uint32(8 + 8 + 4 + 4 + 4 + 8 + 4 + ((len(s.serverOwnerMajor()) + 3) &^ 3) + 4 + ((len(serverScope) + 3) &^ 3) + 4)
	case OP_CREATE_SESSION:
		return 88
	case OP_BIND_CONN_TO_SESSION:
		return 32
	case OP_TEST_STATEID:
		return 12 + 4*op.AsTESTSTATEID4args().TsStateidsCount()
	default:
		return resultErrorSize(op.Disc())
	}
}

func changesHandle(op uint32) bool {
	switch op {
	case OP_OPEN, OP_CREATE, OP_PUTFH, OP_PUTROOTFH, OP_PUTPUBFH,
		OP_LOOKUP, OP_LOOKUPP, OP_RESTOREFH:
		return true
	}
	return false
}

func operationMutates(op uint32) bool {
	switch op {
	case OP_CLOSE, OP_COMMIT, OP_CREATE, OP_OPEN, OP_OPEN_DOWNGRADE, OP_REMOVE,
		OP_RENAME, OP_SETATTR, OP_WRITE, OP_EXCHANGE_ID, OP_CREATE_SESSION,
		OP_DESTROY_SESSION, OP_DESTROY_CLIENTID, OP_BIND_CONN_TO_SESSION,
		OP_RECLAIM_COMPLETE, OP_FREE_STATEID:
		return true
	}
	return false
}

// OPEN/CREATE must have room to report a following handle before their effects.
// Intervening results are bounded up to the GETFH or another handle change.
func (s *Server) handleReplyObligation(ops []NfsArgop4, index int) (uint32, uint32) {
	if ops[index].Disc() != OP_OPEN && ops[index].Disc() != OP_CREATE {
		return 0, NFS4_OK
	}
	end := -1
	for i := index + 1; i < len(ops); i++ {
		if changesHandle(ops[i].Disc()) {
			break
		}
		if ops[i].Disc() == OP_GETFH {
			end = i
			break
		}
	}
	if end < 0 {
		return 0, NFS4_OK
	}
	var bytes uint32
	for _, op := range ops[index+1 : end+1] {
		if op.Disc() == OP_READLINK || op.Disc() == OP_READDIR {
			// These currently consume remaining budget or have only a minimum
			// result bound. Refuse before creation rather than lose its handle.
			return 0, NFS4ERR_UNSAFE_COMPOUND
		}
		bytes += s.operationReplyBound(op)
	}
	if end+1 < len(ops) {
		bytes += resultErrorSize(ops[end+1].Disc())
	}
	return bytes, NFS4_OK
}

func mandatoryUnsupported(op uint32) bool {
	switch op {
	case OP_SETCLIENTID, OP_SETCLIENTID_CONFIRM, OP_OPEN_CONFIRM,
		OP_RELEASE_LOCKOWNER, OP_RENEW:
		return true
	}
	return false
}

func maintenancePosition(op NfsArgop4, st *compoundState, index, count int) uint32 {
	switch op.Disc() {
	case OP_BIND_CONN_TO_SESSION:
		if count != 1 {
			return NFS4ERR_NOT_ONLY_OP
		}
	case OP_EXCHANGE_ID, OP_CREATE_SESSION, OP_DESTROY_CLIENTID:
		if st.session == nil && count != 1 {
			return NFS4ERR_NOT_ONLY_OP
		}
	case OP_DESTROY_SESSION:
		if st.session == nil && count != 1 {
			return NFS4ERR_NOT_ONLY_OP
		}
		id := readSessionID(op.AsDESTROYSESSION4args().DsaSessionid())
		if st.session != nil && id == st.session.id && index != count-1 {
			return NFS4ERR_NOT_ONLY_OP
		}
	}
	return NFS4_OK
}
