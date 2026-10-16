// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The diagnostics API's text for the server's own ids. The eight-byte
// helpers return "" for zero (inspect.Scalar). A stateid, session or
// identity hash of all zero bytes is a value, so those helpers always
// format; a caller leaves such a field "" when it is absent.

func wireText(id uint64) string          { return inspect.Scalar(inspect.KindWireClientID, id) }
func incarnationText(id uint64) string   { return inspect.Scalar(inspect.KindIncarnation, id) }
func inodeText(id vfs.InodeID) string    { return inspect.Scalar(inspect.KindInode, uint64(id)) }
func stateidText(id nfs4.StateID) string { return inspect.ID(inspect.KindStateid, id[:]) }
func sessionText(id SessionID) string    { return inspect.ID(inspect.KindSession, id[:]) }
func identityHashText(name string) string {
	return inspect.IdentityHash(name)
}
