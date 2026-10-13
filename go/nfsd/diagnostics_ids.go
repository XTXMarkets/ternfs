// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
)

// The diagnostics API's text for the server's own ids. The eight-byte
// helpers return "" for zero, which everywhere it appears means "not
// observed": a missing base inode, an unowned staging file, a group with no
// wire record. A stateid, session or identity hash of all zero bytes is a
// value, so those helpers always format; a caller leaves such a field ""
// when it is absent.

func wireText(id uint64) string        { return scalarText(inspect.KindWireClientID, id) }
func incarnationText(id uint64) string { return scalarText(inspect.KindIncarnation, id) }
func inodeText(id InodeID) string      { return scalarText(inspect.KindInode, uint64(id)) }
func stateidText(id StateID) string    { return inspect.ID(inspect.KindStateid, id[:]) }
func sessionText(id SessionID) string  { return inspect.ID(inspect.KindSession, id[:]) }

func scalarText(kind string, id uint64) string {
	if id == 0 {
		return ""
	}
	return inspect.ID(kind, binary.BigEndian.AppendUint64(nil, id))
}

// identityHashText formats an identity directory name, which is the hash
// as 64 hex digits when the store is well formed, or "" when it is not.
func identityHashText(name string) string {
	raw, err := hex.DecodeString(name)
	if err != nil || len(raw) != 32 {
		return ""
	}
	return inspect.ID(inspect.KindIdentityHash, raw)
}
