// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

type OwnerKey struct {
	ClientID uint64
	Owner    string
}

// State is one owner's open of one file. FileID is the filehandle the
// client holds: the published inode at the name when it opened. DataID is
// what its I/O uses: the writer's staging inode, or for a read open the
// published inode current at OPEN.
type State struct {
	ID     nfs4.StateID
	FileID vfs.InodeID
	DataID vfs.InodeID
	Owner  OwnerKey
	// Access is the READ/WRITE share access granted. modes holds bit v for
	// each distinct share_access value v the open was granted, and
	// denyModes bit d for each share_deny value d requested. Deny is kept
	// for OPEN_DOWNGRADE's validation only; it is not enforced
	// (README.md#downgrades).
	Access     uint32
	modes      uint8
	denyModes  uint8
	Generation uint32
	// Marked reports that the open has a durable marker in the client
	// store. An open is marked once it is granted write access, creates
	// its file or recovers a writer's staging, and stays marked; other read
	// opens are recorded in this process only.
	Marked bool
	// Recovered reports an open adopted from an earlier process's staging.
	// Its original grants are unknown, so a downgrade gets only the subset
	// rule (DowngradeStatus).
	Recovered bool
	// learnGeneration: the open was adopted from a request whose stateid
	// carried seqid 0 ("the current one"), so its generation is unknown.
	// The first request to present a seqid sets it.
	learnGeneration bool
	// seq orders opens of one file for choosing the most recent.
	seq uint64
}

// Writes reports write access: the open has a writer.
func (s State) Writes() bool { return s.Access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 }

// Grant is one OPEN or OPEN_DOWNGRADE's share access and deny.
type Grant struct {
	Access, Deny uint32
}

// DowngradeStatus checks an OPEN_DOWNGRADE of state to g (RFC 8881 section
// 18.18.3). The target must be a subset of the current access and deny,
// and equal the union of the recorded values inside it; a recovered open
// has no recorded values and gets only the subset rule.
func (s State) DowngradeStatus(g Grant, recovered bool) uint32 {
	union := func(set uint8, within uint32) uint32 {
		var u uint32
		for v := uint32(0); v <= 3; v++ {
			if set&(1<<v) != 0 && v&^within == 0 {
				u |= v
			}
		}
		return u
	}
	if g.Access&^s.Access != 0 || g.Deny&^union(s.denyModes, 3) != 0 {
		return nfs4.NFS4ERR_INVAL
	}
	if recovered {
		return nfs4.NFS4_OK
	}
	if union(s.modes, g.Access) != g.Access || union(s.denyModes, g.Deny) != g.Deny {
		return nfs4.NFS4ERR_INVAL
	}
	return nfs4.NFS4_OK
}

// narrowLocked applies an accepted downgrade to g: the recorded values
// outside it are dropped. The caller holds os.mu.
func (s *State) narrowLocked(g Grant) {
	s.Access = g.Access
	for v := uint32(0); v <= 3; v++ {
		if v&^g.Access != 0 {
			s.modes &^= 1 << v
		}
		if v&^g.Deny != 0 {
			s.denyModes &^= 1 << v
		}
	}
	s.modes |= 1 << g.Access
	s.denyModes |= 1 << g.Deny
}
