// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Test hooks: nfsd tests outside this package add and count opens.

// testGrant is the grant of a test's read or write OPEN.
func testGrant(write bool) Grant {
	if write {
		return Grant{Access: nfs4.OPEN4_SHARE_ACCESS_BOTH}
	}
	return Grant{Access: nfs4.OPEN4_SHARE_ACCESS_READ}
}

func AddOpenForTest(
	os *Store,
	owner OwnerKey,
	fileID vfs.InodeID,
	write bool,
	id nfs4.StateID,
) (State, uint32) {
	state, err := os.Open(owner, fileID, fileID, testGrant(write), id)
	if err != nil {
		return State{}, nfs4.NFS4ERR_SERVERFAULT
	}
	return state, nfs4.NFS4_OK
}

// StateCount returns the number of live open states.
func (os *Store) StateCount() int {
	os.mu.Lock()
	defer os.mu.Unlock()
	return len(os.states)
}

// FileLockRefs returns the holders and waiters of fileID's open lock.
func (os *Store) FileLockRefs(fileID vfs.InodeID) int {
	return os.fileLocks.Refs(fileID)
}
