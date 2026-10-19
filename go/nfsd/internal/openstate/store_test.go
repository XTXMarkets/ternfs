// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func assertOpenStateIndex(t *testing.T, store *Store) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()

	for key, state := range store.byFile {
		if state.Owner != key.owner || state.FileID != key.fileID {
			t.Fatalf("file index key = %#v, state = %#v", key, state)
		}
		if store.states[state.ID] != state {
			t.Fatalf("file index state %x is absent from stateid index", state.ID)
		}
	}
	if len(store.byFile) != len(store.states) {
		t.Fatalf("file index has %d states, stateid index has %d",
			len(store.byFile), len(store.states))
	}
}

func TestOpenRejectsOccupiedStateID(t *testing.T) {
	store := NewStore()
	owner := OwnerKey{ClientID: 1, Owner: "owner"}
	first, err := store.Open(owner, vfs.MakeInodeID(vfs.InodeTypeFile, 1), vfs.MakeInodeID(vfs.InodeTypeFile, 1), testGrant(false), nfs4.StateID{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(
		owner, vfs.MakeInodeID(vfs.InodeTypeFile, 2), vfs.MakeInodeID(vfs.InodeTypeFile, 2), testGrant(false), first.ID, false,
	); err == nil {
		t.Fatal("occupied stateid was reused for another file")
	}
	assertOpenStateIndex(t, store)
}

// A write OPEN by an owner that holds a read open upgrades the same
// stateid to the new writer's staging (RFC 8881 section 18.16.3).
func TestWriteReopenUpgrades(t *testing.T) {
	store := NewStore()
	owner := OwnerKey{ClientID: 1, Owner: "owner"}
	fileID := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	stagingID := vfs.MakeInodeID(vfs.InodeTypeFile, 2)
	state := addConfirmedOpen(t, store, owner, fileID)
	upgraded, err := store.Open(owner, fileID, stagingID, testGrant(true), nfs4.StateID{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.ID != state.ID || !upgraded.Writes() || upgraded.DataID != stagingID ||
		upgraded.Generation != state.Generation+1 {
		t.Fatalf("upgrade = %+v from %+v", upgraded, state)
	}
	assertOpenStateIndex(t, store)
}

func TestStateidGenerationWrapsToOne(t *testing.T) {
	for _, test := range []struct {
		generation uint32
		want       uint32
	}{
		{0, 1},
		{1, 2},
		{^uint32(0) - 1, ^uint32(0)},
		{^uint32(0), 1},
	} {
		if got := NextGeneration(test.generation); got != test.want {
			t.Errorf("nextStateidGeneration(%d) = %d, want %d",
				test.generation, got, test.want)
		}
	}
}

func addConfirmedOpen(
	t *testing.T,
	store *Store,
	owner OwnerKey,
	fileID vfs.InodeID,
) State {
	t.Helper()
	state, status := AddOpenForTest(store, owner, fileID, false, nfs4.StateID{})
	if status != nfs4.NFS4_OK {
		t.Fatalf("OPEN status = %s", nfs4.Nfsstat4Name(status))
	}
	return state
}

func TestReopenAdvancesGeneration(t *testing.T) {
	store := NewStore()
	defer assertOpenStateIndex(t, store)
	owner := OwnerKey{ClientID: 1, Owner: "owner"}
	fileID := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	first := addConfirmedOpen(t, store, owner, fileID)
	second := addConfirmedOpen(t, store, owner, fileID)
	if second.ID != first.ID || second.Generation != first.Generation+1 {
		t.Fatalf("reopen = %+v after %+v", second, first)
	}
	if _, status := store.Lookup(owner.ClientID, first.ID, first.Generation, fileID); status != nfs4.NFS4ERR_OLD_STATEID {
		t.Fatalf("old generation = %s", nfs4.Nfsstat4Name(status))
	}
	if _, status := store.Lookup(owner.ClientID, first.ID, 0, fileID); status != nfs4.NFS4_OK {
		t.Fatalf("current-generation seqid 0 = %s", nfs4.Nfsstat4Name(status))
	}
}

func TestOpenStateStoreLookupClassifiesStateids(t *testing.T) {
	store := NewStore()
	defer assertOpenStateIndex(t, store)
	owner := OwnerKey{ClientID: 7, Owner: "owner"}
	fileID := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	state := addConfirmedOpen(t, store, owner, fileID)
	for _, test := range []struct {
		name       string
		id         nfs4.StateID
		generation uint32
		fileID     vfs.InodeID
		want       uint32
	}{
		{"current", state.ID, 1, fileID, nfs4.NFS4_OK},
		{"future generation", state.ID, 2, fileID, nfs4.NFS4ERR_BAD_STATEID},
		{"wrong file", state.ID, 1, vfs.MakeInodeID(vfs.InodeTypeFile, 2), nfs4.NFS4ERR_BAD_STATEID},
		{"unknown", nfs4.StateID{}, 0, fileID, nfs4.NFS4ERR_BAD_STATEID},
	} {
		if _, status := store.Lookup(owner.ClientID, test.id, test.generation, test.fileID); status != test.want {
			t.Errorf("%s: status = %s, want %s", test.name,
				nfs4.Nfsstat4Name(status), nfs4.Nfsstat4Name(test.want))
		}
	}
	closed, ok := store.Close(state.ID)
	if !ok || closed.Generation != 2 {
		t.Fatalf("close = %+v, %v", closed, ok)
	}
	if _, status := store.Lookup(owner.ClientID, state.ID, 1, fileID); status != nfs4.NFS4ERR_BAD_STATEID {
		t.Fatalf("closed stateid = %s", nfs4.Nfsstat4Name(status))
	}
	other := addConfirmedOpen(t, store, owner, fileID)
	store.Expire(other.ID)
	if _, status := store.Lookup(owner.ClientID, other.ID, 1, fileID); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("expired stateid = %s", nfs4.Nfsstat4Name(status))
	}
}

func TestOpenStateStoreKeepsOtherOwnersUntilClientPurge(t *testing.T) {
	store := NewStore()
	defer assertOpenStateIndex(t, store)
	clientID := uint64(1)
	var states []State
	for i, test := range []struct {
		owner string
		write bool
	}{
		{"reader", false},
		{"writer", true},
	} {
		fileID := vfs.MakeInodeID(vfs.InodeTypeFile, uint64(i+1))
		state, status := AddOpenForTest(store,
			OwnerKey{ClientID: clientID, Owner: test.owner},
			fileID, test.write, nfs4.StateID{})
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		states = append(states, state)
	}

	if _, ok := store.Close(states[0].ID); !ok {
		t.Fatal("first owner CLOSE found no state")
	}
	if len(store.states) != 1 {
		t.Fatalf("states after CLOSE = %d, want 1", len(store.states))
	}
	if _, status := store.Lookup(
		clientID, states[1].ID, 1, states[1].FileID,
	); status != nfs4.NFS4_OK {
		t.Fatalf("activity by one owner invalidated another: %s",
			nfs4.Nfsstat4Name(status))
	}

	expired := store.ExpireClient(clientID)
	if len(expired) != 1 || expired[0].ID != states[1].ID {
		t.Fatalf("expired states = %v, want stateid %x",
			expired, states[1].ID)
	}
	if len(store.states) != 0 || len(store.byFile) != 0 {
		t.Fatalf("reboot retained states=%d files=%d",
			len(store.states), len(store.byFile))
	}
	if _, status := store.Lookup(
		clientID, states[1].ID, 1, states[1].FileID,
	); status != nfs4.NFS4ERR_EXPIRED {
		t.Fatalf("reboot-purged state = %s", nfs4.Nfsstat4Name(status))
	}
}

func TestOpenStateStoreBoundsExpiredTombstones(t *testing.T) {
	store := NewStore()
	defer assertOpenStateIndex(t, store)
	fileID := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	var firstStateID nfs4.StateID
	for i := 0; i <= maxTombstones; i++ {
		clientID := uint64(i + 1)
		state, status := AddOpenForTest(store,
			OwnerKey{ClientID: clientID, Owner: "owner"},
			fileID, false, nfs4.StateID{})
		if status != nfs4.NFS4_OK {
			t.Fatal(nfs4.Nfsstat4Name(status))
		}
		if i == 0 {
			firstStateID = state.ID
		}
		store.ExpireClient(clientID)
	}

	if len(store.expired) != maxTombstones {
		t.Fatalf("expired stateids = %d, want %d",
			len(store.expired), maxTombstones)
	}
	if _, found := store.expired[firstStateID]; found {
		t.Fatal("oldest expired stateid was not evicted")
	}
}
