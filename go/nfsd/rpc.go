// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// RPC constants.
const (
	rpcCall  = 0
	rpcReply = 1

	rpcVersion = 2

	nfsProg    = 100003
	nfsVersion = 4

	// RPC procedures.
	procNull     = 0
	procCompound = 1

	// Auth flavors.
	authNone = 0
	authSys  = 1

	// Reply status.
	msgAccepted = 0
	msgDenied   = 1

	// Accept status.
	acceptSuccess      = 0
	acceptProgUnavail  = 1
	acceptProgMismatch = 2
	acceptProcUnavail  = 3
	acceptGarbageArgs  = 4
)

// readFrame reads one RPC record-marking frame from a TCP connection.
// Reassembles multi-fragment messages (bit 31 = last fragment flag).
func readFrame(r io.Reader) ([]byte, error) {
	var buf []byte
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, err
		}
		fragLen := binary.BigEndian.Uint32(hdr[:])
		last := fragLen&0x80000000 != 0
		fragLen &= 0x7FFFFFFF

		if fragLen > 1<<20 {
			return nil, fmt.Errorf("fragment too large: %d bytes", fragLen)
		}

		frag := make([]byte, fragLen)
		if _, err := io.ReadFull(r, frag); err != nil {
			return nil, err
		}
		buf = append(buf, frag...)
		if last {
			return buf, nil
		}
	}
}

// writeFrame writes an RPC record-marking frame (single fragment, last=true).
func writeFrame(w net.Conn, data []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(data))|0x80000000)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// rpcRequest holds the parsed ONC RPC call header.
type rpcRequest struct {
	xid        uint32
	prog       uint32
	vers       uint32
	proc       uint32
	credFlavor uint32
	credBody   []byte
	verfFlavor uint32
	verfBody   []byte
	body       []byte // remaining bytes after header
}

// parseRPCCall parses an ONC RPC CALL message.
func parseRPCCall(data []byte) (*rpcRequest, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("RPC message too short: %d bytes", len(data))
	}
	xid := binary.BigEndian.Uint32(data[0:4])
	msgType := binary.BigEndian.Uint32(data[4:8])
	if msgType != rpcCall {
		return nil, fmt.Errorf("expected RPC CALL (0), got %d", msgType)
	}
	rpcVers := binary.BigEndian.Uint32(data[8:12])
	if rpcVers != rpcVersion {
		return nil, fmt.Errorf("expected RPC version 2, got %d", rpcVers)
	}
	prog := binary.BigEndian.Uint32(data[12:16])
	vers := binary.BigEndian.Uint32(data[16:20])
	proc := binary.BigEndian.Uint32(data[20:24])

	off := 24

	// Parse credential.
	if off+8 > len(data) {
		return nil, fmt.Errorf("truncated credential")
	}
	credFlavor := binary.BigEndian.Uint32(data[off : off+4])
	credLen := binary.BigEndian.Uint32(data[off+4 : off+8])
	off += 8
	if off+int(credLen) > len(data) {
		return nil, fmt.Errorf("truncated credential body")
	}
	credBody := data[off : off+int(credLen)]
	off += int(credLen)

	// Parse verifier.
	if off+8 > len(data) {
		return nil, fmt.Errorf("truncated verifier")
	}
	verfFlavor := binary.BigEndian.Uint32(data[off : off+4])
	verfLen := binary.BigEndian.Uint32(data[off+4 : off+8])
	off += 8
	if off+int(verfLen) > len(data) {
		return nil, fmt.Errorf("truncated verifier body")
	}
	verfBody := data[off : off+int(verfLen)]
	off += int(verfLen)

	return &rpcRequest{
		xid:        xid,
		prog:       prog,
		vers:       vers,
		proc:       proc,
		credFlavor: credFlavor,
		credBody:   credBody,
		verfFlavor: verfFlavor,
		verfBody:   verfBody,
		body:       data[off:],
	}, nil
}

// buildRPCReply builds an RPC reply header for an accepted, successful call.
// Returns a buffer that the caller appends procedure-specific data to.
func buildRPCReply(xid uint32, acceptStat uint32) []byte {
	buf := make([]byte, 0, 128)
	buf = binary.BigEndian.AppendUint32(buf, xid)
	buf = binary.BigEndian.AppendUint32(buf, rpcReply)
	buf = binary.BigEndian.AppendUint32(buf, msgAccepted)
	// Verifier: AUTH_NONE, length 0.
	buf = binary.BigEndian.AppendUint32(buf, authNone)
	buf = binary.BigEndian.AppendUint32(buf, 0)
	buf = binary.BigEndian.AppendUint32(buf, acceptStat)
	return buf
}
