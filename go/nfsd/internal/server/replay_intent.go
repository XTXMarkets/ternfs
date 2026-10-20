// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// An intent is written to the session file before a non-idempotent effect
// on TernFS: RENAME, REMOVE and CREATE of a directory or symlink. If the
// process dies between the effect committing in the shard and the reply
// record being written, the next process finds the intent on the slot and
// can decide from TernFS whether the effect happened. The intent carries
// the identity of what was there before, since inode IDs are never reused
// and an edge is identified by its inode and creation time.
//
// Encoding (big-endian): [1] kind, [2] the index in the compound of the
// operation it protects, then the fields below. Names are [2] length +
// bytes. The session file's format version covers this encoding.

const (
	intentRename uint8 = 1
	intentRemove uint8 = 2
	intentCreate uint8 = 3
)

var errBadIntent = errors.New("malformed intent")

// renameIntent describes a RENAME about to run.
type renameIntent struct {
	srcDir  vfs.InodeID
	srcName string
	src     vfs.Edge // the edge at srcName
	dstDir  vfs.InodeID
	dstName string
	dst     vfs.InodeID // what dstName held, or 0
}

// removeIntent describes a REMOVE about to run.
type removeIntent struct {
	dir  vfs.InodeID
	name string
	edge vfs.Edge
}

// createIntent describes a CREATE about to run.
type createIntent struct {
	dir     vfs.InodeID
	name    string
	objType uint32 // NF4DIR or NF4LNK
	target  string // symlink target
}

// intentHeader starts an intent of kind for the operation at index op.
func intentHeader(kind uint8, op int) []byte {
	return binary.BigEndian.AppendUint16([]byte{kind}, uint16(op))
}

func (in renameIntent) encode(op int) []byte {
	b := intentHeader(intentRename, op)
	b = binary.BigEndian.AppendUint64(b, uint64(in.srcDir))
	b = appendName(b, in.srcName)
	b = binary.BigEndian.AppendUint64(b, uint64(in.src.ID))
	b = binary.BigEndian.AppendUint64(b, in.src.CreationTime)
	b = binary.BigEndian.AppendUint64(b, uint64(in.dstDir))
	b = appendName(b, in.dstName)
	return binary.BigEndian.AppendUint64(b, uint64(in.dst))
}

func (in removeIntent) encode(op int) []byte {
	b := intentHeader(intentRemove, op)
	b = binary.BigEndian.AppendUint64(b, uint64(in.dir))
	b = appendName(b, in.name)
	b = binary.BigEndian.AppendUint64(b, uint64(in.edge.ID))
	return binary.BigEndian.AppendUint64(b, in.edge.CreationTime)
}

func (in createIntent) encode(op int) []byte {
	b := intentHeader(intentCreate, op)
	b = binary.BigEndian.AppendUint64(b, uint64(in.dir))
	b = appendName(b, in.name)
	b = binary.BigEndian.AppendUint32(b, in.objType)
	return appendName(b, in.target)
}

// decodeIntent returns the index of the operation an intent protects and
// one of renameIntent, removeIntent or createIntent.
func decodeIntent(b []byte) (int, any, error) {
	if len(b) < 3 {
		return 0, nil, errBadIntent
	}
	op := int(binary.BigEndian.Uint16(b[1:3]))
	in, err := decodeIntentFields(b[0], intentReader{b: b[3:]})
	return op, in, err
}

func decodeIntentFields(kind uint8, r intentReader) (any, error) {
	switch kind {
	case intentRename:
		in := renameIntent{}
		in.srcDir = vfs.InodeID(r.u64())
		in.srcName = r.name()
		in.src.ID = vfs.InodeID(r.u64())
		in.src.CreationTime = r.u64()
		in.dstDir = vfs.InodeID(r.u64())
		in.dstName = r.name()
		in.dst = vfs.InodeID(r.u64())
		return in, r.done()
	case intentRemove:
		in := removeIntent{}
		in.dir = vfs.InodeID(r.u64())
		in.name = r.name()
		in.edge.ID = vfs.InodeID(r.u64())
		in.edge.CreationTime = r.u64()
		return in, r.done()
	case intentCreate:
		in := createIntent{}
		in.dir = vfs.InodeID(r.u64())
		in.name = r.name()
		in.objType = r.u32()
		in.target = r.name()
		if in.objType != nfs4.NF4DIR && in.objType != nfs4.NF4LNK {
			return nil, fmt.Errorf("%w: create type %d", errBadIntent, in.objType)
		}
		return in, r.done()
	}
	return nil, fmt.Errorf("%w: kind %d", errBadIntent, kind)
}

// intentOperation is the operation an intent protects.
func intentOperation(in any) uint32 {
	switch in.(type) {
	case renameIntent:
		return nfs4.OP_RENAME
	case removeIntent:
		return nfs4.OP_REMOVE
	}
	return nfs4.OP_CREATE
}

func appendName(b []byte, name string) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(len(name)))
	return append(b, name...)
}

// intentReader decodes fields and remembers the first failure.
type intentReader struct {
	b   []byte
	bad bool
}

func (r *intentReader) take(n int) []byte {
	if r.bad || len(r.b) < n {
		r.bad = true
		return make([]byte, n)
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}

func (r *intentReader) u64() uint64 { return binary.BigEndian.Uint64(r.take(8)) }
func (r *intentReader) u32() uint32 { return binary.BigEndian.Uint32(r.take(4)) }
func (r *intentReader) name() string {
	n := int(binary.BigEndian.Uint16(r.take(2)))
	return string(r.take(n))
}

func (r *intentReader) done() error {
	if r.bad || len(r.b) != 0 {
		return errBadIntent
	}
	return nil
}
