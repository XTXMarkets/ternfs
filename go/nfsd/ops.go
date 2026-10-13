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

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Stable write modes.
const (
	unstable4 = 0
	dataSync4 = 1
	fileSync4 = 2

	maxTernBytesLength = 255
	maxTernNameLength  = maxTernBytesLength
)

func finishDefaultResponse(w *nfs4.COMPOUND4resWriter, buf []byte, status uint32) uint32 {
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
		return "", nfs4.NFS4ERR_INVAL
	case len(data) > maxTernNameLength:
		return "", nfs4.NFS4ERR_NAMETOOLONG
	case bytes.Equal(data, []byte(".")),
		bytes.Equal(data, []byte("..")),
		bytes.IndexByte(data, '/') >= 0,
		bytes.IndexByte(data, 0) >= 0:
		return "", nfs4.NFS4ERR_BADNAME
	default:
		return string(data), nfs4.NFS4_OK
	}
}

func requireDirectory(id vfs.InodeID) uint32 {
	switch id.Type() {
	case vfs.InodeTypeDir:
		return nfs4.NFS4_OK
	case vfs.InodeTypeSymlink:
		return nfs4.NFS4ERR_SYMLINK
	default:
		return nfs4.NFS4ERR_NOTDIR
	}
}

func requireRegularFile(id vfs.InodeID) uint32 {
	switch id.Type() {
	case vfs.InodeTypeFile:
		return nfs4.NFS4_OK
	case vfs.InodeTypeDir:
		return nfs4.NFS4ERR_ISDIR
	default:
		return nfs4.NFS4ERR_INVAL
	}
}

// extractStateID reads the 12-byte "other" field from a Stateid4 into a StateID.
func extractStateID(s nfs4.Stateid4) StateID {
	var sid StateID
	for i := 0; i < 12; i++ {
		sid[i] = s.Other(i)
	}
	return sid
}

// writeStateID writes the generation and opaque ID into a Stateid4.
func writeStateID(s nfs4.Stateid4, generation uint32, sid StateID) {
	s.SetSeqid(generation)
	for i := 0; i < 12; i++ {
		s.SetOther(i, sid[i])
	}
}

func getData64(v nfs4.Verifier4) (data [8]byte) {
	for i := range data {
		data[i] = v.Data(i)
	}
	return data
}

func setData64(v nfs4.Verifier4, data [8]byte) {
	for i := range data {
		v.SetData(i, data[i])
	}
}

func setChangeInfo(info nfs4.ChangeInfo4, atomic uint32, before, after uint64) {
	info.SetAtomic(atomic)
	info.SetBefore(before)
	info.SetAfter(after)
}

// errToNFS converts a Go error to an NFS status code.
func (s *Server) errToNFS(err error) uint32 {
	if err == nil {
		return nfs4.NFS4_OK
	}
	err = vfs.TernToOSError(err)
	switch {
	case errors.Is(err, syscall.ENOTDIR):
		return nfs4.NFS4ERR_NOTDIR
	case errors.Is(err, syscall.EISDIR):
		return nfs4.NFS4ERR_ISDIR
	case errors.Is(err, syscall.ENOTEMPTY):
		return nfs4.NFS4ERR_NOTEMPTY
	}
	if e, ok := err.(nfs4.Error); ok {
		return uint32(e)
	}
	if os.IsNotExist(err) {
		return nfs4.NFS4ERR_NOENT
	}
	if os.IsPermission(err) {
		return nfs4.NFS4ERR_ACCESS
	}
	if os.IsExist(err) {
		return nfs4.NFS4ERR_EXIST
	}
	s.log.Warn("VFS error", "err", err)
	return nfs4.NFS4ERR_IO
}

// Time helper.
func timeToNFS(t time.Time) (int64, uint32) {
	return t.Unix(), uint32(t.Nanosecond())
}
