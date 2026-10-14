// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"encoding/hex"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Diagnostics snapshots of open state and views, in the inspect API's
// types. Entry points take only the store's own lock and return copies;
// the server composes them.

// The API's text for ids; see inspect.Scalar and inspect.ID.
func inodeText(id vfs.InodeID) string    { return inspect.Scalar(inspect.KindInode, uint64(id)) }
func incarnationText(id uint64) string   { return inspect.Scalar(inspect.KindIncarnation, id) }
func stateidText(id nfs4.StateID) string { return inspect.ID(inspect.KindStateid, id[:]) }

// snapshot copies the open as /v1/opens reports it. The caller holds os.mu.
func (st *State) snapshot() inspect.Open {
	out := inspect.Open{
		Stateid:     stateidText(st.ID),
		FileID:      inodeText(st.FileID),
		DataID:      inodeText(st.DataID),
		Incarnation: incarnationText(st.Owner.ClientID),
		OwnerHex:    hex.EncodeToString([]byte(st.Owner.Owner)),
		Access:      []string{},
		ShareModes:  st.modes,
		DenyModes:   st.denyModes,
		Generation:  st.Generation,
		Seq:         st.seq,
		Writer:      st.Writes(),
		Marked:      st.Marked,
	}
	if st.Access&nfs4.OPEN4_SHARE_ACCESS_READ != 0 {
		out.Access = append(out.Access, "read")
	}
	if st.Access&nfs4.OPEN4_SHARE_ACCESS_WRITE != 0 {
		out.Access = append(out.Access, "write")
	}
	return out
}

// FileOpens counts the live opens whose filehandle or data inode is inode.
func (os *Store) FileOpens(inode vfs.InodeID) int {
	os.mu.Lock()
	defer os.mu.Unlock()
	n := 0
	for _, st := range os.states {
		if st.FileID == inode || st.DataID == inode {
			n++
		}
	}
	return n
}

// StateidOwner classifies one stateid in a single observation, so a stateid
// moving from live to expired is never seen as both or neither.
func (os *Store) StateidOwner(id nfs4.StateID) (owner uint64, live, expired bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	if st := os.states[id]; st != nil {
		return st.Owner.ClientID, true, false
	}
	owner, expired = os.expired[id]
	return owner, false, expired
}

// OpensByIncarnation counts the live opens of each incarnation, or of id
// alone when it is not zero.
func (os *Store) OpensByIncarnation(id uint64) map[uint64]int {
	os.mu.Lock()
	defer os.mu.Unlock()
	counts := make(map[uint64]int)
	for _, st := range os.states {
		if id == 0 || st.Owner.ClientID == id {
			counts[st.Owner.ClientID]++
		}
	}
	return counts
}

// Snapshot copies the opens and expired tombstones matching f for /v1/opens.
// A File filter selects by filehandle or data inode and excludes the
// tombstones, which record neither.
func (os *Store) Snapshot(f inspect.OpenFilter) (records []inspect.Open, expired []inspect.ExpiredStateid) {
	os.mu.Lock()
	defer os.mu.Unlock()
	records, expired = []inspect.Open{}, []inspect.ExpiredStateid{}
	for id, st := range os.states {
		if f.Stateid != nil && id != nfs4.StateID(*f.Stateid) ||
			f.Incarnation != 0 && st.Owner.ClientID != f.Incarnation ||
			f.File != 0 && st.FileID != vfs.InodeID(f.File) && st.DataID != vfs.InodeID(f.File) {
			continue
		}
		records = append(records, st.snapshot())
	}
	for id, owner := range os.expired {
		if f.Stateid != nil && id != nfs4.StateID(*f.Stateid) || f.Incarnation != 0 && owner != f.Incarnation || f.File != 0 {
			continue
		}
		expired = append(expired, inspect.ExpiredStateid{Stateid: stateidText(id), Incarnation: incarnationText(owner)})
	}
	return records, expired
}

// Diagnostics: the aliases and writer records as /v1/opens reports them.
func (vs *ViewStore) Snapshot() inspect.Views {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	out := inspect.Views{Aliases: []inspect.Alias{}, WriterEntries: len(vs.writers)}
	for key, alias := range vs.aliases {
		out.Aliases = append(out.Aliases, inspect.Alias{
			Incarnation: incarnationText(key.ClientID),
			FileID:      inodeText(key.FileID),
			DataID:      inodeText(alias.dataID),
			Expires:     alias.expires,
		})
	}
	return out
}
