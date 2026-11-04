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

// A repeated OPEN by the owner of a closing open makes it ordinary again.
func TestOpenClearsClosing(t *testing.T) {
	store := NewStore()
	owner := OwnerKey{ClientID: 1, Owner: "owner"}
	file := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	state, err := store.Open(owner, file, file, testGrant(true), nfs4.StateID{})
	if err != nil {
		t.Fatal(err)
	}
	store.MarkClosing(state.ID)
	if !store.Closing(state.ID) {
		t.Fatal("mark not set")
	}
	if _, err := store.Open(owner, file, file, testGrant(true), nfs4.StateID{}); err != nil {
		t.Fatal(err)
	}
	if store.Closing(state.ID) {
		t.Fatal("a repeated OPEN left the open closing")
	}
}

// ReaderOf returns only a reader of the same client that is not closing.
func TestReaderOfSkipsOtherClientsAndClosing(t *testing.T) {
	store := NewStore()
	file := vfs.MakeInodeID(vfs.InodeTypeFile, 1)
	staging := vfs.MakeInodeID(vfs.InodeTypeFile, 2)
	open := func(client uint64, owner string) State {
		t.Helper()
		state, err := store.Open(OwnerKey{ClientID: client, Owner: owner}, file, staging, testGrant(false), nfs4.StateID{})
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	writer := open(1, "writer")
	open(2, "other-client")
	closing := open(1, "closing")
	store.MarkClosing(closing.ID)
	if r, ok := store.ReaderOf(staging, writer.ID, 1); ok {
		t.Fatalf("ReaderOf returned %+v", r)
	}
	reader := open(1, "reader")
	if r, ok := store.ReaderOf(staging, writer.ID, 1); !ok || r.ID != reader.ID {
		t.Fatalf("ReaderOf = %+v %t, want the reader", r, ok)
	}
}
