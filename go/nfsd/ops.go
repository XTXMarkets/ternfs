// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"time"
)

// Stable write modes.
const (
	unstable4 = 0
	dataSync4 = 1
	fileSync4 = 2

	maxTernBytesLength = 255
	maxTernNameLength  = maxTernBytesLength
)

func finishDefaultResponse(w *COMPOUND4resWriter, buf []byte, status uint32) uint32 {
	w.Resume(binary.BigEndian.AppendUint32(buf, status))
	return status
}

type statusResponseWriter interface {
	SetStatus(uint32)
}

func finishStatusResponse(r statusResponseWriter, status uint32) uint32 {
	r.SetStatus(status)
	return status
}

func toTernFSName(data []byte) (string, uint32) {
	switch {
	case len(data) == 0:
		return "", NFS4ERR_INVAL
	case len(data) > maxTernNameLength:
		return "", NFS4ERR_NAMETOOLONG
	case bytes.Equal(data, []byte(".")),
		bytes.Equal(data, []byte("..")),
		bytes.IndexByte(data, '/') >= 0,
		bytes.IndexByte(data, 0) >= 0:
		return "", NFS4ERR_BADNAME
	default:
		return string(data), NFS4_OK
	}
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

// extractStateID reads the 12-byte "other" field from a Stateid4 into a StateID.
func extractStateID(s Stateid4) StateID {
	var sid StateID
	for i := 0; i < 12; i++ {
		sid[i] = s.Other(i)
	}
	return sid
}

// writeStateID writes the generation and opaque ID into a Stateid4.
func writeStateID(s Stateid4, generation uint32, sid StateID) {
	s.SetSeqid(generation)
	for i := 0; i < 12; i++ {
		s.SetOther(i, sid[i])
	}
}

func getData64(v Verifier4) (data [8]byte) {
	for i := range data {
		data[i] = v.Data(i)
	}
	return data
}

func setData64(v Verifier4, data [8]byte) {
	for i := range data {
		v.SetData(i, data[i])
	}
}

func setChangeInfo(info ChangeInfo4, atomic uint32, before, after uint64) {
	info.SetAtomic(atomic)
	info.SetBefore(before)
	info.SetAfter(after)
}

// errToNFS converts a Go error to an NFS status code.
func (s *Server) errToNFS(err error) uint32 {
	if err == nil {
		return NFS4_OK
	}
	err = ternToOSError(err)
	switch {
	case errors.Is(err, syscall.ENOTDIR):
		return NFS4ERR_NOTDIR
	case errors.Is(err, syscall.EISDIR):
		return NFS4ERR_ISDIR
	case errors.Is(err, syscall.ENOTEMPTY):
		return NFS4ERR_NOTEMPTY
	}
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
