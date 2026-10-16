// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package nfs4

import (
	"encoding/binary"
)

// Raw access the generated code in nfs.go does not provide.

// Bytes returns the encoded arguments, which identify a retransmission.
func (m SEQUENCE4args) Bytes() []byte {
	return m.m[:]
}

// Len returns the length of the reply encoded so far.
func (w *COMPOUND4resWriter) Len() int {
	return len(w.buf)
}

// AppendEncodedResult appends one resarray entry that encode writes, for
// results the typed writers cannot express.
func (w *COMPOUND4resWriter) AppendEncodedResult(encode func([]byte) []byte) {
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.phase = 1
	w.resarrayCount++
	w.buf = encode(w.buf)
}
